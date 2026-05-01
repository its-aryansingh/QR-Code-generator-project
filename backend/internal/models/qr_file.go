package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// QRFile represents an uploaded file stored in S3
type QRFile struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	QRID   uuid.UUID `gorm:"type:uuid;index" json:"qr_id,omitempty"`
	UserID uuid.UUID `gorm:"type:uuid;index" json:"user_id"`

	// File metadata
	Filename         string `gorm:"size:255;not null" json:"filename"`
	OriginalFilename string `gorm:"size:255" json:"original_filename,omitempty"`
	FileType         string `gorm:"size:50;not null" json:"file_type"` // pdf, image, audio, video
	MimeType         string `gorm:"size:100" json:"mime_type,omitempty"`
	FileSize         int64  `gorm:"" json:"file_size"` // bytes

	// S3 storage
	S3Bucket string `gorm:"size:100" json:"s3_bucket,omitempty"`
	S3Key    string `gorm:"type:text;not null" json:"s3_key"`
	S3URL    string `gorm:"type:text" json:"s3_url,omitempty"`

	// Status
	IsProcessed bool      `gorm:"default:false" json:"is_processed"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	QRRecord *QRRecord `gorm:"foreignKey:QRID" json:"qr_record,omitempty"`
	User     *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (f *QRFile) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

// File type constants
const (
	FileTypePDF   = "pdf"
	FileTypeImage = "image"
	FileTypeAudio = "audio"
	FileTypeVideo = "video"
)

// Allowed MIME types for each file type
var AllowedMimeTypes = map[string][]string{
	FileTypePDF:   {"application/pdf"},
	FileTypeImage: {"image/jpeg", "image/png", "image/gif", "image/webp"},
	FileTypeAudio: {"audio/mpeg", "audio/mp3", "audio/wav", "audio/ogg"},
	FileTypeVideo: {"video/mp4", "video/webm", "video/quicktime"},
}

// MaxFileSizes in bytes for each file type
var MaxFileSizes = map[string]int64{
	FileTypePDF:   50 * 1024 * 1024,  // 50MB
	FileTypeImage: 10 * 1024 * 1024,  // 10MB
	FileTypeAudio: 50 * 1024 * 1024,  // 50MB
	FileTypeVideo: 100 * 1024 * 1024, // 100MB
}
