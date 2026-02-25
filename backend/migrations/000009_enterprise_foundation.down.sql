-- Rollback enterprise foundation

-- Remove enterprise columns from users
ALTER TABLE users DROP COLUMN IF EXISTS default_workspace_id;

-- Remove enterprise columns from qr_records
DROP INDEX IF EXISTS idx_qr_folder;
DROP INDEX IF EXISTS idx_qr_workspace;
ALTER TABLE qr_records DROP COLUMN IF EXISTS geo_restrictions;
ALTER TABLE qr_records DROP COLUMN IF EXISTS scheduled_at;
ALTER TABLE qr_records DROP COLUMN IF EXISTS max_scans;
ALTER TABLE qr_records DROP COLUMN IF EXISTS password;
ALTER TABLE qr_records DROP COLUMN IF EXISTS tags;
ALTER TABLE qr_records DROP COLUMN IF EXISTS folder_id;
ALTER TABLE qr_records DROP COLUMN IF EXISTS workspace_id;

-- Drop tables in dependency order
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS webhook_logs;
DROP TABLE IF EXISTS webhooks;
DROP TABLE IF EXISTS folders;
DROP TABLE IF EXISTS workspace_invites;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;
