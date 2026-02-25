-- Enterprise: Workspaces, Members, Invites, Folders, Webhooks, Audit Logs
-- Also adds enterprise columns to qr_records and users

-- ==========================================
-- WORKSPACES
-- ==========================================
CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    logo_url TEXT,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan VARCHAR(20) DEFAULT 'free',
    max_members INT DEFAULT 1,
    max_qr_codes INT DEFAULT 50,
    max_folders INT DEFAULT 5,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workspaces_owner ON workspaces(owner_id);
CREATE INDEX IF NOT EXISTS idx_workspaces_slug ON workspaces(slug);

-- ==========================================
-- WORKSPACE MEMBERS
-- ==========================================
CREATE TABLE IF NOT EXISTS workspace_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL DEFAULT 'viewer',
    invited_by UUID REFERENCES users(id),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_wm_workspace ON workspace_members(workspace_id);
CREATE INDEX IF NOT EXISTS idx_wm_user ON workspace_members(user_id);

-- ==========================================
-- WORKSPACE INVITES
-- ==========================================
CREATE TABLE IF NOT EXISTS workspace_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'viewer',
    token VARCHAR(64) NOT NULL UNIQUE,
    invited_by UUID NOT NULL REFERENCES users(id),
    status VARCHAR(20) DEFAULT 'pending',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wi_workspace ON workspace_invites(workspace_id);
CREATE INDEX IF NOT EXISTS idx_wi_email ON workspace_invites(email);
CREATE INDEX IF NOT EXISTS idx_wi_token ON workspace_invites(token);

-- ==========================================
-- FOLDERS
-- ==========================================
CREATE TABLE IF NOT EXISTS folders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES folders(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    color VARCHAR(7) DEFAULT '#8B5CF6',
    icon VARCHAR(50),
    sort_order INT DEFAULT 0,
    qr_count INT DEFAULT 0,
    scan_count INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_folders_workspace ON folders(workspace_id);
CREATE INDEX IF NOT EXISTS idx_folders_parent ON folders(parent_id);

-- ==========================================
-- WEBHOOKS
-- ==========================================
CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    secret VARCHAR(64),
    events TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    description TEXT,
    last_triggered TIMESTAMPTZ,
    fail_count INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_workspace ON webhooks(workspace_id);

-- ==========================================
-- WEBHOOK LOGS
-- ==========================================
CREATE TABLE IF NOT EXISTS webhook_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event VARCHAR(50) NOT NULL,
    payload TEXT,
    status_code INT DEFAULT 0,
    response TEXT,
    success BOOLEAN DEFAULT false,
    duration INT DEFAULT 0,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wl_webhook ON webhook_logs(webhook_id);
CREATE INDEX IF NOT EXISTS idx_wl_created ON webhook_logs(created_at);

-- ==========================================
-- AUDIT LOGS
-- ==========================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    action VARCHAR(50) NOT NULL,
    resource VARCHAR(50) NOT NULL,
    resource_id UUID,
    details TEXT,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_al_workspace ON audit_logs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_al_user ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_al_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_al_created ON audit_logs(created_at);

-- ==========================================
-- ADD ENTERPRISE COLUMNS TO QR RECORDS
-- ==========================================
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id);
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS folder_id UUID REFERENCES folders(id) ON DELETE SET NULL;
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS tags TEXT;
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS password VARCHAR(255);
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS max_scans INT;
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMPTZ;
ALTER TABLE qr_records ADD COLUMN IF NOT EXISTS geo_restrictions TEXT;

CREATE INDEX IF NOT EXISTS idx_qr_workspace ON qr_records(workspace_id);
CREATE INDEX IF NOT EXISTS idx_qr_folder ON qr_records(folder_id);

-- ==========================================
-- ADD DEFAULT WORKSPACE TO USERS
-- ==========================================
ALTER TABLE users ADD COLUMN IF NOT EXISTS default_workspace_id UUID REFERENCES workspaces(id) ON DELETE SET NULL;
