"""Plan synchronization triggers matching 00004_enterprise.sql.

Plan §6.1 rule 3 & §7.3:
- org_plan_sync() + organizations_plan_sync trigger
- workspace_plan_from_org() + workspaces_plan_from_org trigger
"""

from typing import Any

from django.db import migrations

PLAN_SYNC_UP = """
CREATE OR REPLACE FUNCTION org_plan_sync() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.plan_id IS DISTINCT FROM OLD.plan_id THEN
        UPDATE workspaces SET plan_id = NEW.plan_id, updated_at = now() WHERE org_id = NEW.id;
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS organizations_plan_sync ON organizations;
CREATE TRIGGER organizations_plan_sync AFTER UPDATE OF plan_id ON organizations
    FOR EACH ROW EXECUTE FUNCTION org_plan_sync();

CREATE OR REPLACE FUNCTION workspace_plan_from_org() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    SELECT plan_id INTO NEW.plan_id FROM organizations WHERE id = NEW.org_id;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS workspaces_plan_from_org ON workspaces;
CREATE TRIGGER workspaces_plan_from_org BEFORE INSERT OR UPDATE OF org_id, plan_id ON workspaces
    FOR EACH ROW EXECUTE FUNCTION workspace_plan_from_org();
"""

PLAN_SYNC_DOWN = """
DROP TRIGGER IF EXISTS workspaces_plan_from_org ON workspaces;
DROP TRIGGER IF EXISTS organizations_plan_sync ON organizations;
DROP FUNCTION IF EXISTS workspace_plan_from_org();
DROP FUNCTION IF EXISTS org_plan_sync();
"""


def apply_sync(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute(PLAN_SYNC_UP)


def revert_sync(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute(PLAN_SYNC_DOWN)


class Migration(migrations.Migration):
    dependencies = [
        ("orgs", "0001_initial"),
        (
            "workspaces",
            "0002_workspacepolicy_remove_invite_invites_role_check_and_more",
        ),
    ]

    operations = [
        migrations.RunPython(apply_sync, revert_sync),
    ]
