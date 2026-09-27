-- 00007_billing.sql: contract invoicing runtime, dunning, payment links, support sessions.

-- +goose Up
ALTER TABLE invoices
    ADD COLUMN period_start      date,
    ADD COLUMN period_end        date,
    ADD COLUMN payment_link_id   text,
    ADD COLUMN payment_link_url  text,
    ADD COLUMN reminders_sent    integer NOT NULL DEFAULT 0,
    ADD COLUMN last_reminder_at  timestamptz,
    ADD COLUMN void_reason       text,
    ADD COLUMN paid_reference    text;
-- One invoice per contract period, so the invoice run is idempotent.
CREATE UNIQUE INDEX invoices_contract_period_uniq ON invoices (contract_id, period_start)
    WHERE contract_id IS NOT NULL AND period_start IS NOT NULL AND status <> 'void';
CREATE INDEX invoices_org_idx ON invoices (org_id, issue_date DESC);
CREATE INDEX invoices_unpaid_idx ON invoices (due_date) WHERE status = 'issued';

-- Dashboard edits are frozen 30 days after an unpaid due date; redirects never stop.
ALTER TABLE organizations ADD COLUMN billing_hold_since timestamptz;

-- A staff member's support session under a customer grant (banner + audit).
CREATE TABLE support_sessions (
    id             uuid PRIMARY KEY,
    grant_id       uuid NOT NULL REFERENCES support_access_grants(id) ON DELETE CASCADE,
    org_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    staff_user_id  uuid NOT NULL REFERENCES users(id),
    reason         text NOT NULL,
    started_at     timestamptz NOT NULL DEFAULT now(),
    expires_at     timestamptz NOT NULL,
    ended_at       timestamptz
);
CREATE INDEX support_sessions_org_idx ON support_sessions (org_id, started_at DESC);
CREATE INDEX support_access_grants_org_idx ON support_access_grants (org_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS support_access_grants_org_idx;
DROP TABLE IF EXISTS support_sessions;
ALTER TABLE organizations DROP COLUMN IF EXISTS billing_hold_since;
DROP INDEX IF EXISTS invoices_unpaid_idx;
DROP INDEX IF EXISTS invoices_org_idx;
DROP INDEX IF EXISTS invoices_contract_period_uniq;
ALTER TABLE invoices DROP COLUMN IF EXISTS paid_reference, DROP COLUMN IF EXISTS void_reason,
    DROP COLUMN IF EXISTS last_reminder_at, DROP COLUMN IF EXISTS reminders_sent, DROP COLUMN IF EXISTS payment_link_url,
    DROP COLUMN IF EXISTS payment_link_id, DROP COLUMN IF EXISTS period_end, DROP COLUMN IF EXISTS period_start;
