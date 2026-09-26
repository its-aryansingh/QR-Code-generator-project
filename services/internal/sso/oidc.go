// Package sso implements the relying-party side of enterprise single sign-on:
// OpenID Connect directly against the IdP (discovery, PKCE, nonce, ID-token verification
// with JWKS), and SAML through an Ory Polis (BoxyHQ Jackson) bridge, which exposes SAML
// IdPs as an OAuth 2.0 provider.
package sso

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the normalised identity asserted by an IdP.
type Claims struct {
	Subject       string         `json:"sub"`
	Email         string         `json:"email"`
	EmailVerified bool           `json:"email_verified"`
	GivenName     string         `json:"given_name"`
	FamilyName    string         `json:"family_name"`
	Name          string         `json:"name"`
	Groups        []string       `json:"groups"`
	Raw           map[string]any `json:"raw"`
}

// NewHTTPClient returns a client that refuses private, loopback and link-local targets
// (SSRF protection for admin-configured IdP URLs) unless allowPrivate is set.
func NewHTTPClient(allowPrivate bool) *http.Client {
	d := &net.Dialer{Timeout: 5 * time.Second}
	if !allowPrivate {
		d.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
				ip.IsUnspecified() || ip.IsMulticast() || ip.Equal(net.ParseIP("169.254.169.254")) {
				return fmt.Errorf("sso: refusing to connect to %s", host)
			}
			if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 64 { // CGNAT 100.64/10
				return fmt.Errorf("sso: refusing to connect to %s", host)
			}
			return nil
		}
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{DialContext: d.DialContext,
		TLSHandshakeTimeout: 5 * time.Second, MaxIdleConns: 20, IdleConnTimeout: 60 * time.Second},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// PKCE returns a verifier and its S256 challenge.
func PKCE() (verifier, challenge string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

// RandomToken returns n random bytes, base64url.
func RandomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Discovery is the subset of the OpenID provider metadata we use.
type Discovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type jwks struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		N   string `json:"n"`
		E   string `json:"e"`
		Crv string `json:"crv"`
		X   string `json:"x"`
		Y   string `json:"y"`
	} `json:"keys"`
}

// OIDC is a provider client with discovery and key caches.
type OIDC struct {
	http *http.Client
	mu   sync.Mutex
	disc map[string]cachedDiscovery
	keys map[string]cachedKeys
}

type cachedDiscovery struct {
	d   Discovery
	exp time.Time
}

type cachedKeys struct {
	keys map[string]any
	exp  time.Time
}

func NewOIDC(client *http.Client) *OIDC {
	return &OIDC{http: client, disc: map[string]cachedDiscovery{}, keys: map[string]cachedKeys{}}
}

func (o *OIDC) getJSON(ctx context.Context, u string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	res, err := o.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", u, res.Status)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(dst)
}

// Discover loads (and caches for 1 h) the provider metadata; the issuer must match exactly.
func (o *OIDC) Discover(ctx context.Context, issuer string) (Discovery, error) {
	issuer = strings.TrimRight(issuer, "/")
	o.mu.Lock()
	c, ok := o.disc[issuer]
	o.mu.Unlock()
	if ok && time.Now().Before(c.exp) {
		return c.d, nil
	}
	var d Discovery
	if err := o.getJSON(ctx, issuer+"/.well-known/openid-configuration", &d); err != nil {
		return d, fmt.Errorf("oidc discovery: %w", err)
	}
	if strings.TrimRight(d.Issuer, "/") != issuer {
		return d, fmt.Errorf("oidc discovery: issuer mismatch (%q)", d.Issuer)
	}
	if d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" || d.JWKSURI == "" {
		return d, errors.New("oidc discovery: incomplete provider metadata")
	}
	o.mu.Lock()
	o.disc[issuer] = cachedDiscovery{d: d, exp: time.Now().Add(time.Hour)}
	o.mu.Unlock()
	return d, nil
}

// AuthURL builds the authorization request (code flow, PKCE S256, nonce).
func AuthURL(endpoint, clientID, redirectURI, scopes, state, nonce, challenge string, extra url.Values) string {
	v := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirectURI},
		"scope": {scopes}, "state": {state}, "nonce": {nonce}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}}
	for k, vs := range extra {
		v[k] = vs
	}
	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}
	return endpoint + sep + v.Encode()
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// Exchange redeems an authorization code.
func Exchange(ctx context.Context, client *http.Client, tokenEndpoint string, form url.Values) (tokenResponse, error) {
	var tr tokenResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return tr, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return tr, err
	}
	defer res.Body.Close()
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&tr); err != nil {
		return tr, fmt.Errorf("token response: %w", err)
	}
	if res.StatusCode != http.StatusOK || tr.Error != "" {
		return tr, fmt.Errorf("token endpoint: %s %s", tr.Error, tr.ErrorDesc)
	}
	return tr, nil
}

// ExchangeCode redeems a code at an OIDC provider.
func (o *OIDC) ExchangeCode(ctx context.Context, d Discovery, clientID, clientSecret, code, verifier, redirectURI string) (tokenResponse, error) {
	return Exchange(ctx, o.http, d.TokenEndpoint, url.Values{"grant_type": {"authorization_code"}, "code": {code},
		"redirect_uri": {redirectURI}, "client_id": {clientID}, "client_secret": {clientSecret}, "code_verifier": {verifier}})
}

func (o *OIDC) keyset(ctx context.Context, jwksURI string, force bool) (map[string]any, error) {
	o.mu.Lock()
	c, ok := o.keys[jwksURI]
	o.mu.Unlock()
	if ok && !force && time.Now().Before(c.exp) {
		return c.keys, nil
	}
	var set jwks
	if err := o.getJSON(ctx, jwksURI, &set); err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	keys := map[string]any{}
	for _, k := range set.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		switch k.Kty {
		case "RSA":
			n, err1 := base64.RawURLEncoding.DecodeString(k.N)
			e, err2 := base64.RawURLEncoding.DecodeString(k.E)
			if err1 != nil || err2 != nil {
				continue
			}
			keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
		case "EC":
			var curve elliptic.Curve
			switch k.Crv {
			case "P-256":
				curve = elliptic.P256()
			case "P-384":
				curve = elliptic.P384()
			default:
				continue
			}
			x, err1 := base64.RawURLEncoding.DecodeString(k.X)
			y, err2 := base64.RawURLEncoding.DecodeString(k.Y)
			if err1 != nil || err2 != nil {
				continue
			}
			keys[k.Kid] = &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}
		}
	}
	o.mu.Lock()
	o.keys[jwksURI] = cachedKeys{keys: keys, exp: time.Now().Add(time.Hour)}
	o.mu.Unlock()
	return keys, nil
}

// VerifyIDToken checks signature (RS256/ES256 via JWKS, refreshed once on an unknown kid),
// issuer, audience, expiry and nonce, and returns the claims.
func (o *OIDC) VerifyIDToken(ctx context.Context, d Discovery, clientID, raw, nonce string) (Claims, error) {
	var out Claims
	parse := func(force bool) (*jwt.Token, error) {
		keys, err := o.keyset(ctx, d.JWKSURI, force)
		if err != nil {
			return nil, err
		}
		return jwt.Parse(raw, func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			k, ok := keys[kid]
			if !ok && len(keys) == 1 && kid == "" {
				for _, only := range keys {
					k, ok = only, true
				}
			}
			if !ok {
				return nil, errUnknownKid
			}
			return k, nil
		}, jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "PS256"}),
			jwt.WithIssuer(d.Issuer), jwt.WithAudience(clientID), jwt.WithExpirationRequired(), jwt.WithLeeway(2*time.Minute),
			jwt.WithIssuedAt())
	}
	tok, err := parse(false)
	if errors.Is(err, errUnknownKid) {
		tok, err = parse(true)
	}
	if err != nil {
		return out, fmt.Errorf("id token: %w", err)
	}
	mc, _ := tok.Claims.(jwt.MapClaims)
	if n, _ := mc["nonce"].(string); nonce == "" || n != nonce {
		return out, errors.New("id token: nonce mismatch")
	}
	// Multiple audiences require azp = our client (OIDC Core §3.1.3.7).
	if aud, ok := mc["aud"].([]any); ok && len(aud) > 1 {
		if azp, _ := mc["azp"].(string); azp != clientID {
			return out, errors.New("id token: azp mismatch")
		}
	}
	return claimsFromMap(mc, "groups"), nil
}

var errUnknownKid = errors.New("unknown signing key")

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func claimsFromMap(m map[string]any, groupsClaim string) Claims {
	c := Claims{Subject: str(m, "sub"), Email: strings.ToLower(strings.TrimSpace(str(m, "email"))),
		GivenName: str(m, "given_name"), FamilyName: str(m, "family_name"), Name: str(m, "name"), Raw: m}
	switch v := m["email_verified"].(type) {
	case bool:
		c.EmailVerified = v
	case string:
		c.EmailVerified = strings.EqualFold(v, "true")
	}
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	switch g := m[groupsClaim].(type) {
	case []any:
		for _, x := range g {
			if s, ok := x.(string); ok && s != "" {
				c.Groups = append(c.Groups, s)
			}
		}
	case string:
		if g != "" {
			c.Groups = []string{g}
		}
	}
	return c
}

// ClaimsWithGroups re-reads the groups claim under a custom name.
func ClaimsWithGroups(c Claims, groupsClaim string) Claims {
	if groupsClaim == "" || groupsClaim == "groups" || c.Raw == nil {
		return c
	}
	n := claimsFromMap(c.Raw, groupsClaim)
	c.Groups = n.Groups
	return c
}
