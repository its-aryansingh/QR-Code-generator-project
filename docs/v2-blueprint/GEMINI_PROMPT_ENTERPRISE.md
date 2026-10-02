# GEMINI ADD-ON PROMPT — QRit v2 ENTERPRISE PHASES (E0–E8)

Paste this **after** the full `GEMINI_PROMPT.md`. Everything in that prompt stays in force, especially **§0 OUTPUT PROTOCOL**, §1 stack, §3 layering, §4 naming, §10 security, §16 forbidden. This add-on only **adds** to it. If the two conflict on an enterprise concern, this add-on wins. The repository already contains blueprint Phases 1–4. Generate only enterprise phases, one per request, starting with **E0**, each beginning with `docs/manifest/phase-E<N>.txt`.

## A. STACK ADDITIONS (use exactly these)

| Area | Choice |
|---|---|
| OIDC relying party | `github.com/coreos/go-oidc/v3/oidc` + `golang.org/x/oauth2` |
| SAML | **Not implemented in Go.** Use the `sso-bridge` service (Ory Polis, pinned official image) via its OAuth endpoints `/api/oauth/authorize`, `/api/oauth/token`, `/api/oauth/userinfo` and its connection admin API (`/api/v1/sso`, API-key auth). Wrap it in `internal/identity/polis`. Pin the image by digest in `deploy/`; `// DECISION:` the exact image reference and env names against the pinned Polis docs |
| TOTP | `github.com/pquerna/otp/totp` |
| WebAuthn | `github.com/go-webauthn/webauthn/webauthn` |
| Phone numbers | `github.com/nyaruka/phonenumbers` |
| S3 / GCS (HMAC) writers | `github.com/aws/aws-sdk-go-v2/service/s3` (custom endpoint for GCS/R2) |
| BigQuery | `cloud.google.com/go/bigquery` |
| EPS | Ghostscript (`gs`) installed in the `render` image (apt `ghostscript`), called with `-dSAFER -sDEVICE=eps2write` |
| Zapier app | `integrations/zapier/` using `zapier-platform-core` (Node 24) |
| DNS verification | DNS-over-HTTPS JSON API (`DOH_URL`, default `https://cloudflare-dns.com/dns-query`); never fetch customer hosts for verification |

## B. REPOSITORY ADDITIONS

```
services/internal/{org,authz,flags,events,identity/{polis,oidcdirect,workos},mfa,scim,approval,auditstream,
                   crypto/envelope,integrations/{slack,teams,zapier,hubspot,salesforce,gsheets,ga4,warehouse},
                   alerts,reports,forms,dsar,pixels,gs1,serial,billing/{contracts,invoice},branding,agency,
                   support,apiusage,print}/
services/internal/middleware/{orggate.go,stepup.go}
services/db/migrations/00003_enterprise.sql
services/db/queries/{enterprise_authz.sql,enterprise_resolve.sql,enterprise_gs1.sql,enterprise_alerts.sql,
                     enterprise_serial.sql,enterprise_approvals.sql, …one file per new aggregate}
services/db/tests/enterprise_test.go         # Go port of tests/test_enterprise.py (same 38 assertions)
apps/web/src/app/(auth)/{sso,mfa}/…          apps/web/src/app/o/[org]/settings/{general,members,domains,sso,directory,
                                               security,roles,groups,audit,audit-streams,billing,branding,clients,support-access,sandbox}/page.tsx
apps/web/src/app/w/[workspace]/{approvals,access,forms,leads,pixels,gs1,serials,integrations,alerts,reports}/…
apps/web/src/app/{v/[serial],c/[token]}/page.tsx      # verify page, consent withdrawal page
apps/web/src/app/admin/{orgs,contracts,invoices,flags,support}/…
apps/render/src/{cmyk.ts,eps.ts,sheets.ts,report.ts,charts.ts,invoice.ts}
integrations/zapier/…      deploy/fly/sso-bridge.toml      docs/{runbooks,compliance}/…
```

## C. DATABASE

Create `services/db/migrations/00003_enterprise.sql` with `-- +goose Up` containing **exactly** this SQL (do not edit it), and a `-- +goose Down` that drops the new objects in reverse order and restores the altered constraints:

```sql
-- =====================================================================
-- QRit v2 — Enterprise layer. goose migration 00003_enterprise.sql
-- Applies on top of 00001_init.sql (+ 00002_river). Safe on an empty DB
-- and on a DB that already holds Phase 1–4 data (backfills included).
-- Conventions as in 00001. CIDR lists below are *policy configuration*
-- entered by admins, not visitor data; the "no visitor IPs" rule still holds.
-- =====================================================================

-- ---------------------------------------------------------------------
-- E1. Organizations: the enterprise tenant above workspaces
-- ---------------------------------------------------------------------
CREATE TABLE organizations (
    id             uuid PRIMARY KEY,
    name           text   NOT NULL,
    slug           citext NOT NULL UNIQUE,
    kind           text   NOT NULL DEFAULT 'standard'
                   CHECK (kind IN ('personal','standard','enterprise','agency')),
    parent_org_id  uuid REFERENCES organizations(id) ON DELETE RESTRICT,   -- agency → client orgs
    plan_id        text   NOT NULL DEFAULT 'free'
                   CHECK (plan_id IN ('free','pro','business','enterprise')),
    data_region    text   NOT NULL DEFAULT 'in' CHECK (data_region IN ('in','eu','us')),
    legal_name     text,
    billing_email  citext,
    gstin          text CHECK (gstin IS NULL OR gstin ~ '^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$'),
    tax_country    char(2) NOT NULL DEFAULT 'IN',
    billing_address jsonb NOT NULL DEFAULT '{}'::jsonb,  -- {line1, line2, city, state_code, postal_code, country}
    settings       jsonb  NOT NULL DEFAULT '{}'::jsonb,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz,
    CONSTRAINT org_no_self_parent CHECK (parent_org_id IS NULL OR parent_org_id <> id)
);
CREATE INDEX organizations_parent_idx ON organizations (parent_org_id) WHERE parent_org_id IS NOT NULL;

CREATE TABLE org_members (
    org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_role    text NOT NULL DEFAULT 'member'
                CHECK (org_role IN ('org_owner','org_admin','billing_admin','member')),
    status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deprovisioned')),
    source      text NOT NULL DEFAULT 'invite' CHECK (source IN ('invite','sso_jit','scim','creator')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, user_id)
);
CREATE INDEX org_members_user_idx ON org_members (user_id);
CREATE UNIQUE INDEX org_one_owner_uniq ON org_members (org_id) WHERE org_role = 'org_owner';

-- Backfill: one organization per existing workspace (same name/slug), owner carried over.
ALTER TABLE workspaces ADD COLUMN org_id uuid REFERENCES organizations(id) ON DELETE CASCADE;

INSERT INTO organizations (id, name, slug, kind, plan_id, created_at)
SELECT w.id, w.name, w.slug, 'standard', w.plan_id, w.created_at FROM workspaces w;   -- org id := workspace id (1:1 at migration time)
UPDATE workspaces SET org_id = id;
INSERT INTO org_members (org_id, user_id, org_role, source, created_at)
SELECT w.id, w.owner_id, 'org_owner', 'creator', w.created_at FROM workspaces w;
INSERT INTO org_members (org_id, user_id, org_role, source, created_at)
SELECT wm.workspace_id, wm.user_id, 'member', 'invite', wm.created_at
FROM workspace_members wm
WHERE wm.role <> 'owner'
ON CONFLICT DO NOTHING;

ALTER TABLE workspaces ALTER COLUMN org_id SET NOT NULL;
CREATE INDEX workspaces_org_idx ON workspaces (org_id);
COMMENT ON COLUMN workspaces.plan_id IS 'DEPRECATED since 00003: entitlements resolve from organizations.plan_id. Dropped in a later contract migration.';

-- Subscriptions move to the organization (consolidated billing).
ALTER TABLE subscriptions ADD COLUMN org_id uuid REFERENCES organizations(id) ON DELETE CASCADE;
UPDATE subscriptions s SET org_id = w.org_id FROM workspaces w WHERE w.id = s.workspace_id;
ALTER TABLE subscriptions ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE subscriptions ALTER COLUMN workspace_id DROP NOT NULL;
CREATE UNIQUE INDEX subscriptions_org_uniq ON subscriptions (org_id);

-- Claimed email domains: SSO routing ("you@acme.com → Acme's IdP") and auto-join.
CREATE TABLE org_domains (
    id                  uuid PRIMARY KEY,
    org_id              uuid   NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    domain              citext NOT NULL UNIQUE,
    verification_token  text   NOT NULL,
    verified_at         timestamptz,
    auto_join           boolean NOT NULL DEFAULT false,      -- verified-domain users may join without invite
    auto_join_workspace_id uuid REFERENCES workspaces(id) ON DELETE SET NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------
-- E2. Identity: SSO connections, linked identities, MFA, SCIM
-- ---------------------------------------------------------------------
CREATE TABLE sso_connections (
    id                      uuid PRIMARY KEY,
    org_id                  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    protocol                text NOT NULL CHECK (protocol IN ('saml','oidc')),
    label                   text NOT NULL,                      -- "Okta", "Entra ID"
    status                  text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','testing','active','disabled')),
    bridge_tenant           text,        -- SAML: Ory Polis tenant (= org slug); product = 'qrit'
    oidc_issuer             text,        -- OIDC direct: https://login.microsoftonline.com/{tid}/v2.0
    oidc_client_id          text,
    oidc_client_secret_ct   bytea,       -- AES-256-GCM
    jit_provisioning        boolean NOT NULL DEFAULT true,
    default_workspace_id    uuid REFERENCES workspaces(id) ON DELETE SET NULL,
    default_role_key        text NOT NULL DEFAULT 'analyst',
    attribute_mapping       jsonb NOT NULL DEFAULT '{"email":"email","first_name":"firstName","last_name":"lastName","groups":"groups"}'::jsonb,
    last_login_at           timestamptz,
    created_by              uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sso_oidc_fields CHECK (protocol <> 'oidc' OR (oidc_issuer IS NOT NULL AND oidc_client_id IS NOT NULL))
);
CREATE INDEX sso_connections_org_idx ON sso_connections (org_id);

CREATE TABLE user_identities (
    id              uuid PRIMARY KEY,
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    connection_id   uuid NOT NULL REFERENCES sso_connections(id) ON DELETE CASCADE,
    subject         text NOT NULL,        -- SAML NameID / OIDC sub (never email alone)
    email_at_login  citext NOT NULL,
    raw_claims      jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_login_at   timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (connection_id, subject)
);

CREATE TABLE user_mfa_factors (
    id              uuid PRIMARY KEY,
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind            text NOT NULL CHECK (kind IN ('totp','webauthn')),
    name            text NOT NULL,
    totp_secret_ct  bytea,
    credential_id   bytea UNIQUE,
    public_key      bytea,
    sign_count      bigint NOT NULL DEFAULT 0,
    aaguid          uuid,
    transports      text[],
    created_at      timestamptz NOT NULL DEFAULT now(),
    last_used_at    timestamptz,
    CONSTRAINT mfa_kind_fields CHECK (
        (kind = 'totp' AND totp_secret_ct IS NOT NULL)
     OR (kind = 'webauthn' AND credential_id IS NOT NULL AND public_key IS NOT NULL))
);
CREATE TABLE user_recovery_codes (
    user_id    uuid  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  bytea NOT NULL,
    used_at    timestamptz,
    PRIMARY KEY (user_id, code_hash)
);

ALTER TABLE sessions
    ADD COLUMN auth_method       text NOT NULL DEFAULT 'password'
        CHECK (auth_method IN ('password','google','magic_link','sso')),
    ADD COLUMN sso_connection_id uuid REFERENCES sso_connections(id) ON DELETE SET NULL,
    ADD COLUMN mfa_verified_at   timestamptz,
    ADD COLUMN step_up_at        timestamptz;   -- last re-auth for sensitive actions

CREATE TABLE scim_directories (
    id                  uuid PRIMARY KEY,
    org_id              uuid  NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    label               text  NOT NULL,
    token_prefix        text  NOT NULL UNIQUE,         -- 'scim_7F3K2M9Q' shown in UI
    token_hash          bytea NOT NULL UNIQUE,         -- sha256(token)
    status              text  NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
    deprovision_action  text  NOT NULL DEFAULT 'suspend' CHECK (deprovision_action IN ('suspend','remove')),
    last_request_at     timestamptz,
    created_by          uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE scim_users (
    id            uuid PRIMARY KEY,                       -- returned to the IdP as SCIM "id"
    directory_id  uuid   NOT NULL REFERENCES scim_directories(id) ON DELETE CASCADE,
    user_id       uuid   NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_id   text,
    user_name     text   NOT NULL,                        -- stored exactly as sent (Entra requirement)
    active        boolean NOT NULL DEFAULT true,
    resource      jsonb  NOT NULL,                        -- last full SCIM representation
    version       integer NOT NULL DEFAULT 1,             -- ETag W/"n"
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (directory_id, user_name),
    UNIQUE (directory_id, user_id)
);
CREATE UNIQUE INDEX scim_users_external_uniq ON scim_users (directory_id, external_id) WHERE external_id IS NOT NULL;

-- Groups: manual (created in UI) or SCIM-sourced. Roles can be bound to groups.
CREATE TABLE groups (
    id            uuid PRIMARY KEY,
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    display_name  text NOT NULL,
    source        text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','scim','sso')),   -- sso = synced from the IdP groups claim at login
    directory_id  uuid REFERENCES scim_directories(id) ON DELETE CASCADE,
    external_id   text,
    version       integer NOT NULL DEFAULT 1,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, display_name),                       -- Entra matches groups on displayName
    CONSTRAINT group_scim_dir CHECK ((source = 'scim') = (directory_id IS NOT NULL))
);
CREATE TABLE group_members (
    group_id  uuid NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id   uuid NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    added_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX group_members_user_idx ON group_members (user_id);

-- ---------------------------------------------------------------------
-- E3. Authorization: permission-based roles, bindings to users or groups,
--     at workspace or folder scope
-- ---------------------------------------------------------------------
CREATE TABLE roles (
    id           uuid PRIMARY KEY,
    org_id       uuid REFERENCES organizations(id) ON DELETE CASCADE,   -- NULL = system role
    key          text NOT NULL CHECK (key ~ '^[a-z][a-z0-9_]{1,40}$'),
    name         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    permissions  text[] NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (org_id, key)
);

INSERT INTO roles (id, org_id, key, name, description, permissions) VALUES
 ('00000000-0000-7000-8000-00000000f001', NULL, 'owner', 'Owner', 'Everything, including billing and deletion',
  ARRAY['*']),
 ('00000000-0000-7000-8000-00000000f002', NULL, 'admin', 'Admin', 'Manage people, domains, keys, integrations and policies',
  ARRAY['workspace.read','workspace.update','member.manage','role.manage','qr.read','qr.create','qr.update','qr.delete',
        'qr.destination.update','qr.destination.approve','qr.design.bypass_lock','folder.manage','campaign.manage',
        'template.manage','analytics.read','analytics.export','analytics.raw','domain.manage','apikey.manage',
        'webhook.manage','integration.manage','policy.manage','audit.read','audit.export','form.manage',
        'lead.read','lead.export','pixel.manage','alert.manage','report.manage','serial.manage','gs1.manage','bulk.run']),
 ('00000000-0000-7000-8000-00000000f003', NULL, 'editor', 'Editor', 'Create and edit QR codes and campaigns',
  ARRAY['workspace.read','qr.read','qr.create','qr.update','qr.destination.update','folder.manage','campaign.manage',
        'template.manage','analytics.read','analytics.export','form.manage','lead.read','alert.manage','report.manage','bulk.run']),
 ('00000000-0000-7000-8000-00000000f004', NULL, 'reviewer', 'Reviewer', 'Approve or reject changes; read everything',
  ARRAY['workspace.read','qr.read','qr.destination.approve','analytics.read','audit.read','lead.read']),
 ('00000000-0000-7000-8000-00000000f005', NULL, 'analyst', 'Analyst', 'Read-only access to codes and analytics',
  ARRAY['workspace.read','qr.read','analytics.read','analytics.export']);

CREATE TABLE role_bindings (
    id              uuid PRIMARY KEY,
    org_id          uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id    uuid NOT NULL REFERENCES workspaces(id)    ON DELETE CASCADE,
    principal_type  text NOT NULL CHECK (principal_type IN ('user','group')),
    principal_id    uuid NOT NULL,
    role_id         uuid NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    scope_type      text NOT NULL DEFAULT 'workspace' CHECK (scope_type IN ('workspace','folder')),
    folder_id       uuid REFERENCES folders(id) ON DELETE CASCADE,
    created_by      uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT binding_scope CHECK ((scope_type = 'folder') = (folder_id IS NOT NULL)),
    UNIQUE NULLS NOT DISTINCT (workspace_id, principal_type, principal_id, role_id, folder_id)
);
CREATE INDEX role_bindings_principal_idx ON role_bindings (principal_type, principal_id);
CREATE INDEX role_bindings_ws_idx        ON role_bindings (workspace_id);

-- Backfill bindings from existing memberships; allow the new reviewer role on workspace_members.
INSERT INTO role_bindings (id, org_id, workspace_id, principal_type, principal_id, role_id, created_at)
SELECT gen_random_uuid(), w.org_id, wm.workspace_id, 'user', wm.user_id, r.id, wm.created_at
FROM workspace_members wm
JOIN workspaces w ON w.id = wm.workspace_id
JOIN roles r ON r.org_id IS NULL AND r.key = wm.role;

ALTER TABLE workspace_members DROP CONSTRAINT workspace_members_role_check;
ALTER TABLE workspace_members ADD CONSTRAINT workspace_members_role_check
    CHECK (role IN ('owner','admin','editor','reviewer','analyst','custom'));
COMMENT ON COLUMN workspace_members.role IS 'Display-only primary role. Authorization reads role_bindings.';

-- ---------------------------------------------------------------------
-- E4. Policies (enforced — see plan §E4 enforcement matrix)
-- ---------------------------------------------------------------------
CREATE TABLE org_security_policies (
    org_id                  uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    enforce_sso             boolean NOT NULL DEFAULT false,
    sso_break_glass_user_ids uuid[] NOT NULL DEFAULT '{}',     -- ≤ 2 owners who may still use password+MFA
    require_mfa             boolean NOT NULL DEFAULT false,     -- applies to non-SSO logins
    allowed_mfa_kinds       text[]  NOT NULL DEFAULT ARRAY['totp','webauthn'],
    session_idle_minutes    integer NOT NULL DEFAULT 0 CHECK (session_idle_minutes = 0 OR session_idle_minutes BETWEEN 5 AND 10080),
    session_max_hours       integer NOT NULL DEFAULT 0 CHECK (session_max_hours = 0 OR session_max_hours BETWEEN 1 AND 2160),
    dashboard_ip_allowlist  cidr[]  NOT NULL DEFAULT '{}',
    api_ip_allowlist        cidr[]  NOT NULL DEFAULT '{}',
    password_min_length     integer NOT NULL DEFAULT 10 CHECK (password_min_length BETWEEN 10 AND 128),
    invite_email_domains    citext[] NOT NULL DEFAULT '{}',     -- empty = any
    api_key_max_days        integer NOT NULL DEFAULT 0 CHECK (api_key_max_days >= 0),   -- 0 = no expiry required
    export_permission       text    NOT NULL DEFAULT 'role'  CHECK (export_permission IN ('role','admins_only','disabled')),
    updated_by              uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_at              timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspace_policies (
    workspace_id               uuid PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    allowed_destination_hosts  text[]  NOT NULL DEFAULT '{}',   -- exact or '*.suffix'; empty = any
    blocked_destination_hosts  text[]  NOT NULL DEFAULT '{}',
    require_https              boolean NOT NULL DEFAULT true,
    approval_mode              text    NOT NULL DEFAULT 'off'
                               CHECK (approval_mode IN ('off','outside_allowlist','all_destination_changes','all_changes')),
    approvals_required         integer NOT NULL DEFAULT 1 CHECK (approvals_required BETWEEN 1 AND 3),
    approval_expiry_hours      integer NOT NULL DEFAULT 168 CHECK (approval_expiry_hours BETWEEN 1 AND 720),
    require_template           boolean NOT NULL DEFAULT false,  -- editors must pick a template
    pixel_consent_mode         text    NOT NULL DEFAULT 'opt_in_all'
                               CHECK (pixel_consent_mode IN ('opt_in_all','opt_in_where_required')),
    disabled_features          text[]  NOT NULL DEFAULT '{}',   -- org admin can switch features off per workspace
    updated_by                 uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_at                 timestamptz NOT NULL DEFAULT now()
);
INSERT INTO workspace_policies (workspace_id) SELECT id FROM workspaces;
INSERT INTO org_security_policies (org_id) SELECT id FROM organizations;

-- ---------------------------------------------------------------------
-- E5. Approval workflow (maker–checker) on destination versions
-- ---------------------------------------------------------------------
ALTER TABLE qr_versions ADD COLUMN approval_status text NOT NULL DEFAULT 'not_required'
    CHECK (approval_status IN ('not_required','pending','approved','rejected','cancelled'));
CREATE INDEX qr_versions_pending_idx ON qr_versions (qr_code_id) WHERE approval_status = 'pending';

CREATE TABLE approval_requests (
    id                  uuid PRIMARY KEY,
    workspace_id        uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    kind                text NOT NULL CHECK (kind IN ('create','destination','rules','bulk_update')),
    qr_code_id          uuid REFERENCES qr_codes(id) ON DELETE CASCADE,   -- single-code requests
    job_id              uuid REFERENCES jobs(id)     ON DELETE CASCADE,   -- bulk requests
    reasons             text[] NOT NULL,         -- e.g. {'host_not_allowlisted','policy_all_changes'}
    requested_by        uuid NOT NULL REFERENCES users(id),
    required_approvals  integer NOT NULL CHECK (required_approvals BETWEEN 1 AND 3),
    status              text NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','approved','rejected','cancelled','expired')),
    note                text,
    override_reason     text,                    -- set when an org owner used break-glass publish
    expires_at          timestamptz NOT NULL,
    decided_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT approval_subject CHECK (
        (kind = 'bulk_update' AND job_id IS NOT NULL AND qr_code_id IS NULL)
     OR (kind <> 'bulk_update' AND qr_code_id IS NOT NULL AND job_id IS NULL))
);
ALTER TABLE qr_versions ADD COLUMN approval_request_id uuid REFERENCES approval_requests(id) ON DELETE SET NULL;
ALTER TABLE qr_versions ADD CONSTRAINT qr_version_pending_has_request
    CHECK (approval_status NOT IN ('pending','approved','rejected') OR approval_request_id IS NOT NULL);
CREATE INDEX qr_versions_approval_req_idx ON qr_versions (approval_request_id) WHERE approval_request_id IS NOT NULL;
CREATE INDEX approval_requests_ws_pending_idx ON approval_requests (workspace_id, created_at) WHERE status = 'pending';

CREATE TABLE approval_decisions (
    request_id   uuid NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    approver_id  uuid NOT NULL REFERENCES users(id),
    decision     text NOT NULL CHECK (decision IN ('approve','reject')),
    comment      text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (request_id, approver_id)
);

-- Four-eyes rule enforced in the database, not only in code.
CREATE FUNCTION approval_no_self_approval() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM approval_requests r
               WHERE r.id = NEW.request_id AND r.requested_by = NEW.approver_id) THEN
        RAISE EXCEPTION 'requester cannot decide on their own approval request'
            USING ERRCODE = 'check_violation';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM approval_requests r
                   WHERE r.id = NEW.request_id AND r.status = 'pending' AND r.expires_at > now()) THEN
        RAISE EXCEPTION 'approval request is not pending' USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER approval_decisions_guard BEFORE INSERT ON approval_decisions
    FOR EACH ROW EXECUTE FUNCTION approval_no_self_approval();

-- ---------------------------------------------------------------------
-- E6. Tamper-evident audit log (hash chain per org, sealed asynchronously)
-- ---------------------------------------------------------------------
ALTER TABLE audit_logs
    ADD COLUMN org_id     uuid,
    ADD COLUMN seq        bigint,          -- 1..n per org, assigned at sealing
    ADD COLUMN prev_hash  bytea,
    ADD COLUMN hash       bytea,
    ADD COLUMN sealed_at  timestamptz;
UPDATE audit_logs a SET org_id = w.org_id FROM workspaces w WHERE w.id = a.workspace_id;
CREATE INDEX audit_logs_unsealed_idx ON audit_logs (org_id, id) WHERE hash IS NULL;
CREATE UNIQUE INDEX audit_logs_org_seq_uniq ON audit_logs (org_id, seq) WHERE seq IS NOT NULL;

-- Canonical bytes of an entry. jsonb::text is deterministic (keys sorted, fixed spacing).
CREATE FUNCTION audit_entry_bytes(a audit_logs) RETURNS bytea LANGUAGE sql IMMUTABLE AS $$
    SELECT convert_to(concat_ws(E'\x1f',
        a.id::text, coalesce(a.org_id::text,''), coalesce(a.workspace_id::text,''), a.actor_type,
        coalesce(a.actor_id::text,''), a.action, a.target_type, coalesce(a.target_id::text,''),
        a.changes::text, coalesce(a.ip_prefix,''), coalesce(a.request_id,''),
        (extract(epoch FROM a.created_at) * 1000000)::bigint::text), 'UTF8')
$$;

-- Append-only: rows may only be touched once, by the sealer, filling the chain columns.
CREATE FUNCTION audit_logs_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF current_setting('qrit.audit_retention', true) = 'on' THEN RETURN OLD; END IF;
        RAISE EXCEPTION 'audit_logs is append-only' USING ERRCODE = 'insufficient_privilege';
    END IF;
    IF OLD.hash IS NOT NULL
       OR NEW.id IS DISTINCT FROM OLD.id OR NEW.org_id IS DISTINCT FROM OLD.org_id
       OR NEW.workspace_id IS DISTINCT FROM OLD.workspace_id OR NEW.actor_type IS DISTINCT FROM OLD.actor_type
       OR NEW.actor_id IS DISTINCT FROM OLD.actor_id OR NEW.action IS DISTINCT FROM OLD.action
       OR NEW.target_type IS DISTINCT FROM OLD.target_type OR NEW.target_id IS DISTINCT FROM OLD.target_id
       OR NEW.changes IS DISTINCT FROM OLD.changes OR NEW.ip_prefix IS DISTINCT FROM OLD.ip_prefix
       OR NEW.user_agent IS DISTINCT FROM OLD.user_agent OR NEW.request_id IS DISTINCT FROM OLD.request_id
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'audit_logs is append-only' USING ERRCODE = 'insufficient_privilege';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER audit_logs_append_only BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_guard();

-- Seal up to p_limit unsealed entries of one org, in id order. Called by the worker every 5 s.
-- The advisory lock serialises sealers per org; inserts are never blocked.
CREATE FUNCTION audit_seal(p_org uuid, p_limit integer DEFAULT 1000) RETURNS integer
LANGUAGE plpgsql AS $$
DECLARE
    v_prev bytea; v_seq bigint; r audit_logs; n integer := 0; v_hash bytea;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('audit_seal:' || p_org::text, 0));
    SELECT hash, seq INTO v_prev, v_seq FROM audit_logs
     WHERE org_id = p_org AND seq IS NOT NULL ORDER BY seq DESC LIMIT 1;
    v_prev := coalesce(v_prev, '\x'::bytea); v_seq := coalesce(v_seq, 0);
    FOR r IN SELECT * FROM audit_logs WHERE org_id = p_org AND hash IS NULL ORDER BY id LIMIT p_limit LOOP
        v_seq  := v_seq + 1;
        v_hash := sha256(v_prev || int8send(v_seq) || audit_entry_bytes(r));
        UPDATE audit_logs SET seq = v_seq, prev_hash = v_prev, hash = v_hash, sealed_at = now() WHERE id = r.id;
        v_prev := v_hash; n := n + 1;
    END LOOP;
    RETURN n;
END $$;

-- Verify the chain; returns the first broken seq, or NULL when intact.
-- After retention pruning, verification starts from the oldest remaining entry's prev_hash,
-- which the worker cross-checks against the WORM anchor for that day.
CREATE FUNCTION audit_verify(p_org uuid) RETURNS bigint LANGUAGE plpgsql STABLE AS $$
DECLARE v_prev bytea; r audit_logs;
BEGIN
    SELECT CASE WHEN seq = 1 THEN '\x'::bytea ELSE prev_hash END INTO v_prev
      FROM audit_logs WHERE org_id = p_org AND seq IS NOT NULL ORDER BY seq LIMIT 1;
    FOR r IN SELECT * FROM audit_logs WHERE org_id = p_org AND seq IS NOT NULL ORDER BY seq LOOP
        IF r.prev_hash IS DISTINCT FROM v_prev
           OR r.hash IS DISTINCT FROM sha256(v_prev || int8send(r.seq) || audit_entry_bytes(r)) THEN
            RETURN r.seq;
        END IF;
        v_prev := r.hash;
    END LOOP;
    RETURN NULL;
END $$;

-- Daily anchors: the head hash per org per day, also written to WORM storage (R2 object lock).
CREATE TABLE audit_anchors (
    org_id      uuid NOT NULL,
    day         date NOT NULL,
    last_seq    bigint NOT NULL,
    head_hash   bytea NOT NULL,
    object_key  text,                    -- r2://qrit-audit-worm/{org}/{day}.json
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, day)
);

CREATE TABLE audit_streams (
    id            uuid PRIMARY KEY,
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    kind          text NOT NULL CHECK (kind IN ('webhook','splunk_hec','datadog','s3')),
    config        jsonb NOT NULL,         -- endpoint/region/bucket/prefix (no secrets)
    secret_ct     bytea,
    cursor_seq    bigint NOT NULL DEFAULT 0,   -- last streamed seq
    status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused','error')),
    last_error    text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------
-- E7. Integrations, alerts, scheduled reports
-- ---------------------------------------------------------------------
CREATE TABLE integrations (
    id               uuid PRIMARY KEY,
    workspace_id     uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider         text NOT NULL CHECK (provider IN (
                         'slack','teams','zapier','make','hubspot','salesforce','google_sheets','ga4',
                         'warehouse_s3','warehouse_gcs','warehouse_bigquery')),
    name             text NOT NULL,
    status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active','error','disabled')),
    config           jsonb NOT NULL DEFAULT '{}'::jsonb,
    credentials_ct   bytea,                 -- OAuth tokens / API secrets, AES-256-GCM
    events           text[] NOT NULL DEFAULT '{}',
    last_success_at  timestamptz,
    last_error       text,
    created_by       uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX integrations_ws_idx ON integrations (workspace_id) WHERE status = 'active';

CREATE TABLE integration_deliveries (
    id               uuid PRIMARY KEY,
    integration_id   uuid NOT NULL REFERENCES integrations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL,
    event_type       text NOT NULL,
    status           text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','succeeded','failed','dead')),
    attempts         integer NOT NULL DEFAULT 0,
    last_error       text,
    next_attempt_at  timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    delivered_at     timestamptz,
    UNIQUE (integration_id, event_id)
);

CREATE TABLE alert_rules (
    id                uuid PRIMARY KEY,
    workspace_id      uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name              text NOT NULL,
    kind              text NOT NULL CHECK (kind IN ('scan_spike','scan_drop','scan_threshold','no_scans',
                          'destination_down','serial_anomaly','security_event','approval_pending')),
    target_type       text NOT NULL CHECK (target_type IN ('workspace','qr_code','campaign','serial_batch')),
    target_id         uuid,
    params            jsonb NOT NULL DEFAULT '{}'::jsonb,   -- {"z": 3, "min_scans": 20} / {"threshold": 1000}
    channels          jsonb NOT NULL DEFAULT '{"emails":[],"integration_ids":[]}'::jsonb,
    cooldown_minutes  integer NOT NULL DEFAULT 60 CHECK (cooldown_minutes BETWEEN 5 AND 10080),
    is_active         boolean NOT NULL DEFAULT true,
    last_fired_at     timestamptz,
    created_by        uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT alert_target CHECK ((target_type = 'workspace') = (target_id IS NULL))
);
CREATE TABLE alert_events (
    id         uuid PRIMARY KEY,
    rule_id    uuid NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    fired_at   timestamptz NOT NULL DEFAULT now(),
    payload    jsonb NOT NULL,
    resolved_at timestamptz
);
CREATE INDEX alert_events_rule_idx ON alert_events (rule_id, fired_at DESC);

CREATE TABLE report_schedules (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         text NOT NULL,
    frequency    text NOT NULL CHECK (frequency IN ('daily','weekly','monthly')),
    weekday      integer CHECK (weekday BETWEEN 1 AND 7),
    month_day    integer CHECK (month_day BETWEEN 1 AND 28),
    hour_local   integer NOT NULL DEFAULT 9 CHECK (hour_local BETWEEN 0 AND 23),
    timezone     text NOT NULL,
    filters      jsonb NOT NULL DEFAULT '{}'::jsonb,   -- same shape as analytics filters
    format       text NOT NULL DEFAULT 'pdf' CHECK (format IN ('pdf','csv','both')),
    recipients   citext[] NOT NULL CHECK (cardinality(recipients) BETWEEN 1 AND 50),
    is_active    boolean NOT NULL DEFAULT true,
    next_run_at  timestamptz NOT NULL,
    last_run_at  timestamptz,
    created_by   uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX report_schedules_due_idx ON report_schedules (next_run_at) WHERE is_active;

-- ---------------------------------------------------------------------
-- E8. Lead capture (encrypted PII, DPDP/GDPR consent) and retargeting pixels
-- ---------------------------------------------------------------------
CREATE TABLE org_data_keys (
    org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key_id      integer NOT NULL,
    dek_ct      bytea NOT NULL,          -- data key wrapped with APP_ENCRYPTION_KEY (KMS later)
    created_at  timestamptz NOT NULL DEFAULT now(),
    retired_at  timestamptz,
    PRIMARY KEY (org_id, key_id)
);

CREATE TABLE forms (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            text NOT NULL,
    fields          jsonb NOT NULL,        -- [{key,type,label{en,hi},required,options}] ≤ 25
    notice          jsonb NOT NULL,        -- itemised consent notice per language + purposes[] + controller + grievance contact
    notice_version  integer NOT NULL DEFAULT 1,
    double_opt_in   boolean NOT NULL DEFAULT false,
    retention_days  integer NOT NULL DEFAULT 180 CHECK (retention_days BETWEEN 1 AND 1095),
    notify_emails   citext[] NOT NULL DEFAULT '{}',
    is_active       boolean NOT NULL DEFAULT true,
    created_by      uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE form_submissions (
    id                 uuid PRIMARY KEY,
    form_id            uuid NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    workspace_id       uuid NOT NULL,
    qr_code_id         uuid,
    key_id             integer NOT NULL,
    data_ct            bytea NOT NULL,           -- AES-256-GCM(JSON answers) with the org DEK
    email_bidx         bytea,                    -- HMAC-SHA256(org blind-index key, lower(email)) for DSAR lookup
    consent            jsonb NOT NULL,           -- {notice_version, purposes_accepted[], lang, at}
    status             text NOT NULL DEFAULT 'confirmed'
                       CHECK (status IN ('pending_confirmation','confirmed','withdrawn','erased')),
    delete_after       date NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    confirmed_at       timestamptz,
    withdrawn_at       timestamptz
);
CREATE INDEX form_submissions_ws_idx    ON form_submissions (workspace_id, created_at DESC);
CREATE INDEX form_submissions_bidx_idx  ON form_submissions (email_bidx) WHERE email_bidx IS NOT NULL;
CREATE INDEX form_submissions_purge_idx ON form_submissions (delete_after) WHERE status <> 'erased';

CREATE TABLE dsar_requests (
    id              uuid PRIMARY KEY,
    org_id          uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    kind            text NOT NULL CHECK (kind IN ('access','erasure','correction')),
    subject_bidx    bytea NOT NULL,
    status          text NOT NULL DEFAULT 'open' CHECK (status IN ('open','completed','rejected')),
    due_at          timestamptz NOT NULL,
    result_file_id  uuid REFERENCES files(id),
    handled_by      uuid REFERENCES users(id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    completed_at    timestamptz
);

CREATE TABLE pixels (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider      text NOT NULL CHECK (provider IN ('meta','google','linkedin','tiktok')),
    external_id   text NOT NULL CHECK (external_id ~ '^[A-Za-z0-9_-]{3,40}$'),   -- pixel / tag id only, never code
    name          text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider, external_id)
);
CREATE TABLE qr_code_pixels (
    qr_code_id  uuid NOT NULL REFERENCES qr_codes(id) ON DELETE CASCADE,
    pixel_id    uuid NOT NULL REFERENCES pixels(id)   ON DELETE CASCADE,
    PRIMARY KEY (qr_code_id, pixel_id)
);
CREATE TABLE pixel_consent_daily (
    qr_code_id  uuid NOT NULL,
    day         date NOT NULL,
    shown       integer NOT NULL DEFAULT 0,
    accepted    integer NOT NULL DEFAULT 0,
    declined    integer NOT NULL DEFAULT 0,
    auto_fired  integer NOT NULL DEFAULT 0,     -- regions where notice-only applies (workspace setting)
    PRIMARY KEY (qr_code_id, day)
);

-- ---------------------------------------------------------------------
-- E9. GS1 Digital Link conformant resolver data
-- ---------------------------------------------------------------------
CREATE TABLE gs1_items (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    domain_id     uuid NOT NULL REFERENCES domains(id),
    qr_code_id    uuid REFERENCES qr_codes(id) ON DELETE SET NULL,   -- analytics anchor
    gtin          char(14) NOT NULL CHECK (gtin ~ '^[0-9]{14}$'),
    cpv           text,     -- AI 22
    lot           text,     -- AI 10
    serial        text,     -- AI 21
    title         text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (domain_id, gtin, cpv, lot, serial)
);
CREATE INDEX gs1_items_lookup_idx ON gs1_items (domain_id, gtin);

CREATE TABLE gs1_links (
    id            uuid PRIMARY KEY,
    item_id       uuid NOT NULL REFERENCES gs1_items(id) ON DELETE CASCADE,
    link_type     text NOT NULL CHECK (link_type ~ '^gs1:[A-Za-z]+$'),   -- gs1:pip, gs1:nutritionalInfo, gs1:recallStatus …
    href          text NOT NULL,
    title         text NOT NULL,
    hreflang      text[] NOT NULL DEFAULT '{}',
    media_type    text,
    is_default    boolean NOT NULL DEFAULT false,
    position      integer NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX gs1_links_one_default_uniq ON gs1_links (item_id) WHERE is_default;
CREATE INDEX gs1_links_item_idx ON gs1_links (item_id, link_type);

-- ---------------------------------------------------------------------
-- E10. Serialization / product authentication
-- ---------------------------------------------------------------------
ALTER TABLE qr_codes DROP CONSTRAINT qr_codes_content_type_check;
ALTER TABLE qr_codes ADD CONSTRAINT qr_codes_content_type_check CHECK (content_type IN (
    'url','text','email','phone','sms','whatsapp','wifi','vcard','event','upi','location',
    'links_page','file','app_store','gs1','form','serial_batch'));
ALTER TABLE qr_codes DROP CONSTRAINT qr_static_types;
ALTER TABLE qr_codes ADD CONSTRAINT qr_static_types CHECK (
    mode = 'dynamic' OR content_type NOT IN ('links_page','file','app_store','gs1','form','serial_batch'));

CREATE TABLE serial_batches (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    qr_code_id      uuid NOT NULL UNIQUE REFERENCES qr_codes(id),     -- anchor: analytics + domain + design
    name            text NOT NULL,
    gtin            char(14) CHECK (gtin IS NULL OR gtin ~ '^[0-9]{14}$'),
    quantity        integer NOT NULL CHECK (quantity BETWEEN 1 AND 10000000),
    rules           jsonb NOT NULL DEFAULT '{"max_scans":20,"multi_country_window_hours":24,"velocity_per_hour":10}'::jsonb,
    verify_page     jsonb NOT NULL DEFAULT '{}'::jsonb,   -- product name, image, batch info, support link
    status          text NOT NULL DEFAULT 'generating' CHECK (status IN ('generating','ready','void')),
    generated       integer NOT NULL DEFAULT 0,
    created_by      uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE serial_codes (
    serial          text PRIMARY KEY CHECK (serial ~ '^[0-9A-HJKMNP-TV-Z]{12}$'),   -- 9 random + 3 MAC chars
    batch_id        uuid NOT NULL REFERENCES serial_batches(id) ON DELETE CASCADE,
    status          text NOT NULL DEFAULT 'active' CHECK (status IN ('active','flagged','void')),
    scan_count      integer NOT NULL DEFAULT 0,
    first_scan_at   timestamptz,
    first_country   char(2),
    first_city      text,
    last_scan_at    timestamptz,
    countries       char(2)[] NOT NULL DEFAULT '{}',
    flagged_reason  text
);
CREATE INDEX serial_codes_batch_idx ON serial_codes (batch_id, status);

ALTER TABLE scan_events ADD COLUMN serial text;   -- set for serial verifications

-- ---------------------------------------------------------------------
-- E11. Enterprise billing: contracts, GST-compliant invoices
-- ---------------------------------------------------------------------
CREATE TABLE contracts (
    id                  uuid PRIMARY KEY,
    org_id              uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                text NOT NULL,
    starts_on           date NOT NULL,
    ends_on             date NOT NULL CHECK (ends_on > starts_on),
    billing_interval    text NOT NULL CHECK (billing_interval IN ('month','quarter','year')),
    currency            char(3) NOT NULL CHECK (currency IN ('INR','USD','EUR','GBP')),
    amount_minor        bigint NOT NULL CHECK (amount_minor >= 0),     -- per interval, pre-tax
    seats               integer NOT NULL CHECK (seats > 0),
    limits_override     jsonb NOT NULL DEFAULT '{}'::jsonb,            -- merged over plan entitlements
    sla_uptime          numeric(5,3) NOT NULL DEFAULT 99.950,
    po_number           text,
    payment_terms_days  integer NOT NULL DEFAULT 30,
    status              text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','expired','terminated')),
    created_by          uuid REFERENCES users(id),
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX contracts_one_active_uniq ON contracts (org_id) WHERE status = 'active';

-- GST: invoice numbers are unique and consecutive per financial year (April–March), ≤ 16 chars.
CREATE TABLE invoice_sequences (
    fy        text PRIMARY KEY CHECK (fy ~ '^[0-9]{4}-[0-9]{2}$'),   -- '2026-27'
    last_seq  integer NOT NULL DEFAULT 0
);
CREATE FUNCTION next_invoice_number(p_date date) RETURNS text LANGUAGE plpgsql AS $$
DECLARE v_fy text; v_seq integer; y integer;
BEGIN
    y := CASE WHEN extract(month FROM p_date) >= 4 THEN extract(year FROM p_date)::int
              ELSE extract(year FROM p_date)::int - 1 END;
    v_fy := y::text || '-' || lpad(((y + 1) % 100)::text, 2, '0');
    INSERT INTO invoice_sequences (fy, last_seq) VALUES (v_fy, 1)
    ON CONFLICT (fy) DO UPDATE SET last_seq = invoice_sequences.last_seq + 1
    RETURNING last_seq INTO v_seq;
    RETURN 'QR/' || replace(v_fy, '-', '') || '/' || lpad(v_seq::text, 5, '0');   -- e.g. QR/202627/00042 (15 chars)
END $$;

CREATE TABLE invoices (
    id               uuid PRIMARY KEY,
    org_id           uuid NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    contract_id      uuid REFERENCES contracts(id),
    number           text NOT NULL UNIQUE CHECK (length(number) <= 16),
    issue_date       date NOT NULL,
    due_date         date NOT NULL,
    currency         char(3) NOT NULL,
    tax_mode         text NOT NULL CHECK (tax_mode IN ('cgst_sgst','igst','export_lut','reverse_charge','none')),
    place_of_supply  text NOT NULL,          -- GST state code, or 'Outside India'
    seller           jsonb NOT NULL,         -- legal name, GSTIN, address, SAC 998314
    buyer            jsonb NOT NULL,         -- legal name, GSTIN (optional), address, state code
    lines            jsonb NOT NULL,         -- [{description, sac, qty, unit_minor, amount_minor}]
    subtotal_minor   bigint NOT NULL,
    cgst_minor       bigint NOT NULL DEFAULT 0,
    sgst_minor       bigint NOT NULL DEFAULT 0,
    igst_minor       bigint NOT NULL DEFAULT 0,
    total_minor      bigint NOT NULL,
    endorsement      text,                   -- 'Supply meant for export under LUT without payment of IGST'
    status           text NOT NULL DEFAULT 'issued' CHECK (status IN ('issued','paid','void')),
    pdf_file_id      uuid REFERENCES files(id),
    paid_at          timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT invoice_totals CHECK (total_minor = subtotal_minor + cgst_minor + sgst_minor + igst_minor),
    CONSTRAINT invoice_export_zero CHECK (tax_mode <> 'export_lut' OR (cgst_minor = 0 AND sgst_minor = 0 AND igst_minor = 0 AND endorsement IS NOT NULL))
);

-- ---------------------------------------------------------------------
-- E12. White-label, developer platform, support access, feature flags
-- ---------------------------------------------------------------------
CREATE TABLE org_branding (
    org_id               uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    app_hostname         citext UNIQUE,          -- dashboard on app.agency.com (Cloudflare for SaaS)
    app_hostname_status  text NOT NULL DEFAULT 'none' CHECK (app_hostname_status IN ('none','pending','active','failed')),
    provider_hostname_id text,
    email_domain         citext UNIQUE,          -- mail.agency.com (Resend domain, DKIM)
    email_domain_status  text NOT NULL DEFAULT 'none' CHECK (email_domain_status IN ('none','pending','active','failed')),
    email_provider_id    text,
    product_name         text,                   -- replaces "QRit" in UI/emails when set
    logo_file_id         uuid REFERENCES files(id),
    favicon_file_id      uuid REFERENCES files(id),
    primary_color        text CHECK (primary_color IS NULL OR primary_color ~ '^#[0-9A-Fa-f]{6}$'),
    support_url          text,
    hide_platform_brand  boolean NOT NULL DEFAULT false,
    updated_at           timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE workspaces ADD COLUMN is_sandbox boolean NOT NULL DEFAULT false;   -- test-mode API keys operate only here

ALTER TABLE jobs DROP CONSTRAINT jobs_kind_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_kind_check CHECK (kind IN (
    'bulk_create','bulk_update','bulk_download','export_scans','export_qr_codes','print_sheet',
    'serial_generate','serial_export','gs1_import','warehouse_export','audit_export','dsar_export','lead_export','report_run'));

ALTER TABLE api_keys
    ADD COLUMN environment   text  NOT NULL DEFAULT 'live' CHECK (environment IN ('live','test')),
    ADD COLUMN ip_allowlist  cidr[] NOT NULL DEFAULT '{}';

CREATE TABLE api_usage_daily (
    api_key_id  uuid NOT NULL,
    day         date NOT NULL,
    requests    integer NOT NULL DEFAULT 0,
    errors      integer NOT NULL DEFAULT 0,
    throttled   integer NOT NULL DEFAULT 0,
    PRIMARY KEY (api_key_id, day)
);

CREATE TABLE support_access_grants (
    id          uuid PRIMARY KEY,
    org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    granted_by  uuid NOT NULL REFERENCES users(id),
    scope       text NOT NULL DEFAULT 'read' CHECK (scope IN ('read','read_write')),
    reason      text NOT NULL,
    expires_at  timestamptz NOT NULL,
    revoked_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT support_grant_max_7d CHECK (expires_at <= created_at + interval '7 days')
);

CREATE TABLE feature_flags (
    key         text NOT NULL,
    org_id      uuid REFERENCES organizations(id) ON DELETE CASCADE,   -- NULL = global default
    enabled     boolean NOT NULL,
    updated_by  uuid REFERENCES users(id),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (key, org_id)
);
```

Put these queries verbatim into the query files (split by concern). Add the further CRUD queries each module needs, following the same conventions:

```sql
-- =====================================================================
-- Enterprise queries (sqlc: services/db/queries/enterprise_*.sql)
-- Every query below was executed against fixture data (plan §E-V).
-- =====================================================================

-- name: EffectivePermissions :many
-- All permissions a user holds in a workspace, from direct and group bindings.
-- folder_id NULL = workspace-wide; otherwise the grant applies to that folder subtree.
WITH RECURSIVE principal AS (
    SELECT 'user'::text AS principal_type, sqlc.arg(user_id)::uuid AS principal_id
    UNION ALL
    SELECT 'group', gm.group_id FROM group_members gm WHERE gm.user_id = sqlc.arg(user_id)::uuid
)
SELECT DISTINCT unnest(r.permissions) AS permission, b.folder_id
FROM role_bindings b
JOIN principal p ON p.principal_type = b.principal_type AND p.principal_id = b.principal_id
JOIN roles r ON r.id = b.role_id
JOIN org_members om ON om.org_id = b.org_id AND om.user_id = sqlc.arg(user_id)::uuid AND om.status = 'active'
WHERE b.workspace_id = sqlc.arg(workspace_id)::uuid;

-- name: FolderAncestors :many
-- A folder-scoped grant on F applies to F and every descendant; the service checks
-- whether any granted folder_id is in the ancestor chain of the target QR's folder.
WITH RECURSIVE chain AS (
    SELECT id, parent_id FROM folders WHERE id = sqlc.arg(folder_id)::uuid AND workspace_id = sqlc.arg(workspace_id)::uuid
    UNION ALL
    SELECT f.id, f.parent_id FROM folders f JOIN chain c ON f.id = c.parent_id
)
SELECT id FROM chain;

-- name: GetResolvedLink :one
-- Redirect resolution. Only versions that need no approval or were approved are eligible.
SELECT q.id, q.workspace_id, q.domain_id, q.campaign_id, q.status, q.safety_status,
       q.starts_at, q.expires_at, q.scan_limit, q.total_scans, q.password_hash, q.fallback_url,
       w.timezone,
       v.id AS version_id, v.version_no, v.destination_kind, v.destination_url, v.rules, v.utm,
       (SELECT min(v2.effective_at) FROM qr_versions v2
         WHERE v2.qr_code_id = q.id AND v2.effective_at > now()
           AND v2.approval_status IN ('not_required','approved')) AS next_change_at
FROM qr_codes q
JOIN workspaces w ON w.id = q.workspace_id
LEFT JOIN LATERAL (
    SELECT * FROM qr_versions v1
    WHERE v1.qr_code_id = q.id AND v1.effective_at <= now()
      AND v1.approval_status IN ('not_required','approved')
    ORDER BY v1.effective_at DESC, v1.version_no DESC
    LIMIT 1
) v ON true
WHERE q.domain_id = sqlc.arg(domain_id)::uuid AND q.short_code = sqlc.arg(short_code)::text;

-- name: GS1Linkset :many
-- Conformant-resolver lookup with qualifier inheritance: every item whose qualifiers are
-- NULL or equal to the request's is a candidate; more specific items rank first.
-- The service builds the RFC 9264 linkset from all rows, and picks the default link
-- (or the requested linkType / language) from the most specific item that has one.
SELECT i.id AS item_id, i.qr_code_id, i.title AS item_title,
       (i.cpv IS NOT NULL)::int + (i.lot IS NOT NULL)::int + (i.serial IS NOT NULL)::int AS specificity,
       l.link_type, l.href, l.title, l.hreflang, l.media_type, l.is_default, l.position
FROM gs1_items i
JOIN gs1_links l ON l.item_id = i.id
WHERE i.domain_id = sqlc.arg(domain_id)::uuid
  AND i.gtin = sqlc.arg(gtin)::text
  AND (i.cpv    IS NULL OR i.cpv    = sqlc.narg(cpv)::text)
  AND (i.lot    IS NULL OR i.lot    = sqlc.narg(lot)::text)
  AND (i.serial IS NULL OR i.serial = sqlc.narg(serial)::text)
ORDER BY specificity DESC, l.is_default DESC, l.position, l.link_type;

-- name: ScanSpikeCandidates :many
-- Anomaly detection (every 5 min): last complete 15-min bucket vs the same bucket on each of
-- the previous 7 days. Returns codes whose z-score >= z_min and scans >= min_scans (spike),
-- or scans <= baseline_mean * drop_ratio when the baseline is meaningful (drop).
WITH cur AS (
    SELECT qr_code_id, workspace_id, scans
    FROM scan_stats_15m
    WHERE bucket_start = sqlc.arg(bucket)::timestamptz
      AND (sqlc.narg(workspace_id)::uuid IS NULL OR workspace_id = sqlc.narg(workspace_id)::uuid)
), hist AS (
    SELECT s.qr_code_id, d AS days_ago, COALESCE(s.scans, 0) AS scans
    FROM generate_series(1, 7) d
    CROSS JOIN (SELECT DISTINCT qr_code_id FROM cur) c
    LEFT JOIN scan_stats_15m s
           ON s.qr_code_id = c.qr_code_id
          AND s.bucket_start = sqlc.arg(bucket)::timestamptz - make_interval(days => d)
), base AS (
    SELECT qr_code_id, avg(scans)::float8 AS mean, stddev_pop(scans)::float8 AS sd
    FROM hist GROUP BY qr_code_id
)
SELECT c.qr_code_id, c.workspace_id, c.scans, b.mean, b.sd,
       CASE WHEN b.sd > 0 THEN (c.scans - b.mean) / b.sd ELSE NULL END AS z
FROM cur c JOIN base b USING (qr_code_id)
WHERE c.scans >= sqlc.arg(min_scans)::int
  AND ((b.sd > 0 AND (c.scans - b.mean) / b.sd >= sqlc.arg(z_min)::float8)
       OR (b.sd = 0 AND c.scans >= GREATEST(b.mean * 3, sqlc.arg(min_scans)::int)));

-- name: RecordSerialVerification :one
-- Called synchronously by POST /v1/public/verify/{serial} (issued by the verify page's JS, so
-- link previews don't burn counts). Applies the batch rules atomically and returns the verdict.
-- Scan analytics for the same visit still flow through the normal stream/ingest path.
WITH b AS (
    SELECT sb.rules FROM serial_batches sb JOIN serial_codes sc ON sc.batch_id = sb.id
    WHERE sc.serial = sqlc.arg(serial)::text
)
UPDATE serial_codes sc SET
    scan_count    = sc.scan_count + 1,
    first_scan_at = COALESCE(sc.first_scan_at, sqlc.arg(at)::timestamptz),
    first_country = COALESCE(sc.first_country, sqlc.narg(country)::char(2)),
    first_city    = COALESCE(sc.first_city, sqlc.narg(city)::text),
    last_scan_at  = sqlc.arg(at)::timestamptz,
    countries     = CASE WHEN sqlc.narg(country)::char(2) IS NULL OR sqlc.narg(country)::char(2) = ANY(sc.countries)
                         THEN sc.countries ELSE sc.countries || sqlc.narg(country)::char(2) END,
    status        = CASE
        WHEN sc.status = 'void' THEN 'void'
        WHEN sc.scan_count + 1 > (b.rules->>'max_scans')::int THEN 'flagged'
        WHEN sqlc.narg(country)::char(2) IS NOT NULL AND sc.first_country IS NOT NULL
             AND sqlc.narg(country)::char(2) <> sc.first_country
             AND sqlc.arg(at)::timestamptz - sc.first_scan_at
                 < make_interval(hours => (b.rules->>'multi_country_window_hours')::int) THEN 'flagged'
        ELSE sc.status END,
    flagged_reason = CASE
        WHEN sc.status IN ('flagged','void') THEN sc.flagged_reason
        WHEN sc.scan_count + 1 > (b.rules->>'max_scans')::int THEN 'max_scans_exceeded'
        WHEN sqlc.narg(country)::char(2) IS NOT NULL AND sc.first_country IS NOT NULL
             AND sqlc.narg(country)::char(2) <> sc.first_country
             AND sqlc.arg(at)::timestamptz - sc.first_scan_at
                 < make_interval(hours => (b.rules->>'multi_country_window_hours')::int) THEN 'multi_country'
        ELSE NULL END
FROM b
WHERE sc.serial = sqlc.arg(serial)::text
RETURNING sc.serial, sc.status, sc.flagged_reason, sc.scan_count, sc.first_scan_at, sc.first_country, sc.first_city;

-- name: PendingApprovalsForApprover :many
-- The approver's inbox: pending requests in workspaces where they hold qr.destination.approve,
-- excluding their own requests and ones they already decided. Single-code requests show the
-- proposed destination; bulk requests show the number of versions they cover.
SELECT ar.id, ar.workspace_id, ar.kind, ar.reasons, ar.requested_by, ar.required_approvals,
       ar.expires_at, ar.created_at, q.name AS qr_name,
       v.destination_url, v.effective_at,
       (SELECT count(*) FROM qr_versions vv WHERE vv.approval_request_id = ar.id) AS version_count
FROM approval_requests ar
LEFT JOIN qr_codes q ON q.id = ar.qr_code_id
LEFT JOIN LATERAL (
    SELECT destination_url, effective_at FROM qr_versions
    WHERE approval_request_id = ar.id ORDER BY version_no DESC LIMIT 1
) v ON ar.kind <> 'bulk_update'
WHERE ar.status = 'pending' AND ar.expires_at > now()
  AND ar.workspace_id = ANY(sqlc.arg(workspace_ids)::uuid[])
  AND ar.requested_by <> sqlc.arg(user_id)::uuid
  AND NOT EXISTS (SELECT 1 FROM approval_decisions d WHERE d.request_id = ar.id AND d.approver_id = sqlc.arg(user_id)::uuid)
ORDER BY ar.created_at;

-- name: FinalizeApproval :exec
-- Run in the decision transaction once approvals >= required_approvals.
-- Approved versions become eligible; a past effective_at is moved to the approval instant so
-- the approved version becomes current rather than slotting in behind newer versions.
WITH req AS (
    UPDATE approval_requests SET status = 'approved', decided_at = now()
    WHERE id = sqlc.arg(request_id)::uuid AND status = 'pending'
    RETURNING id
)
UPDATE qr_versions v SET approval_status = 'approved', effective_at = GREATEST(v.effective_at, now())
FROM req WHERE v.approval_request_id = req.id AND v.approval_status = 'pending';
```

Replace the base `GetResolvedLink` query with the version above (the approval filter is mandatory). `audit_seal`, `audit_verify` and `next_invoice_number` are called from Go as SQL functions.

## D. DOMAIN RULES (implement exactly)

**D1. Request gate order** (`middleware/orggate.go`, for every route under an org or workspace):
1. authenticate (cookie session | API key | staff+grant)
2. session valid (idle/max per org policy)
3. resolve org from route (`/orgs/{org}` or `/workspaces/{ws}` → `workspaces.org_id`)
4. org member `status='active'` (else 404)
5. IP allowlist
6. SSO enforcement (break-glass needs MFA)
7. MFA requirement
8. `authz.Can`
9. entitlements/feature flags/`disabled_features`
10. handler

Error codes: `session_expired` 401, `ip_not_allowed` 403, `sso_required` 403 (+`login_url`), `mfa_required` 403 (+`enroll_url`), `forbidden` 403, `upgrade_required` 402, `feature_disabled` 403. Cache the org policy in Redis `orgpol:{org}` for 60 s; invalidate via pub/sub `orgpol:invalidate`.

**D2. Authorization.** Permission strings exactly as in the `roles` seed plus the org-level set `org.manage, org.billing, org.sso, org.scim, org.policy, org.audit, org.members, org.branding, org.clients` (held by `org_role`: `org_owner` = all; `org_admin` = all except `org.billing` and ownership transfer; `billing_admin` = `org.billing`). `org_owner` and `org_admin` have implicit `admin` in every workspace of their org and, for agencies, in client orgs (tag audit `changes.via_agency`).
- `authz.Can` uses `EffectivePermissions` (Redis `authz:{ws}:{user}` 30 s, pub/sub `authz:invalidate`) and `FolderAncestors` for folder-scoped grants.
- List endpoints filter by `VisibleFolders`.
- API-key scopes map: `qr:read→qr.read`; `qr:write→qr.create,qr.update,qr.destination.update`; `analytics:read→analytics.read`; `webhooks:write→webhook.manage`; `leads:read→lead.read`. Keys can never approve.
- No principal may grant a permission it does not hold.
- The last `owner` binding and the last `org_owner` can't be removed.

**D3. Identity.**
- **SSO start:** `POST /v1/auth/sso/start {email|org_slug}`. Find the verified `org_domains` entry → active `sso_connections`. SAML → Polis authorize URL (tenant = org slug, product = `POLIS_PRODUCT`), OIDC → provider authorize URL. Always use PKCE, `state` in Redis `sso:state:{state}` (TTL 600 s) holding `{org, connection, verifier, return_to, host}`.
- **Callback linking order:** existing `user_identities (connection, subject)` → existing user whose email domain is verified by this org → JIT create (if `jit_provisioning`) → else `403 sso_user_not_provisioned`.
- JIT adds `org_members(source='sso_jit')` and a binding of `default_role_key` in `default_workspace_id`. Groups claim → `groups(source='sso')` membership sync by `display_name`.
- IdP-initiated logins are refused unless a valid `state` exists.
- `email_verified` is required for OIDC.
- **MFA:** TOTP 6 digits/30 s/skew 1; WebAuthn RP ID `WEBAUTHN_RP_ID`, origins `WEBAUTHN_RP_ORIGINS`; 10 recovery codes of 10 Crockford chars, stored sha256; 5 attempts/15 min/session.
- **Step-up:** `sessions.step_up_at` within 600 s for the actions listed in plan §6.1.4, else `401 step_up_required`.
- **Deprovisioning** (suspend/deprovision/SCIM `active=false`/DELETE): revoke sessions, invalidate authz, keep codes, transfer the user's API keys to the org, audit, all within one transaction + after-commit pub/sub.

**D4. SCIM 2.0** (`/scim/v2`, bearer `scim_…`, sha256 lookup):
- Implement ServiceProviderConfig, ResourceTypes, Schemas, Users, Groups exactly as plan §6.1.6.
- Filter grammar: `attr eq "value"` joined by `and`; attributes `userName`, `externalId`, `emails[type eq "work"].value`, `displayName`; case-insensitive attribute names and operators.
- PATCH ops case-insensitive; booleans accept `true/false` or `"True"/"False"`.
- Group PATCH returns 204.
- `application/scim+json`; SCIM error schema; ETag `W/"<version>"`; `startIndex`/`count` pagination (max 200); `ListResponse` always.
- Store the `resource` exactly as received.

**D5. Approvals.**
- `approval.Evaluate(policy, change) []string` returns reasons `host_not_allowlisted`, `policy_all_destination_changes`, `policy_new_code`.
- Blocked hosts / https are hard 422s.
- When reasons exist: create `approval_requests` (`kind`, `qr_code_id` or `job_id`), versions with `approval_status='pending'` + `approval_request_id`, `expires_at = now + approval_expiry_hours`, and respond `202` with the approval object.
- Decisions insert `approval_decisions` (the DB trigger enforces four-eyes/pending). A reject finalizes immediately. When approve count ≥ `required_approvals`, run `FinalizeApproval` in the same transaction, then invalidate the link cache.
- `ApprovalExpire` job hourly. Override: org_owner + step-up + `override_reason`, audited `approval.overridden`, notify approvers.

**D6. Audit.**
- `audit.Record(ctx, tx, Entry)` inside the business transaction, always with `org_id`.
- Redact keys matching `(?i)(secret|token|password|api_key|credential|private|dek)` → `"[REDACTED]"`.
- `AuditSealer` River periodic every 5 s: `SELECT DISTINCT org_id … WHERE hash IS NULL`, then `SELECT audit_seal($1, 1000)`.
- `AuditAnchor` daily 00:10 UTC → `audit_anchors` + object `{org}/{day}.json` in `AUDIT_WORM_BUCKET`.
- `AuditRetention` refuses to delete a day without an anchor and runs with `SET LOCAL qrit.audit_retention='on'`.
- Streams per plan §6.4.3.
- Action names from plan §6.4.2 only.

**D7. Events.**
- Envelope `{id (uuidv7), type, created_at, org_id, workspace_id, actor{type,id}, data}`.
- Insert River `FanOut` with `InsertTx` in the business transaction.
- `FanOut` delivers to webhooks (Standard Webhooks signing, base prompt), integrations and security alert rules.
- `scan.created` fan-out is sampled per workspace at ≤ 50/s (token bucket in Redis) with a daily aggregate event.

**D8. Envelope encryption** (`crypto/envelope`):
- Master key `APP_ENCRYPTION_KEY` (32 B). Per org, a DEK (32 random bytes) wrapped as `0x01 || nonce(12) || AES-256-GCM(master, dek, aad=org_id)` in `org_data_keys`.
- Data ciphertext = `0x01 || nonce(12) || AES-256-GCM(dek, plaintext, aad=org_id||form_id)`.
- Blind index = `HMAC-SHA256(HKDF-SHA256(master, salt=org_id, info="qrit-bidx-v1"), lower(trim(email)))`.
- Unwrapped DEKs cached in process for 5 min, never logged.

**D9. Forms/leads.**
- Plan §6.6 exactly. Turnstile verify server-side (`TURNSTILE_SECRET_KEY`).
- Withdrawal token = 32 random bytes, sha256 stored (add a `consent_tokens` table in a separate migration `00004_forms_tokens.sql` only if needed, with a `// DECISION:`).
- `LeadRetention` daily. DSAR job produces a JSON export into `files(purpose='export')`.

**D10. Pixels.**
- Interstitial `html/template` ≤ 8 KB, per-response nonce CSP `script-src 'nonce-…' https://connect.facebook.net https://www.googletagmanager.com https://snap.licdn.com https://analytics.tiktok.com; connect-src 'self' https://*.facebook.com https://*.google-analytics.com https://*.linkedin.com https://*.tiktok.com`.
- Consent Mode v2 defaults denied.
- Redirect after pixel `onload` or 800 ms.
- `<noscript>` meta refresh.
- Beacon `POST /_px/{code}` → Redis `px:{qr}:{day}:{field}`, flushed hourly to `pixel_consent_daily`.
- Consent region set for `opt_in_where_required`: EU/EEA countries + GB, CH, IN.

**D11. GS1 resolver** (redirect service):
- Routes `GET|HEAD|OPTIONS ^/01/(\d{14})(/22/[^/]+)?(/10/[^/]+)?(/21/[^/]+)?/?$` and `/.well-known/gs1resolver`.
- Validate the GTIN mod-10 check digit and AI values (GS1 charset 82, ≤ 20 chars); else 400 problem.
- `GS1Linkset` → default link of the most specific item (first row with `is_default`).
- `linkType` param: exact type → 200 linkset when `linkset`/`all`, else the link, or 404 if absent. Two equal matches → 300 with the linkset.
- `Accept: application/linkset+json` → 200 linkset (RFC 9264 shape, `Content-Type: application/linkset+json`, `Link` context header).
- Redirects are **307** with `Cache-Control: private, no-store`, passing through all query params except `linkType`.
- CORS: `Access-Control-Allow-Origin: *`, expose `Link, Location, Content-Type`.
- Emit a scan event on `qr_code_id` with `rule_id = linkType` (or `gs1:default`).

**D12. Serialization.**
- `serial9` = 9 chars from the Crockford alphabet via `crypto/rand` rejection sampling.
- `mac3` = first 15 bits of `HMAC-SHA256(SERIAL_MAC_KEY, "qrit-serial-v1:"+serial9)` encoded as 3 Crockford chars (5 bits each, MSB first). `serial = serial9+mac3`; verify in constant time **before** any DB/Redis access.
- `SerialGenerate` writes with `CopyFrom` in chunks of 50,000, updating `serial_batches.generated`, and retries on PK conflict by regenerating the colliding serials.
- Redirect `/V/{serial}` → scan event with `serial` → `302 /v/{serial}`. The redirect service reverse-proxies `/v/*` to `web` exactly like `/p/*`, preserving `Host`. Routes are case-sensitive: `/V/` is the printed entry point, `/v/` is the page.
- Verify page token `t = base64url(HMAC-SHA256(VERIFY_TOKEN_KEY, serial+"."+floor(unix/600)))[:22]`; accept the current and previous window.
- `POST /v1/public/verify/{serial} {t}` → `RecordSerialVerification` → verdict JSON `{status, reason, scan_count, first_scan_at, first_city, first_country, product}`. Rate limit 30/min per IP prefix per serial.
- A transition to `flagged` emits `serial.flagged`.

**D13. Invoices.**
- Amounts in minor units (int64).
- `cgst = sgst = roundHalfUp(subtotal*9/100)`, `igst = roundHalfUp(subtotal*18/100)`.
- Tax mode:
  - buyer country ≠ IN and currency ≠ INR → `export_lut` (zero tax + endorsement text exactly `Supply meant for export under Bond or Letter of Undertaking without payment of Integrated Tax`, plus `SELLER_LUT_REF`);
  - buyer state code = `SELLER_STATE_CODE` → `cgst_sgst`;
  - else `igst`.
- Number via `SELECT next_invoice_number($issue_date)` in the same transaction as the insert.
- PDF by `render /invoice`, including a UPI intent QR for INR invoices (`upi://pay?pa=SELLER_UPI_VPA&pn=…&am=…&cu=INR&tn=<number>`).
- `InvoiceRun` daily. Dunning schedule per plan §6.12.1; overdue > 30 days → org dashboard read-only (API 402 `invoice_overdue` on mutations). **Never stop redirects.**

**D14. Entitlements v2.** `Effective(org) = merge(planDefaults[org.plan_id], activeContract.limits_override) minus workspace_policies.disabled_features`. Enterprise plan features add: `sso, scim, mfa_policy, ip_allowlist, custom_roles, folder_sharing, multi_approvers, audit_stream, audit_7y, integrations_crm, ga4, warehouse, gs1_resolver, serialization, white_label_app, email_domain, agency, sandbox, sla`. Business gets `approvals (1 approver), audit_1y, slack, teams, zapier, gsheets, alerts, reports, forms, pixels`.

**D15. White-label/agency.**
- `GET /v1/public/branding?host=` returns `{product_name, logo_url, favicon_url, primary_color, support_url, hide_platform_brand}` for an **active** `app_hostname` only.
- `web` `proxy.ts` sets CSS variables from it, rejecting colours with contrast < 4.5:1 against `--bg`.
- Email sender selection by the org's verified `email_domain`.
- Child orgs: only `kind='agency'` orgs may create them; client orgs can't see the parent.

**D16. API platform.**
- Test keys (`environment='test'`) are authorized **only** for workspaces with `is_sandbox=true` in the same org; anything else → 404.
- Usage counters `apiuse:{key}:{yyyymmdd}` flushed hourly to `api_usage_daily`.
- Bulk v2 per plan §6.11 (dry run, update mode via approvals, checkpoints every 1,000 rows in `jobs.params.checkpoint`).
- Print: `format=pdf&colorspace=cmyk`, `format=eps`, job `print_sheet {template: avery_l7160|avery_l7163|avery_5160|a5_table_tent|4x6, crop_marks, bleed_mm}`.

## E. API (add to `api/openapi.yaml`; every route declares its permission in an `x-permission` extension and its audit action in `x-audit`)

- **Orgs:** `/v1/orgs` GET; `/v1/orgs/{org}` GET PATCH; `/members` GET, `/members/{userId}` PATCH DELETE; `/workspaces` GET POST; `/domains` GET POST, `/domains/{id}` DELETE, `/domains/{id}/verify` POST; `/sandbox` POST
- **Auth:** `/v1/auth/sso/start` POST; `/v1/auth/sso/callback` GET; `/v1/auth/step-up` POST; `/v1/auth/mfa/verify` POST; `/v1/auth/mfa/webauthn/begin` POST
- **MFA:** `/v1/me/mfa` GET; `/v1/me/mfa/totp` POST; `/v1/me/mfa/totp/confirm` POST; `/v1/me/mfa/webauthn/register/begin|finish` POST; `/v1/me/mfa/{id}` DELETE; `/v1/me/mfa/recovery-codes` POST
- **SSO/SCIM config:** `/v1/orgs/{org}/sso-connections` GET POST, `/{id}` GET PATCH DELETE, `/{id}/test` POST; `/v1/orgs/{org}/scim-directories` GET POST, `/{id}` DELETE, `/{id}/rotate` POST
- **Access:** `/v1/permissions` GET; `/v1/orgs/{org}/roles` GET POST, `/{id}` PATCH DELETE; `/v1/orgs/{org}/groups` GET POST, `/{id}` PATCH DELETE, `/{id}/members` POST DELETE; `/v1/workspaces/{ws}/role-bindings` GET POST, `/{id}` DELETE; `/v1/workspaces/{ws}/access/explain` GET; `/v1/orgs/{org}/access-review` GET
- **Policies and approvals:** `/v1/orgs/{org}/security-policy` GET PUT; `/v1/workspaces/{ws}/policy` GET PUT; `/v1/workspaces/{ws}/approvals` GET, `/{id}/decisions` POST, `/{id}/cancel` POST, `/{id}/override` POST; `/v1/me/approvals` GET
- **Audit:** `/v1/orgs/{org}/audit-logs` GET, `/verify` GET, `/exports` POST; `/v1/orgs/{org}/audit-streams` GET POST, `/{id}` PATCH DELETE, `/{id}/test` POST
- **Integrations:** `/v1/integrations/catalog` GET; `/v1/workspaces/{ws}/integrations` GET POST, `/{id}` PATCH DELETE, `/{id}/test` POST, `/{id}/deliveries` GET; `/v1/integrations/oauth/{provider}/start` GET, `/callback` GET; `/v1/hooks` POST, `/v1/hooks/{id}` DELETE, `/v1/hooks/samples/{event}` GET
- **Alerts and reports:** `/v1/workspaces/{ws}/alert-rules` GET POST, `/{id}` PATCH DELETE; `/v1/workspaces/{ws}/alert-events` GET, `/{id}/resolve` POST; `/v1/workspaces/{ws}/report-schedules` GET POST, `/{id}` PATCH DELETE, `/{id}/send-now` POST
- **Forms and privacy:** `/v1/workspaces/{ws}/forms` GET POST, `/{id}` GET PATCH DELETE, `/{id}/submissions` GET, `/{id}/submissions/export` POST; `/v1/orgs/{org}/dsar-requests` GET POST, `/{id}/complete` POST; public `/v1/public/forms/{code}` GET, `/v1/public/forms/{code}/submissions` POST, `/v1/public/consent/{token}` GET POST, `/v1/public/consent/{token}/confirm` GET
- **Pixels:** `/v1/workspaces/{ws}/pixels` GET POST, `/{id}` DELETE; `/v1/workspaces/{ws}/qr-codes/{id}/pixels` PUT
- **GS1:** `/v1/workspaces/{ws}/gs1/items` GET POST, `/{id}` PATCH DELETE, `/{id}/links` GET POST, `/{id}/links/{linkId}` PATCH DELETE; `/v1/workspaces/{ws}/gs1/import` POST
- **Serials:** `/v1/workspaces/{ws}/serial-batches` GET POST, `/{id}` GET, `/{id}/codes` GET, `/{id}/export` POST, `/{id}/void` POST; public `/v1/public/verify/{serial}` POST
- **Billing:** `/v1/orgs/{org}/billing` GET; `/v1/orgs/{org}/invoices` GET, `/{id}/pdf` GET
- **Branding and agency:** `/v1/orgs/{org}/branding` GET PUT, `/branding/app-hostname` POST, `/branding/app-hostname/verify` POST, `/branding/email-domain` POST, `/branding/email-domain/verify` POST; `/v1/public/branding` GET; `/v1/orgs/{org}/clients` GET POST, `/{child}` PATCH
- **Support and developer:** `/v1/orgs/{org}/support-access` GET POST, `/{id}` DELETE; `/v1/workspaces/{ws}/api-usage` GET
- **Staff:** `/v1/admin/orgs` GET, `/v1/admin/orgs/{org}` GET PATCH, `/v1/admin/orgs/{org}/contracts` POST, `/v1/admin/contracts/{id}/activate` POST, `/v1/admin/invoices/{id}/mark-paid` POST, `/v1/admin/flags` GET PUT, `/v1/admin/support-sessions` POST
- **SCIM:** `/scim/v2/ServiceProviderConfig`, `/ResourceTypes`, `/Schemas`, `/Users[/{id}]`, `/Groups[/{id}]`
- **Redirect service additions:** `/V/{serial}`, `/01/…` (GS1), `/.well-known/gs1resolver`, `/_px/{code}`

## F. UI (apps/web)

- **Org switcher** at the top of the sidebar (orgs, then workspaces). Org settings under `/o/[org]/settings/*`.
- **Login v2:**
  - email-first;
  - "Continue with SSO" when the domain has SSO;
  - password/magic/Google otherwise;
  - MFA challenge screen (TOTP input with 6 boxes, "Use a passkey", "Use a recovery code");
  - step-up dialog component reused everywhere (`<StepUpGate action=…>`).
- **SSO settings:** IdP picker (Okta, Entra ID, Google Workspace, JumpCloud, OneLogin, PingFederate, Custom SAML, Custom OIDC) with per-IdP numbered guide, copy buttons for ACS URL / entity ID / redirect URI, metadata upload or URL, attribute-mapping table, test-login result panel, enforcement toggle with the checklist (tested within 24 h, break-glass owner with MFA).
- **Directory sync:** create directory → token shown once with copy + "I've saved it"; endpoint URL; last request time; provisioned users/groups tables.
- **Security policy:** form sections Sign-in, Sessions, Network (CIDR editor with "your IP" chip and lock-out guard), Passwords, Invitations, API keys, Exports.
- **Roles/Groups/Access:**
  - permission matrix editor (grouped checkboxes, search);
  - group pages;
  - workspace Access tab;
  - folder Share dialog;
  - Explain-access drawer (renders the `via` chain);
  - Access review page with download + "mark reviewed".
- **Approvals:**
  - sidebar badge;
  - inbox list;
  - detail with a destination diff (host highlighted), safety verdict, reasons in plain English, approve/reject with comment;
  - on the QR Versions tab, pending/rejected states;
  - version create UI handles `202` with "Sent for approval".
- **Audit:** timeline with sentence rendering, filters, JSON diff, chain badge (calls `/verify`), exports, streams settings with test button.
- **Integrations:** catalog grid (logo, description, plan badge), connect flows, field mapping (HubSpot/Salesforce), deliveries log.
- **Alerts:** rule builder, history. **Reports:** schedule form with timezone and preview.
- **Forms:** builder (field list with drag handles via `@dnd-kit`, per-language labels, notice editor with purposes), submissions table (decrypted on demand, audited), export, DSAR page.
- **Pixels:** pixel list, per-QR pixel toggle in QR Settings, consent metrics card.
- **GS1:** catalog table, item editor with link list, link-type picker (GS1 Web Vocabulary list bundled), "Recall this lot" action, CSV import, resolver tester.
- **Serials:** batch wizard (quantity, rules, verify page preview), progress bar, map of verifications, flagged queue, exports, void.
- **Public pages:** `/v/[serial]` verify page (mobile-first, four verdict states, report form); `/c/[token]` consent withdrawal.
- **Branding/agency/billing:** branding form with live preview on a mock dashboard; app-hostname and email-domain DNS instructions with status polling; clients list (agency); invoices table with PDF download; contract summary.
- **Staff admin:** orgs, contracts, invoices, flags, support sessions.
- **All pages:** base prompt §11 behaviours (skeletons, empty states, error mapping by problem `code`, keyboard, a11y).

## G. ENV VARS (add to `.env.example`; exact names)

| Service | Variables |
|---|---|
| api/worker | `POLIS_URL` (internal, e.g. http://sso-bridge:5225), `POLIS_EXTERNAL_URL` (https://sso.example.com), `POLIS_ADMIN_API_KEY`, `POLIS_PRODUCT` (qrit), `WEBAUTHN_RP_ID`, `WEBAUTHN_RP_ORIGINS`, `SERIAL_MAC_KEY` (base64 32 B), `VERIFY_TOKEN_KEY` (base64 32 B), `DOH_URL`, `APPS_CNAME_TARGET` (apps.example.com), `SANDBOX_SHORT_DOMAIN`, `AUDIT_WORM_BUCKET`, `SLACK_CLIENT_ID`, `SLACK_CLIENT_SECRET`, `HUBSPOT_CLIENT_ID`, `HUBSPOT_CLIENT_SECRET`, `SALESFORCE_CLIENT_ID`, `SALESFORCE_CLIENT_SECRET`, `GOOGLE_INTEGRATION_CLIENT_ID`, `GOOGLE_INTEGRATION_CLIENT_SECRET`, `SELLER_LEGAL_NAME`, `SELLER_GSTIN`, `SELLER_STATE_CODE`, `SELLER_ADDRESS`, `SELLER_SAC`, `SELLER_LUT_REF`, `SELLER_UPI_VPA`, `RESEND_DOMAINS_ENABLED` (true/false) |
| redirect | `SERIAL_MAC_KEY`, `SANDBOX_SHORT_DOMAIN` |
| render | `GHOSTSCRIPT_PATH` (/usr/bin/gs) |
| sso-bridge | per the pinned Polis version's docs (DB URL for the `polis` schema, API keys = `POLIS_ADMIN_API_KEY`, external URL = `POLIS_EXTERNAL_URL`, admin-portal secret). Add them to `deploy/fly/sso-bridge.toml` and docker-compose |

Local defaults: compose adds `sso-bridge` and a `keycloak` container (realm import JSON in `deploy/dev/keycloak/`) acting as a test SAML IdP; integration providers use fakes when their secrets are empty.

## H. TESTS (must exist and pass)

- `services/db/tests/enterprise_test.go`: the 38 assertions of `tests/test_enterprise.py`, against a testcontainers Postgres with 00001+00002+00003.
- Unit:
  - authz (≥ 50 cases incl. escalation);
  - `approval.Evaluate` (≥ 30);
  - SCIM filter parser (≥ 40) and PATCH engine (≥ 40 incl. Entra string booleans, case-insensitive ops, members add/remove);
  - envelope crypto (round-trip, AAD mismatch fails, rotation);
  - serial MAC (forgery rejection, alphabet, 1 M uniqueness sample);
  - GS1 parser (≥ 60 URI cases);
  - invoice tax modes (intra-state, inter-state, export, rounding);
  - pixel CSP builder;
  - redaction.
- Integration:
  - every §7 enforcement row has a negative test (table-driven, named `TestEnforcement_<policy>`);
  - SAML e2e through Polis with Keycloak;
  - OIDC e2e with Keycloak;
  - SCIM user/group lifecycle including deprovision revoking sessions;
  - approval flow single + bulk;
  - audit sealer + anchor + verify + stream ordering;
  - GS1 resolver responses (307/404/300/400, linkset JSON, CORS, pass-through query);
  - serial verify flow;
  - invoice run;
  - support grant gate;
  - test-key isolation.
- Playwright:
  - SSO login via Keycloak;
  - MFA enrol + challenge;
  - approval approve-from-inbox;
  - pixel interstitial (no third-party request before Allow, decline path redirect < 300 ms);
  - form submit → withdraw;
  - verify page verdicts;
  - white-label host rendering.
- k6: `gs1_resolver.js` (2k rps, p99 < 60 ms), `serial_verify.js` (500 rps), `approvals_heavy.js`.

## I. PHASE ORDER AND FILE SCOPE

1. **E0 Foundations:** 00003 migration + queries; `org`, `authz`, `entitlements` v2, `crypto/envelope`, `middleware/{orggate,stepup}`, `flags`, `events` + `FanOut`, `audit` (catalogue, redaction, sealer, anchor, verify, retention); org API; org switcher + org settings general/members + step-up dialog; `enterprise_test.go`.
2. **E1 Identity:** `identity/*`, `mfa`, `scim`, org domains, SSO/SCIM/security settings pages, login v2, MFA pages; `sso-bridge` deploy + compose + Keycloak dev realm.
3. **E2 Governance:** roles/groups/bindings/explain/access-review; `approval` + inbox + policy UI; `auditstream` + audit UI.
4. **E6b Billing:** `billing/contracts`, `billing/invoice`, `InvoiceRun`, dunning, render `invoice.ts`, staff contract/invoice screens, org billing page.
5. **E3 Integrations:** `integrations/*`, hooks API, Zapier app, `alerts`, `reports` + render `report.ts`/`charts.ts`, UIs.
6. **E4 Leads & pixels:** `forms`, `dsar`, public form/consent endpoints and pages, `pixels` + redirect interstitial + beacon, UIs.
7. **E5 GS1 & serial:** `gs1` (parser, resolver handlers in redirect, management API, import job), `serial` (MAC, generator, verify API, exports), verify page, UIs.
8. **E6a White-label & agency:** `branding`, `agency`, public branding endpoint, `proxy.ts` host theming, email-domain sender, UIs.
9. **E7 Developer & print:** API keys v2 + sandbox + `apiusage`, SDK generation workflow (`.github/workflows/sdk.yml`), bulk v2, render `cmyk.ts`/`eps.ts`/`sheets.ts`, print calculator UI.
10. **E8 Hardening:** RLS migration `00005_rls.sql`, staff console completion, SLA report job + PDF, trust-center pages (`/trust/*` MDX), runbooks (`docs/runbooks/{breach,sso-outage,dr,key-rotation}.md`), k6 scripts, VPAT checklist doc.

## J. FORBIDDEN (in addition to base §16)

Implementing SAML XML parsing or signature validation in Go; linking SSO identities by email for domains not verified by that org; storing SCIM or API tokens in plain text; approving your own change through any path; serving a version whose `approval_status` is `pending`, `rejected` or `cancelled`; writing audit rows outside the business transaction; UPDATE/DELETE on `audit_logs` outside the sealer/retention paths; decrypting lead data in list endpoints without `lead.read` and an audit entry; loading any third-party script on the pixel interstitial before consent in `opt_in_all` mode; rendering customer-supplied HTML/CSS/JS anywhere; fetching customer hosts for domain verification; deactivating redirects for billing reasons.

## K. START

Begin **Enterprise phase E0** now. First file: `docs/manifest/phase-E0.txt`.
