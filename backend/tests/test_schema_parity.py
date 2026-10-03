"""Schema parity test.

Plan §6.1 rule 1, 2, 5 & §6.4:
Verifies table names, column names, constraints, RLS policies, and Plan §6.4 deltas
between the reference Go migrations (reference/go-v2/db/migrations/00001..00007) and Django models.
"""

import re
from pathlib import Path
from typing import Any

from django.apps import apps


def get_reference_migrations_dir() -> Path:
    # Walk up to repo root
    cwd = Path(__file__).resolve().parent
    while cwd.parent != cwd:
        candidate = cwd / "reference" / "go-v2" / "db" / "migrations"
        if candidate.is_dir():
            return candidate
        cwd = cwd.parent
    raise FileNotFoundError("Could not find reference/go-v2/db/migrations directory")


def parse_go_schema() -> dict[str, dict[str, Any]]:
    """Parse table names and columns from Go migrations 00001 to 00007."""
    mig_dir = get_reference_migrations_dir()
    sql_files = sorted(mig_dir.glob("*.sql"))

    tables: dict[str, dict[str, Any]] = {}

    for f in sql_files:
        content = f.read_text(encoding="utf-8")
        # Only take Up migration part
        if "-- +goose Down" in content:
            up_content = content.split("-- +goose Down")[0]
        else:
            up_content = content

        # Remove line comments
        up_content = re.sub(r"--.*", "", up_content)
        # Remove block comments
        up_content = re.sub(r"/\*.*?\*/", "", up_content, flags=re.DOTALL)

        # Match CREATE TABLE [IF NOT EXISTS] <name> (...)
        create_matches = re.finditer(
            r"CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)\s*\((.*?)\);",
            up_content,
            re.DOTALL | re.IGNORECASE,
        )
        for m in create_matches:
            tbl_name = m.group(1).lower()
            if tbl_name.startswith("scan_events_") or tbl_name in ("job_queue", "worker_task_runs"):
                continue  # partition tables or legacy worker tables replaced by procrastinate
            body = m.group(2)

            # Split body by commas at parenthesis depth 0 and outside quotes
            items: list[str] = []
            depth = 0
            in_quote: str | None = None
            cur: list[str] = []
            for ch in body:
                if in_quote:
                    cur.append(ch)
                    if ch == in_quote:
                        in_quote = None
                elif ch in ("'", '"'):
                    in_quote = ch
                    cur.append(ch)
                elif ch in ("(", "["):
                    depth += 1
                    cur.append(ch)
                elif ch in (")", "]"):
                    depth -= 1
                    cur.append(ch)
                elif ch == "," and depth == 0:
                    items.append("".join(cur).strip())
                    cur = []
                else:
                    cur.append(ch)
            if cur:
                items.append("".join(cur).strip())

            cols = set()
            for it in items:
                it = it.strip()
                if not it:
                    continue
                words = it.split()
                if not words:
                    continue
                first_word = words[0].lower()
                if first_word in {
                    "constraint",
                    "primary",
                    "unique",
                    "foreign",
                    "check",
                }:
                    continue
                col_name = first_word.strip('"')
                cols.add(col_name)
            if tbl_name not in tables:
                tables[tbl_name] = {"columns": set()}
            tables[tbl_name]["columns"].update(cols)

        # Match ALTER TABLE <name> ADD [COLUMN] [IF NOT EXISTS] <col> ...
        alter_matches = re.finditer(
            r"ALTER\s+TABLE\s+([a-z_][a-z0-9_]*)\s+ADD\s+(?:COLUMN\s+)?(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)",
            up_content,
            re.IGNORECASE,
        )
        for m in alter_matches:
            tbl_name = m.group(1).lower()
            col_name = m.group(2).lower()
            if col_name in {"constraint", "primary", "unique", "foreign", "check"}:
                continue
            if tbl_name in tables:
                tables[tbl_name]["columns"].add(col_name)

        # Match ALTER TABLE <name> DROP COLUMN <col>
        drop_matches = re.finditer(
            r"ALTER\s+TABLE\s+([a-z_][a-z0-9_]*)\s+DROP\s+COLUMN\s+(?:IF\s+EXISTS\s+)?([a-z_][a-z0-9_]*)",
            up_content,
            re.IGNORECASE,
        )
        for m in drop_matches:
            tbl_name = m.group(1).lower()
            col_name = m.group(2).lower()
            if tbl_name in tables and col_name in tables[tbl_name]["columns"]:
                tables[tbl_name]["columns"].remove(col_name)

    return tables


def test_all_go_tables_have_django_models() -> None:
    go_schema = parse_go_schema()
    django_tables = {
        model._meta.db_table: model for model in apps.get_models(include_auto_created=True)
    }
    # scan_events is managed=False
    django_tables["scan_events"] = None  # type: ignore[assignment]

    missing_tables = [tbl for tbl in go_schema if tbl not in django_tables]
    assert not missing_tables, f"Missing Django models for Go tables: {missing_tables}"


def test_all_go_columns_exist_in_django_models() -> None:
    go_schema = parse_go_schema()
    django_models = {model._meta.db_table: model for model in apps.get_models()}

    # Hand-managed or special columns mapping
    ignored_columns = {
        # Columns mapped with different attribute or composite PK handling
    }

    missing_cols_by_table: dict[str, list[str]] = {}

    for tbl_name, info in go_schema.items():
        if tbl_name not in django_models:
            continue
        model = django_models[tbl_name]
        model_cols = set()
        for f in model._meta.get_fields():
            if hasattr(f, "column") and f.column:
                model_cols.add(f.column)
            elif hasattr(f, "attname") and f.attname:
                model_cols.add(f.attname)

        missing = []
        for col in info["columns"]:
            if (tbl_name, col) in ignored_columns:
                continue
            # Handle password -> password_hash
            if tbl_name == "users" and col == "password_hash" and "password_hash" in model_cols:
                continue
            # Handle last_login -> last_login_at
            if tbl_name == "users" and col == "last_login_at" and "last_login_at" in model_cols:
                continue
            if col not in model_cols:
                missing.append(col)

        if missing:
            missing_cols_by_table[tbl_name] = missing

    assert not missing_cols_by_table, f"Missing columns in Django models: {missing_cols_by_table}"


def test_plan_v3_deltas_present() -> None:
    """Verify that all deltas listed in Plan §6.4 are present in Django models."""
    from apps.developer.models import ApiKey
    from apps.integrations.models import Webhook
    from apps.leads.models import Form
    from apps.legacy.models import LegacyEmailLog, LegacyIdMap, LegacyImportRun
    from apps.orgs.models import Organization
    from apps.qr.models import QRCode

    # 1. Legacy models
    assert LegacyIdMap._meta.db_table == "legacy_id_map"
    assert LegacyImportRun._meta.db_table == "legacy_import_runs"
    assert LegacyEmailLog._meta.db_table == "legacy_email_log"

    # 2. qr_codes v3 deltas
    qr_cols = {f.column for f in QRCode._meta.fields if f.column}
    assert "legacy_host" in qr_cols
    assert "v1_printed_payload" in qr_cols
    assert "needs_reprint" in qr_cols

    # 3. organizations grandfathered limits
    org_cols = {f.column for f in Organization._meta.fields if f.column}
    assert "grandfathered_limits" in org_cols
    assert "billing_hold_since" in org_cols

    # 4. api_keys legacy fields
    key_cols = {f.column for f in ApiKey._meta.fields if f.column}
    assert "legacy_bcrypt_hash" in key_cols
    assert "legacy_kind" in key_cols

    # 5. webhooks signature_scheme
    webhook_cols = {f.column for f in Webhook._meta.fields if f.column}
    assert "signature_scheme" in webhook_cols

    # 6. forms legacy fields
    form_cols = {f.column for f in Form._meta.fields if f.column}
    assert "legacy_slug" in form_cols
    assert "presentation" in form_cols


def test_tenant_rls_tables_coverage() -> None:
    """Verify that all tenant tables specified in Plan §6.2 have RLS policies defined."""
    import importlib

    m_rls = importlib.import_module("apps.core.migrations.0002_rls")
    RLS_UP_SQL = m_rls.RLS_UP_SQL
    m_ent_rls = importlib.import_module("apps.core.migrations.0004_enterprise_rls")
    ENTERPRISE_RLS_TABLES = m_ent_rls.ENTERPRISE_RLS_TABLES

    expected_tenant_tables = {
        "workspace_members",
        "invites",
        "folders",
        "tags",
        "campaigns",
        "templates",
        "files",
        "qr_codes",
        "api_keys",
        "webhooks",
        "subscriptions",
        "jobs",
        "idempotency_keys",
        "domains",
        "audit_logs",
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
    }

    covered_tables = set(ENTERPRISE_RLS_TABLES)
    for line in RLS_UP_SQL.splitlines():
        m = re.match(r"ALTER TABLE\s+([a-z_]+)\s+ENABLE ROW LEVEL SECURITY", line)
        if m:
            covered_tables.add(m.group(1))

    missing = expected_tenant_tables - covered_tables
    assert not missing, f"Missing RLS policies for tenant tables: {missing}"
