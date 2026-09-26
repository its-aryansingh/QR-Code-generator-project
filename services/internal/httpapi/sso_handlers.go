package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/org"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
	"github.com/its-aryansingh/qrit/services/internal/sso"
)

// SSODeps are optional collaborators for identity federation (tests inject fakes).
type SSODeps struct {
	HTTP  *http.Client
	DNS   sso.TXTResolver
	Polis *sso.Polis
}

// SetSSO replaces the SSO collaborators.
func (s *Server) SetSSO(d SSODeps) {
	if d.HTTP != nil {
		s.ssoHTTP = d.HTTP
		s.oidc = sso.NewOIDC(d.HTTP)
	}
	if d.DNS != nil {
		s.dns = d.DNS
	}
	if d.Polis != nil {
		s.polis = d.Polis
	}
}

func (s *Server) initSSO() {
	s.ssoHTTP = sso.NewHTTPClient(s.cfg.IsLocal())
	s.oidc = sso.NewOIDC(s.ssoHTTP)
	s.dns = sso.DoH{Endpoint: s.cfg.DoHURL, Client: sso.NewHTTPClient(false)}
	if s.cfg.PolisURL != "" {
		s.polis = sso.NewPolis(s.ssoHTTP, s.cfg.PolisURL, s.cfg.PolisExternalURL, s.cfg.PolisAdminAPIKey, s.cfg.PolisProduct)
	}
}

func (s *Server) ssoRedirectURI() string {
	return strings.TrimRight(s.cfg.APIPublicURL, "/") + "/v1/auth/sso/callback"
}

func (s *Server) ssoOrgRoutes(r chi.Router) {
	r.With(requireOrg(authz.OrgSSO)).Get("/domains", s.handleListDomains)
	r.With(requireOrg(authz.OrgSSO)).Post("/domains", s.handleCreateDomain)
	r.With(requireOrg(authz.OrgSSO)).Post("/domains/{id}/verify", s.handleVerifyDomain)
	r.With(requireOrg(authz.OrgSSO)).Patch("/domains/{id}", s.handleUpdateDomain)
	r.With(requireOrg(authz.OrgSSO)).Delete("/domains/{id}", s.handleDeleteDomain)
	r.With(requireOrg(authz.OrgSSO)).Get("/sso-connections", s.handleListConnections)
	r.With(requireOrg(authz.OrgSSO), s.requireStepUp(10*time.Minute)).Post("/sso-connections", s.handleCreateConnection)
	r.With(requireOrg(authz.OrgSSO)).Get("/sso-connections/{id}", s.handleGetConnection)
	r.With(requireOrg(authz.OrgSSO), s.requireStepUp(10*time.Minute)).Patch("/sso-connections/{id}", s.handleUpdateConnection)
	r.With(requireOrg(authz.OrgSSO), s.requireStepUp(10*time.Minute)).Delete("/sso-connections/{id}", s.handleDeleteConnection)
	r.With(requireOrg(authz.OrgSSO)).Post("/sso-connections/{id}/test", s.handleTestConnection)
	r.With(requireOrg(authz.OrgSSO)).Get("/sso-connections/{id}/test-result", s.handleTestResult)
}

// ---- claimed domains ---------------------------------------------------------

type domainDTO struct {
	ID                  uuid.UUID  `json:"id"`
	Domain              string     `json:"domain"`
	VerifiedAt          *time.Time `json:"verified_at"`
	AutoJoin            bool       `json:"auto_join"`
	AutoJoinWorkspaceID *uuid.UUID `json:"auto_join_workspace_id"`
	TXTName             string     `json:"txt_record_name"`
	TXTValue            string     `json:"txt_record_value"`
	LastCheckedAt       *time.Time `json:"last_checked_at"`
	CreatedAt           time.Time  `json:"created_at"`
}

const domainCols = `id, domain::text, verification_token, verified_at, auto_join, auto_join_workspace_id, last_checked_at, created_at`

func scanDomain(row pgx.Row) (domainDTO, error) {
	var d domainDTO
	var token string
	err := row.Scan(&d.ID, &d.Domain, &token, &d.VerifiedAt, &d.AutoJoin, &d.AutoJoinWorkspaceID, &d.LastCheckedAt, &d.CreatedAt)
	d.TXTName = "_qrit-challenge." + d.Domain
	d.TXTValue = "qrit-domain-verification=" + token
	return d, err
}

func (s *Server) handleListDomains(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT `+domainCols+` FROM org_domains WHERE org_id = $1 ORDER BY domain`, orgRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list domains"))
		return
	}
	defer rows.Close()
	out := []domainDTO{}
	for rows.Next() {
		if d, err := scanDomain(rows); err == nil {
			out = append(out, d)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

var domainPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

type domainReq struct {
	Domain              string     `json:"domain"`
	AutoJoin            *bool      `json:"auto_join"`
	AutoJoinWorkspaceID *uuid.UUID `json:"auto_join_workspace_id"`
}

func (s *Server) handleCreateDomain(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	var req domainReq
	if !decode(w, r, &req) {
		return
	}
	d := strings.TrimSuffix(trimLower(req.Domain), ".")
	if !domainPattern.MatchString(d) {
		fail(w, unprocessable("invalid_domain", "enter a domain such as acme.com"))
		return
	}
	if sso.PublicMailDomains[d] {
		fail(w, unprocessable("public_domain", "public email domains can't be claimed"))
		return
	}
	token := sso.RandomToken(24)
	var dto domainDTO
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var taken bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM org_domains WHERE domain = $1 AND verified_at IS NOT NULL)`, d).
			Scan(&taken); err != nil {
			return err
		}
		if taken {
			return apierr.Conflict("domain_claimed", "this domain is already verified by an organisation")
		}
		var err error
		dto, err = scanDomain(tx.QueryRow(r.Context(), `INSERT INTO org_domains (id, org_id, domain, verification_token)
			VALUES ($1, $2, $3, $4) RETURNING `+domainCols, idgen.New(), o.ID, d, token))
		if err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "org.domain.added", "org_domain", &dto.ID, map[string]any{"domain": d})
	})
	if err != nil {
		if pgCode(err) == sqlUniqueViolation {
			fail(w, apierr.Conflict("domain_exists", "this organisation has already added this domain"))
			return
		}
		fail(w, problemOr500(err, "failed to add domain"))
		return
	}
	writeJSON(w, http.StatusCreated, dto)
}

// POST /orgs/{org}/domains/{id}/verify: look for the TXT record now.
func (s *Server) handleVerifyDomain(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("domain not found"))
		return
	}
	d, err := scanDomain(s.pool.QueryRow(r.Context(), `SELECT `+domainCols+` FROM org_domains WHERE id = $1 AND org_id = $2`, id, o.ID))
	if err != nil {
		fail(w, apierr.NotFound("domain not found"))
		return
	}
	if d.VerifiedAt != nil {
		writeJSON(w, http.StatusOK, d)
		return
	}
	verified, err := s.CheckDomainTXT(r.Context(), d.Domain, strings.TrimPrefix(d.TXTValue, "qrit-domain-verification="))
	_, _ = s.pool.Exec(r.Context(), `UPDATE org_domains SET last_checked_at = now(), check_attempts = check_attempts + 1 WHERE id = $1`, id)
	if err != nil {
		fail(w, apierr.New(http.StatusBadGateway, "dns_lookup_failed", "Bad Gateway", "couldn't query DNS right now; we'll keep checking"))
		return
	}
	if !verified {
		fail(w, apierr.Conflict("txt_record_not_found", "add a TXT record "+d.TXTName+" with value "+d.TXTValue+" (DNS changes can take a while)"))
		return
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `UPDATE org_domains SET verified_at = now() WHERE id = $1`, id); err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "org.domain.verified", "org_domain", &id, map[string]any{"domain": d.Domain})
	})
	if err != nil {
		if pgCode(err) == sqlUniqueViolation {
			fail(w, apierr.Conflict("domain_claimed", "another organisation verified this domain first"))
			return
		}
		fail(w, apierr.Internal("failed to mark verified"))
		return
	}
	d, _ = scanDomain(s.pool.QueryRow(r.Context(), `SELECT `+domainCols+` FROM org_domains WHERE id = $1`, id))
	writeJSON(w, http.StatusOK, d)
}

// CheckDomainTXT reports whether _qrit-challenge.<domain> holds our token.
func (s *Server) CheckDomainTXT(ctx context.Context, domain, token string) (bool, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	txt, err := s.dns.LookupTXT(cctx, "_qrit-challenge."+domain)
	if err != nil {
		return false, err
	}
	for _, t := range txt {
		if strings.TrimSpace(t) == "qrit-domain-verification="+token {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) handleUpdateDomain(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("domain not found"))
		return
	}
	var req domainReq
	if !decode(w, r, &req) {
		return
	}
	if req.AutoJoinWorkspaceID != nil {
		var n int
		_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM workspaces WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL`,
			*req.AutoJoinWorkspaceID, o.ID).Scan(&n)
		if n == 0 {
			fail(w, unprocessable("invalid_workspace", "auto_join_workspace_id must be a workspace of this organisation"))
			return
		}
	}
	if req.AutoJoin != nil && *req.AutoJoin && req.AutoJoinWorkspaceID == nil {
		fail(w, unprocessable("workspace_required", "auto-join needs auto_join_workspace_id"))
		return
	}
	var d domainDTO
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var err error
		d, err = scanDomain(tx.QueryRow(r.Context(), `UPDATE org_domains SET auto_join = COALESCE($3, auto_join),
			auto_join_workspace_id = COALESCE($4, auto_join_workspace_id) WHERE id = $1 AND org_id = $2 RETURNING `+domainCols,
			id, o.ID, req.AutoJoin, req.AutoJoinWorkspaceID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierr.NotFound("domain not found")
			}
			return err
		}
		if d.AutoJoin && d.VerifiedAt == nil {
			return apierr.Conflict("domain_unverified", "verify the domain before enabling auto-join")
		}
		return s.auditOrg(r, q, o.ID, "org.domain.updated", "org_domain", &id, map[string]any{"auto_join": d.AutoJoin})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to update domain"))
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("domain not found"))
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `DELETE FROM org_domains WHERE id = $1 AND org_id = $2`, id, o.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apierr.NotFound("domain not found")
		}
		return s.auditOrg(r, q, o.ID, "org.domain.removed", "org_domain", &id, nil)
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to remove domain"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- SSO connections ---------------------------------------------------------

type connection struct {
	ID                 uuid.UUID         `json:"id"`
	OrgID              uuid.UUID         `json:"org_id"`
	Protocol           string            `json:"protocol"`
	Label              string            `json:"label"`
	Status             string            `json:"status"`
	IdPKind            string            `json:"idp_kind"`
	BridgeTenant       *string           `json:"bridge_tenant,omitempty"`
	OIDCIssuer         *string           `json:"oidc_issuer,omitempty"`
	OIDCClientID       *string           `json:"oidc_client_id,omitempty"`
	HasClientSecret    bool              `json:"has_client_secret"`
	OIDCScopes         string            `json:"oidc_scopes"`
	GroupsClaim        string            `json:"groups_claim"`
	JIT                bool              `json:"jit_provisioning"`
	DefaultWorkspaceID *uuid.UUID        `json:"default_workspace_id"`
	DefaultRoleKey     string            `json:"default_role_key"`
	AttributeMapping   map[string]string `json:"attribute_mapping"`
	SAMLMetadataURL    *string           `json:"saml_metadata_url,omitempty"`
	TestPassedAt       *time.Time        `json:"test_passed_at"`
	LastLoginAt        *time.Time        `json:"last_login_at"`
	CreatedAt          time.Time         `json:"created_at"`
	ACSURL             string            `json:"acs_url,omitempty"`
	RedirectURI        string            `json:"redirect_uri"`
	secretCT           []byte
}

const connCols = `id, org_id, protocol, label, status, idp_kind, bridge_tenant, oidc_issuer, oidc_client_id, oidc_client_secret_ct,
	oidc_scopes, groups_claim, jit_provisioning, default_workspace_id, default_role_key, attribute_mapping, saml_metadata_url,
	test_passed_at, last_login_at, created_at`

func (s *Server) scanConnection(row pgx.Row) (connection, error) {
	var c connection
	var mapping []byte
	err := row.Scan(&c.ID, &c.OrgID, &c.Protocol, &c.Label, &c.Status, &c.IdPKind, &c.BridgeTenant, &c.OIDCIssuer, &c.OIDCClientID,
		&c.secretCT, &c.OIDCScopes, &c.GroupsClaim, &c.JIT, &c.DefaultWorkspaceID, &c.DefaultRoleKey, &mapping, &c.SAMLMetadataURL,
		&c.TestPassedAt, &c.LastLoginAt, &c.CreatedAt)
	_ = json.Unmarshal(mapping, &c.AttributeMapping)
	c.HasClientSecret = len(c.secretCT) > 0
	c.RedirectURI = s.ssoRedirectURI()
	if c.Protocol == "saml" && s.polis.Configured() {
		c.ACSURL = strings.TrimRight(s.cfg.PolisExternalURL, "/") + "/api/oauth/saml"
	}
	return c, err
}

func (s *Server) handleListConnections(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT `+connCols+` FROM sso_connections WHERE org_id = $1 ORDER BY created_at`, orgRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list connections"))
		return
	}
	defer rows.Close()
	out := []connection{}
	for rows.Next() {
		if c, err := s.scanConnection(rows); err == nil {
			out = append(out, c)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

type connectionReq struct {
	Protocol           string            `json:"protocol"`
	Label              *string           `json:"label"`
	IdPKind            *string           `json:"idp_kind"`
	OIDCIssuer         *string           `json:"oidc_issuer"`
	OIDCClientID       *string           `json:"oidc_client_id"`
	OIDCClientSecret   *string           `json:"oidc_client_secret"`
	OIDCScopes         *string           `json:"oidc_scopes"`
	GroupsClaim        *string           `json:"groups_claim"`
	SAMLMetadataXML    string            `json:"saml_metadata_xml"`
	SAMLMetadataURL    *string           `json:"saml_metadata_url"`
	JIT                *bool             `json:"jit_provisioning"`
	DefaultWorkspaceID *uuid.UUID        `json:"default_workspace_id"`
	DefaultRoleKey     *string           `json:"default_role_key"`
	AttributeMapping   map[string]string `json:"attribute_mapping"`
	Status             *string           `json:"status"`
}

func (s *Server) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	if err := s.requireFeatureOrg(r, entitlements.FeatureSSO); err != nil {
		fail(w, err)
		return
	}
	var req connectionReq
	if !decode(w, r, &req) {
		return
	}
	label := "SSO"
	if req.Label != nil && strings.TrimSpace(*req.Label) != "" {
		label = strings.TrimSpace(*req.Label)
	}
	idpKind := "custom"
	if req.IdPKind != nil {
		idpKind = *req.IdPKind
	}
	role := "analyst"
	if req.DefaultRoleKey != nil {
		role = *req.DefaultRoleKey
	}
	if !assignableMemberRole(role) {
		fail(w, unprocessable("invalid_role", "default_role_key must be admin, editor, reviewer or analyst"))
		return
	}
	if req.DefaultWorkspaceID != nil {
		var n int
		_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM workspaces WHERE id = $1 AND org_id = $2`, *req.DefaultWorkspaceID, o.ID).Scan(&n)
		if n == 0 {
			fail(w, unprocessable("invalid_workspace", "default_workspace_id must belong to this organisation"))
			return
		}
	}
	id := idgen.New()
	var secretCT []byte
	var tenant *string
	switch req.Protocol {
	case "oidc":
		if req.OIDCIssuer == nil || req.OIDCClientID == nil || req.OIDCClientSecret == nil {
			fail(w, unprocessable("oidc_fields_required", "oidc_issuer, oidc_client_id and oidc_client_secret are required"))
			return
		}
		iss := strings.TrimRight(strings.TrimSpace(*req.OIDCIssuer), "/")
		if u, err := url.Parse(iss); err != nil || (u.Scheme != "https" && !s.cfg.IsLocal()) || u.Host == "" {
			fail(w, unprocessable("invalid_issuer", "oidc_issuer must be an https URL"))
			return
		}
		if _, err := s.oidc.Discover(r.Context(), iss); err != nil {
			fail(w, unprocessable("discovery_failed", "couldn't read "+iss+"/.well-known/openid-configuration: "+err.Error()))
			return
		}
		req.OIDCIssuer = &iss
		var err error
		if secretCT, err = s.keyring.Encrypt(r.Context(), o.ID, []byte(*req.OIDCClientSecret)); err != nil {
			fail(w, apierr.Internal("failed to store secret"))
			return
		}
	case "saml":
		if !s.polis.Configured() {
			fail(w, apierr.New(http.StatusServiceUnavailable, "saml_bridge_unavailable", "Service Unavailable", "SAML needs the SSO bridge (POLIS_URL)"))
			return
		}
		if req.SAMLMetadataXML == "" && (req.SAMLMetadataURL == nil || *req.SAMLMetadataURL == "") {
			fail(w, unprocessable("metadata_required", "provide saml_metadata_xml or saml_metadata_url"))
			return
		}
		t := o.Slug
		tenant = &t
		if err := s.polis.CreateConnection(r.Context(), t, label, req.SAMLMetadataXML, derefOr(req.SAMLMetadataURL, ""),
			s.ssoRedirectURI(), s.ssoRedirectURI()); err != nil {
			fail(w, unprocessable("saml_metadata_rejected", err.Error()))
			return
		}
	default:
		fail(w, unprocessable("invalid_protocol", "protocol must be oidc or saml"))
		return
	}
	mapping, _ := json.Marshal(mergeMapping(req.AttributeMapping))
	userID, _ := actorIDs(r)
	var c connection
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var err error
		c, err = s.scanConnection(tx.QueryRow(r.Context(), `INSERT INTO sso_connections (id, org_id, protocol, label, idp_kind,
			bridge_tenant, oidc_issuer, oidc_client_id, oidc_client_secret_ct, oidc_scopes, groups_claim, jit_provisioning,
			default_workspace_id, default_role_key, attribute_mapping, saml_metadata_url, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,COALESCE($10,'openid email profile'),COALESCE($11,'groups'),COALESCE($12,true),$13,$14,$15,$16,$17)
			RETURNING `+connCols, id, o.ID, req.Protocol, label, idpKind, tenant, req.OIDCIssuer, req.OIDCClientID, secretCT,
			req.OIDCScopes, req.GroupsClaim, req.JIT, req.DefaultWorkspaceID, role, mapping, req.SAMLMetadataURL, userID))
		if err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "sso.connection.created", "sso_connection", &id, map[string]any{"protocol": req.Protocol, "label": label})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to create connection"))
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func mergeMapping(in map[string]string) map[string]string {
	m := map[string]string{"email": "email", "first_name": "firstName", "last_name": "lastName", "groups": "groups"}
	for k, v := range in {
		if v != "" {
			m[k] = v
		}
	}
	return m
}

func (s *Server) loadConnection(r *http.Request) (connection, error) {
	id, ok := uuidParam(r, "id")
	if !ok {
		return connection{}, apierr.NotFound("connection not found")
	}
	c, err := s.scanConnection(s.pool.QueryRow(r.Context(), `SELECT `+connCols+` FROM sso_connections WHERE id = $1 AND org_id = $2`,
		id, orgRow(r).ID))
	if err != nil {
		return c, apierr.NotFound("connection not found")
	}
	return c, nil
}

func (s *Server) handleGetConnection(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadConnection(r)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleUpdateConnection(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	c, err := s.loadConnection(r)
	if err != nil {
		fail(w, err)
		return
	}
	var req connectionReq
	if !decode(w, r, &req) {
		return
	}
	if req.Status != nil {
		switch *req.Status {
		case "active":
			if c.TestPassedAt == nil {
				fail(w, apierr.Conflict("test_required", "run a successful test login before activating the connection"))
				return
			}
		case "disabled":
			if pol, _ := s.access.Policy(r.Context(), o.ID); pol != nil && pol.EnforceSSO && c.Status == "active" {
				var others int
				_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM sso_connections WHERE org_id = $1 AND status = 'active' AND id <> $2`,
					o.ID, c.ID).Scan(&others)
				if others == 0 {
					fail(w, apierr.Conflict("sso_enforced", "turn off SSO enforcement before disabling the last active connection"))
					return
				}
			}
		case "draft", "testing":
		default:
			fail(w, unprocessable("invalid_status", "status must be draft, testing, active or disabled"))
			return
		}
	}
	if req.DefaultRoleKey != nil && !assignableMemberRole(*req.DefaultRoleKey) {
		fail(w, unprocessable("invalid_role", "default_role_key must be admin, editor, reviewer or analyst"))
		return
	}
	var secretCT []byte
	if req.OIDCClientSecret != nil {
		if secretCT, err = s.keyring.Encrypt(r.Context(), o.ID, []byte(*req.OIDCClientSecret)); err != nil {
			fail(w, apierr.Internal("failed to store secret"))
			return
		}
	}
	var mapping []byte
	if req.AttributeMapping != nil {
		mapping, _ = json.Marshal(mergeMapping(req.AttributeMapping))
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var err error
		c, err = s.scanConnection(tx.QueryRow(r.Context(), `UPDATE sso_connections SET
			label = COALESCE($3, label), status = COALESCE($4, status), oidc_client_id = COALESCE($5, oidc_client_id),
			oidc_client_secret_ct = COALESCE($6, oidc_client_secret_ct), oidc_scopes = COALESCE($7, oidc_scopes),
			groups_claim = COALESCE($8, groups_claim), jit_provisioning = COALESCE($9, jit_provisioning),
			default_workspace_id = COALESCE($10, default_workspace_id), default_role_key = COALESCE($11, default_role_key),
			attribute_mapping = COALESCE($12, attribute_mapping), updated_at = now()
			WHERE id = $1 AND org_id = $2 RETURNING `+connCols, c.ID, o.ID, req.Label, req.Status, req.OIDCClientID, secretCT,
			req.OIDCScopes, req.GroupsClaim, req.JIT, req.DefaultWorkspaceID, req.DefaultRoleKey, mapping))
		if err != nil {
			return err
		}
		changes := map[string]any{"status": req.Status, "label": req.Label}
		if req.OIDCClientSecret != nil {
			changes["client_secret"] = "rotated"
		}
		return s.auditOrg(r, q, o.ID, "sso.connection.updated", "sso_connection", &c.ID, changes)
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to update connection"))
		return
	}
	s.access.Invalidate(r.Context())
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	c, err := s.loadConnection(r)
	if err != nil {
		fail(w, err)
		return
	}
	if pol, _ := s.access.Policy(r.Context(), o.ID); pol != nil && pol.EnforceSSO && c.Status == "active" {
		var others int
		_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM sso_connections WHERE org_id = $1 AND status = 'active' AND id <> $2`,
			o.ID, c.ID).Scan(&others)
		if others == 0 {
			fail(w, apierr.Conflict("sso_enforced", "turn off SSO enforcement before deleting the last active connection"))
			return
		}
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `DELETE FROM sso_connections WHERE id = $1`, c.ID); err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "sso.connection.deleted", "sso_connection", &c.ID, map[string]any{"label": c.Label})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to delete connection"))
		return
	}
	if c.Protocol == "saml" && c.BridgeTenant != nil && s.polis.Configured() {
		var left int
		_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM sso_connections WHERE org_id = $1 AND protocol = 'saml'`, o.ID).Scan(&left)
		if left == 0 {
			if err := s.polis.DeleteConnections(r.Context(), *c.BridgeTenant); err != nil {
				slog.Warn("polis cleanup", "error", err)
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- login flow --------------------------------------------------------------

type ssoState struct {
	ConnectionID uuid.UUID  `json:"c"`
	Verifier     string     `json:"v"`
	Nonce        string     `json:"n"`
	Redirect     string     `json:"r"`
	Test         bool       `json:"t"`
	TesterID     *uuid.UUID `json:"u,omitempty"`
}

// beginLogin stores the state and returns the IdP URL.
func (s *Server) beginLogin(ctx context.Context, c connection, redirect string, test bool, tester *uuid.UUID, forceAuthn bool) (string, error) {
	if s.rdb == nil {
		return "", apierr.Internal("SSO needs Redis for login state")
	}
	state := sso.RandomToken(24)
	verifier, challenge := sso.PKCE()
	st := ssoState{ConnectionID: c.ID, Verifier: verifier, Nonce: sso.RandomToken(16), Redirect: redirect, Test: test, TesterID: tester}
	b, _ := json.Marshal(st)
	if err := s.rdb.Set(ctx, "sso:state:"+state, b, 10*time.Minute).Err(); err != nil {
		return "", err
	}
	switch c.Protocol {
	case "oidc":
		d, err := s.oidc.Discover(ctx, *c.OIDCIssuer)
		if err != nil {
			return "", apierr.New(http.StatusBadGateway, "idp_unavailable", "Bad Gateway", "the identity provider could not be reached")
		}
		extra := url.Values{}
		if forceAuthn {
			extra.Set("prompt", "login")
		}
		return sso.AuthURL(d.AuthorizationEndpoint, *c.OIDCClientID, s.ssoRedirectURI(), c.OIDCScopes, state, st.Nonce, challenge, extra), nil
	case "saml":
		if !s.polis.Configured() || c.BridgeTenant == nil {
			return "", apierr.New(http.StatusServiceUnavailable, "saml_bridge_unavailable", "Service Unavailable", "SAML bridge not configured")
		}
		return s.polis.AuthURL(*c.BridgeTenant, s.ssoRedirectURI(), state, challenge, forceAuthn), nil
	}
	return "", apierr.Internal("unknown protocol")
}

type ssoStartReq struct {
	Email    string `json:"email"`
	Org      string `json:"org"`
	Redirect string `json:"redirect"`
	Prompt   string `json:"prompt"`
}

// safeRedirect keeps post-login redirects on our app (relative paths only).
func safeRedirect(p string) string {
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.Contains(p, "\\") {
		return "/"
	}
	return p
}

// POST /auth/sso/start {email | org}: find the organisation's active connection.
func (s *Server) handleSSOStart(w http.ResponseWriter, r *http.Request) {
	var req ssoStartReq
	if !decode(w, r, &req) {
		return
	}
	if ok, _, retry, _ := s.limiter.Allow(r.Context(), "rl:sso:"+ipKey(r), 30, time.Minute); !ok {
		fail(w, tooMany("too many sign-in attempts", retry))
		return
	}
	var orgID uuid.UUID
	switch {
	case req.Email != "":
		e, ok := validEmail(req.Email)
		if !ok {
			fail(w, unprocessable("invalid_email", "enter your work email"))
			return
		}
		domain := e[strings.LastIndex(e, "@")+1:]
		if err := s.pool.QueryRow(r.Context(), `SELECT org_id FROM org_domains WHERE domain = $1 AND verified_at IS NOT NULL`, domain).Scan(&orgID); err != nil {
			fail(w, apierr.NotFound("single sign-on isn't set up for "+domain))
			return
		}
	case req.Org != "":
		o, err := org.GetBySlug(r.Context(), s.pool, trimLower(req.Org))
		if err != nil {
			fail(w, apierr.NotFound("organisation not found"))
			return
		}
		orgID = o.ID
	default:
		fail(w, unprocessable("email_required", "enter your work email"))
		return
	}
	c, err := s.scanConnection(s.pool.QueryRow(r.Context(), `SELECT `+connCols+` FROM sso_connections
		WHERE org_id = $1 AND status = 'active' ORDER BY last_login_at DESC NULLS LAST LIMIT 1`, orgID))
	if err != nil {
		fail(w, apierr.NotFound("single sign-on isn't active for this organisation"))
		return
	}
	u, err := s.beginLogin(r.Context(), c, safeRedirect(req.Redirect), false, nil, req.Prompt == "login")
	if err != nil {
		fail(w, problemOr500(err, "failed to start sign-in"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"redirect_url": u, "protocol": c.Protocol})
}

// POST /orgs/{org}/sso-connections/{id}/test: an admin tries the connection before activating.
func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadConnection(r)
	if err != nil {
		fail(w, err)
		return
	}
	uid := principal(r).UserID
	u, err := s.beginLogin(r.Context(), c, "/settings/sso", true, &uid, true)
	if err != nil {
		fail(w, problemOr500(err, "failed to start test"))
		return
	}
	if c.Status == "draft" {
		_, _ = s.pool.Exec(r.Context(), `UPDATE sso_connections SET status = 'testing' WHERE id = $1`, c.ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"redirect_url": u})
}

func (s *Server) handleTestResult(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadConnection(r)
	if err != nil {
		fail(w, err)
		return
	}
	if s.rdb == nil {
		fail(w, apierr.NotFound("no test result"))
		return
	}
	b, err := s.rdb.Get(r.Context(), "sso:test:"+c.ID.String()).Bytes()
	if err != nil {
		fail(w, apierr.NotFound("no recent test login"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

func (s *Server) ssoFailRedirect(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, strings.TrimRight(s.cfg.AppBaseURL, "/")+"/login?sso_error="+url.QueryEscape(code), http.StatusFound)
}

// GET /auth/sso/callback: finish the login, link or provision the user, start a session.
func (s *Server) handleSSOCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("error") != "" {
		s.ssoFailRedirect(w, r, "idp_error")
		return
	}
	stateKey := q.Get("state")
	if stateKey == "" || s.rdb == nil {
		s.ssoFailRedirect(w, r, "invalid_state")
		return
	}
	raw, err := s.rdb.GetDel(r.Context(), "sso:state:"+stateKey).Bytes() // single use: blocks replay and IdP-initiated logins
	if err != nil {
		s.ssoFailRedirect(w, r, "invalid_state")
		return
	}
	var st ssoState
	_ = json.Unmarshal(raw, &st)
	c, err := s.scanConnection(s.pool.QueryRow(r.Context(), `SELECT `+connCols+` FROM sso_connections WHERE id = $1`, st.ConnectionID))
	if err != nil || c.Status == "disabled" || (!st.Test && c.Status != "active") {
		s.ssoFailRedirect(w, r, "connection_inactive")
		return
	}
	claims, err := s.ssoClaims(r.Context(), c, q.Get("code"), st)
	if err != nil {
		slog.Warn("sso callback", "connection", c.ID, "error", err)
		s.ssoFailRedirect(w, r, "assertion_rejected")
		return
	}
	if claims.Email == "" {
		// Entra ID puts the UPN in preferred_username and often omits email.
		for _, k := range []string{"preferred_username", "upn"} {
			if v, _ := claims.Raw[k].(string); strings.Contains(v, "@") {
				if e, ok := validEmail(v); ok {
					claims.Email, claims.EmailVerified = e, false
					break
				}
			}
		}
	}
	// An address the IdP hasn't vouched for is accepted only on a domain this organisation
	// has proven it owns (Entra never sends email_verified).
	if claims.Email == "" || (c.Protocol == "oidc" && !claims.EmailVerified && !s.orgOwnsDomain(r.Context(), c.OrgID, claims.Email)) {
		s.ssoFailRedirect(w, r, "email_not_verified")
		return
	}
	if st.Test {
		result := map[string]any{"ok": true, "at": time.Now().UTC(), "subject": claims.Subject, "email": claims.Email,
			"first_name": claims.GivenName, "last_name": claims.FamilyName, "groups": claims.Groups}
		b, _ := json.Marshal(result)
		_ = s.rdb.Set(r.Context(), "sso:test:"+c.ID.String(), b, time.Hour).Err()
		_, _ = s.pool.Exec(r.Context(), `UPDATE sso_connections SET test_passed_at = now(),
			status = CASE WHEN status IN ('draft','testing') THEN 'active' ELSE status END WHERE id = $1`, c.ID)
		http.Redirect(w, r, strings.TrimRight(s.cfg.AppBaseURL, "/")+st.Redirect+"?sso_test=ok", http.StatusFound)
		return
	}
	user, err := s.linkSSOUser(r, c, claims)
	if err != nil {
		var pd *apierr.ProblemDetails
		code := "login_failed"
		if errors.As(err, &pd) {
			code = pd.Code
		} else {
			slog.Error("sso link", "error", err)
		}
		s.ssoFailRedirect(w, r, code)
		return
	}
	if _, err := s.startSession(w, r, user.ID, "sso", &c.ID); err != nil {
		s.ssoFailRedirect(w, r, "session_failed")
		return
	}
	_ = s.q.SetLastLogin(r.Context(), user.ID)
	http.Redirect(w, r, strings.TrimRight(s.cfg.AppBaseURL, "/")+st.Redirect, http.StatusFound)
}

func (s *Server) orgOwnsDomain(ctx context.Context, orgID uuid.UUID, email string) bool {
	var ok bool
	_ = s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM org_domains WHERE org_id = $1 AND domain = $2 AND verified_at IS NOT NULL)`,
		orgID, email[strings.LastIndex(email, "@")+1:]).Scan(&ok)
	return ok
}

func (s *Server) ssoClaims(ctx context.Context, c connection, code string, st ssoState) (sso.Claims, error) {
	if code == "" {
		return sso.Claims{}, errors.New("missing code")
	}
	switch c.Protocol {
	case "oidc":
		d, err := s.oidc.Discover(ctx, *c.OIDCIssuer)
		if err != nil {
			return sso.Claims{}, err
		}
		secret, err := s.keyring.Decrypt(ctx, c.OrgID, c.secretCT)
		if err != nil {
			return sso.Claims{}, err
		}
		tr, err := s.oidc.ExchangeCode(ctx, d, *c.OIDCClientID, string(secret), code, st.Verifier, s.ssoRedirectURI())
		if err != nil {
			return sso.Claims{}, err
		}
		claims, err := s.oidc.VerifyIDToken(ctx, d, *c.OIDCClientID, tr.IDToken, st.Nonce)
		if err != nil {
			return claims, err
		}
		return sso.ClaimsWithGroups(claims, c.GroupsClaim), nil
	case "saml":
		token, err := s.polis.Exchange(ctx, *c.BridgeTenant, code, st.Verifier, s.ssoRedirectURI())
		if err != nil {
			return sso.Claims{}, err
		}
		return s.polis.UserInfo(ctx, token, *c.BridgeTenant, c.AttributeMapping)
	}
	return sso.Claims{}, errors.New("unknown protocol")
}

// linkSSOUser resolves the asserted identity to a user: an existing link, else an existing
// account whose email domain this organisation has verified, else JIT provisioning. It
// never links by email on an unverified domain (that would let a rogue IdP take over accounts).
func (s *Server) linkSSOUser(r *http.Request, c connection, claims sso.Claims) (dbgen.User, error) {
	ctx := r.Context()
	var user dbgen.User
	err := s.inTx(ctx, func(q *dbgen.Queries, tx pgx.Tx) error {
		var userID uuid.UUID
		err := tx.QueryRow(ctx, `SELECT user_id FROM user_identities WHERE connection_id = $1 AND subject = $2`, c.ID, claims.Subject).Scan(&userID)
		newLink := errors.Is(err, pgx.ErrNoRows)
		if err != nil && !newLink {
			return err
		}
		domain := claims.Email[strings.LastIndex(claims.Email, "@")+1:]
		var domainVerified bool
		_ = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM org_domains WHERE org_id = $1 AND domain = $2 AND verified_at IS NOT NULL)`,
			c.OrgID, domain).Scan(&domainVerified)
		created := false
		if newLink {
			existing, err := q.GetUserByEmail(ctx, claims.Email)
			switch {
			case err == nil && domainVerified:
				userID = existing.ID
			case err == nil:
				// Unverified domain: never link an existing account by email.
				return forbidden("sso_account_exists", "an account with this email exists; ask your administrator to verify the email domain")
			case isNotFound(err) && c.JIT:
				name := strings.TrimSpace(claims.GivenName + " " + claims.FamilyName)
				if name == "" {
					name = strings.TrimSpace(claims.Name)
				}
				if name == "" {
					name = claims.Email[:strings.Index(claims.Email, "@")]
				}
				u, err := q.CreateUser(ctx, dbgen.CreateUserParams{ID: idgen.New(), Email: claims.Email, Name: name, Locale: "en", Timezone: "UTC"})
				if err != nil {
					return err
				}
				if err := q.MarkEmailVerified(ctx, u.ID); err != nil {
					return err
				}
				userID, created = u.ID, true
			case isNotFound(err):
				pd := forbidden("sso_user_not_provisioned", "your account hasn't been provisioned for this organisation")
				return pd
			default:
				return err
			}
			raw, _ := json.Marshal(claims.Raw)
			if _, err := tx.Exec(ctx, `INSERT INTO user_identities (id, user_id, connection_id, subject, email_at_login, raw_claims, last_login_at)
				VALUES ($1, $2, $3, $4, $5, $6, now())`, idgen.New(), userID, c.ID, claims.Subject, claims.Email, raw); err != nil {
				return err
			}
		} else {
			raw, _ := json.Marshal(claims.Raw)
			if _, err := tx.Exec(ctx, `UPDATE user_identities SET email_at_login = $3, raw_claims = $4, last_login_at = now()
				WHERE connection_id = $1 AND subject = $2`, c.ID, claims.Subject, claims.Email, raw); err != nil {
				return err
			}
		}
		// Organisation membership.
		m, err := org.GetMember(ctx, tx, c.OrgID, userID)
		switch {
		case errors.Is(err, org.ErrNotMember):
			if !c.JIT && !created {
				return forbidden("sso_user_not_provisioned", "your account isn't a member of this organisation")
			}
			if _, err := tx.Exec(ctx, `INSERT INTO org_members (org_id, user_id, org_role, source) VALUES ($1, $2, 'member', 'sso_jit')`,
				c.OrgID, userID); err != nil {
				return err
			}
		case err != nil:
			return err
		case m.Status != "active":
			return forbidden("org_access_suspended", "your access to this organisation is suspended")
		}
		// Default workspace for JIT users without any grant there.
		if c.DefaultWorkspaceID != nil {
			var has bool
			_ = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2)`,
				*c.DefaultWorkspaceID, userID).Scan(&has)
			if !has {
				if err := q.AddWorkspaceMember(ctx, dbgen.AddWorkspaceMemberParams{WorkspaceID: *c.DefaultWorkspaceID, UserID: userID,
					Role: c.DefaultRoleKey}); err != nil {
					return err
				}
				if err := org.BindMemberRole(ctx, tx, c.OrgID, *c.DefaultWorkspaceID, userID, c.DefaultRoleKey, nil); err != nil {
					return err
				}
			}
		}
		if err := syncSSOGroups(ctx, tx, c.OrgID, userID, claims.Groups); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE sso_connections SET last_login_at = now() WHERE id = $1`, c.ID); err != nil {
			return err
		}
		uid := userID
		action := "auth.sso.login"
		if created {
			action = "auth.sso.user_provisioned"
		}
		if err := s.aud().Record(ctx, q, auditEntryFor(r, &c.OrgID, uid, action, map[string]any{"connection": c.ID, "groups": len(claims.Groups)})); err != nil {
			return err
		}
		user, err = q.GetUserByID(ctx, userID)
		return err
	})
	if err == nil {
		s.access.Invalidate(ctx)
	}
	return user, err
}

// syncSSOGroups makes the user's SSO-sourced group memberships match the IdP's claim.
func syncSSOGroups(ctx context.Context, tx pgx.Tx, orgID, userID uuid.UUID, groups []string) error {
	if len(groups) > 200 {
		groups = groups[:200]
	}
	keep := []uuid.UUID{}
	for _, name := range groups {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 200 {
			continue
		}
		// A group of the same name that SCIM or an admin manages is left alone: those
		// memberships have their own source of truth.
		var gid uuid.UUID
		err := tx.QueryRow(ctx, `INSERT INTO groups (id, org_id, display_name, source) VALUES ($1, $2, $3, 'sso')
			ON CONFLICT (org_id, display_name) DO UPDATE SET updated_at = groups.updated_at WHERE groups.source = 'sso'
			RETURNING id`, idgen.New(), orgID, name).Scan(&gid)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		keep = append(keep, gid)
		if _, err := tx.Exec(ctx, `INSERT INTO group_members (group_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, gid, userID); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `DELETE FROM group_members gm USING groups g
		WHERE g.id = gm.group_id AND g.org_id = $1 AND g.source = 'sso' AND gm.user_id = $2 AND NOT (gm.group_id = ANY($3))`,
		orgID, userID, keep)
	return err
}
