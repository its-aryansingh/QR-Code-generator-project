-- Enhance users table with subscription and API access
ALTER TABLE users ADD COLUMN IF NOT EXISTS plan VARCHAR(20) DEFAULT 'free';
ALTER TABLE users ADD COLUMN IF NOT EXISTS plan_expires_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS api_key VARCHAR(64) UNIQUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS api_calls_today INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS api_calls_reset_at DATE DEFAULT CURRENT_DATE;

-- User profile info
ALTER TABLE users ADD COLUMN IF NOT EXISTS name VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS company VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT;

-- Stripe integration
ALTER TABLE users ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS stripe_subscription_id VARCHAR(255);

CREATE INDEX idx_users_plan ON users(plan);
CREATE INDEX idx_users_api_key ON users(api_key) WHERE api_key IS NOT NULL;
