-- Enhance qr_records table with dynamic QR and customization support

-- Add short code for dynamic QR redirects
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS short_code VARCHAR(20) UNIQUE;

-- Dynamic QR support
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS is_dynamic BOOLEAN DEFAULT false;
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS redirect_url TEXT;

-- Type-specific metadata (vCard fields, WiFi settings, etc.)
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}';

-- Customization settings (colors, logo, shape)
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS customization JSONB DEFAULT '{}';

-- QR lifecycle management
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true;
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS title VARCHAR(255);
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS scan_count INTEGER DEFAULT 0;

-- Update qr_type to reference qr_types table
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS qr_type_id VARCHAR(50) REFERENCES qr_types(id);

-- Index for short code lookups (critical for redirects)
CREATE UNIQUE INDEX IF NOT EXISTS idx_qr_records_short_code ON qr_records(short_code) WHERE short_code IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_qr_records_is_dynamic ON qr_records(is_dynamic) WHERE is_dynamic = true;
CREATE INDEX IF NOT EXISTS idx_qr_records_is_active ON qr_records(is_active);
