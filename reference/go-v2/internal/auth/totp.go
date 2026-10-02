package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	TOTPPeriod = 30
	TOTPDigits = 6
)

// GenerateTOTPSecret creates a new cryptographically random Base32 TOTP secret.
func GenerateTOTPSecret() (string, error) {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes), nil
}

// GenerateTOTPCode generates the current 6-digit code for a given secret at time t.
func GenerateTOTPCode(secret string, t time.Time) (string, error) {
	cleanSecret := strings.ToUpper(strings.TrimSpace(secret))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleanSecret)
	if err != nil {
		return "", fmt.Errorf("invalid base32 secret: %w", err)
	}

	counter := uint64(t.Unix() / TOTPPeriod)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	h := mac.Sum(nil)

	offset := h[len(h)-1] & 0x0f
	codeInt := (int(h[offset])&0x7f)<<24 |
		(int(h[offset+1])&0xff)<<16 |
		(int(h[offset+2])&0xff)<<8 |
		(int(h[offset+3]) & 0xff)

	codeInt = codeInt % 1000000
	return fmt.Sprintf("%06d", codeInt), nil
}

// VerifyTOTPCode verifies a 6-digit TOTP code with ±1 step tolerance (90 second window).
func VerifyTOTPCode(secret, code string, now time.Time) bool {
	if len(code) != TOTPDigits {
		return false
	}

	for _, offset := range []int64{-1, 0, 1} {
		t := now.Add(time.Duration(offset*TOTPPeriod) * time.Second)
		expected, err := GenerateTOTPCode(secret, t)
		if err != nil {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(code), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}

// GenerateKeyURI returns the otpauth:// URI for rendering as a QR code in authenticator apps.
func GenerateKeyURI(issuer, accountName, secret string) string {
	label := fmt.Sprintf("%s:%s", issuer, accountName)
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")

	u := url.URL{
		Scheme:   "otpauth",
		Host:     "totp",
		Path:     "/" + label,
		RawQuery: v.Encode(),
	}
	return u.String()
}

// GenerateRecoveryCodes generates 10 single-use recovery backup codes (plain and SHA-256 hashed).
func GenerateRecoveryCodes(count int) (plain []string, hashed []string, err error) {
	if count <= 0 {
		count = 10
	}
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // readable, unambiguous chars

	for i := 0; i < count; i++ {
		bytes := make([]byte, 8)
		if _, err := rand.Read(bytes); err != nil {
			return nil, nil, err
		}
		var sb strings.Builder
		for _, b := range bytes {
			sb.WriteByte(charset[int(b)%len(charset)])
		}
		code := fmt.Sprintf("%s-%s", sb.String()[:4], sb.String()[4:])
		h := sha256.Sum256([]byte(code))

		plain = append(plain, code)
		hashed = append(hashed, hex.EncodeToString(h[:]))
	}

	return plain, hashed, nil
}

// VerifyRecoveryCode checks a plain recovery code against a stored SHA-256 hex hash.
func VerifyRecoveryCode(storedHash, plainCode string) bool {
	clean := strings.ToUpper(strings.TrimSpace(plainCode))
	h := sha256.Sum256([]byte(clean))
	computedHash := hex.EncodeToString(h[:])
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(computedHash)) == 1
}
