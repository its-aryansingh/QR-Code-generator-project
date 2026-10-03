"""Row-Level Security (RLS) policies for enterprise tenant tables matching 00004_enterprise.sql lines 891-947.

Plan §6.1 rule 3 & §6.2:
- role_bindings
- workspace_policies
- approval_requests
- integrations
- alert_rules
- report_schedules
- forms
- form_submissions
- pixels
- gs1_items
- serial_batches
"""

from typing import Any

from django.db import migrations

ENTERPRISE_RLS_TABLES = [
    "role_bindings",
    "workspace_policies",
    "approval_requests",
    "integrations",
    "alert_rules",
    "report_schedules",
    "forms",
    "form_submissions",
    "pixels",
    "gs1_items",
    "serial_batches",
]

UP_SQL_PARTS = []
DOWN_SQL_PARTS = []

for table in ENTERPRISE_RLS_TABLES:
    UP_SQL_PARTS.append(f"""
ALTER TABLE {table} ENABLE ROW LEVEL SECURITY;
ALTER TABLE {table} FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS {table}_tenant_isolation ON {table};
CREATE POLICY {table}_tenant_isolation ON {table}
    USING (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace())
    WITH CHECK (qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace());
""")
    DOWN_SQL_PARTS.append(f"""
DROP POLICY IF EXISTS {table}_tenant_isolation ON {table};
ALTER TABLE {table} NO FORCE ROW LEVEL SECURITY;
ALTER TABLE {table} DISABLE ROW LEVEL SECURITY;
""")

ENTERPRISE_RLS_UP = "\n".join(UP_SQL_PARTS)
ENTERPRISE_RLS_DOWN = "\n".join(DOWN_SQL_PARTS)


def apply_enterprise_rls(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute(ENTERPRISE_RLS_UP)


def revert_enterprise_rls(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute(ENTERPRISE_RLS_DOWN)


class Migration(migrations.Migration):
    dependencies = [
        ("core", "0003_orgdatakey_featureflag"),
        ("access", "0003_initial"),
        (
            "workspaces",
            "0002_workspacepolicy_remove_invite_invites_role_check_and_more",
        ),
        ("approvals", "0001_initial"),
        (
            "integrations",
            "0003_integration_integrationdelivery_and_more",
        ),
        ("alerts", "0001_initial"),
        ("reports", "0001_initial"),
        ("leads", "0002_initial"),
        ("pixels", "0001_initial"),
        ("gs1", "0001_initial"),
        ("serials", "0001_initial"),
    ]

    operations = [
        migrations.RunPython(apply_enterprise_rls, revert_enterprise_rls),
    ]
