package httpapi

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestAuthLifecycle(t *testing.T) {
	h := newHarness(t)
	c, _, em := h.register("alice")

	me := c.must(200, "GET", "/v1/me", nil)
	if dig(me, "user", "email") != em || dig(me, "user", "has_password") != true {
		t.Fatalf("me: %v", me)
	}
	if strings.Contains(string(c.do("GET", "/v1/me", nil).Body), "password_hash") {
		t.Fatal("password hash leaked")
	}
	// Duplicate email.
	h.client().must(409, "POST", "/v1/auth/register", map[string]any{"email": em, "password": "correct horse battery"})
	// CSRF: cookie-authenticated mutation without the header is refused.
	r := c.do("PATCH", "/v1/me", map[string]any{"name": "A"}, "X-CSRF-Token", "wrong")
	if r.Status != 403 {
		t.Fatalf("csrf: want 403 got %d", r.Status)
	}
	c.must(200, "PATCH", "/v1/me", map[string]any{"name": "Alice", "timezone": "Asia/Kolkata"})
	c.must(422, "PATCH", "/v1/me", map[string]any{"timezone": "Mars/Base"})

	// Email verification via the mailed token.
	mail, ok := h.mail.Last(em)
	if !ok {
		t.Fatal("no verification email")
	}
	tok := tokenInLink.FindStringSubmatch(mail.Body)[1]
	h.client().must(200, "POST", "/v1/auth/verify-email", map[string]any{"token": tok})
	h.client().must(410, "POST", "/v1/auth/verify-email", map[string]any{"token": tok})

	// Login, wrong password, lockout.
	lc := h.client()
	lc.must(401, "POST", "/v1/auth/login", map[string]any{"email": em, "password": "nope nope nope"})
	login := lc.must(200, "POST", "/v1/auth/login", map[string]any{"email": em, "password": "correct horse battery"})
	access := login["token"].(string)

	// Logout revokes the session: the old access token stops working immediately.
	bearer := h.client()
	bearer.token = access
	bearer.must(200, "GET", "/v1/me", nil)
	lc.must(204, "POST", "/v1/auth/logout", nil)
	bearer.must(401, "GET", "/v1/me", nil)

	// Refresh rotation and reuse detection.
	rc := h.client()
	rc.must(200, "POST", "/v1/auth/login", map[string]any{"email": em, "password": "correct horse battery"})
	var old string
	for _, ck := range rc.http.Jar.Cookies(mustURL(h.ts.URL)) {
		if ck.Name == "qrit_refresh" {
			old = ck.Value
		}
	}
	rc.must(200, "POST", "/v1/auth/refresh", nil)
	replay := h.client()
	replay.must(401, "POST", "/v1/auth/refresh", nil, "X-Refresh-Token", old)
	rc.must(401, "POST", "/v1/auth/refresh", nil) // whole family revoked

	// Lockout after repeated failures (Redis-backed).
	if h.rdb != nil {
		_, _, em2 := h.register("locky")
		x := h.client()
		for i := 0; i < 5; i++ {
			x.must(401, "POST", "/v1/auth/login", map[string]any{"email": em2, "password": "bad password!!"})
		}
		x.must(429, "POST", "/v1/auth/login", map[string]any{"email": em2, "password": "correct horse battery"})
	}

	// Forgot/reset password signs out every session.
	h.client().must(202, "POST", "/v1/auth/password/forgot", map[string]any{"email": em})
	h.client().must(202, "POST", "/v1/auth/password/forgot", map[string]any{"email": "nobody@example.com"})
	deadline := time.Now().Add(3 * time.Second)
	var resetTok string
	for time.Now().Before(deadline) {
		if m, ok := h.mail.Last(em); ok && strings.Contains(m.Body, "reset-password") {
			resetTok = tokenInLink.FindStringSubmatch(m.Body)[1]
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if resetTok == "" {
		t.Fatal("no reset email")
	}
	h.client().must(422, "POST", "/v1/auth/password/reset", map[string]any{"token": resetTok, "password": "short"})
	h.client().must(204, "POST", "/v1/auth/password/reset", map[string]any{"token": resetTok, "password": "a brand new passphrase"})
	c.must(401, "GET", "/v1/me", nil)
	h.client().must(200, "POST", "/v1/auth/login", map[string]any{"email": em, "password": "a brand new passphrase"})
}

func TestQRCodeLifecycle(t *testing.T) {
	h := newHarness(t)
	c, ws, _ := h.register("owner")
	base := "/v1/workspaces/" + ws + "/qr-codes"

	// Dynamic URL code.
	q := c.must(201, "POST", base, map[string]any{"name": "Menu", "destination_url": "https://example.com/menu"})
	code, _ := q["short_code"].(string)
	if !regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{7}$`).MatchString(code) {
		t.Fatalf("short code %q", code)
	}
	if q["short_url"] != "https://qr.test/"+code || q["encoded_payload"] != "HTTPS://QR.TEST/"+code {
		t.Fatalf("urls: %v %v", q["short_url"], q["encoded_payload"])
	}
	if dig(q, "current_version", "destination_url") != "https://example.com/menu" || q["destination_url"] != "https://example.com/menu" {
		t.Fatalf("current version: %v", q["current_version"])
	}
	id := q["id"].(string)

	// Static codes: payload encoded server-side, no short link.
	wifi := c.must(201, "POST", base, map[string]any{"name": "Guest WiFi", "mode": "static", "content_type": "wifi",
		"static_content": map[string]any{"ssid": "Cafe;Guest", "password": "p@ss", "auth": "WPA"}})
	if wifi["encoded_payload"] != `WIFI:T:WPA;S:Cafe\;Guest;P:p@ss;H:false;;` || wifi["short_code"] != nil {
		t.Fatalf("wifi: %v", wifi)
	}
	c.must(422, "POST", base, map[string]any{"name": "t", "content_type": "text", "destination_url": "https://x.com"})
	c.must(422, "POST", base, map[string]any{"name": "bad", "destination_url": "javascript:alert(1)"})
	c.must(422, "POST", base, map[string]any{"name": "self", "destination_url": "https://qr.test/ABCDEFG"})
	h.safety.UnsafeURLs["https://malware.example/"] = true
	c.must(422, "POST", base, map[string]any{"name": "evil", "destination_url": "https://malware.example/"})

	// Plan gates on free: scheduling, rules, and the dynamic code quota (3).
	future := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	r := c.do("POST", base+"/"+id+"/versions", map[string]any{"destination_url": "https://example.com/v2", "effective_at": future})
	if r.Status != 402 || r.json(t)["required_plan"] != "pro" {
		t.Fatalf("scheduling gate: %d %s", r.Status, r.Body)
	}
	c.must(201, "POST", base, map[string]any{"name": "Two", "destination_url": "https://example.com/2"})
	c.must(201, "POST", base, map[string]any{"name": "Three", "destination_url": "https://example.com/3"})
	r = c.do("POST", base, map[string]any{"name": "Four", "destination_url": "https://example.com/4"})
	if r.Status != 402 || r.json(t)["code"] != "limit_reached" {
		t.Fatalf("quota: %d %s", r.Status, r.Body)
	}

	// Versions: immediate change, schedule + cancel (pro), restore.
	v2 := c.must(201, "POST", base+"/"+id+"/versions", map[string]any{"destination_url": "https://example.com/v2", "change_note": "spring"})
	if v2["status"] != "current" || v2["version_no"] != float64(2) {
		t.Fatalf("v2: %v", v2)
	}
	h.exec(`UPDATE workspaces SET plan_id = 'pro' WHERE id = $1`, ws)
	sched := c.must(201, "POST", base+"/"+id+"/versions", map[string]any{"destination_url": "https://example.com/v3", "effective_at": future})
	if sched["status"] != "scheduled" {
		t.Fatalf("scheduled: %v", sched)
	}
	got := c.must(200, "GET", base+"/"+id, nil)
	if dig(got, "scheduled_version", "id") != sched["id"] || dig(got, "current_version", "version_no") != float64(2) {
		t.Fatalf("detail: %v", got)
	}
	c.must(204, "DELETE", base+"/"+id+"/versions/"+sched["id"].(string), nil)
	c.must(409, "DELETE", base+"/"+id+"/versions/"+v2["id"].(string), nil)
	versions := c.must(200, "GET", base+"/"+id+"/versions", nil)["data"].([]any)
	first := versions[len(versions)-1].(map[string]any)
	restored := c.must(201, "POST", base+"/"+id+"/versions/"+first["id"].(string)+"/restore", nil)
	if restored["destination_url"] != "https://example.com/menu" || restored["restored_from"] != first["id"] {
		t.Fatalf("restore: %v", restored)
	}

	// Settings: expiry window, password (never echoed), tags and folder.
	folder := c.must(201, "POST", "/v1/workspaces/"+ws+"/folders", map[string]any{"name": "Print"})
	tag := c.must(201, "POST", "/v1/workspaces/"+ws+"/tags", map[string]any{"name": "Q3", "color": "#FF0000"})
	upd := c.must(200, "PATCH", base+"/"+id, map[string]any{"name": "Menu (EN)", "password": "secret", "scan_limit": 100,
		"folder_id": folder["id"], "tag_ids": []any{tag["id"]}, "expires_at": future})
	if upd["has_password"] != true || upd["scan_limit"] != float64(100) || upd["folder_id"] != folder["id"] || len(upd["tags"].([]any)) != 1 {
		t.Fatalf("update: %v", upd)
	}
	if strings.Contains(string(c.do("GET", base+"/"+id, nil).Body), "argon2") {
		t.Fatal("password hash leaked")
	}
	c.must(200, "PATCH", base+"/"+id, map[string]any{"password": nil, "scan_limit": nil})
	c.must(409, "DELETE", "/v1/workspaces/"+ws+"/folders/"+folder["id"].(string), nil) // not empty
	list := c.must(200, "GET", base+"?folder_id="+folder["id"].(string), nil)["data"].([]any)
	if len(list) != 1 {
		t.Fatalf("folder filter: %d", len(list))
	}

	// Pagination.
	pg := c.must(200, "GET", base+"?limit=2", nil)
	if len(pg["data"].([]any)) != 2 || pg["next_cursor"] == nil {
		t.Fatalf("page 1: %v", pg)
	}
	pg2 := c.must(200, "GET", base+"?limit=2&cursor="+pg["next_cursor"].(string), nil)
	if len(pg2["data"].([]any)) == 0 {
		t.Fatal("page 2 empty")
	}

	// Lifecycle: pause, resolve preview, delete, restore.
	c.must(200, "POST", base+"/"+id+"/pause", nil)
	pv := c.must(200, "POST", base+"/"+id+"/resolve-preview", map[string]any{})
	if pv["outcome"] != "paused" {
		t.Fatalf("preview paused: %v", pv)
	}
	c.must(200, "POST", base+"/"+id+"/resume", nil)
	c.must(204, "DELETE", base+"/"+id, nil)
	c.must(404, "GET", base+"/"+id, nil)
	c.must(200, "POST", base+"/"+id+"/restore", nil)
	c.must(200, "GET", base+"/"+id, nil)
}

func TestRulesPreviewAndIdempotency(t *testing.T) {
	h := newHarness(t)
	c, ws, _ := h.register("biz")
	h.exec(`UPDATE workspaces SET plan_id = 'business' WHERE id = $1`, ws)
	base := "/v1/workspaces/" + ws + "/qr-codes"
	body := map[string]any{"name": "Geo", "destination_url": "https://example.com/global",
		"utm": map[string]any{"source": "qr", "medium": "print"},
		"rules": []any{map[string]any{"id": "in", "enabled": true, "destination_url": "https://example.in/",
			"when": map[string]any{"all": []any{map[string]any{"field": "country", "op": "in", "value": []any{"IN"}}}}}}}
	a := c.do("POST", base, body, "Idempotency-Key", "create-geo-0001")
	b := c.do("POST", base, body, "Idempotency-Key", "create-geo-0001")
	if a.Status != 201 || b.Status != 201 || string(a.Body) != string(b.Body) || b.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("idempotent replay: %d %d %s", a.Status, b.Status, b.Header)
	}
	body["name"] = "Other"
	if r := c.do("POST", base, body, "Idempotency-Key", "create-geo-0001"); r.Status != 422 {
		t.Fatalf("key reuse: %d %s", r.Status, r.Body)
	}
	id := a.json(t)["id"].(string)
	in := c.must(200, "POST", base+"/"+id+"/resolve-preview", map[string]any{"country": "in"})
	if in["rule_id"] != "in" || !strings.HasPrefix(in["destination"].(string), "https://example.in/?utm_") {
		t.Fatalf("preview IN: %v", in)
	}
	us := c.must(200, "POST", base+"/"+id+"/resolve-preview", map[string]any{"country": "US"})
	if us["rule_id"] != "" || !strings.Contains(us["destination"].(string), "utm_source=qr") {
		t.Fatalf("preview US: %v", us)
	}
	// Split weights must sum to 100.
	c.must(422, "POST", base, map[string]any{"name": "AB", "destination_url": "https://example.com",
		"rules": []any{map[string]any{"enabled": true, "split": []any{
			map[string]any{"weight": 60, "destination_url": "https://a.example.com"},
			map[string]any{"weight": 30, "destination_url": "https://b.example.com"}}}}})
}

func TestTenantIsolationAndTeams(t *testing.T) {
	h := newHarness(t)
	alice, aws, _ := h.register("alice")
	bob, bws, _ := h.register("bob")
	q := alice.must(201, "POST", "/v1/workspaces/"+aws+"/qr-codes", map[string]any{"name": "Secret", "destination_url": "https://example.com/s"})
	id := q["id"].(string)

	// Bob cannot see Alice's workspace, nor reach her code through his own workspace (IDOR).
	bob.must(404, "GET", "/v1/workspaces/"+aws, nil)
	bob.must(404, "GET", "/v1/workspaces/"+bws+"/qr-codes/"+id, nil)
	bob.must(404, "GET", "/v1/workspaces/"+bws+"/qr-codes/"+id+"/versions", nil)
	bob.must(404, "POST", "/v1/workspaces/"+bws+"/qr-codes/"+id+"/versions", map[string]any{"destination_url": "https://evil.example"})

	// Seats: free plan has one seat, so inviting needs an upgrade.
	r := alice.do("POST", "/v1/workspaces/"+aws+"/invites", map[string]any{"email": "carol@example.com", "role": "editor"})
	if r.Status != 402 {
		t.Fatalf("seat gate: %d %s", r.Status, r.Body)
	}
	h.exec(`UPDATE workspaces SET plan_id = 'business' WHERE id = $1`, aws)
	carol, _, cem := h.register("carol")
	inv := alice.must(201, "POST", "/v1/workspaces/"+aws+"/invites", map[string]any{"email": cem, "role": "editor"})
	token := inv["accept_url"].(string)[strings.LastIndex(inv["accept_url"].(string), "/")+1:]
	pv := h.client().must(200, "GET", "/v1/invites/"+token, nil)
	if pv["status"] != "pending" || strings.Contains(pv["email"].(string), cem[1:6]) {
		t.Fatalf("preview: %v", pv)
	}
	bob.must(403, "POST", "/v1/invites/"+token+"/accept", nil) // wrong account
	carol.must(200, "POST", "/v1/invites/"+token+"/accept", nil)
	carol.must(410, "POST", "/v1/invites/"+token+"/accept", nil)

	// Carol (editor) works with codes but cannot manage members or delete.
	carol.must(200, "GET", "/v1/workspaces/"+aws+"/qr-codes/"+id, nil)
	carol.must(201, "POST", "/v1/workspaces/"+aws+"/qr-codes/"+id+"/versions", map[string]any{"destination_url": "https://example.com/c"})
	carol.must(403, "DELETE", "/v1/workspaces/"+aws+"/qr-codes/"+id, nil)
	carol.must(403, "POST", "/v1/workspaces/"+aws+"/invites", map[string]any{"email": "x@example.com"})
	members := alice.must(200, "GET", "/v1/workspaces/"+aws+"/members", nil)["data"].([]any)
	if len(members) != 2 {
		t.Fatalf("members: %v", members)
	}
	var carolID string
	for _, m := range members {
		if m.(map[string]any)["email"] == cem {
			carolID = m.(map[string]any)["user_id"].(string)
		}
	}
	alice.must(200, "PATCH", "/v1/workspaces/"+aws+"/members/"+carolID, map[string]any{"role": "analyst"})
	carol.must(403, "POST", "/v1/workspaces/"+aws+"/qr-codes/"+id+"/versions", map[string]any{"destination_url": "https://example.com/d"})
	carol.must(204, "POST", "/v1/workspaces/"+aws+"/leave", nil)
	carol.must(404, "GET", "/v1/workspaces/"+aws, nil)

	// Audit trail recorded the story.
	var n int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE workspace_id = $1 AND action IN
		('qr.created','invite.created','invite.accepted','member.role.changed','member.left','qr.version.created')`, aws).Scan(&n); err != nil || n < 6 {
		t.Fatalf("audit rows: %d %v", n, err)
	}
}

// TestRowLevelSecurity checks the database itself refuses cross-tenant access when the
// transaction is pinned to a workspace (as the API does), using a non-superuser role.
func TestRowLevelSecurity(t *testing.T) {
	h := newHarness(t)
	_, aws, _ := h.register("rls-a")
	bobC, bws, _ := h.register("rls-b")
	bobC.must(201, "POST", "/v1/workspaces/"+bws+"/qr-codes", map[string]any{"name": "B", "destination_url": "https://example.com/b"})
	ctx := context.Background()
	h.exec(`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'qrit_rls_test') THEN
		CREATE ROLE qrit_rls_test NOLOGIN; END IF; END $$`)
	h.exec(`GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO qrit_rls_test`)
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	mustExec := func(sql string, args ...any) {
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	mustExec(`SET LOCAL ROLE qrit_rls_test`)
	mustExec(`SELECT set_config('app.workspace_id', $1, true)`, aws)
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM qr_codes WHERE workspace_id = $1`, bws).Scan(&n); err != nil || n != 0 {
		t.Fatalf("cross-tenant read visible: %d %v", n, err)
	}
	_, err = tx.Exec(ctx, `UPDATE qr_codes SET name = 'pwned' WHERE workspace_id = $1`, bws)
	if err != nil {
		t.Fatal(err)
	}
	var name string
	_ = h.pool.QueryRow(ctx, `SELECT name FROM qr_codes WHERE workspace_id = $1`, bws).Scan(&name)
	if name != "B" {
		t.Fatal("cross-tenant write succeeded")
	}
	_, err = tx.Exec(ctx, `INSERT INTO folders (id, workspace_id, name) VALUES (gen_random_uuid(), $1, 'x')`, bws)
	if err == nil || !strings.Contains(err.Error(), "row-level security") {
		t.Fatalf("cross-tenant insert: %v", err)
	}
	_ = pgx.ErrNoRows
}
