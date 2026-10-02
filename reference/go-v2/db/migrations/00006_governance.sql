-- 00006_governance.sql: delivery state for SIEM audit streams.

-- +goose Up
ALTER TABLE audit_streams
    ADD COLUMN label             text NOT NULL DEFAULT '',
    ADD COLUMN failing_since     timestamptz,
    ADD COLUMN last_delivered_at timestamptz,
    ADD COLUMN created_by        uuid REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX audit_streams_org_idx ON audit_streams (org_id);

-- Approval inbox and expiry scans.
CREATE INDEX IF NOT EXISTS approval_requests_pending_expiry_idx ON approval_requests (expires_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS approval_requests_requester_idx ON approval_requests (requested_by, created_at DESC);

-- The reviewer role (approvals) can be granted by invitation.
ALTER TABLE invites DROP CONSTRAINT invites_role_check;
ALTER TABLE invites ADD CONSTRAINT invites_role_check CHECK (role IN ('admin','editor','reviewer','analyst'));

-- +goose Down
UPDATE invites SET role = 'analyst' WHERE role = 'reviewer';
ALTER TABLE invites DROP CONSTRAINT invites_role_check;
ALTER TABLE invites ADD CONSTRAINT invites_role_check CHECK (role IN ('admin','editor','analyst'));
DROP INDEX IF EXISTS approval_requests_requester_idx;
DROP INDEX IF EXISTS approval_requests_pending_expiry_idx;
DROP INDEX IF EXISTS audit_streams_org_idx;
ALTER TABLE audit_streams DROP COLUMN IF EXISTS created_by, DROP COLUMN IF EXISTS last_delivered_at,
    DROP COLUMN IF EXISTS failing_since, DROP COLUMN IF EXISTS label;
