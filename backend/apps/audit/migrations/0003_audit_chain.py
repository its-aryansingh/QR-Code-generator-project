"""Tamper-evident audit chain functions, triggers, and identity setting matching 00004_enterprise.sql.

Plan §6.1 rule 3 & §7.12:
- audit_logs.id GENERATED ALWAYS AS IDENTITY
- audit_entry_bytes
- audit_logs_guard + audit_logs_append_only trigger
- audit_seal
- audit_verify
"""

from typing import Any

from django.db import migrations

AUDIT_CHAIN_UP = """
DO $$
BEGIN
    ALTER TABLE audit_logs ALTER COLUMN id SET GENERATED ALWAYS;
EXCEPTION WHEN OTHERS THEN
    NULL;
END $$;

CREATE OR REPLACE FUNCTION audit_entry_bytes(a audit_logs) RETURNS bytea LANGUAGE sql IMMUTABLE AS $$
    SELECT convert_to(concat_ws(E'\x1f',
        a.id::text, coalesce(a.org_id::text,''), coalesce(a.workspace_id::text,''), a.actor_type,
        coalesce(a.actor_id::text,''), a.action, a.target_type, coalesce(a.target_id::text,''),
        a.changes::text, coalesce(a.ip_prefix,''), coalesce(a.request_id,''),
        (extract(epoch FROM a.created_at) * 1000000)::bigint::text), 'UTF8')
$$;

CREATE OR REPLACE FUNCTION audit_logs_guard() RETURNS trigger LANGUAGE plpgsql AS $$
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

DROP TRIGGER IF EXISTS audit_logs_append_only ON audit_logs;
CREATE TRIGGER audit_logs_append_only BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_guard();

CREATE OR REPLACE FUNCTION audit_seal(p_org uuid, p_limit integer DEFAULT 1000) RETURNS integer
LANGUAGE plpgsql AS $$
DECLARE
    v_prev bytea; v_seq bigint; r audit_logs; n integer := 0; v_hash bytea;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('audit_seal:' || p_org::text, 0));
    SELECT hash, seq INTO v_prev, v_seq FROM audit_logs
     WHERE org_id = p_org AND seq IS NOT NULL ORDER BY seq DESC LIMIT 1;
    v_prev := coalesce(v_prev, '\\x'::bytea); v_seq := coalesce(v_seq, 0);
    FOR r IN SELECT * FROM audit_logs WHERE org_id = p_org AND hash IS NULL ORDER BY id LIMIT p_limit LOOP
        v_seq  := v_seq + 1;
        v_hash := sha256(v_prev || int8send(v_seq) || audit_entry_bytes(r));
        UPDATE audit_logs SET seq = v_seq, prev_hash = v_prev, hash = v_hash, sealed_at = now() WHERE id = r.id;
        v_prev := v_hash; n := n + 1;
    END LOOP;
    RETURN n;
END $$;

CREATE OR REPLACE FUNCTION audit_verify(p_org uuid) RETURNS bigint LANGUAGE plpgsql STABLE AS $$
DECLARE v_prev bytea; r audit_logs;
BEGIN
    SELECT CASE WHEN seq = 1 THEN '\\x'::bytea ELSE prev_hash END INTO v_prev
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
"""

AUDIT_CHAIN_DOWN = """
DROP FUNCTION IF EXISTS audit_verify(uuid);
DROP FUNCTION IF EXISTS audit_seal(uuid, integer);
DROP TRIGGER IF EXISTS audit_logs_append_only ON audit_logs;
DROP FUNCTION IF EXISTS audit_logs_guard();
DROP FUNCTION IF EXISTS audit_entry_bytes(audit_logs);
"""


def apply_audit_chain(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        with schema_editor.connection.cursor() as cursor:
            cursor.execute(AUDIT_CHAIN_UP)


def revert_audit_chain(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        with schema_editor.connection.cursor() as cursor:
            cursor.execute(AUDIT_CHAIN_DOWN)


class Migration(migrations.Migration):
    dependencies = [
        ("audit", "0002_auditanchor_auditstream_auditlog_hash_and_more"),
    ]

    operations = [
        migrations.RunPython(apply_audit_chain, revert_audit_chain),
    ]
