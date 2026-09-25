-- 00002_rls.sql: Row-Level Security (RLS) for tenant isolation
-- Enables defense-in-depth isolation between workspaces.

-- Helper function to check tenant match
CREATE OR REPLACE FUNCTION qrit_current_workspace() RETURNS uuid AS $$
BEGIN
    RETURN NULLIF(current_setting('app.workspace_id', true), '')::uuid;
EXCEPTION WHEN OTHERS THEN
    RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

CREATE OR REPLACE FUNCTION qrit_is_bypass_rls() RETURNS boolean AS $$
BEGIN
    RETURN COALESCE(current_setting('app.bypass_rls', true) = 'on', false);
EXCEPTION WHEN OTHERS THEN
    RETURN false;
END;
$$ LANGUAGE plpgsql STABLE;

-- 1. qr_codes
ALTER TABLE qr_codes ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS qr_codes_tenant_isolation ON qr_codes;
CREATE POLICY qr_codes_tenant_isolation ON qr_codes
    USING (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace());

-- 2. custom_domains
ALTER TABLE custom_domains ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS custom_domains_tenant_isolation ON custom_domains;
CREATE POLICY custom_domains_tenant_isolation ON custom_domains
    USING (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace());

-- 3. campaigns
ALTER TABLE campaigns ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS campaigns_tenant_isolation ON campaigns;
CREATE POLICY campaigns_tenant_isolation ON campaigns
    USING (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace());

-- 4. qr_templates
ALTER TABLE qr_templates ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS qr_templates_tenant_isolation ON qr_templates;
CREATE POLICY qr_templates_tenant_isolation ON qr_templates
    USING (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace());

-- 5. folders
ALTER TABLE folders ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS folders_tenant_isolation ON folders;
CREATE POLICY folders_tenant_isolation ON folders
    USING (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace());

-- 6. api_keys
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS api_keys_tenant_isolation ON api_keys;
CREATE POLICY api_keys_tenant_isolation ON api_keys
    USING (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace());

-- 7. webhooks
ALTER TABLE webhooks ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS webhooks_tenant_isolation ON webhooks;
CREATE POLICY webhooks_tenant_isolation ON webhooks
    USING (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace());

-- 8. audit_logs
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS audit_logs_tenant_isolation ON audit_logs;
CREATE POLICY audit_logs_tenant_isolation ON audit_logs
    USING (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace());

-- 9. workspace_members
ALTER TABLE workspace_members ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS workspace_members_tenant_isolation ON workspace_members;
CREATE POLICY workspace_members_tenant_isolation ON workspace_members
    USING (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace());

-- 10. workspace_invites
ALTER TABLE workspace_invites ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS workspace_invites_tenant_isolation ON workspace_invites;
CREATE POLICY workspace_invites_tenant_isolation ON workspace_invites
    USING (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_is_bypass_rls() OR workspace_id = qrit_current_workspace());
