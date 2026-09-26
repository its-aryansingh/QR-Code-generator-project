package config

import (
	"crypto/hkdf"
	"crypto/sha256"
)

// deriveKey derives a 32-byte subkey from the master key with HKDF-SHA256 and a label,
// so independent secrets (visitor salts, serial MACs, verify tokens) never share bytes.
func deriveKey(master []byte, label string) []byte {
	k, err := hkdf.Key(sha256.New, master, nil, "qrit:"+label, 32)
	if err != nil {
		panic(err) // only possible for invalid lengths, which are constants here
	}
	return k
}
