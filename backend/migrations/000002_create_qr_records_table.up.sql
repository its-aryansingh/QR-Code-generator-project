CREATE TABLE IF NOT EXISTS qr_records (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content         TEXT NOT NULL,
    qr_type         VARCHAR(50) DEFAULT 'url',
    size            INTEGER DEFAULT 256,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_qr_records_user_id ON qr_records(user_id);
CREATE INDEX IF NOT EXISTS idx_qr_records_created_at ON qr_records(created_at);
