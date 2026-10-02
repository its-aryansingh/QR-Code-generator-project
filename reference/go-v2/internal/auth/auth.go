package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/platform/crypto"
)

const (
	AccessTokenDuration  = 10 * time.Minute
	RefreshTokenDuration = 30 * 24 * time.Hour
	MagicTokenDuration   = 15 * time.Minute
	VerifyTokenDuration  = 24 * time.Hour
	ResetTokenDuration   = 1 * time.Hour

	AccessCookieName  = "qrit_access"
	RefreshCookieName = "qrit_refresh"
	CSRFCookieName    = "qrit_csrf"
	CSRFHeaderName    = "X-CSRF-Token"
)

var (
	ErrInvalidToken     = errors.New("invalid or expired token")
	ErrPasswordTooShort = errors.New("password must be at least 10 characters long")
	ErrPasswordBreached = errors.New("password is too common or easily guessed")
	ErrCSRFMismatch     = errors.New("csrf token mismatch")
)

var commonPasswords = map[string]struct{}{
	"1234567890": {}, "password123": {}, "qwertyuiop": {}, "12345678901": {},
	"administrator": {}, "changeme123": {}, "welcome1234": {}, "iloveyou123": {},
}

// Claims defines JWT payload structure.
type Claims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
	// StaffGrant is set on tokens minted for a staff support session (customer-granted).
	StaffGrant string `json:"sg,omitempty"`
}

// Principal represents the authenticated caller: a user session, an API key, or a staff
// member acting under a customer-granted support session.
type Principal struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	IsCookie  bool

	// API key principals (UserID is the key creator, or uuid.Nil)
	APIKeyID       uuid.UUID
	KeyWorkspaceID uuid.UUID
	Scopes         []string
	Environment    string // live | test

	// Staff support sessions
	StaffGrantID uuid.UUID
}

// IsAPIKey reports whether the principal authenticated with an API key.
func (p *Principal) IsAPIKey() bool { return p != nil && p.APIKeyID != uuid.Nil }

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, p)
}

// GetPrincipal returns the authenticated caller. A nil principal (cleared by a guard
// middleware after revocation) reports ok=false.
func GetPrincipal(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalContextKey{}).(*Principal)
	return p, ok && p != nil
}

// TokenManager handles Ed25519 JWT access tokens.
type TokenManager struct {
	keyID      string
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// NewTokenManager initializes a TokenManager. If privKey is empty, an in-memory keypair is generated.
func NewTokenManager(keyID string, privKey ed25519.PrivateKey) (*TokenManager, error) {
	if len(privKey) == 0 {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate ed25519 key: %w", err)
		}
		return &TokenManager{
			keyID:      keyID,
			privateKey: priv,
			publicKey:  pub,
		}, nil
	}

	pubKey := privKey.Public().(ed25519.PublicKey)
	return &TokenManager{
		keyID:      keyID,
		privateKey: privKey,
		publicKey:  pubKey,
	}, nil
}

// CreateAccessToken signs a 10-minute Ed25519 JWT for the given user and session.
func (tm *TokenManager) CreateAccessToken(userID, sessionID uuid.UUID) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenDuration)),
			Issuer:    "qrit",
		},
		SessionID: sessionID.String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = tm.keyID
	return token.SignedString(tm.privateKey)
}

// CreateStaffAccessToken signs a short-lived token for a staff member acting under a
// customer's support-access grant. It never outlives ttl (≤ the access token duration).
func (tm *TokenManager) CreateStaffAccessToken(userID, sessionID, grantID uuid.UUID, ttl time.Duration) (string, error) {
	if ttl <= 0 || ttl > AccessTokenDuration {
		ttl = AccessTokenDuration
	}
	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "qrit",
		},
		SessionID:  sessionID.String(),
		StaffGrant: grantID.String(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = tm.keyID
	return token.SignedString(tm.privateKey)
}

// VerifyAccessToken parses and validates an Ed25519 JWT.
func (tm *TokenManager) VerifyAccessToken(tokenStr string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
			return nil, fmt.Errorf("unexpected signing algorithm: %v", t.Header["alg"])
		}
		return tm.publicKey, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// HashToken returns SHA-256 hash of a plaintext token string.
func HashToken(plain string) []byte {
	h := sha256.Sum256([]byte(plain))
	return h[:]
}

// GenerateRandomToken generates n cryptographically secure random bytes as hex string and its SHA-256 hash.
func GenerateRandomToken(n int) (plain string, hash []byte, err error) {
	b, err := crypto.RandomBytes(n)
	if err != nil {
		return "", nil, err
	}
	plain = hex.EncodeToString(b)
	hash = HashToken(plain)
	return plain, hash, nil
}

// ValidatePasswordStrength checks length and common password denylist.
func ValidatePasswordStrength(password string) error {
	if len(password) < 10 {
		return ErrPasswordTooShort
	}
	lower := strings.ToLower(strings.TrimSpace(password))
	if _, bad := commonPasswords[lower]; bad {
		return ErrPasswordBreached
	}
	return nil
}

// SetAuthCookies sets access, refresh, and CSRF cookies on the response.
func SetAuthCookies(w http.ResponseWriter, accessToken, refreshToken, csrfToken string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    accessToken,
		Path:     "/",
		MaxAge:   int(AccessTokenDuration.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	if refreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     RefreshCookieName,
			Value:    refreshToken,
			Path:     "/",
			MaxAge:   int(RefreshTokenDuration.Seconds()),
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteStrictMode,
		})
	}

	if csrfToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     CSRFCookieName,
			Value:    csrfToken,
			Path:     "/",
			MaxAge:   int(RefreshTokenDuration.Seconds()),
			HttpOnly: false, // readable by client JavaScript to supply X-CSRF-Token header
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

// ClearAuthCookies clears all authentication cookies.
func ClearAuthCookies(w http.ResponseWriter, secure bool) {
	for _, name := range []string{AccessCookieName, RefreshCookieName, CSRFCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: name != CSRFCookieName,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

// ExtractAccessToken extracts the JWT from the Authorization header or the access cookie.
func ExtractAccessToken(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer "), false
	}
	if cookie, err := r.Cookie(AccessCookieName); err == nil && cookie.Value != "" {
		return cookie.Value, true
	}
	return "", false
}

// AuthenticateMiddleware extracts and validates access token, setting Principal in context.
func AuthenticateMiddleware(tm *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr, isCookie := ExtractAccessToken(r)
			if tokenStr == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := tm.VerifyAccessToken(tokenStr)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			sessionID, err := uuid.Parse(claims.SessionID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			p := &Principal{
				UserID:    userID,
				SessionID: sessionID,
				IsCookie:  isCookie,
			}
			if claims.StaffGrant != "" {
				g, err := uuid.Parse(claims.StaffGrant)
				if err != nil || isCookie {
					next.ServeHTTP(w, r) // staff tokens are bearer-only
					return
				}
				p.StaffGrantID = g
			}
			ctx := WithPrincipal(r.Context(), p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth enforces that a valid Principal exists in the request context.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := GetPrincipal(r.Context()); !ok {
			apierr.Render(w, apierr.Unauthorized("authentication required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CSRFMiddleware validates double-submit CSRF cookie against X-CSRF-Token header on state-modifying requests.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := GetPrincipal(r.Context())
		// CSRF applies only to cookie-authenticated non-safe requests
		if !ok || !p.IsCookie {
			next.ServeHTTP(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(CSRFCookieName)
		if err != nil || cookie.Value == "" {
			apierr.Render(w, apierr.Forbidden("missing csrf cookie"))
			return
		}

		headerToken := r.Header.Get(CSRFHeaderName)
		if headerToken == "" || headerToken != cookie.Value {
			apierr.Render(w, apierr.Forbidden("csrf token mismatch"))
			return
		}

		next.ServeHTTP(w, r)
	})
}
