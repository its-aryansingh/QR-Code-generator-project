-- Create QR scans analytics table
CREATE TABLE IF NOT EXISTS qr_scans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    qr_id UUID NOT NULL REFERENCES qr_records(id) ON DELETE CASCADE,
    scanned_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Network info
    ip_address INET,
    
    -- GeoIP data (MaxMind)
    country_code VARCHAR(2),
    country_name VARCHAR(100),
    city VARCHAR(100),
    region VARCHAR(100),
    latitude DECIMAL(9, 6),
    longitude DECIMAL(9, 6),
    
    -- Device info
    device_type VARCHAR(20),  -- mobile, tablet, desktop, unknown
    os VARCHAR(50),
    os_version VARCHAR(20),
    browser VARCHAR(50),
    browser_version VARCHAR(20),
    
    -- Request info
    user_agent TEXT,
    referrer TEXT,
    language VARCHAR(10)
);

-- Indexes for analytics queries
CREATE INDEX idx_qr_scans_qr_id ON qr_scans(qr_id);
CREATE INDEX idx_qr_scans_scanned_at ON qr_scans(scanned_at);
CREATE INDEX idx_qr_scans_country ON qr_scans(country_code);
CREATE INDEX idx_qr_scans_device ON qr_scans(device_type);

-- Composite index for common analytics queries
CREATE INDEX idx_qr_scans_qr_date ON qr_scans(qr_id, scanned_at DESC);
