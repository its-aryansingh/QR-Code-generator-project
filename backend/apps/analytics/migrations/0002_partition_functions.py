"""SECURITY DEFINER partition maintenance functions for scan_events.

Plan §6.1 rule 3 & §7.10:
- qrit_ensure_scan_partitions(p_from date, p_to date)
- qrit_drop_scan_partitions(p_before date)
"""

from typing import Any

from django.db import migrations

PARTITION_FUNCTIONS_UP = """
CREATE OR REPLACE FUNCTION qrit_ensure_scan_partitions(p_from date, p_to date)
RETURNS integer
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
    cur_from date;
    cur_to   date;
    p_name   text;
    attached boolean;
    created_count integer := 0;
BEGIN
    cur_from := date_trunc('month', p_from)::date;
    WHILE cur_from <= date_trunc('month', p_to)::date LOOP
        cur_to := (cur_from + interval '1 month')::date;
        p_name := 'scan_events_' || to_char(cur_from, 'YYYY_MM');

        SELECT EXISTS (
            SELECT 1 FROM pg_inherits i
            JOIN pg_class c ON c.oid = i.inhrelid
            WHERE i.inhparent = 'scan_events'::regclass AND c.relname = p_name
        ) INTO attached;

        IF NOT attached THEN
            EXECUTE format('CREATE TABLE IF NOT EXISTS %I (LIKE scan_events INCLUDING DEFAULTS INCLUDING CONSTRAINTS)', p_name);
            EXECUTE format('WITH moved AS (DELETE FROM scan_events_default WHERE occurred_at >= %L AND occurred_at < %L RETURNING *) INSERT INTO %I SELECT * FROM moved', cur_from, cur_to, p_name);
            EXECUTE format('ALTER TABLE scan_events ATTACH PARTITION %I FOR VALUES FROM (%L) TO (%L)', p_name, cur_from, cur_to);
            created_count := created_count + 1;
        END IF;

        cur_from := cur_to;
    END LOOP;
    RETURN created_count;
END;
$$;

CREATE OR REPLACE FUNCTION qrit_drop_scan_partitions(p_before date)
RETURNS integer
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
    r record;
    part_date date;
    dropped_count integer := 0;
BEGIN
    FOR r IN (
        SELECT c.relname
        FROM pg_inherits i
        JOIN pg_class c ON c.oid = i.inhrelid
        WHERE i.inhparent = 'scan_events'::regclass
          AND c.relname ~ '^scan_events_[0-9]{4}_[0-9]{2}$'
    ) LOOP
        part_date := to_date(substring(r.relname from 13 for 7), 'YYYY_MM');
        IF (part_date + interval '1 month')::date <= p_before THEN
            EXECUTE format('ALTER TABLE scan_events DETACH PARTITION %I', r.relname);
            EXECUTE format('DROP TABLE IF EXISTS %I', r.relname);
            dropped_count := dropped_count + 1;
        END IF;
    END LOOP;
    RETURN dropped_count;
END;
$$;

REVOKE ALL ON FUNCTION qrit_ensure_scan_partitions(date, date) FROM PUBLIC;
REVOKE ALL ON FUNCTION qrit_drop_scan_partitions(date) FROM PUBLIC;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'qrit_app') THEN
        GRANT EXECUTE ON FUNCTION qrit_ensure_scan_partitions(date, date) TO qrit_app;
        GRANT EXECUTE ON FUNCTION qrit_drop_scan_partitions(date) TO qrit_app;
    END IF;
END $$;
"""

PARTITION_FUNCTIONS_DOWN = """
DROP FUNCTION IF EXISTS qrit_drop_scan_partitions(date);
DROP FUNCTION IF EXISTS qrit_ensure_scan_partitions(date, date);
"""


def apply_fn(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute(PARTITION_FUNCTIONS_UP)


def revert_fn(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute(PARTITION_FUNCTIONS_DOWN)


class Migration(migrations.Migration):
    dependencies = [
        ("analytics", "0001_initial"),
    ]

    operations = [
        migrations.RunPython(apply_fn, revert_fn),
    ]
