"""Row-Level Security (RLS) migration matching 00002_rls.sql.

Plan §6.1 rule 3 & §7.1:
- qrit_current_workspace() function
- ENABLE & FORCE ROW LEVEL SECURITY on tenant tables
- Tenant isolation policies
"""

from typing import Any

from django.db import migrations

RLS_UP_SQL = """
CREATE OR REPLACE FUNCTION qrit_current_workspace() RETURNS uuid
LANGUAGE sql STABLE PARALLEL SAFE AS $$
    SELECT NULLIF(current_setting('app.workspace_id', true), '')::uuid
$$;

ALTER TABLE workspace_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE workspace_members FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_members_tenant_isolation ON workspace_members
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE invites ENABLE ROW LEVEL SECURITY;
ALTER TABLE invites FORCE ROW LEVEL SECURITY;
CREATE POLICY invites_tenant_isolation ON invites
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE folders ENABLE ROW LEVEL SECURITY;
ALTER TABLE folders FORCE ROW LEVEL SECURITY;
CREATE POLICY folders_tenant_isolation ON folders
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE tags ENABLE ROW LEVEL SECURITY;
ALTER TABLE tags FORCE ROW LEVEL SECURITY;
CREATE POLICY tags_tenant_isolation ON tags
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE campaigns ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaigns FORCE ROW LEVEL SECURITY;
CREATE POLICY campaigns_tenant_isolation ON campaigns
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE templates FORCE ROW LEVEL SECURITY;
CREATE POLICY templates_tenant_isolation ON templates
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE files ENABLE ROW LEVEL SECURITY;
ALTER TABLE files FORCE ROW LEVEL SECURITY;
CREATE POLICY files_tenant_isolation ON files
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE qr_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE qr_codes FORCE ROW LEVEL SECURITY;
CREATE POLICY qr_codes_tenant_isolation ON qr_codes
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY api_keys_tenant_isolation ON api_keys
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE webhooks ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhooks FORCE ROW LEVEL SECURITY;
CREATE POLICY webhooks_tenant_isolation ON webhooks
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions FORCE ROW LEVEL SECURITY;
CREATE POLICY subscriptions_tenant_isolation ON subscriptions
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobs FORCE ROW LEVEL SECURITY;
CREATE POLICY jobs_tenant_isolation ON jobs
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE idempotency_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE idempotency_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY idempotency_keys_tenant_isolation ON idempotency_keys
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE domains ENABLE ROW LEVEL SECURITY;
ALTER TABLE domains FORCE ROW LEVEL SECURITY;
CREATE POLICY domains_tenant_isolation ON domains
    USING (qrit_current_workspace() IS NULL OR workspace_id IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id IS NULL OR workspace_id = qrit_current_workspace());

ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY;
CREATE POLICY audit_logs_tenant_isolation ON audit_logs
    USING (qrit_current_workspace() IS NULL OR workspace_id IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id IS NULL OR workspace_id = qrit_current_workspace());
"""

RLS_DOWN_SQL = """
DROP POLICY IF EXISTS workspace_members_tenant_isolation ON workspace_members;
ALTER TABLE workspace_members NO FORCE ROW LEVEL SECURITY;
ALTER TABLE workspace_members DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS invites_tenant_isolation ON invites;
ALTER TABLE invites NO FORCE ROW LEVEL SECURITY;
ALTER TABLE invites DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS folders_tenant_isolation ON folders;
ALTER TABLE folders NO FORCE ROW LEVEL SECURITY;
ALTER TABLE folders DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tags_tenant_isolation ON tags;
ALTER TABLE tags NO FORCE ROW LEVEL SECURITY;
ALTER TABLE tags DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS campaigns_tenant_isolation ON campaigns;
ALTER TABLE campaigns NO FORCE ROW LEVEL SECURITY;
ALTER TABLE campaigns DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS templates_tenant_isolation ON templates;
ALTER TABLE templates NO FORCE ROW LEVEL SECURITY;
ALTER TABLE templates DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS files_tenant_isolation ON files;
ALTER TABLE files NO FORCE ROW LEVEL SECURITY;
ALTER TABLE files DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS qr_codes_tenant_isolation ON qr_codes;
ALTER TABLE qr_codes NO FORCE ROW LEVEL SECURITY;
ALTER TABLE qr_codes DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS api_keys_tenant_isolation ON api_keys;
ALTER TABLE api_keys NO FORCE ROW LEVEL SECURITY;
ALTER TABLE api_keys DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS webhooks_tenant_isolation ON webhooks;
ALTER TABLE webhooks NO FORCE ROW LEVEL SECURITY;
ALTER TABLE webhooks DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS subscriptions_tenant_isolation ON subscriptions;
ALTER TABLE subscriptions NO FORCE ROW LEVEL SECURITY;
ALTER TABLE subscriptions DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS jobs_tenant_isolation ON jobs;
ALTER TABLE jobs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE jobs DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS idempotency_keys_tenant_isolation ON idempotency_keys;
ALTER TABLE idempotency_keys NO FORCE ROW LEVEL SECURITY;
ALTER TABLE idempotency_keys DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS domains_tenant_isolation ON domains;
ALTER TABLE domains NO FORCE ROW LEVEL SECURITY;
ALTER TABLE domains DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS audit_logs_tenant_isolation ON audit_logs;
ALTER TABLE audit_logs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE audit_logs DISABLE ROW LEVEL SECURITY;

DROP FUNCTION IF EXISTS qrit_current_workspace();
"""


def apply_rls(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute(RLS_UP_SQL)


def revert_rls(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute(RLS_DOWN_SQL)


class Migration(migrations.Migration):
    dependencies = [
        ("core", "0001_initial"),
        ("workspaces", "0001_initial"),
        ("qr", "0002_initial"),
        ("developer", "0002_initial"),
        ("integrations", "0002_initial"),
        ("billing", "0002_initial"),
        ("audit", "0001_initial"),
    ]

    operations = [
        migrations.RunPython(apply_rls, revert_rls),
    ]
