package generators

import (
	"encoding/json"
	"fmt"

	"github.com/qrapp/backend/internal/models"
)

// Generator interface for type-specific QR content generation
type Generator interface {
	// GenerateContent creates the QR code content string
	GenerateContent(metadata json.RawMessage) (string, error)
	// ValidateMetadata validates the type-specific metadata
	ValidateMetadata(metadata json.RawMessage) error
}

// GetGenerator returns the appropriate generator for a QR type
func GetGenerator(qrType string) (Generator, error) {
	switch qrType {
	case models.TypeURL:
		return &URLGenerator{}, nil
	case models.TypeText:
		return &TextGenerator{}, nil
	case models.TypeWiFi:
		return &WiFiGenerator{}, nil
	case models.TypeVCard:
		return &VCardGenerator{}, nil
	case models.TypeEmail:
		return &EmailGenerator{}, nil
	case models.TypeSMS:
		return &SMSGenerator{}, nil
	case models.TypePhone:
		return &PhoneGenerator{}, nil
	case models.TypeFacebook, models.TypeInstagram, models.TypeWhatsApp, models.TypeSocial:
		return &SocialGenerator{}, nil
	case models.TypeMeCard:
		return &MeCardGenerator{}, nil
	case models.TypeBusiness:
		return &BusinessGenerator{}, nil
	case models.TypeCoupon:
		return &CouponGenerator{}, nil
	case models.TypePDF, models.TypeVideo, models.TypeMP3, models.TypeImages:
		// Media types use URL generator - they encode the URL to the media file
		return &URLGenerator{}, nil
	default:
		// Default to URL generator for unknown types
		return &URLGenerator{}, nil
	}
}

// GenerateQRContent is a convenience function that generates content for any type
func GenerateQRContent(qrType string, metadata json.RawMessage) (string, error) {
	generator, err := GetGenerator(qrType)
	if err != nil {
		return "", err
	}

	if err := generator.ValidateMetadata(metadata); err != nil {
		return "", fmt.Errorf("invalid metadata: %w", err)
	}

	return generator.GenerateContent(metadata)
}
