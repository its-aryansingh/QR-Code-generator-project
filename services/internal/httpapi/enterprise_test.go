package httpapi

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/worker"
)

func gstinWithCheck(prefix14 string) string {
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	sum := 0
	for i := 0; i < 14; i++ {
		f := 1
		if i%2 == 1 {
			f = 2
		}
		p := strings.IndexByte(chars, prefix14[i]) * f
		sum += p/36 + p%36
	}
	return prefix14 + string(chars[(36-sum%36)%36])
}

func TestOrganisationsAndAccess(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	alice, aws, _ := h.register("orgowner")
	me := alice.must(200, "GET", "/v1/me", nil)
	orgs := me["organizations"].([]any)
	if len(orgs) != 1 || orgs[0].(map[string]any)["org_role"] != "org_owner" {
		t.Fatalf("personal org: %v", orgs)
	}
	orgID := orgs[0].(map[string]any)["id"].(string)
	wsInfo := alice.must(200, "GET", "/v1/workspaces/"+aws, nil)
	if dig(wsInfo, "workspace", "org_id") != orgID {
		t.Fatalf("workspace org: %v", wsInfo)
	}

	// Org profile with GSTIN validation.
	good := gstinWithCheck("09AAACQ1234F1Z")
	alice.must(422, "PATCH", "/v1/orgs/"+orgID, map[string]any{"gstin": "09AAACQ1234F1ZZ"[:14] + "0"})
	o := alice.must(200, "PATCH", "/v1/orgs/"+orgID, map[string]any{"name": "Acme India", "gstin": good, "billing_email": "Billing@Acme.example"})
	if o["gstin"] != good || o["billing_email"] != "billing@acme.example" {
		t.Fatalf("org update: %v", o)
	}

	// Workspaces per org follow the org plan.
	alice.must(402, "POST", "/v1/orgs/"+orgID+"/workspaces", map[string]any{"name": "Marketing"})
	h.setPlan(aws, "business")
	mk := alice.must(201, "POST", "/v1/orgs/"+orgID+"/workspaces", map[string]any{"name": "Marketing"})
	mws := mk["id"].(string)
	if len(alice.must(200, "GET", "/v1/orgs/"+orgID+"/workspaces", nil)["data"].([]any)) != 2 {
		t.Fatal("org workspaces")
	}

	// Invite Carol to the first workspace: she joins the org as a member.
	carol, _, cem := h.register("carol")
	inv := alice.must(201, "POST", "/v1/workspaces/"+aws+"/invites", map[string]any{"email": cem, "role": "editor"})
	tok := inv["accept_url"].(string)[strings.LastIndex(inv["accept_url"].(string), "/")+1:]
	carol.must(200, "POST", "/v1/invites/"+tok+"/accept", nil)
	carol.must(404, "GET", "/v1/workspaces/"+mws, nil) // not in Marketing
	members := alice.must(200, "GET", "/v1/orgs/"+orgID+"/members", nil)["data"].([]any)
	var carolID string
	for _, m := range members {
		mm := m.(map[string]any)
		if mm["email"] == cem {
			carolID = mm["user_id"].(string)
			if mm["org_role"] != "member" {
				t.Fatalf("carol org role: %v", mm)
			}
		}
	}
	carol.must(403, "GET", "/v1/orgs/"+orgID+"/members", nil)

	// Org admins have implicit admin on every workspace of the org.
	alice.must(200, "PATCH", "/v1/orgs/"+orgID+"/members/"+carolID, map[string]any{"org_role": "org_admin"})
	carol.must(200, "GET", "/v1/workspaces/"+mws, nil)
	carol.must(201, "POST", "/v1/workspaces/"+mws+"/qr-codes", map[string]any{"name": "Promo", "destination_url": "https://example.com/p"})
	list := carol.must(200, "GET", "/v1/workspaces", nil)["data"].([]any)
	if len(list) != 3 { // her own personal workspace + both workspaces of Acme
		t.Fatalf("admin sees org workspaces: %d", len(list))
	}
	carol.must(403, "PATCH", "/v1/orgs/"+orgID+"/members/"+carolID, map[string]any{"org_role": "member"}) // not herself

	// Suspension cuts access at once; reactivation restores it.
	alice.must(200, "PATCH", "/v1/orgs/"+orgID+"/members/"+carolID, map[string]any{"status": "suspended"})
	carol.must(404, "GET", "/v1/workspaces/"+aws, nil)
	carol.must(404, "GET", "/v1/orgs/"+orgID, nil)
	alice.must(200, "PATCH", "/v1/orgs/"+orgID+"/members/"+carolID, map[string]any{"status": "active"})
	carol.must(200, "GET", "/v1/workspaces/"+aws, nil)

	// Security policy: step-up, lock-out protection, then the gate.
	alice.must(401, "PUT", "/v1/orgs/"+orgID+"/security-policy", map[string]any{"require_mfa": false})
	alice.must(401, "POST", "/v1/auth/step-up", map[string]any{"password": "wrong password"})
	alice.must(200, "POST", "/v1/auth/step-up", map[string]any{"password": "correct horse battery"})
	alice.must(402, "PUT", "/v1/orgs/"+orgID+"/security-policy", map[string]any{"dashboard_ip_allowlist": []any{"127.0.0.0/8"}})
	h.setPlan(aws, "enterprise")
	alice.must(422, "PUT", "/v1/orgs/"+orgID+"/security-policy", map[string]any{"dashboard_ip_allowlist": []any{"10.0.0.0/8"}})
	pol := alice.must(200, "PUT", "/v1/orgs/"+orgID+"/security-policy", map[string]any{"dashboard_ip_allowlist": []any{"127.0.0.1"},
		"session_idle_minutes": 30, "invite_email_domains": []any{"acme.example", "example.com"}})
	if pol["dashboard_ip_allowlist"].([]any)[0] != "127.0.0.1/32" || pol["session_idle_minutes"] != float64(30) {
		t.Fatalf("policy: %v", pol)
	}
	alice.must(403, "POST", "/v1/workspaces/"+aws+"/invites", map[string]any{"email": "someone@gmail.com"})
	alice.must(409, "PUT", "/v1/orgs/"+orgID+"/security-policy", map[string]any{"enforce_sso": true})

	h.exec(`UPDATE org_security_policies SET dashboard_ip_allowlist = '{10.0.0.0/8}' WHERE org_id = $1`, orgID)
	h.srv.access.Invalidate(ctx)
	r := carol.do("GET", "/v1/workspaces/"+aws+"/qr-codes", nil)
	if r.Status != 403 || r.json(t)["code"] != "ip_not_allowed" {
		t.Fatalf("ip gate: %d %s", r.Status, r.Body)
	}
	h.exec(`UPDATE org_security_policies SET dashboard_ip_allowlist = '{}', require_mfa = true WHERE org_id = $1`, orgID)
	h.srv.access.Invalidate(ctx)
	if r := carol.do("GET", "/v1/workspaces/"+aws, nil); r.Status != 403 || r.json(t)["code"] != "mfa_required" {
		t.Fatalf("mfa gate: %d %s", r.Status, r.Body)
	}
	h.exec(`UPDATE org_security_policies SET require_mfa = false, session_idle_minutes = 5 WHERE org_id = $1`, orgID)
	h.exec(`UPDATE sessions SET last_used_at = now() - interval '20 minutes' WHERE user_id = $1`, carolID)
	h.srv.access.Invalidate(ctx)
	h.srv.sessions.evictAll()
	if r := carol.do("GET", "/v1/workspaces/"+aws, nil); r.Status != 401 || r.json(t)["code"] != "session_expired" {
		t.Fatalf("idle gate: %d %s", r.Status, r.Body)
	}
	carol.must(401, "GET", "/v1/me", nil) // the session was revoked
	h.exec(`UPDATE org_security_policies SET session_idle_minutes = 0 WHERE org_id = $1`, orgID)
	h.srv.access.Invalidate(ctx)

	// Contract overrides the plan.
	h.exec(`INSERT INTO contracts (id, org_id, name, starts_on, ends_on, billing_interval, currency, amount_minor, seats,
		limits_override, status) VALUES ($1, $2, 'MSA 2026', current_date - 1, current_date + 365, 'year', 'INR', 1200000000, 250,
		'{"dynamic_codes": 250000, "disabled_features": ["gs1"]}', 'active')`, uuid.New(), orgID)
	h.srv.plans.Invalidate()
	ent := alice.must(200, "GET", "/v1/orgs/"+orgID+"/entitlements", nil)
	if dig(ent, "limits", "dynamic_codes") != float64(250000) || dig(ent, "limits", "seats") != float64(250) ||
		strings.Contains(strings.Join(anyStrings(ent["features"]), ","), "gs1") {
		t.Fatalf("contract entitlements: %v", ent)
	}

	// Workspace policy: host allow/block lists and required templates.
	alice.must(200, "PUT", "/v1/workspaces/"+aws+"/policies", map[string]any{"allowed_destination_hosts": []any{"*.example.com"},
		"blocked_destination_hosts": []any{"bad.example.com"}, "require_template": true})
	base := "/v1/workspaces/" + aws + "/qr-codes"
	if r := alice.do("POST", base, map[string]any{"name": "x", "destination_url": "https://www.other.test/"}); r.Status != 422 ||
		r.json(t)["code"] != "host_not_allowed" {
		t.Fatalf("allowlist: %d %s", r.Status, r.Body)
	}
	if r := alice.do("POST", base, map[string]any{"name": "x", "destination_url": "https://bad.example.com/"}); r.json(t)["code"] != "host_blocked" {
		t.Fatalf("blocklist: %s", r.Body)
	}
	if r := alice.do("POST", base, map[string]any{"name": "x", "destination_url": "https://www.example.com/"}); r.Status != 201 {
		t.Fatalf("owner bypasses the template requirement: %d %s", r.Status, r.Body)
	}
	ed, _, eem := h.register("editor")
	inv2 := alice.must(201, "POST", "/v1/workspaces/"+aws+"/invites", map[string]any{"email": eem, "role": "editor"})
	tok2 := inv2["accept_url"].(string)[strings.LastIndex(inv2["accept_url"].(string), "/")+1:]
	ed.must(200, "POST", "/v1/invites/"+tok2+"/accept", nil)
	if r := ed.do("POST", base, map[string]any{"name": "x", "destination_url": "https://www.example.com/"}); r.json(t)["code"] != "template_required" {
		t.Fatalf("editor must use a template: %s", r.Body)
	}
	h.exec(`UPDATE workspace_policies SET require_template = false WHERE workspace_id = $1`, aws)
	wsPolicyCache.Purge()
	alice.must(201, "POST", base, map[string]any{"name": "ok", "destination_url": "https://www.example.com/"})
}

func anyStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func TestTamperEvidentAudit(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	c, ws, _ := h.register("auditor")
	h.setPlan(ws, "business")
	for i := 0; i < 3; i++ {
		c.must(201, "POST", "/v1/workspaces/"+ws+"/tags", map[string]any{"name": "t" + string(rune('a'+i))})
		c.must(201, "POST", "/v1/workspaces/"+ws+"/qr-codes", map[string]any{"name: ": "", "name": "Q" + string(rune('a'+i)),
			"destination_url": "https://example.com/" + string(rune('a'+i))})
	}
	var orgID string
	_ = h.pool.QueryRow(ctx, `SELECT org_id FROM workspaces WHERE id = $1`, ws).Scan(&orgID)
	d := &worker.Deps{Pool: h.pool}
	if err := d.SealAudit(ctx); err != nil {
		t.Fatal(err)
	}
	logs := c.must(200, "GET", "/v1/orgs/"+orgID+"/audit-logs?limit=2", nil)
	entries := logs["data"].([]any)
	if len(entries) != 2 || logs["next_cursor"] == nil || entries[0].(map[string]any)["hash"] == nil {
		t.Fatalf("audit page: %v", logs)
	}
	page2 := c.must(200, "GET", "/v1/orgs/"+orgID+"/audit-logs?cursor="+logs["next_cursor"].(string)+"&action=qr.*", nil)
	for _, e := range page2["data"].([]any) {
		if !strings.HasPrefix(e.(map[string]any)["action"].(string), "qr.") {
			t.Fatal("action filter")
		}
	}
	c.must(200, "GET", "/v1/workspaces/"+ws+"/audit-logs?target_type=qr_code", nil)
	v := c.must(200, "GET", "/v1/orgs/"+orgID+"/audit-logs/verify", nil)
	if v["status"] != "intact" || v["sealed_entries"].(float64) < 6 || v["unsealed_entries"] != float64(0) {
		t.Fatalf("verify: %v", v)
	}
	// Append-only: the database refuses edits and deletes.
	if _, err := h.pool.Exec(ctx, `UPDATE audit_logs SET action = 'x' WHERE org_id = $1`, orgID); err == nil ||
		!strings.Contains(err.Error(), "append-only") {
		t.Fatalf("update allowed: %v", err)
	}
	if _, err := h.pool.Exec(ctx, `DELETE FROM audit_logs WHERE org_id = $1`, orgID); err == nil {
		t.Fatal("delete allowed")
	}
	// A superuser who bypasses the trigger is still caught by the chain.
	h.exec(`ALTER TABLE audit_logs DISABLE TRIGGER audit_logs_append_only`)
	h.exec(`UPDATE audit_logs SET changes = '{"name":"forged"}' WHERE org_id = $1 AND seq = 2`, orgID)
	h.exec(`ALTER TABLE audit_logs ENABLE TRIGGER audit_logs_append_only`)
	v = c.must(200, "GET", "/v1/orgs/"+orgID+"/audit-logs/verify", nil)
	if v["status"] != "broken" || v["broken_at_seq"] != float64(2) {
		t.Fatalf("tamper not detected: %v", v)
	}
	// Export needs step-up, and is itself audited.
	c.must(401, "GET", "/v1/orgs/"+orgID+"/audit-logs/export", nil)
	c.must(200, "POST", "/v1/auth/step-up", map[string]any{"password": "correct horse battery"})
	exp := c.do("GET", "/v1/orgs/"+orgID+"/audit-logs/export?format=csv", nil)
	if exp.Status != 200 || !strings.HasPrefix(string(exp.Body), "id,seq,created_at") {
		t.Fatalf("export: %d", exp.Status)
	}
	var exported int
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE org_id = $1 AND action = 'audit.exported'`, orgID).Scan(&exported)
	if exported != 1 {
		t.Fatal("export not audited")
	}
	// Free plans cannot browse the audit log.
	h.setPlan(ws, "free")
	c.must(402, "GET", "/v1/orgs/"+orgID+"/audit-logs", nil)
}

func TestEnvelopeAndFlags(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, ws1, _ := h.register("keys1")
	_, ws2, _ := h.register("keys2")
	var o1, o2 uuid.UUID
	_ = h.pool.QueryRow(ctx, `SELECT org_id FROM workspaces WHERE id = $1`, ws1).Scan(&o1)
	_ = h.pool.QueryRow(ctx, `SELECT org_id FROM workspaces WHERE id = $1`, ws2).Scan(&o2)
	k := h.srv.keyring
	ct, err := k.Encrypt(ctx, o1, []byte("asha@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if pt, err := k.Decrypt(ctx, o1, ct); err != nil || string(pt) != "asha@example.com" {
		t.Fatalf("roundtrip: %v", err)
	}
	if _, err := k.Decrypt(ctx, o2, ct); err == nil {
		t.Fatal("ciphertext opened under another organisation")
	}
	if err := k.Rotate(ctx, o1); err != nil {
		t.Fatal(err)
	}
	ct2, _ := k.Encrypt(ctx, o1, []byte("x"))
	if ct2[3] == ct[3] {
		t.Fatal("rotation did not change the key id")
	}
	if pt, err := k.Decrypt(ctx, o1, ct); err != nil || string(pt) != "asha@example.com" {
		t.Fatal("old ciphertext unreadable after rotation")
	}
	if string(k.BlindIndex(o1, " Asha@Example.com")) != string(k.BlindIndex(o1, "asha@example.com")) ||
		string(k.BlindIndex(o1, "asha@example.com")) == string(k.BlindIndex(o2, "asha@example.com")) {
		t.Fatal("blind index")
	}
	key := "test.flag." + time.Now().Format("150405.000000")
	_ = h.srv.flags.Set(ctx, key, nil, false, nil)
	_ = h.srv.flags.Set(ctx, key, &o1, true, nil)
	if !h.srv.flags.Enabled(ctx, key, o1) || h.srv.flags.Enabled(ctx, key, o2) {
		t.Fatal("flag override")
	}
}
