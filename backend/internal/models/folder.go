package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Folder represents a folder/campaign for organizing QR codes within a workspace
type Folder struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID uuid.UUID  `gorm:"type:uuid;not null;index" json:"workspace_id"`
	ParentID    *uuid.UUID `gorm:"type:uuid;index" json:"parent_id,omitempty"`
	Name        string     `gorm:"size:255;not null" json:"name"`
	Description string     `gorm:"type:text" json:"description,omitempty"`
	Color       string     `gorm:"size:7;default:'#8B5CF6'" json:"color"`
	Icon        string     `gorm:"size:50" json:"icon,omitempty"`
	SortOrder   int        `gorm:"default:0" json:"sort_order"`

	// Statistics (denormalized for performance)
	QRCount   int `gorm:"default:0" json:"qr_count"`
	ScanCount int `gorm:"default:0" json:"scan_count"`

	// Timestamps
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	Workspace *Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
	Parent    *Folder    `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Children  []Folder   `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	QRCodes   []QRRecord `gorm:"foreignKey:FolderID" json:"qr_codes,omitempty"`
}

func (f *Folder) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}
