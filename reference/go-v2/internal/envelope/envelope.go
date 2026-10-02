// Package envelope implements per-organisation envelope encryption.
//
// Each organisation has data-encryption keys (DEKs) in org_data_keys, wrapped with the
// platform master key (APP_ENCRYPTION_KEY; a KMS can replace it later). Ciphertexts are
// keyID(4 bytes) || nonce || AES-256-GCM(dek, plaintext, aad = org id), so a ciphertext
// cannot be replayed into another organisation, and rotated keys keep decrypting.
package envelope

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrCiphertext = errors.New("envelope: malformed or foreign ciphertext")

type Keyring struct {
	pool   *pgxpool.Pool
	master []byte
	mu     sync.RWMutex
	deks   map[string][]byte // org:keyID -> dek
	active map[uuid.UUID]uint32
}

func NewKeyring(pool *pgxpool.Pool, master []byte) (*Keyring, error) {
	if len(master) != 32 {
		return nil, errors.New("envelope: master key must be 32 bytes")
	}
	return &Keyring{pool: pool, master: master, deks: map[string][]byte{}, active: map[uuid.UUID]uint32{}}, nil
}

func seal(key, plaintext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, aad), nil
}

func open(key, ct, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ct) < gcm.NonceSize() {
		return nil, ErrCiphertext
	}
	pt, err := gcm.Open(nil, ct[:gcm.NonceSize()], ct[gcm.NonceSize():], aad)
	if err != nil {
		return nil, ErrCiphertext
	}
	return pt, nil
}

func dekKey(org uuid.UUID, id uint32) string { return fmt.Sprintf("%s:%d", org, id) }

// activeKey returns (and lazily creates) the org's current DEK.
func (k *Keyring) activeKey(ctx context.Context, org uuid.UUID) (uint32, []byte, error) {
	k.mu.RLock()
	id, ok := k.active[org]
	dek := k.deks[dekKey(org, id)]
	k.mu.RUnlock()
	if ok && dek != nil {
		return id, dek, nil
	}
	var kid int32
	var wrapped []byte
	err := k.pool.QueryRow(ctx, `SELECT key_id, dek_ct FROM org_data_keys WHERE org_id = $1 AND retired_at IS NULL
		ORDER BY key_id DESC LIMIT 1`, org).Scan(&kid, &wrapped)
	if errors.Is(err, pgx.ErrNoRows) {
		kid, wrapped, err = k.create(ctx, org)
	}
	if err != nil {
		return 0, nil, err
	}
	dek, err = open(k.master, wrapped, org[:])
	if err != nil {
		return 0, nil, fmt.Errorf("unwrap data key: %w", err)
	}
	k.mu.Lock()
	k.active[org] = uint32(kid)
	k.deks[dekKey(org, uint32(kid))] = dek
	k.mu.Unlock()
	return uint32(kid), dek, nil
}

func (k *Keyring) create(ctx context.Context, org uuid.UUID) (int32, []byte, error) {
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('org_dek:' || $1::text, 0))`, org); err != nil {
		return 0, nil, err
	}
	var kid int32
	var wrapped []byte
	err = tx.QueryRow(ctx, `SELECT key_id, dek_ct FROM org_data_keys WHERE org_id = $1 AND retired_at IS NULL
		ORDER BY key_id DESC LIMIT 1`, org).Scan(&kid, &wrapped)
	if err == nil {
		return kid, wrapped, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, err
	}
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return 0, nil, err
	}
	if wrapped, err = seal(k.master, dek, org[:]); err != nil {
		return 0, nil, err
	}
	if err := tx.QueryRow(ctx, `INSERT INTO org_data_keys (org_id, key_id, dek_ct)
		VALUES ($1, COALESCE((SELECT max(key_id) FROM org_data_keys WHERE org_id = $1), 0) + 1, $2) RETURNING key_id`,
		org, wrapped).Scan(&kid); err != nil {
		return 0, nil, err
	}
	return kid, wrapped, tx.Commit(ctx)
}

func (k *Keyring) keyByID(ctx context.Context, org uuid.UUID, id uint32) ([]byte, error) {
	k.mu.RLock()
	dek := k.deks[dekKey(org, id)]
	k.mu.RUnlock()
	if dek != nil {
		return dek, nil
	}
	var wrapped []byte
	if err := k.pool.QueryRow(ctx, `SELECT dek_ct FROM org_data_keys WHERE org_id = $1 AND key_id = $2`, org, int32(id)).Scan(&wrapped); err != nil {
		return nil, ErrCiphertext
	}
	dek, err := open(k.master, wrapped, org[:])
	if err != nil {
		return nil, err
	}
	k.mu.Lock()
	k.deks[dekKey(org, id)] = dek
	k.mu.Unlock()
	return dek, nil
}

// Encrypt seals plaintext for an organisation.
func (k *Keyring) Encrypt(ctx context.Context, org uuid.UUID, plaintext []byte) ([]byte, error) {
	id, dek, err := k.activeKey(ctx, org)
	if err != nil {
		return nil, err
	}
	body, err := seal(dek, plaintext, org[:])
	if err != nil {
		return nil, err
	}
	out := make([]byte, 4, 4+len(body))
	binary.BigEndian.PutUint32(out, id)
	return append(out, body...), nil
}

// Decrypt opens a ciphertext produced by Encrypt for the same organisation.
func (k *Keyring) Decrypt(ctx context.Context, org uuid.UUID, ct []byte) ([]byte, error) {
	if len(ct) < 4 {
		return nil, ErrCiphertext
	}
	dek, err := k.keyByID(ctx, org, binary.BigEndian.Uint32(ct[:4]))
	if err != nil {
		return nil, err
	}
	return open(dek, ct[4:], org[:])
}

// KeyID returns the key id a ciphertext was sealed with.
func KeyID(ct []byte) (uint32, bool) {
	if len(ct) < 4 {
		return 0, false
	}
	return binary.BigEndian.Uint32(ct[:4]), true
}

// Rotate retires the active key; new encryptions use a fresh DEK, old data still decrypts.
func (k *Keyring) Rotate(ctx context.Context, org uuid.UUID) error {
	if _, err := k.pool.Exec(ctx, `UPDATE org_data_keys SET retired_at = now() WHERE org_id = $1 AND retired_at IS NULL`, org); err != nil {
		return err
	}
	k.mu.Lock()
	delete(k.active, org)
	k.mu.Unlock()
	_, _, err := k.activeKey(ctx, org)
	return err
}

// BlindIndex is a keyed, org-specific hash for exact-match lookups of encrypted values
// (e.g. finding a data subject's leads by email) without storing the value.
func (k *Keyring) BlindIndex(org uuid.UUID, value string) []byte {
	key, _ := hkdf.Key(sha256.New, k.master, org[:], "qrit-bidx-v1", 32)
	m := hmac.New(sha256.New, key)
	m.Write([]byte(strings.ToLower(strings.TrimSpace(value))))
	return m.Sum(nil)
}

// SealPlatform encrypts platform-level secrets that belong to no organisation.
func (k *Keyring) SealPlatform(plaintext []byte, label string) ([]byte, error) {
	return seal(k.master, plaintext, []byte("platform:"+label))
}

// OpenPlatform decrypts SealPlatform output.
func (k *Keyring) OpenPlatform(ct []byte, label string) ([]byte, error) {
	return open(k.master, ct, []byte("platform:"+label))
}
