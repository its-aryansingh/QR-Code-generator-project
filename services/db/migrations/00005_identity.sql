-- 00005_identity.sql: runtime columns for SSO, MFA and domain verification.

-- +goose Up
-- A session created for a user with a second factor starts "pending" until the factor is
-- verified; only MFA endpoints are reachable meanwhile.
ALTER TABLE sessions ADD COLUMN mfa_pending boolean NOT NULL DEFAULT false;

ALTER TABLE sso_connections
    ADD COLUMN oidc_scopes      text NOT NULL DEFAULT 'openid email profile',
    ADD COLUMN groups_claim     text NOT NULL DEFAULT 'groups',
    ADD COLUMN test_passed_at   timestamptz,
    ADD COLUMN saml_metadata_url text,
    ADD COLUMN idp_kind         text NOT NULL DEFAULT 'custom'
        CHECK (idp_kind IN ('okta','entra','google','jumpcloud','onelogin','ping','keycloak','custom'));

ALTER TABLE org_domains
    ADD COLUMN last_checked_at timestamptz,
    ADD COLUMN check_attempts  integer NOT NULL DEFAULT 0;

-- Several organisations may claim a domain while verification is pending; only one can hold
-- it verified. (A global unique on the claim would let a squatter block the real owner.)
ALTER TABLE org_domains DROP CONSTRAINT org_domains_domain_key;
ALTER TABLE org_domains ADD CONSTRAINT org_domains_org_domain_key UNIQUE (org_id, domain);
CREATE UNIQUE INDEX org_domains_verified_uniq ON org_domains (domain) WHERE verified_at IS NOT NULL;

-- Group bindings are the SCIM/SSO way to grant access: role_bindings with principal_type
-- 'group' already exist; this index serves membership lookups at login.
CREATE INDEX IF NOT EXISTS groups_org_source_idx ON groups (org_id, source);

-- +goose Down
DROP INDEX IF EXISTS groups_org_source_idx;
DELETE FROM org_domains a USING org_domains b
    WHERE a.domain = b.domain AND a.id <> b.id AND (a.verified_at IS NULL AND (b.verified_at IS NOT NULL OR a.created_at > b.created_at));
DROP INDEX IF EXISTS org_domains_verified_uniq;
ALTER TABLE org_domains DROP CONSTRAINT IF EXISTS org_domains_org_domain_key;
ALTER TABLE org_domains ADD CONSTRAINT org_domains_domain_key UNIQUE (domain);
ALTER TABLE org_domains DROP COLUMN IF EXISTS check_attempts, DROP COLUMN IF EXISTS last_checked_at;
ALTER TABLE sso_connections DROP COLUMN IF EXISTS idp_kind, DROP COLUMN IF EXISTS saml_metadata_url,
    DROP COLUMN IF EXISTS test_passed_at, DROP COLUMN IF EXISTS groups_claim, DROP COLUMN IF EXISTS oidc_scopes;
ALTER TABLE sessions DROP COLUMN IF EXISTS mfa_pending;
