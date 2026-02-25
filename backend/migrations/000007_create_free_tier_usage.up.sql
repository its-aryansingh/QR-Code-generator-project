-- Free tier usage tracking (10 QRs per IP per day)
CREATE TABLE IF NOT EXISTS free_tier_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_address INET NOT NULL,
    session_id VARCHAR(64),
    qr_count INTEGER DEFAULT 0,
    date DATE DEFAULT CURRENT_DATE,
    first_used TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_used TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Ensure one record per IP per day
    CONSTRAINT unique_ip_per_day UNIQUE (ip_address, date)
);

CREATE INDEX idx_free_tier_ip_date ON free_tier_usage(ip_address, date);

-- Cleanup old records (keep 30 days)
-- Run this periodically: DELETE FROM free_tier_usage WHERE date < CURRENT_DATE - INTERVAL '30 days';
