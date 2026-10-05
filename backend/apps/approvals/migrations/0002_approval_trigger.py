"""Approval four-eyes rule trigger matching 00004_enterprise.sql.

Plan §6.1 rule 3 & §7.11:
- approval_no_self_approval function
- approval_decisions_guard trigger
"""

from typing import Any

from django.db import migrations

TRIGGER_UP = """
CREATE OR REPLACE FUNCTION approval_no_self_approval() RETURNS trigger LANGUAGE plpgsql AS $$
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

DROP TRIGGER IF EXISTS approval_decisions_guard ON approval_decisions;
CREATE TRIGGER approval_decisions_guard BEFORE INSERT ON approval_decisions
    FOR EACH ROW EXECUTE FUNCTION approval_no_self_approval();
"""

TRIGGER_DOWN = """
DROP TRIGGER IF EXISTS approval_decisions_guard ON approval_decisions;
DROP FUNCTION IF EXISTS approval_no_self_approval();
"""


def apply_trigger(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        with schema_editor.connection.cursor() as cursor:
            cursor.execute(TRIGGER_UP)


def revert_trigger(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        with schema_editor.connection.cursor() as cursor:
            cursor.execute(TRIGGER_DOWN)


class Migration(migrations.Migration):
    dependencies = [
        ("approvals", "0001_initial"),
    ]

    operations = [
        migrations.RunPython(apply_trigger, revert_trigger),
    ]
