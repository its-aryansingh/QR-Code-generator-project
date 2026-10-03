"""Invoice numbering sequence function matching 00004_enterprise.sql.

Plan §6.1 rule 3 & §7.13:
- next_invoice_number(p_date date) RETURNS text
"""

from typing import Any

from django.db import migrations

NEXT_INVOICE_NUMBER_UP = """
CREATE OR REPLACE FUNCTION next_invoice_number(p_date date) RETURNS text LANGUAGE plpgsql AS $$
DECLARE v_fy text; v_seq integer; y integer;
BEGIN
    y := CASE WHEN extract(month FROM p_date) >= 4 THEN extract(year FROM p_date)::int
              ELSE extract(year FROM p_date)::int - 1 END;
    v_fy := y::text || '-' || lpad(((y + 1) % 100)::text, 2, '0');
    INSERT INTO invoice_sequences (fy, last_seq) VALUES (v_fy, 1)
    ON CONFLICT (fy) DO UPDATE SET last_seq = invoice_sequences.last_seq + 1
    RETURNING last_seq INTO v_seq;
    RETURN 'QR/' || replace(v_fy, '-', '') || '/' || lpad(v_seq::text, 5, '0');
END $$;
"""

NEXT_INVOICE_NUMBER_DOWN = """
DROP FUNCTION IF EXISTS next_invoice_number(date);
"""


def apply_fn(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute(NEXT_INVOICE_NUMBER_UP)


def revert_fn(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute(NEXT_INVOICE_NUMBER_DOWN)


class Migration(migrations.Migration):
    dependencies = [
        ("billing", "0003_contract_invoice_invoicesequence_subscription_org_and_more"),
    ]

    operations = [
        migrations.RunPython(apply_fn, revert_fn),
    ]
