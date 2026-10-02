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
