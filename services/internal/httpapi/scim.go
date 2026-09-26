package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/audit"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/org"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
)

// SCIM 2.0 service provider (RFC 7643/7644) with the quirks of Microsoft Entra ID and Okta:
// case-insensitive ops and attribute names, "True"/"False" strings for booleans, PATCH on
// Groups answered with 204, values stored exactly as sent, ListResponse for zero results.

const (
	scimUserSchema  = "urn:ietf:params:scim:schemas:core:2.0:User"
	scimGroupSchema = "urn:ietf:params:scim:schemas:core:2.0:Group"
	scimEntSchema   = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
	scimListSchema  = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	scimErrSchema   = "urn:ietf:params:scim:api:messages:2.0:Error"
	scimPatchSchema = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
)

type scimDir struct {
	ID                uuid.UUID
	OrgID             uuid.UUID
	DeprovisionAction string
}

type scimCtxKey struct{}

func scimDirectory(r *http.Request) scimDir {
	d, _ := r.Context().Value(scimCtxKey{}).(scimDir)
	return d
}

func scimError(w http.ResponseWriter, status int, scimType, detail string) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	body := map[string]any{"schemas": []string{scimErrSchema}, "status": strconv.Itoa(status), "detail": detail}
	if scimType != "" {
		body["scimType"] = scimType
	}
	_ = json.NewEncoder(w).Encode(body)
}

func scimJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// scimRoutes mounts /scim/v2 (bearer-token authenticated, independent of user sessions).
func (s *Server) scimRoutes(r chi.Router) {
	r.Route("/scim/v2", func(r chi.Router) {
		r.Use(s.scimAuth)
		r.Get("/ServiceProviderConfig", s.scimServiceProviderConfig)
		r.Get("/ResourceTypes", s.scimResourceTypes)
		r.Get("/Schemas", s.scimSchemas)
		r.Get("/Users", s.scimListUsers)
		r.Post("/Users", s.scimCreateUser)
		r.Get("/Users/{id}", s.scimGetUser)
		r.Put("/Users/{id}", s.scimReplaceUser)
		r.Patch("/Users/{id}", s.scimPatchUser)
		r.Delete("/Users/{id}", s.scimDeleteUser)
		r.Get("/Groups", s.scimListGroups)
		r.Post("/Groups", s.scimCreateGroup)
		r.Get("/Groups/{id}", s.scimGetGroup)
		r.Put("/Groups/{id}", s.scimReplaceGroup)
		r.Patch("/Groups/{id}", s.scimPatchGroup)
		r.Delete("/Groups/{id}", s.scimDeleteGroup)
	})
}

func (s *Server) scimAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			scimError(w, http.StatusUnauthorized, "", "bearer token required")
			return
		}
		sum := sha256.Sum256([]byte(strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))))
		var d scimDir
		var status string
		err := s.pool.QueryRow(r.Context(), `SELECT sd.id, sd.org_id, sd.deprovision_action, sd.status FROM scim_directories sd
			JOIN organizations o ON o.id = sd.org_id AND o.deleted_at IS NULL WHERE sd.token_hash = $1`, sum[:]).
			Scan(&d.ID, &d.OrgID, &d.DeprovisionAction, &status)
		if err != nil || status != "active" {
			scimError(w, http.StatusUnauthorized, "", "invalid token")
			return
		}
		if ok, _, _, _ := s.limiter.Allow(r.Context(), "rl:scim:"+d.ID.String(), 600, time.Minute); !ok {
			w.Header().Set("Retry-After", "60")
			scimError(w, http.StatusTooManyRequests, "", "rate limit exceeded")
			return
		}
		_, _ = s.pool.Exec(r.Context(), `UPDATE scim_directories SET last_request_at = now() WHERE id = $1
			AND (last_request_at IS NULL OR last_request_at < now() - interval '1 minute')`, d.ID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), scimCtxKey{}, d)))
	})
}

func (s *Server) scimBase() string { return strings.TrimRight(s.cfg.APIPublicURL, "/") + "/scim/v2" }

func (s *Server) scimServiceProviderConfig(w http.ResponseWriter, r *http.Request) {
	scimJSON(w, http.StatusOK, map[string]any{
		"schemas":          []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"documentationUri": "https://qrit.io/docs/scim",
		"patch":            map[string]any{"supported": true},
		"bulk":             map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
		"filter":           map[string]any{"supported": true, "maxResults": 200},
		"changePassword":   map[string]any{"supported": false},
		"sort":             map[string]any{"supported": false},
		"etag":             map[string]any{"supported": true},
		"authenticationSchemes": []any{map[string]any{"type": "oauthbearertoken", "name": "OAuth Bearer Token",
			"description": "Directory token issued in QRit organisation settings", "primary": true}},
		"meta": map[string]any{"resourceType": "ServiceProviderConfig", "location": s.scimBase() + "/ServiceProviderConfig"},
	})
}

func (s *Server) scimResourceTypes(w http.ResponseWriter, r *http.Request) {
	scimJSON(w, http.StatusOK, map[string]any{"schemas": []string{scimListSchema}, "totalResults": 2, "startIndex": 1, "itemsPerPage": 2,
		"Resources": []any{
			map[string]any{"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"}, "id": "User", "name": "User",
				"endpoint": "/Users", "schema": scimUserSchema,
				"schemaExtensions": []any{map[string]any{"schema": scimEntSchema, "required": false}},
				"meta": map[string]any{"resourceType": "ResourceType", "location": s.scimBase() + "/ResourceTypes/User"}},
			map[string]any{"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"}, "id": "Group", "name": "Group",
				"endpoint": "/Groups", "schema": scimGroupSchema,
				"meta": map[string]any{"resourceType": "ResourceType", "location": s.scimBase() + "/ResourceTypes/Group"}},
		}})
}

func (s *Server) scimSchemas(w http.ResponseWriter, r *http.Request) {
	attr := func(name, typ string, multi, req bool) map[string]any {
		return map[string]any{"name": name, "type": typ, "multiValued": multi, "required": req, "mutability": "readWrite",
			"returned": "default", "uniqueness": "none", "caseExact": false}
	}
	user := map[string]any{"id": scimUserSchema, "name": "User", "attributes": []any{
		attr("userName", "string", false, true), attr("name", "complex", false, false), attr("displayName", "string", false, false),
		attr("emails", "complex", true, false), attr("active", "boolean", false, false), attr("externalId", "string", false, false),
		attr("title", "string", false, false)}}
	group := map[string]any{"id": scimGroupSchema, "name": "Group", "attributes": []any{
		attr("displayName", "string", false, true), attr("members", "complex", true, false)}}
	ent := map[string]any{"id": scimEntSchema, "name": "EnterpriseUser", "attributes": []any{
		attr("department", "string", false, false), attr("employeeNumber", "string", false, false)}}
	scimJSON(w, http.StatusOK, map[string]any{"schemas": []string{scimListSchema}, "totalResults": 3, "startIndex": 1,
		"itemsPerPage": 3, "Resources": []any{user, group, ent}})
}

// ---- filter ---------------------------------------------------------------------

type scimClause struct{ attr, value string }

var clauseRe = regexp.MustCompile(`(?i)^\s*([a-z0-9_.:]+(?:\[[^\]]*\])?(?:\.[a-z]+)?)\s+eq\s+"((?:[^"\\]|\\.)*)"\s*$`)

func parseSCIMFilter(f string) ([]scimClause, error) {
	if strings.TrimSpace(f) == "" {
		return nil, nil
	}
	parts := regexp.MustCompile(`(?i)\s+and\s+`).Split(f, -1)
	var out []scimClause
	for _, p := range parts {
		m := clauseRe.FindStringSubmatch(p)
		if m == nil {
			return nil, fmt.Errorf("unsupported filter %q", p)
		}
		a := strings.ToLower(strings.ReplaceAll(m[1], " ", ""))
		out = append(out, scimClause{attr: a, value: strings.ReplaceAll(m[2], `\"`, `"`)})
	}
	return out, nil
}

func pageParams(r *http.Request) (int, int) {
	start, _ := strconv.Atoi(r.URL.Query().Get("startIndex"))
	if start < 1 {
		start = 1
	}
	count, err := strconv.Atoi(r.URL.Query().Get("count"))
	if err != nil || count < 0 {
		count = 100
	}
	if count > 200 {
		count = 200
	}
	return start, count
}

// ---- users ---------------------------------------------------------------------

type scimUser struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	ExternalID *string
	UserName   string
	Active     bool
	Resource   map[string]any
	Version    int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

const scimUserCols = `id, user_id, external_id, user_name, active, resource, version, created_at, updated_at`

func scanSCIMUser(row pgx.Row) (scimUser, error) {
	var u scimUser
	var res []byte
	err := row.Scan(&u.ID, &u.UserID, &u.ExternalID, &u.UserName, &u.Active, &res, &u.Version, &u.CreatedAt, &u.UpdatedAt)
	_ = json.Unmarshal(res, &u.Resource)
	return u, err
}

func (s *Server) scimUserJSON(u scimUser) map[string]any {
	out := map[string]any{}
	for k, v := range u.Resource {
		out[k] = v
	}
	if _, ok := out["schemas"]; !ok {
		out["schemas"] = []string{scimUserSchema}
	}
	out["id"] = u.ID.String()
	out["userName"] = u.UserName
	out["active"] = u.Active
	if u.ExternalID != nil {
		out["externalId"] = *u.ExternalID
	}
	delete(out, "password")
	out["meta"] = map[string]any{"resourceType": "User", "created": u.CreatedAt.UTC().Format(time.RFC3339),
		"lastModified": u.UpdatedAt.UTC().Format(time.RFC3339), "location": s.scimBase() + "/Users/" + u.ID.String(),
		"version": fmt.Sprintf(`W/"%d"`, u.Version)}
	return out
}

func (s *Server) scimListUsers(w http.ResponseWriter, r *http.Request) {
	d := scimDirectory(r)
	clauses, err := parseSCIMFilter(r.URL.Query().Get("filter"))
	if err != nil {
		scimError(w, http.StatusBadRequest, "invalidFilter", err.Error())
		return
	}
	where := []string{"directory_id = $1"}
	args := []any{d.ID}
	for _, c := range clauses {
		args = append(args, c.value)
		n := "$" + strconv.Itoa(len(args))
		switch c.attr {
		case "username":
			where = append(where, "lower(user_name) = lower("+n+")")
		case "externalid":
			where = append(where, "external_id = "+n)
		case "id":
			where = append(where, "id::text = "+n)
		case "emails.value", `emails[typeeq"work"].value`, `emails[primaryeqtrue].value`:
			where = append(where, `EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(resource->'emails','[]')) e WHERE lower(e->>'value') = lower(`+n+`))`)
		default:
			scimError(w, http.StatusBadRequest, "invalidFilter", "unsupported filter attribute "+c.attr)
			return
		}
	}
	start, count := pageParams(r)
	var total int
	_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM scim_users WHERE `+strings.Join(where, " AND "), args...).Scan(&total)
	args = append(args, count, start-1)
	rows, err := s.pool.Query(r.Context(), `SELECT `+scimUserCols+` FROM scim_users WHERE `+strings.Join(where, " AND ")+
		fmt.Sprintf(` ORDER BY created_at, id LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		scimError(w, http.StatusInternalServerError, "", "query failed")
		return
	}
	defer rows.Close()
	res := []any{}
	for rows.Next() {
		if u, err := scanSCIMUser(rows); err == nil {
			res = append(res, s.scimUserJSON(u))
		}
	}
	scimJSON(w, http.StatusOK, map[string]any{"schemas": []string{scimListSchema}, "totalResults": total, "startIndex": start,
		"itemsPerPage": len(res), "Resources": res})
}

// userFields extracts what we act on from a SCIM user representation.
func userFields(res map[string]any) (userName, email, given, family, display string, externalID *string, active bool) {
	userName, _ = res["userName"].(string)
	active = true
	if v, ok := res["active"]; ok {
		active = scimBool(v, true)
	}
	if v, ok := res["externalId"].(string); ok && v != "" {
		externalID = &v
	}
	if n, ok := res["name"].(map[string]any); ok {
		given, _ = n["givenName"].(string)
		family, _ = n["familyName"].(string)
	}
	display, _ = res["displayName"].(string)
	if emails, ok := res["emails"].([]any); ok {
		for _, e := range emails {
			m, _ := e.(map[string]any)
			v, _ := m["value"].(string)
			if v == "" {
				continue
			}
			if email == "" || scimBool(m["primary"], false) || m["type"] == "work" {
				email = v
			}
		}
	}
	if email == "" && strings.Contains(userName, "@") {
		email = userName
	}
	return
}

func scimBool(v any, def bool) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(x, "true")
	}
	return def
}

func (s *Server) readSCIMBody(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	var m map[string]any
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		scimError(w, http.StatusBadRequest, "invalidSyntax", "malformed JSON")
		return nil, false
	}
	return m, true
}

func (s *Server) scimCreateUser(w http.ResponseWriter, r *http.Request) {
	d := scimDirectory(r)
	res, ok := s.readSCIMBody(w, r)
	if !ok {
		return
	}
	userName, email, given, family, display, externalID, active := userFields(res)
	emailAddr, valid := validEmail(email)
	if userName == "" || !valid {
		scimError(w, http.StatusBadRequest, "invalidValue", "userName and a valid email are required")
		return
	}
	var su scimUser
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended('scim:' || $1::text, 0))`, d.ID); err != nil {
			return err
		}
		var exists bool
		_ = tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM scim_users WHERE directory_id = $1 AND lower(user_name) = lower($2))`,
			d.ID, userName).Scan(&exists)
		if exists {
			return errSCIMConflict
		}
		userID, managed, err := s.scimResolveUser(r.Context(), q, tx, d, emailAddr, given, family, display)
		if err != nil {
			return err
		}
		status := "active"
		if !active {
			status = "suspended"
		}
		source := "scim"
		if !managed {
			source = "invite"
		}
		if _, err := tx.Exec(r.Context(), `INSERT INTO org_members (org_id, user_id, org_role, status, source) VALUES ($1, $2, 'member', $3, $4)
			ON CONFLICT (org_id, user_id) DO UPDATE SET status = CASE WHEN org_members.org_role = 'org_owner' THEN org_members.status
			ELSE EXCLUDED.status END, updated_at = now()`, d.OrgID, userID, status, source); err != nil {
			return err
		}
		b, _ := json.Marshal(res)
		su, err = scanSCIMUser(tx.QueryRow(r.Context(), `INSERT INTO scim_users (id, directory_id, user_id, external_id, user_name, active, resource)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+scimUserCols, idgen.New(), d.ID, userID, externalID, userName, active, b))
		if err != nil {
			if pgCode(err) == sqlUniqueViolation {
				return errSCIMConflict
			}
			return err
		}
		return s.aud().Record(r.Context(), q, scimAudit(r, d, "scim.user.provisioned", &userID, map[string]any{"user_name": userName}))
	})
	if s.scimFail(w, err) {
		return
	}
	s.access.Invalidate(r.Context())
	w.Header().Set("Location", s.scimBase()+"/Users/"+su.ID.String())
	w.Header().Set("ETag", fmt.Sprintf(`W/"%d"`, su.Version))
	scimJSON(w, http.StatusCreated, s.scimUserJSON(su))
}

var (
	errSCIMConflict   = errors.New("scim: uniqueness")
	errSCIMNotFound   = errors.New("scim: not found")
	errSCIMPrecond    = errors.New("scim: precondition")
	errSCIMBadRequest = errors.New("scim: bad request")
)

func (s *Server) scimFail(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, errSCIMConflict):
		scimError(w, http.StatusConflict, "uniqueness", "a resource with this identifier already exists")
	case errors.Is(err, errSCIMNotFound):
		scimError(w, http.StatusNotFound, "", "resource not found")
	case errors.Is(err, errSCIMPrecond):
		scimError(w, http.StatusPreconditionFailed, "", "version mismatch")
	case errors.Is(err, errSCIMBadRequest):
		scimError(w, http.StatusBadRequest, "invalidValue", err.Error())
	default:
		slogIf(err, "scim")
		scimError(w, http.StatusInternalServerError, "", "internal error")
	}
	return true
}

// scimResolveUser finds or creates the account for an email. Existing accounts are linked
// only when the organisation has verified the email's domain; otherwise creation fails with
// a uniqueness error rather than taking over someone else's account.
func (s *Server) scimResolveUser(ctx context.Context, q *dbgen.Queries, tx pgx.Tx, d scimDir, email, given, family, display string) (uuid.UUID, bool, error) {
	u, err := q.GetUserByEmail(ctx, email)
	if err == nil {
		var verified, member bool
		domain := email[strings.LastIndex(email, "@")+1:]
		_ = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM org_domains WHERE org_id = $1 AND domain = $2 AND verified_at IS NOT NULL)`,
			d.OrgID, domain).Scan(&verified)
		_ = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM org_members WHERE org_id = $1 AND user_id = $2)`, d.OrgID, u.ID).Scan(&member)
		if !verified && !member {
			return uuid.Nil, false, errSCIMConflict
		}
		var source string
		_ = tx.QueryRow(ctx, `SELECT source FROM org_members WHERE org_id = $1 AND user_id = $2`, d.OrgID, u.ID).Scan(&source)
		return u.ID, source == "scim" || source == "sso_jit", nil
	}
	if !isNotFound(err) {
		return uuid.Nil, false, err
	}
	name := strings.TrimSpace(given + " " + family)
	if name == "" {
		name = display
	}
	if name == "" {
		name = email[:strings.Index(email, "@")]
	}
	nu, err := q.CreateUser(ctx, dbgen.CreateUserParams{ID: idgen.New(), Email: email, Name: name, Locale: "en", Timezone: "UTC"})
	if err != nil {
		return uuid.Nil, false, err
	}
	return nu.ID, true, q.MarkEmailVerified(ctx, nu.ID)
}

func scimAudit(r *http.Request, d scimDir, action string, target *uuid.UUID, changes map[string]any) audit.Entry {
	e := auditUserEntry(r, uuid.Nil, action)
	e.ActorType, e.ActorID = "system", nil
	e.OrgID, e.TargetType, e.TargetID = &d.OrgID, "user", target
	if changes == nil {
		changes = map[string]any{}
	}
	changes["via"] = "scim"
	changes["directory_id"] = d.ID.String()
	e.Changes = changes
	return e
}

func (s *Server) loadSCIMUser(ctx context.Context, db pgx.Tx, d scimDir, id string, lock bool) (scimUser, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return scimUser{}, errSCIMNotFound
	}
	sql := `SELECT ` + scimUserCols + ` FROM scim_users WHERE id = $1 AND directory_id = $2`
	if lock {
		sql += ` FOR UPDATE`
	}
	var u scimUser
	if db != nil {
		u, err = scanSCIMUser(db.QueryRow(ctx, sql, uid, d.ID))
	} else {
		u, err = scanSCIMUser(s.pool.QueryRow(ctx, sql, uid, d.ID))
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return u, errSCIMNotFound
	}
	return u, err
}

func (s *Server) scimGetUser(w http.ResponseWriter, r *http.Request) {
	u, err := s.loadSCIMUser(r.Context(), nil, scimDirectory(r), chi.URLParam(r, "id"), false)
	if s.scimFail(w, err) {
		return
	}
	w.Header().Set("ETag", fmt.Sprintf(`W/"%d"`, u.Version))
	scimJSON(w, http.StatusOK, s.scimUserJSON(u))
}

func ifMatch(r *http.Request, version int) error {
	if h := r.Header.Get("If-Match"); h != "" && h != "*" && h != fmt.Sprintf(`W/"%d"`, version) {
		return errSCIMPrecond
	}
	return nil
}

// applySCIMUser stores the new representation and propagates active/name/email.
func (s *Server) applySCIMUser(r *http.Request, q *dbgen.Queries, tx pgx.Tx, d scimDir, u scimUser, res map[string]any) (scimUser, error) {
	ctx := r.Context()
	userName, email, given, family, display, externalID, active := userFields(res)
	if userName == "" {
		userName = u.UserName
		res["userName"] = userName
	}
	b, _ := json.Marshal(res)
	nu, err := scanSCIMUser(tx.QueryRow(ctx, `UPDATE scim_users SET user_name = $2, external_id = $3, active = $4, resource = $5,
		version = version + 1, updated_at = now() WHERE id = $1 RETURNING `+scimUserCols, u.ID, userName, externalID, active, b))
	if err != nil {
		if pgCode(err) == sqlUniqueViolation {
			return nu, errSCIMConflict
		}
		return nu, err
	}
	var source, role, status string
	_ = tx.QueryRow(ctx, `SELECT source, org_role, status FROM org_members WHERE org_id = $1 AND user_id = $2`, d.OrgID, u.UserID).
		Scan(&source, &role, &status)
	managed := source == "scim" || source == "sso_jit"
	if role != org.RoleOwner {
		want := "active"
		if !active {
			want = "suspended"
		}
		if want != status {
			if _, err := tx.Exec(ctx, `UPDATE org_members SET status = $3, updated_at = now() WHERE org_id = $1 AND user_id = $2`,
				d.OrgID, u.UserID, want); err != nil {
				return nu, err
			}
			action := "scim.user.reactivated"
			if want == "suspended" {
				action = "member.deprovisioned"
				if managed {
					if err := q.RevokeUserSessions(ctx, u.UserID); err != nil {
						return nu, err
					}
				}
			}
			if err := s.aud().Record(ctx, q, scimAudit(r, d, action, &u.UserID, nil)); err != nil {
				return nu, err
			}
		}
	}
	if managed {
		name := strings.TrimSpace(given + " " + family)
		if name == "" {
			name = display
		}
		if name != "" {
			if _, err := tx.Exec(ctx, `UPDATE users SET name = $2, updated_at = now() WHERE id = $1`, u.UserID, name); err != nil {
				return nu, err
			}
		}
		if e, ok := validEmail(email); ok {
			if _, err := tx.Exec(ctx, `UPDATE users SET email = $2, updated_at = now() WHERE id = $1 AND email <> $2
				AND NOT EXISTS (SELECT 1 FROM users x WHERE x.email = $2)`, u.UserID, e); err != nil {
				return nu, err
			}
		}
	}
	return nu, nil
}

func (s *Server) scimReplaceUser(w http.ResponseWriter, r *http.Request) {
	d := scimDirectory(r)
	res, ok := s.readSCIMBody(w, r)
	if !ok {
		return
	}
	var out scimUser
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		u, err := s.loadSCIMUser(r.Context(), tx, d, chi.URLParam(r, "id"), true)
		if err != nil {
			return err
		}
		if err := ifMatch(r, u.Version); err != nil {
			return err
		}
		out, err = s.applySCIMUser(r, q, tx, d, u, res)
		return err
	})
	if s.scimFail(w, err) {
		return
	}
	s.afterSCIMChange(r)
	w.Header().Set("ETag", fmt.Sprintf(`W/"%d"`, out.Version))
	scimJSON(w, http.StatusOK, s.scimUserJSON(out))
}

type patchOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

type patchReq struct {
	Schemas    []string  `json:"schemas"`
	Operations []patchOp `json:"Operations"`
}

var emailTypePath = regexp.MustCompile(`(?i)^emails\[type eq "([a-z]+)"\]\.value$`)

// applyUserPatch mutates a SCIM user map according to RFC 7644 §3.5.2 (the subset IdPs use).
func applyUserPatch(res map[string]any, ops []patchOp) error {
	for _, op := range ops {
		kind := strings.ToLower(op.Op)
		if kind != "add" && kind != "replace" && kind != "remove" {
			return fmt.Errorf("%w: unsupported op %q", errSCIMBadRequest, op.Op)
		}
		var val any
		if len(op.Value) > 0 {
			if err := json.Unmarshal(op.Value, &val); err != nil {
				return fmt.Errorf("%w: invalid value", errSCIMBadRequest)
			}
		}
		path := strings.TrimSpace(op.Path)
		if path == "" {
			obj, ok := val.(map[string]any)
			if !ok {
				return fmt.Errorf("%w: a patch without path needs an object value", errSCIMBadRequest)
			}
			for k, v := range obj {
				setUserAttr(res, k, v, kind)
			}
			continue
		}
		if m := emailTypePath.FindStringSubmatch(path); m != nil {
			emails, _ := res["emails"].([]any)
			found := false
			for i, e := range emails {
				em, _ := e.(map[string]any)
				if strings.EqualFold(fmt.Sprint(em["type"]), m[1]) {
					if kind == "remove" {
						emails = append(emails[:i], emails[i+1:]...)
					} else {
						em["value"] = val
					}
					found = true
					break
				}
			}
			if !found && kind != "remove" {
				emails = append(emails, map[string]any{"type": m[1], "value": val, "primary": len(emails) == 0})
			}
			res["emails"] = emails
			continue
		}
		setUserAttr(res, path, val, kind)
	}
	return nil
}

func setUserAttr(res map[string]any, path string, val any, kind string) {
	// Enterprise extension attributes arrive either fully qualified or nested.
	if strings.HasPrefix(strings.ToLower(path), strings.ToLower(scimEntSchema)+":") {
		attr := path[len(scimEntSchema)+1:]
		ext, _ := res[scimEntSchema].(map[string]any)
		if ext == nil {
			ext = map[string]any{}
		}
		if kind == "remove" {
			delete(ext, attr)
		} else {
			ext[attr] = val
		}
		res[scimEntSchema] = ext
		return
	}
	canon := map[string]string{"active": "active", "username": "userName", "externalid": "externalId", "displayname": "displayName",
		"title": "title", "name.givenname": "name.givenName", "name.familyname": "name.familyName", "name.formatted": "name.formatted",
		"emails": "emails", "name": "name", "preferredlanguage": "preferredLanguage", "locale": "locale", "timezone": "timezone"}
	key, ok := canon[strings.ToLower(path)]
	if !ok {
		key = path
	}
	if key == "active" {
		val = scimBool(val, true)
	}
	if i := strings.Index(key, "."); i > 0 {
		parent, child := key[:i], key[i+1:]
		m, _ := res[parent].(map[string]any)
		if m == nil {
			m = map[string]any{}
		}
		if kind == "remove" {
			delete(m, child)
		} else {
			m[child] = val
		}
		res[parent] = m
		return
	}
	if kind == "remove" {
		delete(res, key)
		return
	}
	res[key] = val
}

func (s *Server) scimPatchUser(w http.ResponseWriter, r *http.Request) {
	d := scimDirectory(r)
	var req patchReq
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Operations) == 0 {
		scimError(w, http.StatusBadRequest, "invalidSyntax", "PatchOp with Operations required")
		return
	}
	var out scimUser
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		u, err := s.loadSCIMUser(r.Context(), tx, d, chi.URLParam(r, "id"), true)
		if err != nil {
			return err
		}
		if err := ifMatch(r, u.Version); err != nil {
			return err
		}
		res := u.Resource
		if res == nil {
			res = map[string]any{}
		}
		res["active"] = u.Active
		if err := applyUserPatch(res, req.Operations); err != nil {
			return err
		}
		out, err = s.applySCIMUser(r, q, tx, d, u, res)
		return err
	})
	if s.scimFail(w, err) {
		return
	}
	s.afterSCIMChange(r)
	w.Header().Set("ETag", fmt.Sprintf(`W/"%d"`, out.Version))
	scimJSON(w, http.StatusOK, s.scimUserJSON(out))
}

// DELETE /Users/{id}: deprovision per directory setting (suspend or remove from org).
func (s *Server) scimDeleteUser(w http.ResponseWriter, r *http.Request) {
	d := scimDirectory(r)
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		u, err := s.loadSCIMUser(r.Context(), tx, d, chi.URLParam(r, "id"), true)
		if err != nil {
			return err
		}
		var role, source string
		_ = tx.QueryRow(r.Context(), `SELECT org_role, source FROM org_members WHERE org_id = $1 AND user_id = $2`, d.OrgID, u.UserID).Scan(&role, &source)
		if role != org.RoleOwner {
			if d.DeprovisionAction == "remove" {
				if err := org.Remove(r.Context(), tx, d.OrgID, u.UserID); err != nil && !errors.Is(err, org.ErrNotMember) {
					return err
				}
			} else if _, err := tx.Exec(r.Context(), `UPDATE org_members SET status = 'deprovisioned', updated_at = now()
				WHERE org_id = $1 AND user_id = $2`, d.OrgID, u.UserID); err != nil {
				return err
			}
			if source == "scim" || source == "sso_jit" {
				if err := q.RevokeUserSessions(r.Context(), u.UserID); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(r.Context(), `DELETE FROM scim_users WHERE id = $1`, u.ID); err != nil {
			return err
		}
		return s.aud().Record(r.Context(), q, scimAudit(r, d, "member.deprovisioned", &u.UserID, map[string]any{"action": d.DeprovisionAction}))
	})
	if s.scimFail(w, err) {
		return
	}
	s.afterSCIMChange(r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) afterSCIMChange(r *http.Request) {
	s.access.Invalidate(r.Context())
	s.sessions.evictAll()
}

// ---- groups --------------------------------------------------------------------

type scimGroup struct {
	ID          uuid.UUID
	DisplayName string
	ExternalID  *string
	Version     int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (s *Server) loadSCIMGroup(ctx context.Context, db pgx.Tx, d scimDir, id string) (scimGroup, error) {
	gid, err := uuid.Parse(id)
	if err != nil {
		return scimGroup{}, errSCIMNotFound
	}
	var g scimGroup
	q := `SELECT id, display_name, external_id, version, created_at, updated_at FROM groups WHERE id = $1 AND directory_id = $2`
	var row pgx.Row
	if db != nil {
		row = db.QueryRow(ctx, q+` FOR UPDATE`, gid, d.ID)
	} else {
		row = s.pool.QueryRow(ctx, q, gid, d.ID)
	}
	err = row.Scan(&g.ID, &g.DisplayName, &g.ExternalID, &g.Version, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, errSCIMNotFound
	}
	return g, err
}

func (s *Server) scimGroupJSON(ctx context.Context, g scimGroup, withMembers bool) map[string]any {
	out := map[string]any{"schemas": []string{scimGroupSchema}, "id": g.ID.String(), "displayName": g.DisplayName,
		"meta": map[string]any{"resourceType": "Group", "created": g.CreatedAt.UTC().Format(time.RFC3339),
			"lastModified": g.UpdatedAt.UTC().Format(time.RFC3339), "location": s.scimBase() + "/Groups/" + g.ID.String(),
			"version": fmt.Sprintf(`W/"%d"`, g.Version)}}
	if g.ExternalID != nil {
		out["externalId"] = *g.ExternalID
	}
	if withMembers {
		members := []any{}
		rows, err := s.pool.Query(ctx, `SELECT su.id, su.user_name FROM group_members gm JOIN groups g ON g.id = gm.group_id
			JOIN scim_users su ON su.user_id = gm.user_id AND su.directory_id = g.directory_id WHERE gm.group_id = $1 ORDER BY su.user_name`, g.ID)
		if err == nil {
			for rows.Next() {
				var id uuid.UUID
				var name string
				if rows.Scan(&id, &name) == nil {
					members = append(members, map[string]any{"value": id.String(), "display": name,
						"$ref": s.scimBase() + "/Users/" + id.String()})
				}
			}
			rows.Close()
		}
		out["members"] = members
	}
	return out
}

func (s *Server) scimListGroups(w http.ResponseWriter, r *http.Request) {
	d := scimDirectory(r)
	clauses, err := parseSCIMFilter(r.URL.Query().Get("filter"))
	if err != nil {
		scimError(w, http.StatusBadRequest, "invalidFilter", err.Error())
		return
	}
	where := []string{"directory_id = $1"}
	args := []any{d.ID}
	for _, c := range clauses {
		args = append(args, c.value)
		n := "$" + strconv.Itoa(len(args))
		switch c.attr {
		case "displayname":
			where = append(where, "display_name = "+n)
		case "externalid":
			where = append(where, "external_id = "+n)
		case "id":
			where = append(where, "id::text = "+n)
		default:
			scimError(w, http.StatusBadRequest, "invalidFilter", "unsupported filter attribute "+c.attr)
			return
		}
	}
	withMembers := !strings.Contains(strings.ToLower(r.URL.Query().Get("excludedAttributes")), "members")
	start, count := pageParams(r)
	var total int
	_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM groups WHERE `+strings.Join(where, " AND "), args...).Scan(&total)
	args = append(args, count, start-1)
	rows, err := s.pool.Query(r.Context(), `SELECT id, display_name, external_id, version, created_at, updated_at FROM groups WHERE `+
		strings.Join(where, " AND ")+fmt.Sprintf(` ORDER BY display_name LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		scimError(w, http.StatusInternalServerError, "", "query failed")
		return
	}
	var gs []scimGroup
	for rows.Next() {
		var g scimGroup
		if rows.Scan(&g.ID, &g.DisplayName, &g.ExternalID, &g.Version, &g.CreatedAt, &g.UpdatedAt) == nil {
			gs = append(gs, g)
		}
	}
	rows.Close()
	res := []any{}
	for _, g := range gs {
		res = append(res, s.scimGroupJSON(r.Context(), g, withMembers))
	}
	scimJSON(w, http.StatusOK, map[string]any{"schemas": []string{scimListSchema}, "totalResults": total, "startIndex": start,
		"itemsPerPage": len(res), "Resources": res})
}

// memberUserIDs maps SCIM user ids (member values) to QRit user ids in this directory.
func memberUserIDs(ctx context.Context, tx pgx.Tx, d scimDir, values []string) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for _, v := range values {
		sid, err := uuid.Parse(v)
		if err != nil {
			continue
		}
		var uid uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT user_id FROM scim_users WHERE id = $1 AND directory_id = $2`, sid, d.ID).Scan(&uid); err == nil {
			ids = append(ids, uid)
		}
	}
	return ids, nil
}

func memberValues(v any) []string {
	var out []string
	switch x := v.(type) {
	case []any:
		for _, m := range x {
			if mm, ok := m.(map[string]any); ok {
				if s, ok := mm["value"].(string); ok {
					out = append(out, s)
				}
			}
		}
	case map[string]any:
		if s, ok := x["value"].(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func (s *Server) scimCreateGroup(w http.ResponseWriter, r *http.Request) {
	d := scimDirectory(r)
	res, ok := s.readSCIMBody(w, r)
	if !ok {
		return
	}
	name, _ := res["displayName"].(string)
	if strings.TrimSpace(name) == "" {
		scimError(w, http.StatusBadRequest, "invalidValue", "displayName is required")
		return
	}
	var ext *string
	if v, ok := res["externalId"].(string); ok && v != "" {
		ext = &v
	}
	var g scimGroup
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended('scim:' || $1::text, 0))`, d.ID); err != nil {
			return err
		}
		// A group first seen in an SSO groups claim is adopted by the directory (its
		// bindings are kept); a manual or another directory's group of that name conflicts.
		err := tx.QueryRow(r.Context(), `INSERT INTO groups (id, org_id, display_name, source, directory_id, external_id)
			VALUES ($1, $2, $3, 'scim', $4, $5)
			ON CONFLICT (org_id, display_name) DO UPDATE SET source = 'scim', directory_id = EXCLUDED.directory_id,
				external_id = EXCLUDED.external_id, version = groups.version + 1, updated_at = now()
				WHERE groups.source = 'sso'
			RETURNING id, display_name, external_id, version, created_at, updated_at`,
			idgen.New(), d.OrgID, name, d.ID, ext).Scan(&g.ID, &g.DisplayName, &g.ExternalID, &g.Version, &g.CreatedAt, &g.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return errSCIMConflict
		}
		if err != nil {
			if pgCode(err) == sqlUniqueViolation {
				return errSCIMConflict
			}
			return err
		}
		if _, err := tx.Exec(r.Context(), `DELETE FROM group_members WHERE group_id = $1`, g.ID); err != nil {
			return err
		}
		uids, _ := memberUserIDs(r.Context(), tx, d, memberValues(res["members"]))
		for _, uid := range uids {
			if _, err := tx.Exec(r.Context(), `INSERT INTO group_members (group_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, g.ID, uid); err != nil {
				return err
			}
		}
		e := scimAudit(r, d, "scim.group.created", &g.ID, map[string]any{"display_name": name, "members": len(uids)})
		e.TargetType = "group"
		return s.aud().Record(r.Context(), q, e)
	})
	if s.scimFail(w, err) {
		return
	}
	s.afterSCIMChange(r)
	w.Header().Set("Location", s.scimBase()+"/Groups/"+g.ID.String())
	scimJSON(w, http.StatusCreated, s.scimGroupJSON(r.Context(), g, true))
}

func (s *Server) scimGetGroup(w http.ResponseWriter, r *http.Request) {
	g, err := s.loadSCIMGroup(r.Context(), nil, scimDirectory(r), chi.URLParam(r, "id"))
	if s.scimFail(w, err) {
		return
	}
	w.Header().Set("ETag", fmt.Sprintf(`W/"%d"`, g.Version))
	withMembers := !strings.Contains(strings.ToLower(r.URL.Query().Get("excludedAttributes")), "members")
	scimJSON(w, http.StatusOK, s.scimGroupJSON(r.Context(), g, withMembers))
}

func (s *Server) scimReplaceGroup(w http.ResponseWriter, r *http.Request) {
	d := scimDirectory(r)
	res, ok := s.readSCIMBody(w, r)
	if !ok {
		return
	}
	var g scimGroup
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var err error
		if g, err = s.loadSCIMGroup(r.Context(), tx, d, chi.URLParam(r, "id")); err != nil {
			return err
		}
		if err := ifMatch(r, g.Version); err != nil {
			return err
		}
		if name, _ := res["displayName"].(string); name != "" {
			g.DisplayName = name
		}
		if _, err := tx.Exec(r.Context(), `UPDATE groups SET display_name = $2, version = version + 1, updated_at = now() WHERE id = $1`,
			g.ID, g.DisplayName); err != nil {
			if pgCode(err) == sqlUniqueViolation {
				return errSCIMConflict
			}
			return err
		}
		if _, err := tx.Exec(r.Context(), `DELETE FROM group_members WHERE group_id = $1`, g.ID); err != nil {
			return err
		}
		uids, _ := memberUserIDs(r.Context(), tx, d, memberValues(res["members"]))
		for _, uid := range uids {
			if _, err := tx.Exec(r.Context(), `INSERT INTO group_members (group_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, g.ID, uid); err != nil {
				return err
			}
		}
		g, err = s.loadSCIMGroup(r.Context(), tx, d, g.ID.String())
		return err
	})
	if s.scimFail(w, err) {
		return
	}
	s.afterSCIMChange(r)
	scimJSON(w, http.StatusOK, s.scimGroupJSON(r.Context(), g, true))
}

var memberFilterPath = regexp.MustCompile(`(?i)^members\[value eq "([^"]+)"\]$`)

// PATCH /Groups/{id}: add/remove members, rename. Answers 204 (Entra's expectation).
func (s *Server) scimPatchGroup(w http.ResponseWriter, r *http.Request) {
	d := scimDirectory(r)
	var req patchReq
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Operations) == 0 {
		scimError(w, http.StatusBadRequest, "invalidSyntax", "PatchOp with Operations required")
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		// Entra sends membership PATCHes in parallel: serialise per directory.
		if _, err := tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended('scim:' || $1::text, 0))`, d.ID); err != nil {
			return err
		}
		g, err := s.loadSCIMGroup(r.Context(), tx, d, chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		if err := ifMatch(r, g.Version); err != nil {
			return err
		}
		added, removed := 0, 0
		for _, op := range req.Operations {
			kind := strings.ToLower(op.Op)
			var val any
			if len(op.Value) > 0 {
				_ = json.Unmarshal(op.Value, &val)
			}
			path := strings.TrimSpace(op.Path)
			switch {
			case strings.EqualFold(path, "members") || (path == "" && memberValues(mapGet(val, "members")) != nil):
				vals := memberValues(val)
				if path == "" {
					vals = memberValues(mapGet(val, "members"))
				}
				uids, _ := memberUserIDs(r.Context(), tx, d, vals)
				if kind == "replace" {
					if _, err := tx.Exec(r.Context(), `DELETE FROM group_members WHERE group_id = $1`, g.ID); err != nil {
						return err
					}
				}
				for _, uid := range uids {
					if kind == "remove" {
						if _, err := tx.Exec(r.Context(), `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`, g.ID, uid); err != nil {
							return err
						}
						removed++
					} else {
						if _, err := tx.Exec(r.Context(), `INSERT INTO group_members (group_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, g.ID, uid); err != nil {
							return err
						}
						added++
					}
				}
				if kind == "remove" && len(vals) == 0 && strings.EqualFold(path, "members") {
					if _, err := tx.Exec(r.Context(), `DELETE FROM group_members WHERE group_id = $1`, g.ID); err != nil {
						return err
					}
				}
			case memberFilterPath.MatchString(path) && kind == "remove":
				m := memberFilterPath.FindStringSubmatch(path)
				uids, _ := memberUserIDs(r.Context(), tx, d, []string{m[1]})
				for _, uid := range uids {
					if _, err := tx.Exec(r.Context(), `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`, g.ID, uid); err != nil {
						return err
					}
					removed++
				}
			case strings.EqualFold(path, "displayName") || (path == "" && mapGet(val, "displayName") != nil):
				name, _ := val.(string)
				if path == "" {
					name, _ = mapGet(val, "displayName").(string)
				}
				if name != "" {
					if _, err := tx.Exec(r.Context(), `UPDATE groups SET display_name = $2 WHERE id = $1`, g.ID, name); err != nil {
						if pgCode(err) == sqlUniqueViolation {
							return errSCIMConflict
						}
						return err
					}
				}
			case strings.EqualFold(path, "externalId"):
				ext, _ := val.(string)
				if _, err := tx.Exec(r.Context(), `UPDATE groups SET external_id = NULLIF($2, '') WHERE id = $1`, g.ID, ext); err != nil {
					return err
				}
			default:
				return fmt.Errorf("%w: unsupported path %q", errSCIMBadRequest, path)
			}
		}
		if _, err := tx.Exec(r.Context(), `UPDATE groups SET version = version + 1, updated_at = now() WHERE id = $1`, g.ID); err != nil {
			return err
		}
		e := scimAudit(r, d, "scim.group.updated", &g.ID, map[string]any{"added": added, "removed": removed})
		e.TargetType = "group"
		return s.aud().Record(r.Context(), q, e)
	})
	if s.scimFail(w, err) {
		return
	}
	s.afterSCIMChange(r)
	w.WriteHeader(http.StatusNoContent)
}

func mapGet(v any, k string) any {
	if m, ok := v.(map[string]any); ok {
		for key, val := range m {
			if strings.EqualFold(key, k) {
				return val
			}
		}
	}
	return nil
}

func (s *Server) scimDeleteGroup(w http.ResponseWriter, r *http.Request) {
	d := scimDirectory(r)
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		g, err := s.loadSCIMGroup(r.Context(), tx, d, chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `DELETE FROM role_bindings WHERE principal_type = 'group' AND principal_id = $1`, g.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `DELETE FROM groups WHERE id = $1`, g.ID); err != nil {
			return err
		}
		e := scimAudit(r, d, "scim.group.deleted", &g.ID, map[string]any{"display_name": g.DisplayName})
		e.TargetType = "group"
		return s.aud().Record(r.Context(), q, e)
	})
	if s.scimFail(w, err) {
		return
	}
	s.afterSCIMChange(r)
	w.WriteHeader(http.StatusNoContent)
}

// ---- directory management (org admins) -----------------------------------------

func (s *Server) scimDirectoryRoutes(r chi.Router) {
	r.With(requireOrg(authz.OrgSCIM)).Get("/scim-directories", s.handleListSCIMDirectories)
	r.With(requireOrg(authz.OrgSCIM), s.requireStepUp(10*time.Minute)).Post("/scim-directories", s.handleCreateSCIMDirectory)
	r.With(requireOrg(authz.OrgSCIM), s.requireStepUp(10*time.Minute)).Post("/scim-directories/{id}/token", s.handleRotateSCIMToken)
	r.With(requireOrg(authz.OrgSCIM)).Patch("/scim-directories/{id}", s.handleUpdateSCIMDirectory)
	r.With(requireOrg(authz.OrgSCIM), s.requireStepUp(10*time.Minute)).Delete("/scim-directories/{id}", s.handleDeleteSCIMDirectory)
}

type scimDirDTO struct {
	ID                uuid.UUID  `json:"id"`
	Label             string     `json:"label"`
	TokenPrefix       string     `json:"token_prefix"`
	Status            string     `json:"status"`
	DeprovisionAction string     `json:"deprovision_action"`
	LastRequestAt     *time.Time `json:"last_request_at"`
	CreatedAt         time.Time  `json:"created_at"`
	BaseURL           string     `json:"base_url"`
	Users             int        `json:"users"`
	Groups            int        `json:"groups"`
}

const scimDirCols = `id, label, token_prefix, status, deprovision_action, last_request_at, created_at,
	(SELECT count(*) FROM scim_users u WHERE u.directory_id = scim_directories.id)::int,
	(SELECT count(*) FROM groups g WHERE g.directory_id = scim_directories.id)::int`

func (s *Server) scanSCIMDir(row pgx.Row) (scimDirDTO, error) {
	var d scimDirDTO
	err := row.Scan(&d.ID, &d.Label, &d.TokenPrefix, &d.Status, &d.DeprovisionAction, &d.LastRequestAt, &d.CreatedAt, &d.Users, &d.Groups)
	d.BaseURL = s.scimBase()
	return d, err
}

func newSCIMToken() (string, []byte, string) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	var sb strings.Builder
	for _, x := range b {
		sb.WriteByte(alphabet[int(x)%32])
	}
	tok := "scim_" + sb.String()
	sum := sha256.Sum256([]byte(tok))
	return tok, sum[:], tok[:13]
}

func (s *Server) handleListSCIMDirectories(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT `+scimDirCols+` FROM scim_directories WHERE org_id = $1 ORDER BY created_at`, orgRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list directories"))
		return
	}
	defer rows.Close()
	out := []scimDirDTO{}
	for rows.Next() {
		if d, err := s.scanSCIMDir(rows); err == nil {
			out = append(out, d)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

type scimDirReq struct {
	Label             string  `json:"label"`
	DeprovisionAction *string `json:"deprovision_action"`
	Status            *string `json:"status"`
}

func (s *Server) handleCreateSCIMDirectory(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	if err := s.requireFeatureOrg(r, entitlements.FeatureSCIM); err != nil {
		fail(w, err)
		return
	}
	var req scimDirReq
	if !decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Label) == "" {
		req.Label = "Directory"
	}
	action := "suspend"
	if req.DeprovisionAction != nil {
		action = *req.DeprovisionAction
	}
	if action != "suspend" && action != "remove" {
		fail(w, unprocessable("invalid_deprovision_action", "deprovision_action must be suspend or remove"))
		return
	}
	tok, hash, prefix := newSCIMToken()
	userID, _ := actorIDs(r)
	var d scimDirDTO
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var err error
		d, err = s.scanSCIMDir(tx.QueryRow(r.Context(), `INSERT INTO scim_directories (id, org_id, label, token_prefix, token_hash,
			deprovision_action, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+scimDirCols,
			idgen.New(), o.ID, strings.TrimSpace(req.Label), prefix, hash, action, userID))
		if err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "scim.directory.created", "scim_directory", &d.ID, map[string]any{"label": d.Label})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to create directory"))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"directory": d, "token": tok})
}

func (s *Server) handleRotateSCIMToken(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("directory not found"))
		return
	}
	tok, hash, prefix := newSCIMToken()
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `UPDATE scim_directories SET token_hash = $3, token_prefix = $4 WHERE id = $1 AND org_id = $2`,
			id, o.ID, hash, prefix)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apierr.NotFound("directory not found")
		}
		return s.auditOrg(r, q, o.ID, "scim.token.rotated", "scim_directory", &id, nil)
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to rotate token"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": tok, "token_prefix": prefix})
}

func (s *Server) handleUpdateSCIMDirectory(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("directory not found"))
		return
	}
	var req scimDirReq
	if !decode(w, r, &req) {
		return
	}
	if req.DeprovisionAction != nil && *req.DeprovisionAction != "suspend" && *req.DeprovisionAction != "remove" {
		fail(w, unprocessable("invalid_deprovision_action", "deprovision_action must be suspend or remove"))
		return
	}
	if req.Status != nil && *req.Status != "active" && *req.Status != "disabled" {
		fail(w, unprocessable("invalid_status", "status must be active or disabled"))
		return
	}
	var label *string
	if strings.TrimSpace(req.Label) != "" {
		l := strings.TrimSpace(req.Label)
		label = &l
	}
	var d scimDirDTO
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var err error
		d, err = s.scanSCIMDir(tx.QueryRow(r.Context(), `UPDATE scim_directories SET label = COALESCE($3, label),
			deprovision_action = COALESCE($4, deprovision_action), status = COALESCE($5, status)
			WHERE id = $1 AND org_id = $2 RETURNING `+scimDirCols, id, o.ID, label, req.DeprovisionAction, req.Status))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierr.NotFound("directory not found")
			}
			return err
		}
		return s.auditOrg(r, q, o.ID, "scim.directory.updated", "scim_directory", &id, map[string]any{"status": req.Status})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to update directory"))
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleDeleteSCIMDirectory(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("directory not found"))
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `DELETE FROM role_bindings WHERE principal_type = 'group'
			AND principal_id IN (SELECT g.id FROM groups g JOIN scim_directories sd ON sd.id = g.directory_id
			WHERE sd.id = $1 AND sd.org_id = $2)`, id, o.ID); err != nil {
			return err
		}
		tag, err := tx.Exec(r.Context(), `DELETE FROM scim_directories WHERE id = $1 AND org_id = $2`, id, o.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apierr.NotFound("directory not found")
		}
		return s.auditOrg(r, q, o.ID, "scim.directory.deleted", "scim_directory", &id, nil)
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to delete directory"))
		return
	}
	s.access.Invalidate(r.Context())
	w.WriteHeader(http.StatusNoContent)
}
