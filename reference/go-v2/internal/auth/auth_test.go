package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/its-aryansingh/qrit/services/internal/auth"
)

func TestTokenManager(t *testing.T) {
	tm, err := auth.NewTokenManager("test-key", nil)
	if err != nil {
		t.Fatalf("NewTokenManager failed: %v", err)
	}

	userID := uuid.New()
	sessionID := uuid.New()

	tokenStr, err := tm.CreateAccessToken(userID, sessionID)
	if err != nil {
		t.Fatalf("CreateAccessToken failed: %v", err)
	}

	claims, err := tm.VerifyAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("VerifyAccessToken failed: %v", err)
	}

	if claims.Subject != userID.String() {
		t.Errorf("Subject = %q; want %q", claims.Subject, userID.String())
	}
	if claims.SessionID != sessionID.String() {
		t.Errorf("SessionID = %q; want %q", claims.SessionID, sessionID.String())
	}
}

func TestPasswordStrength(t *testing.T) {
	if err := auth.ValidatePasswordStrength("short"); err != auth.ErrPasswordTooShort {
		t.Errorf("expected ErrPasswordTooShort, got %v", err)
	}

	if err := auth.ValidatePasswordStrength("password123"); err != auth.ErrPasswordBreached {
		t.Errorf("expected ErrPasswordBreached, got %v", err)
	}

	if err := auth.ValidatePasswordStrength("superSecurePassword!2026"); err != nil {
		t.Errorf("unexpected error on strong password: %v", err)
	}
}

func TestGenerateRandomToken(t *testing.T) {
	plain, hash, err := auth.GenerateRandomToken(32)
	if err != nil {
		t.Fatalf("GenerateRandomToken failed: %v", err)
	}
	if len(plain) != 64 { // 32 bytes hex encoded = 64 chars
		t.Errorf("plain length = %d; want 64", len(plain))
	}
	if len(hash) != 32 { // sha256 output = 32 bytes
		t.Errorf("hash length = %d; want 32", len(hash))
	}

	expectedHash := auth.HashToken(plain)
	if string(hash) != string(expectedHash) {
		t.Errorf("hash mismatch")
	}
}

func TestAuthAndCSRFMiddleware(t *testing.T) {
	tm, err := auth.NewTokenManager("test-key", nil)
	if err != nil {
		t.Fatalf("NewTokenManager failed: %v", err)
	}

	userID := uuid.New()
	sessionID := uuid.New()
	token, _ := tm.CreateAccessToken(userID, sessionID)

	protectedHandler := auth.AuthenticateMiddleware(tm)(
		auth.RequireAuth(
			auth.CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})),
		),
	)

	// Case 1: Bearer token auth on POST (no CSRF needed for Bearer tokens)
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	protectedHandler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("bearer POST expected 200, got %d", w.Code)
	}

	// Case 2: Cookie auth on GET (no CSRF needed for GET)
	req = httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: token})
	w = httptest.NewRecorder()
	protectedHandler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("cookie GET expected 200, got %d", w.Code)
	}

	// Case 3: Cookie auth on POST without CSRF header (should fail 403)
	req = httptest.NewRequest("POST", "/test", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: token})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "csrf123"})
	w = httptest.NewRecorder()
	protectedHandler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("cookie POST without CSRF header expected 403, got %d", w.Code)
	}

	// Case 4: Cookie auth on POST with matching CSRF header (should succeed 200)
	req = httptest.NewRequest("POST", "/test", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: token})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "csrf123"})
	req.Header.Set(auth.CSRFHeaderName, "csrf123")
	w = httptest.NewRecorder()
	protectedHandler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("cookie POST with valid CSRF expected 200, got %d", w.Code)
	}

	// Case 5: No auth at all (should fail 401)
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	protectedHandler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated expected 401, got %d", w.Code)
	}
}
