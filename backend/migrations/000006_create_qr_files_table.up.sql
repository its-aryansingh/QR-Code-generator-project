-- Create file uploads table for PDF, images, etc. (stored in S3)
CREATE TABLE IF NOT EXISTS qr_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    qr_id UUID REFERENCES qr_records(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    
    -- File metadata
    filename VARCHAR(255) NOT NULL,
    original_filename VARCHAR(255),
    file_type VARCHAR(50) NOT NULL,  -- pdf, image, audio, video
    mime_type VARCHAR(100),
    file_size INTEGER,  -- bytes
    
    -- S3 storage
    s3_bucket VARCHAR(100),
    s3_key TEXT NOT NULL,
    s3_url TEXT,
    
    -- Status
    is_processed BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_qr_files_qr_id ON qr_files(qr_id);
CREATE INDEX idx_qr_files_user_id ON qr_files(user_id);
CREATE INDEX idx_qr_files_type ON qr_files(file_type);
