package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/jobs"
	"github.com/its-aryansingh/qrit/services/internal/netutil"
	"github.com/its-aryansingh/qrit/services/internal/redirect"
	"github.com/its-aryansingh/qrit/services/internal/stdwebhook"
	"github.com/its-aryansingh/qrit/services/internal/worker"
)

// join registers a user and adds them to ws with role through an invite.
func (h *harness) join(owner *client, ws, name, role string) (*client, string, string) {
	h.t.Helper()
	c, _, em := h.register(name)
	inv := owner.must(201, "POST", "/v1/workspaces/"+ws+"/invites", map[string]any{"email": em, "role": role})
	tok := inv["accept_url"].(string)[strings.LastIndex(inv["accept_url"].(string), "/")+1:]
	c.must(200, "POST", "/v1/invites/"+tok+"/accept", nil)
	return c, em, h.userID(em)
}

func (h *harness) userID(email string) string {
	h.t.Helper()
	var id string
	if err := h.pool.QueryRow(context.Background(), `SELECT id::text FROM users WHERE email = $1`, email).Scan(&id); err != nil {
		h.t.Fatalf("user %s: %v", email, err)
	}
	return id
}

func ids(list []any) map[string]bool {
	out := map[string]bool{}
	for _, x := range list {
		out[x.(map[string]any)["id"].(string)] = true
	}
	return out
}

// ---- roles, groups, bindings, inspector, access review -----------------------------------

func TestAccessGovernance(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	const pw = "correct horse battery"
	alice, aws, _ := h.register("govowner")
	orgID := orgOf(t, alice)
	ob := "/v1/orgs/" + orgID
	wb := "/v1/workspaces/" + aws

	// Custom roles need the Business plan.
	agencyRole := map[string]any{"key": "agency_editor", "name": "Agency Editor",
		"permissions": []any{"qr.read", "qr.create", "qr.update", "qr.destination.update"}}
	alice.must(402, "POST", ob+"/roles", agencyRole)
	h.setPlan(aws, "business")
	alice.must(409, "POST", ob+"/roles", map[string]any{"key": "admin", "name": "Admin 2", "permissions": []any{"qr.read"}})
	alice.must(422, "POST", ob+"/roles", map[string]any{"key": "x1", "name": "Wild", "permissions": []any{"*"}})
	alice.must(422, "POST", ob+"/roles", map[string]any{"key": "x2", "name": "Bad", "permissions": []any{"qr.fly"}})
	alice.must(422, "POST", ob+"/roles", map[string]any{"key": "Bad Key", "name": "Bad", "permissions": []any{"qr.read"}})
	role := alice.must(201, "POST", ob+"/roles", agencyRole)
	roleID := role["id"].(string)
	if !strings.Contains(strings.Join(anyStrings(role["permissions"]), ","), "workspace.read") || role["system"] != false {
		t.Fatalf("role: %v", role)
	}
	alice.must(409, "POST", ob+"/roles", agencyRole)
	roles := alice.must(200, "GET", ob+"/roles", nil)["data"].([]any)
	if len(roles) != 6 || roles[0].(map[string]any)["key"] != "owner" {
		t.Fatalf("roles: %v", roles)
	}
	adminRoleID := roles[1].(map[string]any)["id"].(string)
	alice.must(403, "PATCH", ob+"/roles/"+adminRoleID, map[string]any{"name": "Boss"})
	alice.must(403, "DELETE", ob+"/roles/"+adminRoleID, nil)
	alice.must(200, "GET", wb+"/roles", nil)

	// Folder tree Campaigns/Diwali plus codes inside and outside it.
	camp := alice.must(201, "POST", wb+"/folders", map[string]any{"name": "Campaigns"})
	diwali := alice.must(201, "POST", wb+"/folders", map[string]any{"name": "Diwali", "parent_id": camp["id"]})
	dID := diwali["id"].(string)
	inCode := alice.must(201, "POST", wb+"/qr-codes", map[string]any{"name": "Diwali promo", "destination_url": "https://example.com/d",
		"folder_id": dID})["id"].(string)
	outCode := alice.must(201, "POST", wb+"/qr-codes", map[string]any{"name": "Root code", "destination_url": "https://example.com/r"})["id"].(string)

	// Bob belongs to the organisation through another workspace only.
	ws2 := alice.must(201, "POST", ob+"/workspaces", map[string]any{"name": "Other"})["id"].(string)
	bob, _, bobID := h.join(alice, ws2, "agencybob", "analyst")
	bob.must(404, "GET", wb, nil)

	// Group "Agency" bound to the custom role on the Diwali folder.
	alice.must(422, "POST", ob+"/groups", map[string]any{"display_name": "Agency", "user_ids": []any{uuid.NewString()}})
	grp := alice.must(201, "POST", ob+"/groups", map[string]any{"display_name": "Agency", "user_ids": []any{bobID}})
	gid := grp["id"].(string)
	if grp["member_count"] != float64(1) || grp["source"] != "manual" {
		t.Fatalf("group: %v", grp)
	}
	alice.must(409, "POST", ob+"/groups", map[string]any{"display_name": "Agency"})
	alice.must(409, "POST", wb+"/role-bindings", map[string]any{"principal_type": "user", "principal_id": bobID, "role_key": "editor"})
	alice.must(422, "POST", wb+"/role-bindings", map[string]any{"principal_type": "group", "principal_id": gid, "role_id": roleID,
		"folder_id": uuid.NewString()})
	b := alice.must(201, "POST", wb+"/role-bindings", map[string]any{"principal_type": "group", "principal_id": gid, "role_id": roleID,
		"folder_id": dID})
	if b["scope_type"] != "folder" || b["principal_name"] != "Agency" || b["role_key"] != "agency_editor" || b["managed_by"] != "binding" {
		t.Fatalf("binding: %v", b)
	}
	alice.must(409, "POST", wb+"/role-bindings", map[string]any{"principal_type": "group", "principal_id": gid, "role_id": roleID, "folder_id": dID})

	// Bob now sees the workspace but only the Diwali subtree, and can create only there.
	wsList := bob.must(200, "GET", "/v1/workspaces", nil)["data"].([]any)
	if !ids(wsList)[aws] {
		t.Fatalf("workspace via group binding not listed: %v", wsList)
	}
	codes := bob.must(200, "GET", wb+"/qr-codes", nil)["data"].([]any)
	if seen := ids(codes); len(codes) != 1 || !seen[inCode] {
		t.Fatalf("folder-scoped list: %v", codes)
	}
	bob.must(200, "GET", wb+"/qr-codes/"+inCode, nil)
	bob.must(404, "GET", wb+"/qr-codes/"+outCode, nil)
	bob.must(201, "POST", wb+"/qr-codes", map[string]any{"name": "Bob's", "destination_url": "https://example.com/b", "folder_id": dID})
	bob.must(403, "POST", wb+"/qr-codes", map[string]any{"name": "Nope", "destination_url": "https://example.com/n"})
	bob.must(403, "GET", wb+"/role-bindings", nil)

	// The inspector explains why.
	ex := alice.must(200, "GET", wb+"/access/explain?user_id="+bobID+"&permission=qr.create&qr_id="+inCode, nil)
	via := ex["via"].([]any)
	if ex["allowed"] != true || len(via) != 1 || via[0].(map[string]any)["kind"] != "group_binding" ||
		via[0].(map[string]any)["group_name"] != "Agency" || via[0].(map[string]any)["folder_name"] != "Diwali" {
		t.Fatalf("explain inside: %v", ex)
	}
	if ex := alice.must(200, "GET", wb+"/access/explain?user_id="+bobID+"&permission=qr.create&qr_id="+outCode, nil); ex["allowed"] != false {
		t.Fatalf("explain outside: %v", ex)
	}
	if ex := alice.must(200, "GET", wb+"/access/explain?user_id="+bobID+"&permission=qr.delete&qr_id="+inCode, nil); ex["allowed"] != false {
		t.Fatalf("explain missing permission: %v", ex)
	}
	aliceID := h.userID(dig(alice.must(200, "GET", "/v1/me", nil), "user", "email").(string))
	if ex := alice.must(200, "GET", wb+"/access/explain?user_id="+aliceID+"&permission=billing.manage", nil); ex["allowed"] != true {
		t.Fatalf("explain owner: %v", ex)
	}

	// No privilege escalation: a member with role.manage via a custom role can grant only
	// what they hold.
	binder := alice.must(201, "POST", ob+"/roles", map[string]any{"key": "binder", "name": "Binder", "permissions": []any{"role.manage", "qr.read"}})
	dave, _, daveID := h.join(alice, aws, "dave", "editor")
	alice.must(201, "POST", wb+"/role-bindings", map[string]any{"principal_type": "user", "principal_id": daveID, "role_id": binder["id"]})
	if r := dave.do("POST", wb+"/role-bindings", map[string]any{"principal_type": "group", "principal_id": gid, "role_key": "admin"}); r.Status != 403 ||
		r.json(t)["code"] != "privilege_escalation" {
		t.Fatalf("escalation: %d %s", r.Status, r.Body)
	}
	dave.must(403, "POST", wb+"/role-bindings", map[string]any{"principal_type": "group", "principal_id": gid, "role_key": "owner"})
	analystBinding := dave.must(201, "POST", wb+"/role-bindings", map[string]any{"principal_type": "group", "principal_id": gid, "role_key": "analyst"})
	dave.must(204, "DELETE", wb+"/role-bindings/"+analystBinding["id"].(string), nil)
	list := alice.must(200, "GET", wb+"/role-bindings", nil)["data"].([]any)
	var membership string
	for _, x := range list {
		m := x.(map[string]any)
		if m["principal_id"] == daveID && m["role_key"] == "editor" {
			membership = m["id"].(string)
			if m["managed_by"] != "membership" {
				t.Fatalf("membership binding: %v", m)
			}
		}
	}
	alice.must(409, "DELETE", wb+"/role-bindings/"+membership, nil)

	// Roles in use can't be deleted; editing a role changes access at once.
	alice.must(409, "DELETE", ob+"/roles/"+roleID, nil)
	alice.must(200, "PATCH", ob+"/roles/"+roleID, map[string]any{"permissions": []any{"qr.read"}})
	bob.must(403, "POST", wb+"/qr-codes", map[string]any{"name": "x", "destination_url": "https://example.com/x", "folder_id": dID})
	// Leaving the group removes access immediately (cache invalidated).
	alice.must(204, "DELETE", ob+"/groups/"+gid+"/members/"+bobID, nil)
	bob.must(404, "GET", wb, nil)

	// IdP-managed groups are read-only here.
	var ssoGroup string
	_ = h.pool.QueryRow(ctx, `INSERT INTO groups (id, org_id, display_name, source) VALUES (gen_random_uuid(), $1, 'Okta Everyone', 'sso')
		RETURNING id::text`, orgID).Scan(&ssoGroup)
	if r := alice.do("POST", ob+"/groups/"+ssoGroup+"/members", map[string]any{"user_ids": []any{bobID}}); r.json(t)["code"] != "group_managed_externally" {
		t.Fatalf("sso group: %s", r.Body)
	}
	alice.must(409, "PATCH", ob+"/groups/"+ssoGroup, map[string]any{"display_name": "x"})
	g := alice.must(200, "GET", ob+"/groups/"+gid, nil)
	if g["member_count"] != float64(0) || len(g["members"].([]any)) != 0 {
		t.Fatalf("group detail: %v", g)
	}

	// Deleting a group removes its bindings; then the role can go.
	alice.must(204, "DELETE", ob+"/groups/"+gid, nil)
	alice.must(204, "DELETE", ob+"/roles/"+roleID, nil)

	// Access review export (step-up) and completion record.
	alice.must(401, "GET", ob+"/access-review", nil)
	alice.must(200, "POST", "/v1/auth/step-up", map[string]any{"password": pw})
	rev := alice.must(200, "GET", ob+"/access-review", nil)
	rows := rev["rows"].([]any)
	var sawOwner, sawDave bool
	for _, x := range rows {
		m := x.(map[string]any)
		if m["role"] == "owner" && m["scope"] == "organisation" {
			sawOwner = true
		}
		if m["user_id"] == daveID && m["role"] == "binder" && m["via"] == "direct" {
			sawDave = true
		}
	}
	if !sawOwner || !sawDave || rev["last_review"] != nil {
		t.Fatalf("review rows: %v", rev)
	}
	r := alice.do("GET", ob+"/access-review?format=csv", nil)
	if r.Status != 200 || !strings.HasPrefix(string(r.Body), "user_id,email,name,org_role") || !strings.Contains(r.Header.Get("Content-Disposition"), "access-review-") {
		t.Fatalf("csv: %d %s", r.Status, r.Body)
	}
	alice.must(200, "POST", ob+"/access-review/complete", map[string]any{"note": "Q3 review: removed Agency group"})
	if rev := alice.must(200, "GET", ob+"/access-review", nil); dig(rev, "last_review", "note") != "Q3 review: removed Agency group" {
		t.Fatalf("last review: %v", rev["last_review"])
	}
	var n int
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE org_id = $1 AND action IN ('role.created','role.updated','role.deleted',
		'group.created','group.member_removed','group.deleted','access_review.exported','access_review.completed')`, orgID).Scan(&n)
	_ = h.pool.QueryRow(ctx, `SELECT count(*) + $2 FROM audit_logs WHERE workspace_id = $1 AND action IN ('binding.created','binding.deleted')`, aws, n).Scan(&n)
	if n < 14 {
		t.Fatalf("governance audit entries: %d", n)
	}
}

// ---- approvals ----------------------------------------------------------------------------

func TestApprovals(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	const pw = "correct horse battery"
	alice, aws, aem := h.register("approwner")
	wb := "/v1/workspaces/" + aws
	h.setPlan(aws, "business")
	alice.must(200, "PUT", wb+"/policies", map[string]any{"approval_mode": "outside_allowlist", "allowed_destination_hosts": []any{"example.com"}})
	rita, rem, _ := h.join(alice, aws, "rita", "reviewer")
	ed, eem, _ := h.join(alice, aws, "ed", "editor")

	liveURL := func(codeID string) string {
		t.Helper()
		var dom uuid.UUID
		var sc string
		if err := h.pool.QueryRow(ctx, `SELECT domain_id, short_code FROM qr_codes WHERE id = $1`, codeID).Scan(&dom, &sc); err != nil {
			t.Fatal(err)
		}
		link, err := redirect.FetchLink(h.pool)(ctx, dom, sc)
		if err != nil {
			t.Fatal(err)
		}
		if link.Version == nil {
			return ""
		}
		return link.Version.URL
	}

	// Inside the allowlist: live at once.
	code := ed.must(201, "POST", wb+"/qr-codes", map[string]any{"name": "Menu", "destination_url": "https://example.com/menu"})
	cid := code["id"].(string)
	if liveURL(cid) != "https://example.com/menu" {
		t.Fatal("initial destination")
	}

	// Outside: held for approval, never served while pending.
	held := ed.must(202, "POST", wb+"/qr-codes/"+cid+"/versions", map[string]any{"destination_url": "https://other.test/x", "change_note": "new menu"})
	appr := held["approval"].(map[string]any)
	aid := appr["id"].(string)
	if appr["status"] != "pending" || anyStrings(appr["reasons"])[0] != "host_not_allowlisted" || dig(held, "version", "status") != "pending_approval" {
		t.Fatalf("held: %v", held)
	}
	if liveURL(cid) != "https://example.com/menu" {
		t.Fatal("pending version served")
	}
	det := ed.must(200, "GET", wb+"/qr-codes/"+cid, nil)
	if dig(det, "current_version", "destination_url") != "https://example.com/menu" || det["scheduled_version"] != nil {
		t.Fatalf("detail during approval: %v", det)
	}
	vers := ed.must(200, "GET", wb+"/qr-codes/"+cid+"/versions", nil)["data"].([]any)
	if vers[0].(map[string]any)["status"] != "pending_approval" {
		t.Fatalf("versions: %v", vers)
	}

	// Approvers are notified by the worker after commit.
	wd := &worker.Deps{Pool: h.pool, Mail: h.mail, AppBaseURL: "http://app.test"}
	to, err := wd.Approvers(ctx, uuid.MustParse(aid))
	if err != nil || strings.Join(to, ",") != strings.Join(sortedStrings(aem, rem), ",") {
		t.Fatalf("approvers: %v %v", to, err)
	}
	var payload []byte
	_ = h.pool.QueryRow(ctx, `SELECT payload FROM job_queue WHERE kind = 'approval.notify' ORDER BY id DESC LIMIT 1`).Scan(&payload)
	if err := wd.NotifyApprovers(ctx, &jobs.Job{Kind: "approval.notify", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if m, ok := h.mail.Last(rem); !ok || !strings.HasPrefix(m.Subject, "Approval needed: Menu") || !strings.Contains(m.Body, "https://other.test/x") {
		t.Fatalf("approval email: %+v", m)
	}
	if m, ok := h.mail.Last(eem); ok && strings.HasPrefix(m.Subject, "Approval needed") {
		t.Fatal("the requester must not be asked to approve")
	}

	// Inbox, four-eyes, validation.
	inbox := rita.must(200, "GET", "/v1/me/approvals", nil)
	if inbox["count"] != float64(1) {
		t.Fatalf("inbox: %v", inbox)
	}
	if ed.must(200, "GET", "/v1/me/approvals", nil)["count"] != float64(0) {
		t.Fatal("editor inbox")
	}
	ed.must(403, "POST", wb+"/approvals/"+aid+"/decisions", map[string]any{"decision": "approve"})
	rita.must(422, "POST", wb+"/approvals/"+aid+"/decisions", map[string]any{"decision": "reject"})
	rita.must(422, "POST", wb+"/approvals/"+aid+"/decisions", map[string]any{"decision": "maybe"})
	view := rita.must(200, "GET", wb+"/approvals/"+aid, nil)
	if view["can_decide"] != true || dig(view, "current_version", "destination_url") != "https://example.com/menu" ||
		len(view["versions"].([]any)) != 1 || view["note"] != "new menu" {
		t.Fatalf("approval view: %v", view)
	}
	done := rita.must(200, "POST", wb+"/approvals/"+aid+"/decisions", map[string]any{"decision": "approve", "comment": "ok"})
	if done["status"] != "approved" || len(done["decisions"].([]any)) != 1 {
		t.Fatalf("decided: %v", done)
	}
	if liveURL(cid) != "https://other.test/x" {
		t.Fatal("approved version not live")
	}
	rita.must(409, "POST", wb+"/approvals/"+aid+"/decisions", map[string]any{"decision": "approve"})

	// The owner is subject to four-eyes too.
	own := alice.must(202, "POST", wb+"/qr-codes/"+cid+"/versions", map[string]any{"destination_url": "https://third.test/"})
	oid := dig(own, "approval", "id").(string)
	if r := alice.do("POST", wb+"/approvals/"+oid+"/decisions", map[string]any{"decision": "approve"}); r.Status != 403 || r.json(t)["code"] != "self_approval" {
		t.Fatalf("self approval: %d %s", r.Status, r.Body)
	}
	rej := rita.must(200, "POST", wb+"/approvals/"+oid+"/decisions", map[string]any{"decision": "reject", "comment": "not our domain"})
	if rej["status"] != "rejected" || liveURL(cid) != "https://other.test/x" {
		t.Fatalf("rejected: %v", rej)
	}

	// Two approvals required.
	alice.must(200, "PUT", wb+"/policies", map[string]any{"approvals_required": 2})
	two := dig(ed.must(202, "POST", wb+"/qr-codes/"+cid+"/versions", map[string]any{"destination_url": "https://four.test/"}), "approval", "id").(string)
	if s := rita.must(200, "POST", wb+"/approvals/"+two+"/decisions", map[string]any{"decision": "approve"}); s["status"] != "pending" || s["approvals"] != float64(1) {
		t.Fatalf("first of two: %v", s)
	}
	if liveURL(cid) != "https://other.test/x" {
		t.Fatal("published after one of two approvals")
	}
	alice.must(200, "POST", wb+"/approvals/"+two+"/decisions", map[string]any{"decision": "approve"})
	if liveURL(cid) != "https://four.test/" {
		t.Fatal("two approvals")
	}
	alice.must(200, "PUT", wb+"/policies", map[string]any{"approvals_required": 1})

	// Cancel by the requester.
	cn := dig(ed.must(202, "POST", wb+"/qr-codes/"+cid+"/versions", map[string]any{"destination_url": "https://five.test/"}), "approval", "id").(string)
	rita.must(403, "POST", wb+"/approvals/"+cn+"/cancel", nil)
	if c := ed.must(200, "POST", wb+"/approvals/"+cn+"/cancel", nil); c["status"] != "cancelled" ||
		dig(c["versions"].([]any)[0].(map[string]any), "status") != "cancelled" {
		t.Fatalf("cancel: %v", c)
	}

	// Break-glass override by the org owner with step-up and a reason.
	ov := dig(ed.must(202, "POST", wb+"/qr-codes/"+cid+"/versions", map[string]any{"destination_url": "https://six.test/"}), "approval", "id").(string)
	alice.must(401, "POST", wb+"/approvals/"+ov+"/override", map[string]any{"reason": "printer deadline in one hour"})
	alice.must(200, "POST", "/v1/auth/step-up", map[string]any{"password": pw})
	alice.must(422, "POST", wb+"/approvals/"+ov+"/override", map[string]any{"reason": "urgent"})
	rita.must(200, "POST", "/v1/auth/step-up", map[string]any{"password": pw})
	rita.must(403, "POST", wb+"/approvals/"+ov+"/override", map[string]any{"reason": "printer deadline in one hour"})
	o := alice.must(200, "POST", wb+"/approvals/"+ov+"/override", map[string]any{"reason": "printer deadline in one hour"})
	if o["status"] != "approved" || o["override_reason"] != "printer deadline in one hour" || liveURL(cid) != "https://six.test/" {
		t.Fatalf("override: %v", o)
	}

	// Expiry.
	ex := dig(ed.must(202, "POST", wb+"/qr-codes/"+cid+"/versions", map[string]any{"destination_url": "https://seven.test/"}), "approval", "id").(string)
	h.exec(`UPDATE approval_requests SET expires_at = now() - interval '1 minute' WHERE id = $1`, ex)
	if l := rita.must(200, "GET", wb+"/approvals?status=expired", nil)["data"].([]any); len(l) != 1 {
		t.Fatalf("expired list: %v", l)
	}
	rita.must(409, "POST", wb+"/approvals/"+ex+"/decisions", map[string]any{"decision": "approve"})
	if n, err := wd.ExpireApprovals(ctx); err != nil || n != 1 {
		t.Fatalf("expire: %d %v", n, err)
	}
	var st, vst string
	_ = h.pool.QueryRow(ctx, `SELECT ar.status, v.approval_status FROM approval_requests ar JOIN qr_versions v ON v.approval_request_id = ar.id
		WHERE ar.id = $1`, ex).Scan(&st, &vst)
	if st != "expired" || vst != "cancelled" || liveURL(cid) != "https://six.test/" {
		t.Fatalf("expired: %s %s", st, vst)
	}

	// all_changes: new codes wait for approval before going live.
	alice.must(200, "PUT", wb+"/policies", map[string]any{"approval_mode": "all_changes"})
	nc := ed.must(201, "POST", wb+"/qr-codes", map[string]any{"name": "Launch", "destination_url": "https://example.com/launch"})
	ncID := dig(nc, "qr_code", "id").(string)
	if dig(nc, "pending_approval", "approval", "status") != "pending" || liveURL(ncID) != "" {
		t.Fatalf("new code pending: %v", nc)
	}
	nid := dig(nc, "pending_approval", "approval", "id").(string)
	if l := rita.must(200, "GET", wb+"/approvals?status=pending", nil)["data"].([]any); len(l) != 1 || l[0].(map[string]any)["kind"] != "create" {
		t.Fatalf("pending list: %v", l)
	}
	rita.must(200, "POST", wb+"/approvals/"+nid+"/decisions", map[string]any{"decision": "approve"})
	if liveURL(ncID) != "https://example.com/launch" {
		t.Fatal("approved new code not live")
	}
	if all := alice.must(200, "GET", wb+"/approvals?status=all", nil)["data"].([]any); len(all) != 7 {
		t.Fatalf("all approvals: %d", len(all))
	}

	var trail []string
	rows, _ := h.pool.Query(ctx, `SELECT action FROM audit_logs WHERE workspace_id = $1 AND action IN
		('approval.requested','approval.decided','qr.version.activated','approval.cancelled','approval.overridden','approval.expired')
		ORDER BY id`, aws)
	for rows.Next() {
		var a string
		_ = rows.Scan(&a)
		trail = append(trail, a)
	}
	rows.Close()
	joined := strings.Join(trail, ",")
	for _, want := range []string{"approval.requested,approval.decided,qr.version.activated", "approval.cancelled", "approval.overridden", "approval.expired"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("audit trail missing %s: %s", want, joined)
		}
	}
}

func sortedStrings(a ...string) []string {
	out := append([]string{}, a...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// ---- audit streams -------------------------------------------------------------------------

func TestAuditStreams(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	const pw = "correct horse battery"
	alice, aws, _ := h.register("streamowner")
	orgID := orgOf(t, alice)
	ob := "/v1/orgs/" + orgID

	var mu sync.Mutex
	var seqs []int64
	var bodies int
	fail := false
	var secret string
	recv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		if err := stdwebhook.Verify(secret, r.Header, body, 5*time.Minute, time.Now()); err != nil {
			w.WriteHeader(401)
			return
		}
		if fail {
			w.WriteHeader(500)
			return
		}
		var b struct {
			Events []struct {
				Seq int64 `json:"seq"`
			} `json:"events"`
		}
		_ = json.Unmarshal(body, &b)
		for _, e := range b.Events {
			seqs = append(seqs, e.Seq)
		}
		bodies++
		w.WriteHeader(204)
	}))
	defer recv.Close()

	alice.must(200, "POST", "/v1/auth/step-up", map[string]any{"password": pw})
	alice.must(402, "POST", ob+"/audit-streams", map[string]any{"kind": "webhook", "config": map[string]any{"url": recv.URL}})
	h.setPlan(aws, "enterprise")
	alice.must(422, "POST", ob+"/audit-streams", map[string]any{"kind": "syslog", "config": map[string]any{"url": recv.URL}})
	alice.must(422, "POST", ob+"/audit-streams", map[string]any{"kind": "splunk_hec", "config": map[string]any{"url": recv.URL}})
	wd := &worker.Deps{Pool: h.pool, Keyring: h.srv.Keyring()}
	if err := wd.SealAudit(ctx); err != nil {
		t.Fatal(err)
	}
	cr := alice.must(201, "POST", ob+"/audit-streams", map[string]any{"kind": "webhook", "label": "SIEM", "config": map[string]any{"url": recv.URL}})
	secret = cr["secret"].(string)
	sid := dig(cr, "stream", "id").(string)
	start := dig(cr, "stream", "cursor_seq").(float64)
	if !strings.HasPrefix(secret, "whsec_") || start != dig(cr, "stream", "latest_seq").(float64) {
		t.Fatalf("stream: %v", cr)
	}
	if strings.Contains(string(mustJSON(alice.must(200, "GET", ob+"/audit-streams", nil))), secret) {
		t.Fatal("secret must only be shown once")
	}

	// Test delivery.
	if res := alice.must(200, "POST", ob+"/audit-streams/"+sid+"/test", nil); res["ok"] != true {
		t.Fatalf("test: %v", res)
	}
	// New activity is sealed and streamed in seq order.
	for i := 0; i < 3; i++ {
		alice.must(200, "PATCH", ob, map[string]any{"name": "Stream Org " + itoa(i)})
	}
	if err := wd.SealAudit(ctx); err != nil {
		t.Fatal(err)
	}
	client := netutil.SafeHTTPClient(true, 10*time.Second)
	n, err := wd.DeliverAuditStreams(ctx, client, 5)
	if err != nil || n < 4 {
		t.Fatalf("delivered %d: %v", n, err)
	}
	mu.Lock()
	for i := 1; i < len(seqs); i++ {
		if seqs[i] != seqs[i-1]+1 {
			t.Fatalf("out of order or gap: %v", seqs)
		}
	}
	if seqs[0] != 0 || seqs[1] != int64(start)+1 {
		t.Fatalf("first delivered seq: %v (start %v)", seqs, start)
	}
	delivered := len(seqs)
	mu.Unlock()
	st := alice.must(200, "GET", ob+"/audit-streams", nil)["data"].([]any)[0].(map[string]any)
	if st["cursor_seq"] != st["latest_seq"] || st["last_delivered_at"] == nil {
		t.Fatalf("cursor: %v", st)
	}

	// Failures keep the cursor; after 24 h of errors the stream pauses and says so.
	mu.Lock()
	fail = true
	mu.Unlock()
	alice.must(200, "PATCH", ob, map[string]any{"name": "Stream Org X"})
	_ = wd.SealAudit(ctx)
	_, _ = wd.DeliverAuditStreams(ctx, client, 5)
	st = alice.must(200, "GET", ob+"/audit-streams", nil)["data"].([]any)[0].(map[string]any)
	if st["status"] != "active" || st["last_error"] == nil || st["failing_since"] == nil || st["cursor_seq"] == st["latest_seq"] {
		t.Fatalf("failing: %v", st)
	}
	h.exec(`UPDATE audit_streams SET failing_since = now() - interval '25 hours' WHERE id = $1`, sid)
	_, _ = wd.DeliverAuditStreams(ctx, client, 5)
	st = alice.must(200, "GET", ob+"/audit-streams", nil)["data"].([]any)[0].(map[string]any)
	var paused int
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE org_id = $1 AND action = 'audit_stream.paused'`, orgID).Scan(&paused)
	if st["status"] != "error" || paused != 1 {
		t.Fatalf("paused: %v %d", st, paused)
	}

	// Re-activation resumes from the cursor: nothing is lost.
	mu.Lock()
	fail = false
	mu.Unlock()
	alice.must(200, "PATCH", ob+"/audit-streams/"+sid, map[string]any{"status": "active"})
	_ = wd.SealAudit(ctx)
	if _, err := wd.DeliverAuditStreams(ctx, client, 5); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	for i := delivered + 1; i < len(seqs); i++ {
		if seqs[i] != seqs[i-1]+1 {
			t.Fatalf("gap after resume: %v", seqs)
		}
	}
	mu.Unlock()
	st = alice.must(200, "GET", ob+"/audit-streams", nil)["data"].([]any)[0].(map[string]any)
	if st["cursor_seq"] != st["latest_seq"] || st["last_error"] != nil {
		t.Fatalf("resumed: %v", st)
	}

	// Replay from the start.
	alice.must(200, "POST", ob+"/audit-streams/"+sid+"/replay", map[string]any{"from_seq": 1})
	mu.Lock()
	seqs = nil
	mu.Unlock()
	_, _ = wd.DeliverAuditStreams(ctx, client, 5)
	mu.Lock()
	if len(seqs) == 0 || seqs[0] != 1 {
		t.Fatalf("replay: %v", seqs)
	}
	mu.Unlock()
	alice.must(204, "DELETE", ob+"/audit-streams/"+sid, nil)
}
