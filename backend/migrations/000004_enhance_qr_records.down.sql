-- Revert qr_records enhancements
DROP INDEX IF EXISTS idx_qr_records_is_active;
DROP INDEX IF EXISTS idx_qr_records_is_dynamic;
DROP INDEX IF EXISTS idx_qr_records_short_code;

ALTER TABLE qr_records DROP COLUMN IF EXISTS qr_type_id;
ALTER TABLE qr_records DROP COLUMN IF EXISTS scan_count;
ALTER TABLE qr_records DROP COLUMN IF EXISTS title;
ALTER TABLE qr_records DROP COLUMN IF EXISTS expires_at;
ALTER TABLE qr_records DROP COLUMN IF EXISTS is_active;
ALTER TABLE qr_records DROP COLUMN IF EXISTS customization;
ALTER TABLE qr_records DROP COLUMN IF EXISTS metadata;
ALTER TABLE qr_records DROP COLUMN IF EXISTS redirect_url;
ALTER TABLE qr_records DROP COLUMN IF EXISTS is_dynamic;
ALTER TABLE qr_records DROP COLUMN IF EXISTS short_code;
