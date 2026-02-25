-- Create QR types lookup table
CREATE TABLE IF NOT EXISTS qr_types (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    icon VARCHAR(50),
    category VARCHAR(50),
    is_premium BOOLEAN DEFAULT false,
    fields_schema JSONB,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Insert default QR types
INSERT INTO qr_types (id, name, description, icon, category, is_premium, sort_order) VALUES
    ('url', 'Website', 'Link to any website URL', 'globe', 'links', false, 1),
    ('text', 'Text', 'Plain text message', 'file-text', 'basic', false, 2),
    ('wifi', 'WiFi', 'Connect to a Wi-Fi network', 'wifi', 'technical', false, 3),
    ('vcard', 'vCard', 'Share a digital business card', 'user-plus', 'business', false, 4),
    ('email', 'Email', 'Send an email', 'mail', 'basic', false, 5),
    ('sms', 'SMS', 'Send a text message', 'message-square', 'basic', false, 6),
    ('phone', 'Phone', 'Make a phone call', 'phone', 'basic', false, 7),
    ('pdf', 'PDF', 'Show a PDF document', 'file', 'media', true, 8),
    ('images', 'Images', 'Share multiple images', 'image', 'media', true, 9),
    ('video', 'Video', 'Show a video', 'video', 'media', true, 10),
    ('mp3', 'MP3', 'Share an audio file', 'music', 'media', true, 11),
    ('facebook', 'Facebook', 'Share your Facebook page', 'facebook', 'social', false, 12),
    ('instagram', 'Instagram', 'Share your Instagram', 'instagram', 'social', false, 13),
    ('whatsapp', 'WhatsApp', 'Get WhatsApp messages', 'message-circle', 'social', false, 14),
    ('social', 'Social Media', 'Share your social channels', 'share-2', 'social', false, 15),
    ('apps', 'Apps', 'Redirect to an app store', 'smartphone', 'technical', false, 16),
    ('menu', 'Menu', 'Create a restaurant menu', 'menu', 'business', true, 17),
    ('coupon', 'Coupon', 'Share a coupon', 'tag', 'business', true, 18),
    ('business', 'Business', 'Share information about your business', 'briefcase', 'business', true, 19),
    ('links', 'List of Links', 'Share multiple links', 'link', 'links', false, 20)
ON CONFLICT (id) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_qr_types_category ON qr_types(category);
CREATE INDEX IF NOT EXISTS idx_qr_types_sort_order ON qr_types(sort_order);
