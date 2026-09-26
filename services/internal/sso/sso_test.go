package sso

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSSRFGuard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	if _, err := NewHTTPClient(false).Get(srv.URL); err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("loopback must be refused: %v", err)
	}
	res, err := NewHTTPClient(true).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
}

func TestPKCEAndTokens(t *testing.T) {
	v, c := PKCE()
	sum := sha256.Sum256([]byte(v))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != c || len(v) < 43 {
		t.Fatal("PKCE S256")
	}
	if RandomToken(24) == RandomToken(24) {
		t.Fatal("random tokens repeat")
	}
}

func TestDoHJoinsSplitStrings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") != "TXT" || r.Header.Get("Accept") != "application/dns-json" {
			w.WriteHeader(400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Status": 0, "Answer": []any{
			map[string]any{"type": 16, "data": `"qrit-domain-verification=" "abc123"`},
			map[string]any{"type": 5, "data": "cname.example."},
			map[string]any{"type": 16, "data": `"v=spf1 -all"`}}})
	}))
	defer srv.Close()
	txt, err := DoH{Endpoint: srv.URL, Client: srv.Client()}.LookupTXT(context.Background(), "_qrit-challenge.acme.com")
	if err != nil || len(txt) != 2 || txt[0] != "qrit-domain-verification=abc123" {
		t.Fatalf("txt: %v %v", txt, err)
	}
}

func TestVerifyIDToken(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": srv.URL, "authorization_endpoint": srv.URL + "/a",
				"token_endpoint": srv.URL + "/t", "jwks_uri": srv.URL + "/k"})
		case "/k":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "k1", "use": "sig",
				"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
		}
	}))
	defer srv.Close()
	o := NewOIDC(NewHTTPClient(true))
	ctx := context.Background()
	d, err := o.Discover(ctx, srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.Discover(ctx, "http://127.0.0.1:1"); err == nil {
		t.Fatal("unreachable issuer")
	}
	sign := func(k *rsa.PrivateKey, m jwt.MapClaims, mutate ...func(*jwt.Token)) string {
		base := jwt.MapClaims{"iss": srv.URL, "aud": "client", "sub": "u1", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(),
			"nonce": "n1", "email": "A@Acme.com", "email_verified": "true", "roles": []any{"x", "y"}}
		for k, v := range m {
			if v == nil {
				delete(base, k)
			} else {
				base[k] = v
			}
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, base)
		tok.Header["kid"] = "k1"
		for _, f := range mutate {
			f(tok)
		}
		s, err := tok.SignedString(k)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	c, err := o.VerifyIDToken(ctx, d, "client", sign(key, nil), "n1")
	if err != nil || c.Email != "a@acme.com" || !c.EmailVerified || c.Subject != "u1" {
		t.Fatalf("valid token: %+v %v", c, err)
	}
	if g := ClaimsWithGroups(c, "roles").Groups; len(g) != 2 {
		t.Fatalf("custom groups claim: %v", g)
	}
	bad := map[string]string{
		"wrong nonce":    sign(key, nil),
		"wrong audience": sign(key, jwt.MapClaims{"aud": "someone-else"}),
		"wrong issuer":   sign(key, jwt.MapClaims{"iss": "https://evil.example"}),
		"expired":        sign(key, jwt.MapClaims{"exp": time.Now().Add(-10 * time.Minute).Unix()}),
		"no expiry":      sign(key, jwt.MapClaims{"exp": nil}),
		"foreign key":    sign(other, nil),
		"unknown kid":    sign(key, nil, func(t *jwt.Token) { t.Header["kid"] = "k9" }),
		"multi aud":      sign(key, jwt.MapClaims{"aud": []any{"client", "other"}}),
	}
	for name, tok := range bad {
		nonce := "n1"
		if name == "wrong nonce" {
			nonce = "n2"
		}
		if _, err := o.VerifyIDToken(ctx, d, "client", tok, nonce); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// alg=none and HMAC-with-public-key confusion are refused.
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"iss": srv.URL, "aud": "client", "exp": time.Now().Add(time.Minute).Unix(),
		"nonce": "n1"}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := o.VerifyIDToken(ctx, d, "client", none, "n1"); err == nil {
		t.Error("alg none accepted")
	}
	hs, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": srv.URL, "aud": "client", "exp": time.Now().Add(time.Minute).Unix(),
		"nonce": "n1"}).SignedString(key.N.Bytes())
	if _, err := o.VerifyIDToken(ctx, d, "client", hs, "n1"); err == nil {
		t.Error("HS256 accepted")
	}
	// Multiple audiences are fine when azp names us.
	if _, err := o.VerifyIDToken(ctx, d, "client", sign(key, jwt.MapClaims{"aud": []any{"client", "other"}, "azp": "client"}), "n1"); err != nil {
		t.Errorf("azp: %v", err)
	}
}
