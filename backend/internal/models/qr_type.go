package models

import (
	"time"

	"gorm.io/datatypes"
)

// QRType represents a type of QR code (URL, WiFi, vCard, etc.)
type QRType struct {
	ID           string         `gorm:"primaryKey;size:50" json:"id"`
	Name         string         `gorm:"not null;size:100" json:"name"`
	Description  string         `gorm:"type:text" json:"description"`
	Icon         string         `gorm:"size:50" json:"icon"`
	Category     string         `gorm:"size:50" json:"category"`
	IsPremium    bool           `gorm:"default:false" json:"is_premium"`
	FieldsSchema datatypes.JSON `gorm:"type:jsonb" json:"fields_schema,omitempty"`
	SortOrder    int            `gorm:"default:0" json:"sort_order"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

// QR Type Categories
const (
	CategoryLinks     = "links"
	CategoryBasic     = "basic"
	CategoryBusiness  = "business"
	CategorySocial    = "social"
	CategoryMedia     = "media"
	CategoryTechnical = "technical"
)

// QR Type IDs
const (
	TypeURL       = "url"
	TypeText      = "text"
	TypeWiFi      = "wifi"
	TypeVCard     = "vcard"
	TypeEmail     = "email"
	TypeSMS       = "sms"
	TypePhone     = "phone"
	TypePDF       = "pdf"
	TypeImages    = "images"
	TypeVideo     = "video"
	TypeMP3       = "mp3"
	TypeFacebook  = "facebook"
	TypeInstagram = "instagram"
	TypeWhatsApp  = "whatsapp"
	TypeSocial    = "social"
	TypeApps      = "apps"
	TypeMeCard    = "mecard"
	TypeMenu      = "menu"
	TypeCoupon    = "coupon"
	TypeBusiness  = "business"
	TypeLinks     = "links"
)

// IsPremiumType returns true if the QR type requires a paid subscription
func IsPremiumType(typeID string) bool {
	premiumTypes := map[string]bool{
		TypePDF:      true,
		TypeImages:   true,
		TypeVideo:    true,
		TypeMP3:      true,
		TypeMenu:     true,
		TypeCoupon:   true,
		TypeBusiness: true,
	}
	return premiumTypes[typeID]
}
