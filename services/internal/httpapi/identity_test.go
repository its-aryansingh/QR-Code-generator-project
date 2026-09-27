package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/worker"
)

// ---- fakes ---------------------------------------------------------------------

type fakeDNS struct {
	mu   sync.Mutex
	recs map[string][]string
	err  error
}

func (f *fakeDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.recs[name], nil
}

func (f *fakeDNS) set(name, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recs[name] = append(f.recs[name], value)
}

type idpGrant struct {
	claims    map[string]any
	challenge string
	nonce     string
}

// fakeIdP is a minimal OpenID provider: discovery, JWKS, token endpoint with PKCE checks.
type fakeIdP struct {
	t      *testing.T
	srv    *httptest.Server
	key    *rsa.PrivateKey
	mu     sync.Mutex
	grants map[string]idpGrant
	tokens int
}

const idpClientID, idpSecret = "qrit-client", "s3cret-value"

func newFakeIdP(t *testing.T) *fakeIdP {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIdP{t: t, key: key, grants: map[string]idpGrant{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": f.srv.URL, "authorization_endpoint": f.srv.URL + "/authorize",
			"token_endpoint": f.srv.URL + "/token", "jwks_uri": f.srv.URL + "/jwks"})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		bad := func(code string) {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
		}
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("client_id") != idpClientID || r.Form.Get("client_secret") != idpSecret {
			bad("invalid_client")
			return
		}
		f.mu.Lock()
		g, ok := f.grants[r.Form.Get("code")]
		delete(f.grants, r.Form.Get("code"))
		f.tokens++
		f.mu.Unlock()
		if !ok {
			bad("invalid_grant")
			return
		}
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge {
			bad("invalid_grant") // PKCE
			return
		}
		mc := jwt.MapClaims{"iss": f.srv.URL, "aud": idpClientID, "iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(),
			"nonce": g.nonce}
		for k, v := range g.claims {
			mc[k] = v
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, mc)
		tok.Header["kid"] = "k1"
		signed, err := tok.SignedString(key)
		if err != nil {
			bad("server_error")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "id_token": signed})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// authorize plays the IdP's login page: it accepts the authorization request and returns
// the callback path QRit would be redirected to.
func (f *fakeIdP) authorize(redirectURL string, claims map[string]any, nonceOverride ...string) string {
	f.t.Helper()
	u, err := url.Parse(redirectURL)
	if err != nil || !strings.HasPrefix(redirectURL, f.srv.URL+"/authorize?") {
		f.t.Fatalf("unexpected authorization URL %q", redirectURL)
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("client_id") != idpClientID || q.Get("response_type") != "code" {
		f.t.Fatalf("authorization request: %v", q)
	}
	nonce := q.Get("nonce")
	if len(nonceOverride) > 0 {
		nonce = nonceOverride[0]
	}
	code := "code-" + strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", "")
	f.mu.Lock()
	f.grants[code] = idpGrant{claims: claims, challenge: q.Get("code_challenge"), nonce: nonce}
	f.mu.Unlock()
	return "/v1/auth/sso/callback?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(q.Get("state"))
}

func location(t *testing.T, r resp) string {
	t.Helper()
	if r.Status != http.StatusFound {
		t.Fatalf("want redirect, got %d: %s", r.Status, r.Body)
	}
	return r.Header.Get("Location")
}

func orgOf(t *testing.T, c *client) string {
	t.Helper()
	orgs := c.must(200, "GET", "/v1/me", nil)["organizations"].([]any)
	return orgs[0].(map[string]any)["id"].(string)
}

// enrollTOTP sets up an authenticator for the client's user and returns the secret and codes.
func enrollTOTP(t *testing.T, c *client) (string, []string) {
	t.Helper()
	begin := c.must(200, "POST", "/v1/me/mfa/totp", map[string]any{"name": "Phone"})
	secret := begin["secret"].(string)
	if !strings.HasPrefix(begin["otpauth_uri"].(string), "otpauth://totp/") {
		t.Fatalf("otpauth uri: %v", begin)
	}
	code, _ := auth.GenerateTOTPCode(secret, time.Now())
	done := c.must(201, "POST", "/v1/me/mfa/totp/confirm", map[string]any{"code": code})
	return secret, anyStrings(done["recovery_codes"])
}

// ---- MFA --------------------------------------------------------------------------

func TestMFA(t *testing.T) {
	h := newHarness(t)
	alice, aws, aem := h.register("mfa")
	const pw = "correct horse battery"

	begin := alice.must(200, "POST", "/v1/me/mfa/totp", nil)
	alice.must(422, "POST", "/v1/me/mfa/totp/confirm", map[string]any{"code": "000000"})
	secret := begin["secret"].(string)
	now, _ := auth.GenerateTOTPCode(secret, time.Now())
	done := alice.must(201, "POST", "/v1/me/mfa/totp/confirm", map[string]any{"code": now})
	codes := anyStrings(done["recovery_codes"])
	if len(codes) != 10 {
		t.Fatalf("recovery codes: %v", done)
	}
	st := alice.must(200, "GET", "/v1/me/mfa", nil)
	if len(st["factors"].([]any)) != 1 || st["session_verified"] != true || st["recovery_codes_remaining"] != float64(10) {
		t.Fatalf("mfa status: %v", st)
	}
	if dig(alice.must(200, "GET", "/v1/me", nil), "mfa", "enabled") != true {
		t.Fatal("me.mfa")
	}
	alice.must(200, "GET", "/v1/workspaces/"+aws, nil) // the enrolling session counts as verified

	// A new sign-in is confined to the MFA endpoints until the second factor is verified.
	login := func() *client {
		c := h.client()
		m := c.must(200, "POST", "/v1/auth/login", map[string]any{"email": aem, "password": pw})
		if m["mfa_required"] != true {
			t.Fatalf("login: %v", m)
		}
		return c
	}
	c1 := login()
	if r := c1.do("GET", "/v1/workspaces", nil); r.Status != 403 || r.json(t)["code"] != "mfa_verification_required" {
		t.Fatalf("pending session: %d %s", r.Status, r.Body)
	}
	c1.must(200, "GET", "/v1/me", nil)
	c1.must(401, "POST", "/v1/auth/mfa/verify", map[string]any{"code": "123456"})
	next, _ := auth.GenerateTOTPCode(secret, time.Now().Add(30*time.Second))
	c1.must(200, "POST", "/v1/auth/mfa/verify", map[string]any{"code": next})
	c1.must(200, "GET", "/v1/workspaces", nil)

	// Each code works once; recovery codes are single use and case-insensitive.
	c2 := login()
	c2.must(401, "POST", "/v1/auth/mfa/verify", map[string]any{"code": next})
	v := c2.must(200, "POST", "/v1/auth/mfa/verify", map[string]any{"recovery_code": strings.ToLower(codes[0])})
	if v["recovery_codes_remaining"] != float64(9) {
		t.Fatalf("recovery: %v", v)
	}
	c3 := login()
	c3.must(401, "POST", "/v1/auth/mfa/verify", map[string]any{"recovery_code": codes[0]})
	c3.must(200, "POST", "/v1/auth/mfa/verify", map[string]any{"recovery_code": codes[1]})

	// Step-up needs the second factor too.
	if r := alice.do("POST", "/v1/auth/step-up", map[string]any{"password": pw}); r.Status != 401 || r.json(t)["code"] != "mfa_code_required" {
		t.Fatalf("step-up without code: %d %s", r.Status, r.Body)
	}
	prev, _ := auth.GenerateTOTPCode(secret, time.Now().Add(-30*time.Second))
	alice.must(200, "POST", "/v1/auth/step-up", map[string]any{"password": pw, "code": prev})
	regen := alice.must(200, "POST", "/v1/me/mfa/recovery-codes", nil)
	if len(anyStrings(regen["recovery_codes"])) != 10 {
		t.Fatal("regenerate")
	}
	c4 := login()
	c4.must(401, "POST", "/v1/auth/mfa/verify", map[string]any{"recovery_code": codes[2]}) // old set is gone

	// An organisation requiring MFA blocks removing the last factor.
	factorID := st["factors"].([]any)[0].(map[string]any)["id"].(string)
	orgID := orgOf(t, alice)
	h.exec(`UPDATE org_security_policies SET require_mfa = true WHERE org_id = $1`, orgID)
	if r := alice.do("DELETE", "/v1/me/mfa/"+factorID, nil); r.Status != 409 || r.json(t)["code"] != "mfa_required_by_org" {
		t.Fatalf("remove last factor: %d %s", r.Status, r.Body)
	}
	h.exec(`UPDATE org_security_policies SET require_mfa = false WHERE org_id = $1`, orgID)
	alice.must(204, "DELETE", "/v1/me/mfa/"+factorID, nil)
	var left int
	_ = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM user_recovery_codes rc JOIN users u ON u.id = rc.user_id WHERE u.email = $1`, aem).Scan(&left)
	if left != 0 {
		t.Fatalf("recovery codes left after removing the last factor: %d", left)
	}
	if m := h.client().must(200, "POST", "/v1/auth/login", map[string]any{"email": aem, "password": pw}); m["mfa_required"] != false {
		t.Fatalf("login after removal: %v", m)
	}
	var audits int
	_ = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs a JOIN users u ON u.id = a.actor_id
		WHERE u.email = $1 AND a.action LIKE 'auth.mfa.%'`, aem).Scan(&audits)
	if audits < 6 {
		t.Fatalf("mfa audit entries: %d", audits)
	}
}

// ---- domains + OIDC SSO -------------------------------------------------------------

func TestDomainsAndOIDC(t *testing.T) {
	dns := &fakeDNS{recs: map[string][]string{}}
	h := newHarness(t, func(s *Server) { s.SetSSO(SSODeps{DNS: dns}) })
	ctx := context.Background()
	idp := newFakeIdP(t)
	const pw = "correct horse battery"

	alice, aws, _ := h.register("ssoadmin")
	orgID := orgOf(t, alice)
	var aliceID string
	_ = h.pool.QueryRow(ctx, `SELECT user_id::text FROM org_members WHERE org_id = $1 AND org_role = 'org_owner'`, orgID).Scan(&aliceID)
	base := "/v1/orgs/" + orgID
	domain := "acme" + strings.ReplaceAll(time.Now().Format("150405.000000"), ".", "") + ".test"

	// Claimed domains: public mail providers refused; TXT proof required.
	alice.must(422, "POST", base+"/domains", map[string]any{"domain": "gmail.com"})
	alice.must(422, "POST", base+"/domains", map[string]any{"domain": "not a domain"})
	d := alice.must(201, "POST", base+"/domains", map[string]any{"domain": strings.ToUpper(domain)})
	if d["domain"] != domain || d["txt_record_name"] != "_qrit-challenge."+domain || d["verified_at"] != nil {
		t.Fatalf("domain: %v", d)
	}
	alice.must(409, "POST", base+"/domains", map[string]any{"domain": domain})
	did := d["id"].(string)

	bob, _, bem := h.register("squatter")
	bobOrg := orgOf(t, bob)
	bob.must(201, "POST", "/v1/orgs/"+bobOrg+"/domains", map[string]any{"domain": domain}) // pending claims don't block

	if r := alice.do("POST", base+"/domains/"+did+"/verify", nil); r.Status != 409 || r.json(t)["code"] != "txt_record_not_found" {
		t.Fatalf("verify without record: %d %s", r.Status, r.Body)
	}
	dns.err = errors.New("resolver down")
	alice.must(502, "POST", base+"/domains/"+did+"/verify", nil)
	dns.err = nil
	dns.set("_qrit-challenge."+domain, d["txt_record_value"].(string))
	if v := alice.must(200, "POST", base+"/domains/"+did+"/verify", nil); v["verified_at"] == nil {
		t.Fatalf("verified: %v", v)
	}
	if r := bob.do("POST", "/v1/orgs/"+bobOrg+"/domains", map[string]any{"domain": "sub." + domain}); r.Status != 201 {
		t.Fatalf("subdomain claim: %d %s", r.Status, r.Body)
	}
	// Once verified, nobody else can claim the domain.
	h.exec(`DELETE FROM org_domains WHERE org_id = $1 AND domain = $2`, bobOrg, domain)
	if r := bob.do("POST", "/v1/orgs/"+bobOrg+"/domains", map[string]any{"domain": domain}); r.Status != 409 || r.json(t)["code"] != "domain_claimed" {
		t.Fatalf("verified domain claimed again: %d %s", r.Status, r.Body)
	}

	// Background verification by the worker.
	beta := "beta-" + domain
	bd := alice.must(201, "POST", base+"/domains", map[string]any{"domain": beta})
	wd := &worker.Deps{Pool: h.pool}
	if n, err := wd.VerifyPendingDomains(ctx, dns); err != nil || n != 0 {
		t.Fatalf("poll without record: %d %v", n, err)
	}
	h.exec(`UPDATE org_domains SET last_checked_at = now() - interval '1 hour' WHERE id = $1`, bd["id"])
	dns.set("_qrit-challenge."+beta, bd["txt_record_value"].(string))
	if n, err := wd.VerifyPendingDomains(ctx, dns); err != nil || n != 1 {
		t.Fatalf("background verify: %d %v", n, err)
	}
	var sysAudit int
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE org_id = $1 AND action = 'org.domain.verified' AND actor_type = 'system'`, orgID).Scan(&sysAudit)
	if sysAudit != 1 {
		t.Fatalf("worker audit: %d", sysAudit)
	}

	// SSO connections need the plan and a recent step-up.
	connReq := map[string]any{"protocol": "oidc", "label": "Okta", "idp_kind": "okta", "oidc_issuer": idp.srv.URL,
		"oidc_client_id": idpClientID, "oidc_client_secret": idpSecret, "default_workspace_id": aws, "default_role_key": "editor"}
	alice.must(401, "POST", base+"/sso-connections", connReq)
	alice.must(200, "POST", "/v1/auth/step-up", map[string]any{"password": pw})
	alice.must(402, "POST", base+"/sso-connections", connReq)
	h.setPlan(aws, "enterprise")
	alice.must(422, "POST", base+"/sso-connections", map[string]any{"protocol": "oidc", "oidc_issuer": "https://127.0.0.1:1",
		"oidc_client_id": "x", "oidc_client_secret": "y"})
	alice.must(503, "POST", base+"/sso-connections", map[string]any{"protocol": "saml", "saml_metadata_xml": "<x/>"})
	conn := alice.must(201, "POST", base+"/sso-connections", connReq)
	connID := conn["id"].(string)
	if conn["status"] != "draft" || conn["has_client_secret"] != true || conn["redirect_uri"] == "" {
		t.Fatalf("connection: %v", conn)
	}
	var secretCT []byte
	_ = h.pool.QueryRow(ctx, `SELECT oidc_client_secret_ct FROM sso_connections WHERE id = $1`, connID).Scan(&secretCT)
	if strings.Contains(string(secretCT), idpSecret) {
		t.Fatal("client secret stored in clear")
	}

	// Not active yet: nobody can use it. The admin's test login activates it.
	dave := h.client()
	daveEmail := "dave@" + domain
	dave.must(404, "POST", "/v1/auth/sso/start", map[string]any{"email": daveEmail})
	dave.must(404, "POST", "/v1/auth/sso/start", map[string]any{"email": "someone@unclaimed.test"})
	test := alice.must(200, "POST", base+"/sso-connections/"+connID+"/test", nil)
	cb := idp.authorize(test["redirect_url"].(string), map[string]any{"sub": "admin-1", "email": "admin@" + domain, "email_verified": true,
		"groups": []any{"Admins"}})
	if u, _ := url.Parse(test["redirect_url"].(string)); u.Query().Get("prompt") != "login" {
		t.Fatal("test login must force re-authentication")
	}
	if loc := location(t, alice.do("GET", cb, nil)); !strings.HasSuffix(loc, "/settings/sso?sso_test=ok") {
		t.Fatalf("test callback: %s", loc)
	}
	tr := alice.must(200, "GET", base+"/sso-connections/"+connID+"/test-result", nil)
	if tr["ok"] != true || tr["email"] != "admin@"+domain {
		t.Fatalf("test result: %v", tr)
	}
	var status string
	_ = h.pool.QueryRow(ctx, `SELECT status FROM sso_connections WHERE id = $1`, connID).Scan(&status)
	if status != "active" {
		t.Fatalf("status after test: %s", status)
	}
	var identities int
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FROM user_identities WHERE connection_id = $1`, connID).Scan(&identities)
	if identities != 0 {
		t.Fatal("a test login must not provision users")
	}

	// JIT provisioning on a verified domain, with default workspace role and group sync.
	ssoLogin := func(c *client, email string, claims map[string]any, nonce ...string) string {
		t.Helper()
		start := c.must(200, "POST", "/v1/auth/sso/start", map[string]any{"email": email, "redirect": "/dashboard"})
		if start["protocol"] != "oidc" {
			t.Fatalf("start: %v", start)
		}
		return location(t, c.do("GET", idp.authorize(start["redirect_url"].(string), claims, nonce...), nil))
	}
	loc := ssoLogin(dave, daveEmail, map[string]any{"sub": "dave-1", "email": daveEmail, "email_verified": true,
		"given_name": "Dave", "family_name": "Diaz", "groups": []any{"Engineering", "Marketing"}})
	if loc != "http://app.test/dashboard" {
		t.Fatalf("login redirect: %s", loc)
	}
	me := dave.must(200, "GET", "/v1/me", nil)
	if dig(me, "user", "name") != "Dave Diaz" {
		t.Fatalf("jit user: %v", me)
	}
	dave.must(200, "GET", "/v1/workspaces/"+aws, nil)
	dave.must(201, "POST", "/v1/workspaces/"+aws+"/qr-codes", map[string]any{"name": "sso", "destination_url": "https://example.com/"})
	var source, am string
	_ = h.pool.QueryRow(ctx, `SELECT m.source FROM org_members m JOIN users u ON u.id = m.user_id WHERE m.org_id = $1 AND u.email = $2`,
		orgID, daveEmail).Scan(&source)
	_ = h.pool.QueryRow(ctx, `SELECT s.auth_method FROM sessions s JOIN users u ON u.id = s.user_id WHERE u.email = $1 ORDER BY s.created_at DESC LIMIT 1`,
		daveEmail).Scan(&am)
	if source != "sso_jit" || am != "sso" {
		t.Fatalf("membership source %q, auth method %q", source, am)
	}
	groupsOf := func(email string) []string {
		rows, _ := h.pool.Query(ctx, `SELECT g.display_name FROM group_members gm JOIN groups g ON g.id = gm.group_id
			JOIN users u ON u.id = gm.user_id WHERE u.email = $1 AND g.org_id = $2 ORDER BY 1`, email, orgID)
		defer rows.Close()
		var out []string
		for rows.Next() {
			var n string
			_ = rows.Scan(&n)
			out = append(out, n)
		}
		return out
	}
	if g := strings.Join(groupsOf(daveEmail), ","); g != "Engineering,Marketing" {
		t.Fatalf("groups: %s", g)
	}
	// Groups follow the IdP on every login; a manually managed group of the same name is untouched.
	h.exec(`INSERT INTO groups (id, org_id, display_name, source) VALUES (gen_random_uuid(), $1, 'Finance', 'manual')`, orgID)
	dave2 := h.client()
	ssoLogin(dave2, daveEmail, map[string]any{"sub": "dave-1", "email": daveEmail, "email_verified": true, "groups": []any{"Engineering", "Finance"}})
	if g := strings.Join(groupsOf(daveEmail), ","); g != "Engineering" {
		t.Fatalf("groups after resync: %s", g)
	}

	// State is single use; tampered nonces and unverified emails are rejected.
	start := h.client().must(200, "POST", "/v1/auth/sso/start", map[string]any{"email": daveEmail})
	cbURL := idp.authorize(start["redirect_url"].(string), map[string]any{"sub": "dave-1", "email": daveEmail, "email_verified": true})
	location(t, h.client().do("GET", cbURL, nil))
	if loc := location(t, h.client().do("GET", cbURL, nil)); !strings.HasSuffix(loc, "sso_error=invalid_state") {
		t.Fatalf("replayed state: %s", loc)
	}
	if loc := ssoLogin(h.client(), daveEmail, map[string]any{"sub": "dave-1", "email": daveEmail, "email_verified": true}, "forged"); !strings.HasSuffix(loc, "sso_error=assertion_rejected") {
		t.Fatalf("nonce: %s", loc)
	}
	if loc := ssoLogin(h.client(), daveEmail, map[string]any{"sub": "eve-1", "email": "eve@elsewhere.test"}); !strings.HasSuffix(loc, "sso_error=email_not_verified") {
		t.Fatalf("unverified email: %s", loc)
	}
	// An existing account on a domain the org hasn't verified is never taken over by email.
	if loc := ssoLogin(h.client(), daveEmail, map[string]any{"sub": "bob-1", "email": bem, "email_verified": true}); !strings.HasSuffix(loc, "sso_error=sso_account_exists") {
		t.Fatalf("account takeover guard: %s", loc)
	}
	// Entra: no email claim, UPN in preferred_username, accepted on the verified domain.
	gina := h.client()
	ssoLogin(gina, daveEmail, map[string]any{"sub": "gina-oid", "preferred_username": "Gina@" + strings.ToUpper(domain), "name": "Gina G"})
	if m := gina.must(200, "GET", "/v1/me", nil); !strings.Contains(string(mustJSON(m)), "gina@"+domain) {
		t.Fatalf("entra upn user: %v", m)
	}

	// An existing password account on the verified domain is linked, not duplicated.
	frank := h.client()
	frankEmail := "frank@" + domain
	frank.must(201, "POST", "/v1/auth/register", map[string]any{"email": frankEmail, "password": pw, "name": "Frank"})
	frankSSO := h.client()
	ssoLogin(frankSSO, frankEmail, map[string]any{"sub": "frank-1", "email": frankEmail, "email_verified": true})
	var linked int
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FROM user_identities i JOIN users u ON u.id = i.user_id WHERE u.email = $1`, frankEmail).Scan(&linked)
	if linked != 1 {
		t.Fatalf("frank linked: %d", linked)
	}

	// JIT off: unknown users are refused.
	alice.must(200, "PATCH", base+"/sso-connections/"+connID, map[string]any{"jit_provisioning": false})
	if loc := ssoLogin(h.client(), daveEmail, map[string]any{"sub": "ivy-1", "email": "ivy@" + domain, "email_verified": true}); !strings.HasSuffix(loc, "sso_error=sso_user_not_provisioned") {
		t.Fatalf("jit off: %s", loc)
	}
	ssoLogin(h.client(), daveEmail, map[string]any{"sub": "dave-1", "email": daveEmail, "email_verified": true}) // existing members still sign in

	// Enforcing SSO: needs a break-glass owner with MFA; password sessions lose access.
	alice.must(409, "PUT", base+"/security-policy", map[string]any{"enforce_sso": true})
	enrollTOTP(t, alice)
	alice.must(200, "PUT", base+"/security-policy", map[string]any{"enforce_sso": true, "sso_break_glass_user_ids": []any{aliceID}})
	if r := frank.do("GET", "/v1/workspaces/"+aws, nil); r.Status != 403 || r.json(t)["code"] != "sso_required" {
		t.Fatalf("password session under enforcement: %d %s", r.Status, r.Body)
	}
	frankSSO.must(200, "GET", "/v1/workspaces/"+aws, nil)
	dave2.must(200, "GET", "/v1/workspaces/"+aws, nil)
	alice.must(200, "GET", "/v1/workspaces/"+aws, nil) // break-glass owner with verified MFA
	frank.must(200, "GET", "/v1/workspaces", nil)      // his own organisation is unaffected

	var logins int
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE org_id = $1 AND action IN ('auth.sso.login','auth.sso.user_provisioned')`, orgID).Scan(&logins)
	if logins < 5 {
		t.Fatalf("sso audit entries: %d", logins)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// ---- SCIM ---------------------------------------------------------------------------

func TestSCIM(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	const pw = "correct horse battery"
	alice, aws, _ := h.register("scimadmin")
	orgID := orgOf(t, alice)
	base := "/v1/orgs/" + orgID
	domain := "corp" + strings.ReplaceAll(time.Now().Format("150405.000000"), ".", "") + ".test"

	alice.must(401, "POST", base+"/scim-directories", map[string]any{"label": "Entra"})
	alice.must(200, "POST", "/v1/auth/step-up", map[string]any{"password": pw})
	alice.must(402, "POST", base+"/scim-directories", map[string]any{"label": "Entra"})
	h.setPlan(aws, "enterprise")
	alice.must(422, "POST", base+"/scim-directories", map[string]any{"label": "Entra", "deprovision_action": "delete"})
	dir := alice.must(201, "POST", base+"/scim-directories", map[string]any{"label": "Entra"})
	token := dir["token"].(string)
	dirID := dig(dir, "directory", "id").(string)
	if !strings.HasPrefix(token, "scim_") || len(token) != 37 || dig(dir, "directory", "token_prefix") != token[:13] {
		t.Fatalf("directory: %v", dir)
	}
	list := alice.must(200, "GET", base+"/scim-directories", nil)["data"].([]any)
	if len(list) != 1 || strings.Contains(string(mustJSON(list)), token) {
		t.Fatal("token must only be shown once")
	}

	idp := h.client()
	idp.token = token
	anon := h.client()
	anon.token = "scim_WRONG"
	anon.must(401, "GET", "/scim/v2/Users", nil)
	spc := idp.must(200, "GET", "/scim/v2/ServiceProviderConfig", nil)
	if dig(spc, "patch", "supported") != true {
		t.Fatalf("spc: %v", spc)
	}
	idp.must(200, "GET", "/scim/v2/ResourceTypes", nil)
	idp.must(200, "GET", "/scim/v2/Schemas", nil)

	// Entra's connectivity probe: a filter that matches nothing returns an empty ListResponse.
	probe := idp.must(200, "GET", `/scim/v2/Users?filter=`+url.QueryEscape(`userName eq "nobody-`+domain+`"`), nil)
	if probe["totalResults"] != float64(0) || len(probe["Resources"].([]any)) != 0 {
		t.Fatalf("probe: %v", probe)
	}
	idp.must(400, "GET", `/scim/v2/Users?filter=`+url.QueryEscape(`title co "x"`), nil)

	newUser := func(local, ext string, active any) map[string]any {
		return map[string]any{"schemas": []any{scimUserSchema, scimEntSchema}, "userName": local + "@" + domain, "externalId": ext,
			"active": active, "name": map[string]any{"givenName": strings.ToUpper(local[:1]) + local[1:], "familyName": "Test"},
			"emails": []any{map[string]any{"primary": true, "type": "work", "value": local + "@" + domain}}}
	}
	hank := idp.must(201, "POST", "/scim/v2/Users", newUser("hank", "ext-hank", "True"))
	hankID := hank["id"].(string)
	if hank["active"] != true || dig(hank, "meta", "version") != `W/"1"` || hank["externalId"] != "ext-hank" {
		t.Fatalf("created: %v", hank)
	}
	idp.must(409, "POST", "/scim/v2/Users", newUser("hank", "ext-other", true))
	ivy := idp.must(201, "POST", "/scim/v2/Users", newUser("ivy", "ext-ivy", true))
	ivyID := ivy["id"].(string)

	found := idp.must(200, "GET", `/scim/v2/Users?filter=`+url.QueryEscape(`userName eq "HANK@`+strings.ToUpper(domain)+`"`), nil)
	if found["totalResults"] != float64(1) {
		t.Fatalf("filter: %v", found)
	}
	byExt := idp.must(200, "GET", `/scim/v2/Users?filter=`+url.QueryEscape(`externalId eq "ext-ivy"`), nil)
	if byExt["totalResults"] != float64(1) {
		t.Fatalf("filter externalId: %v", byExt)
	}
	page := idp.must(200, "GET", "/scim/v2/Users?startIndex=2&count=1", nil)
	if page["totalResults"] != float64(2) || page["itemsPerPage"] != float64(1) || page["startIndex"] != float64(2) {
		t.Fatalf("paging: %v", page)
	}

	// An existing account outside a verified domain can't be taken over by the directory.
	_, _, bem := h.register("outsider")
	outsider := newUser("x", "ext-x", true)
	outsider["userName"], outsider["emails"] = bem, []any{map[string]any{"value": bem, "primary": true}}
	idp.must(409, "POST", "/scim/v2/Users", outsider)

	// Give hank a password and a session so deprovisioning can be observed.
	h.exec(`UPDATE users SET password_hash = (SELECT password_hash FROM users WHERE id = (SELECT user_id FROM org_members
		WHERE org_id = $1 AND org_role = 'org_owner')) WHERE email = $2`, orgID, "hank@"+domain)
	hc := h.client()
	hc.must(200, "POST", "/v1/auth/login", map[string]any{"email": "hank@" + domain, "password": pw})
	hc.must(200, "GET", "/v1/orgs/"+orgID, nil)

	// Entra-style PATCH: capitalised op, "False" string.
	idp.must(412, "PATCH", "/scim/v2/Users/"+hankID, map[string]any{"schemas": []any{scimPatchSchema},
		"Operations": []any{map[string]any{"op": "Replace", "path": "active", "value": "False"}}}, "If-Match", `W/"7"`)
	p := idp.must(200, "PATCH", "/scim/v2/Users/"+hankID, map[string]any{"schemas": []any{scimPatchSchema},
		"Operations": []any{map[string]any{"op": "Replace", "path": "active", "value": "False"}}})
	if p["active"] != false || dig(p, "meta", "version") != `W/"2"` {
		t.Fatalf("deactivate: %v", p)
	}
	memberStatus := func(email string) string {
		var st string
		_ = h.pool.QueryRow(ctx, `SELECT m.status FROM org_members m JOIN users u ON u.id = m.user_id WHERE m.org_id = $1 AND u.email = $2`,
			orgID, email).Scan(&st)
		return st
	}
	if memberStatus("hank@"+domain) != "suspended" {
		t.Fatal("suspended")
	}
	hc.must(401, "GET", "/v1/me", nil) // managed account: sessions revoked

	// Okta-style PATCH without path; name and enterprise attributes; typed email path.
	idp.must(200, "PATCH", "/scim/v2/Users/"+hankID, map[string]any{"Operations": []any{
		map[string]any{"op": "replace", "value": map[string]any{"active": true}},
		map[string]any{"op": "replace", "path": "name.givenName", "value": "Henry"},
		map[string]any{"op": "add", "path": scimEntSchema + ":department", "value": "Sales"},
		map[string]any{"op": "replace", "path": `emails[type eq "work"].value`, "value": "henry@" + domain},
	}})
	got := idp.must(200, "GET", "/scim/v2/Users/"+hankID, nil)
	if got["active"] != true || dig(got, scimEntSchema, "department") != "Sales" || dig(got, "name", "givenName") != "Henry" {
		t.Fatalf("patched: %v", got)
	}
	var name, email string
	_ = h.pool.QueryRow(ctx, `SELECT u.name, u.email FROM users u JOIN scim_users s ON s.user_id = u.id WHERE s.id = $1`, hankID).Scan(&name, &email)
	if name != "Henry Test" || email != "henry@"+domain || memberStatus(email) != "active" {
		t.Fatalf("user sync: %q %q %q", name, email, memberStatus(email))
	}
	idp.must(400, "PATCH", "/scim/v2/Users/"+hankID, map[string]any{"Operations": []any{map[string]any{"op": "move", "path": "x"}}})

	// PUT replaces the representation.
	put := newUser("ivy", "ext-ivy", false)
	put["displayName"] = "Ivy T."
	idp.must(200, "PUT", "/scim/v2/Users/"+ivyID, put)
	if memberStatus("ivy@"+domain) != "suspended" {
		t.Fatal("put active=false")
	}
	put["active"] = true
	idp.must(200, "PUT", "/scim/v2/Users/"+ivyID, put)

	// Groups: create with members, add/remove via PATCH (204), filter, adopt an SSO group.
	g := idp.must(201, "POST", "/scim/v2/Groups", map[string]any{"schemas": []any{scimGroupSchema}, "displayName": "Engineering",
		"externalId": "g-eng", "members": []any{map[string]any{"value": hankID}}})
	gid := g["id"].(string)
	if len(g["members"].([]any)) != 1 {
		t.Fatalf("group: %v", g)
	}
	idp.must(409, "POST", "/scim/v2/Groups", map[string]any{"displayName": "Engineering"})
	idp.must(204, "PATCH", "/scim/v2/Groups/"+gid, map[string]any{"schemas": []any{scimPatchSchema}, "Operations": []any{
		map[string]any{"op": "Add", "path": "members", "value": []any{map[string]any{"value": ivyID}}}}})
	gg := idp.must(200, "GET", "/scim/v2/Groups/"+gid, nil)
	if len(gg["members"].([]any)) != 2 {
		t.Fatalf("members after add: %v", gg)
	}
	idp.must(204, "PATCH", "/scim/v2/Groups/"+gid, map[string]any{"Operations": []any{
		map[string]any{"op": "Remove", "path": `members[value eq "` + hankID + `"]`},
		map[string]any{"op": "Replace", "path": "displayName", "value": "Platform Engineering"}}})
	gg = idp.must(200, "GET", "/scim/v2/Groups/"+gid+"?excludedAttributes=members", nil)
	if gg["displayName"] != "Platform Engineering" || gg["members"] != nil {
		t.Fatalf("group after patch: %v", gg)
	}
	gl := idp.must(200, "GET", `/scim/v2/Groups?filter=`+url.QueryEscape(`displayName eq "Platform Engineering"`), nil)
	if gl["totalResults"] != float64(1) || len(dig(gl["Resources"].([]any)[0].(map[string]any), "members").([]any)) != 1 {
		t.Fatalf("group filter: %v", gl)
	}
	h.exec(`INSERT INTO groups (id, org_id, display_name, source) VALUES (gen_random_uuid(), $1, 'Design', 'sso')`, orgID)
	h.exec(`INSERT INTO groups (id, org_id, display_name, source) VALUES (gen_random_uuid(), $1, 'Board', 'manual')`, orgID)
	idp.must(201, "POST", "/scim/v2/Groups", map[string]any{"displayName": "Design"})
	idp.must(409, "POST", "/scim/v2/Groups", map[string]any{"displayName": "Board"})
	var designSource string
	_ = h.pool.QueryRow(ctx, `SELECT source FROM groups WHERE org_id = $1 AND display_name = 'Design'`, orgID).Scan(&designSource)
	if designSource != "scim" {
		t.Fatalf("adopted group source: %s", designSource)
	}

	// Group role bindings disappear with the group.
	var roleID string
	_ = h.pool.QueryRow(ctx, `SELECT id::text FROM roles WHERE org_id IS NULL AND key = 'analyst'`).Scan(&roleID)
	h.exec(`INSERT INTO role_bindings (id, org_id, workspace_id, principal_type, principal_id, role_id) VALUES (gen_random_uuid(), $1, $2, 'group', $3, $4)`,
		orgID, aws, gid, roleID)
	idp.must(204, "DELETE", "/scim/v2/Groups/"+gid, nil)
	idp.must(404, "GET", "/scim/v2/Groups/"+gid, nil)
	var bindings int
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FROM role_bindings WHERE principal_id = $1`, gid).Scan(&bindings)
	if bindings != 0 {
		t.Fatal("group bindings left behind")
	}

	// DELETE deprovisions: suspend (default) keeps the membership row, remove deletes it.
	idp.must(204, "DELETE", "/scim/v2/Users/"+hankID, nil)
	idp.must(404, "GET", "/scim/v2/Users/"+hankID, nil)
	if memberStatus("henry@"+domain) != "deprovisioned" {
		t.Fatalf("deprovisioned: %q", memberStatus("henry@"+domain))
	}
	alice.must(200, "PATCH", base+"/scim-directories/"+dirID, map[string]any{"deprovision_action": "remove"})
	idp.must(204, "DELETE", "/scim/v2/Users/"+ivyID, nil)
	if memberStatus("ivy@"+domain) != "" {
		t.Fatal("removed")
	}

	// Token rotation and disabling.
	rot := alice.must(200, "POST", base+"/scim-directories/"+dirID+"/token", nil)
	idp.must(401, "GET", "/scim/v2/Users", nil)
	idp.token = rot["token"].(string)
	idp.must(200, "GET", "/scim/v2/Users", nil)
	alice.must(200, "PATCH", base+"/scim-directories/"+dirID, map[string]any{"status": "disabled"})
	idp.must(401, "GET", "/scim/v2/Users", nil)

	var scimAudits int
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE org_id = $1 AND actor_type = 'system' AND changes->>'via' = 'scim'`, orgID).Scan(&scimAudits)
	if scimAudits < 8 {
		t.Fatalf("scim audit entries: %d", scimAudits)
	}
	alice.must(204, "DELETE", base+"/scim-directories/"+dirID, nil)
}
