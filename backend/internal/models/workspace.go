package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Workspace roles
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

// Invite statuses
const (
	InvitePending  = "pending"
	InviteAccepted = "accepted"
	InviteExpired  = "expired"
)

// Workspace represents an organization/team workspace
type Workspace struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Slug        string    `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	LogoURL     string    `gorm:"type:text" json:"logo_url,omitempty"`
	OwnerID     uuid.UUID `gorm:"type:uuid;not null;index" json:"owner_id"`

	// Plan & Limits (inherited from owner or set per workspace)
	Plan       string `gorm:"size:20;default:'free'" json:"plan"`
	MaxMembers int    `gorm:"default:1" json:"max_members"`
	MaxQRCodes int    `gorm:"default:50" json:"max_qr_codes"`
	MaxFolders int    `gorm:"default:5" json:"max_folders"`

	// White-labeling
	CustomDomain   string `gorm:"size:255" json:"custom_domain,omitempty"`
	BrandColor     string `gorm:"size:7;default:'#8B5CF6'" json:"brand_color"`
	BrandLogo      string `gorm:"type:text" json:"brand_logo,omitempty"`
	FaviconURL     string `gorm:"type:text" json:"favicon_url,omitempty"`
	CustomCSS      string `gorm:"type:text" json:"custom_css,omitempty"`
	RemoveBranding bool   `gorm:"default:false" json:"remove_branding"`
	CustomFooter   string `gorm:"type:text" json:"custom_footer,omitempty"`

	// SSO Configuration
	SSOEnabled  bool   `gorm:"default:false" json:"sso_enabled"`
	SSOProvider string `gorm:"size:20" json:"sso_provider,omitempty"` // "saml" or "oidc"
	SSOConfig   string `gorm:"type:text" json:"sso_config,omitempty"` // JSON config

	// Timestamps
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	Owner   *User             `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Members []WorkspaceMember `gorm:"foreignKey:WorkspaceID;constraint:OnDelete:CASCADE" json:"members,omitempty"`
	Invites []WorkspaceInvite `gorm:"foreignKey:WorkspaceID;constraint:OnDelete:CASCADE" json:"invites,omitempty"`
	Folders []Folder          `gorm:"foreignKey:WorkspaceID;constraint:OnDelete:CASCADE" json:"folders,omitempty"`
	QRCodes []QRRecord        `gorm:"foreignKey:WorkspaceID" json:"qr_codes,omitempty"`
}

func (w *Workspace) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// WorkspaceMember represents a user's membership in a workspace
type WorkspaceMember struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_workspace_user" json:"workspace_id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_workspace_user" json:"user_id"`
	Role        string    `gorm:"size:20;not null;default:'viewer'" json:"role"`
	InvitedBy   uuid.UUID `gorm:"type:uuid" json:"invited_by,omitempty"`
	JoinedAt    time.Time `gorm:"autoCreateTime" json:"joined_at"`

	// Relations
	Workspace *Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
	User      *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (m *WorkspaceMember) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// CanManageQR returns true if member can create/edit/delete QR codes
func (m *WorkspaceMember) CanManageQR() bool {
	return m.Role == RoleOwner || m.Role == RoleAdmin || m.Role == RoleEditor
}

// CanManageMembers returns true if member can invite/remove members
func (m *WorkspaceMember) CanManageMembers() bool {
	return m.Role == RoleOwner || m.Role == RoleAdmin
}

// CanDeleteWorkspace returns true if member can delete the workspace
func (m *WorkspaceMember) CanDeleteWorkspace() bool {
	return m.Role == RoleOwner
}

// WorkspaceInvite represents a pending invitation to join a workspace
type WorkspaceInvite struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID uuid.UUID `gorm:"type:uuid;not null;index" json:"workspace_id"`
	Email       string    `gorm:"size:255;not null;index" json:"email"`
	Role        string    `gorm:"size:20;not null;default:'viewer'" json:"role"`
	Token       string    `gorm:"size:64;uniqueIndex;not null" json:"-"`
	InvitedBy   uuid.UUID `gorm:"type:uuid;not null" json:"invited_by"`
	Status      string    `gorm:"size:20;default:'pending'" json:"status"`
	ExpiresAt   time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relations
	Workspace *Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
}

func (i *WorkspaceInvite) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

// IsExpired returns true if the invite has expired
func (i *WorkspaceInvite) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}
