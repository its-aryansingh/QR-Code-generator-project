package shortcode

import (
	"crypto/rand"
	"errors"
	"math/big"
	"regexp"
	"strings"
)

// Alphabet is Crockford's Base32 alphabet without I, L, O, U to avoid confusion and accidental profanity.
const Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

const CodeLength = 7

var (
	alphabetLen = big.NewInt(int64(len(Alphabet)))
	codeRegex   = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{7}$`)

	ErrInvalidLength = errors.New("short code must be exactly 7 characters")
	ErrInvalidFormat = errors.New("short code contains invalid characters")
	ErrDenylisted    = errors.New("short code contains reserved or forbidden terms")
)

// Normalise cleans and standardises a short code:
// - strips whitespace
// - strips a single trailing slash
// - converts to uppercase
// - replaces confusing characters: O -> 0, I -> 1, L -> 1
func Normalise(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "/")
	s = strings.ToUpper(s)
	s = strings.ReplaceAll(s, "O", "0")
	s = strings.ReplaceAll(s, "I", "1")
	s = strings.ReplaceAll(s, "L", "1")
	return s
}

// Validate checks if a normalised short code meets all format, length, and denylist rules.
func Validate(code string) error {
	norm := Normalise(code)
	if len(norm) != CodeLength {
		return ErrInvalidLength
	}
	if !codeRegex.MatchString(norm) {
		return ErrInvalidFormat
	}
	if ContainsDenylistedWord(norm) {
		return ErrDenylisted
	}
	return nil
}

// Generate creates a cryptographically random 7-character Crockford Base32 short code,
// regenerating if it collides with the denylist.
func Generate() (string, error) {
	for attempts := 0; attempts < 100; attempts++ {
		b := make([]byte, CodeLength)
		for i := 0; i < CodeLength; i++ {
			idx, err := rand.Int(rand.Reader, alphabetLen)
			if err != nil {
				return "", err
			}
			b[i] = Alphabet[idx.Int64()]
		}
		candidate := string(b)
		if !ContainsDenylistedWord(candidate) {
			return candidate, nil
		}
	}
	return "", errors.New("failed to generate safe short code after max attempts")
}

// Payload returns the canonical uppercase redirect payload URL for dynamic QR codes:
// HTTPS://HOSTNAME/CODE
func Payload(hostname, code string) string {
	return strings.ToUpper("https://" + strings.TrimRight(hostname, "/") + "/" + Normalise(code))
}
