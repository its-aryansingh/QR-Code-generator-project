package generators

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// SocialGenerator generates QR content for social media types
type SocialGenerator struct{}

type SocialMetadata struct {
	Platform string `json:"platform"` // facebook, instagram, twitter, linkedin, tiktok, youtube, whatsapp
	Username string `json:"username"`
	URL      string `json:"url"`
	Phone    string `json:"phone"`   // For WhatsApp
	Message  string `json:"message"` // For WhatsApp pre-filled message
}

// Platform URL templates
var socialURLTemplates = map[string]string{
	"facebook":  "https://facebook.com/%s",
	"instagram": "https://instagram.com/%s",
	"twitter":   "https://twitter.com/%s",
	"linkedin":  "https://linkedin.com/in/%s",
	"tiktok":    "https://tiktok.com/@%s",
	"youtube":   "https://youtube.com/@%s",
	"github":    "https://github.com/%s",
	"telegram":  "https://t.me/%s",
	"snapchat":  "https://snapchat.com/add/%s",
	"pinterest": "https://pinterest.com/%s",
}

func (g *SocialGenerator) ValidateMetadata(metadata json.RawMessage) error {
	var data SocialMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return err
	}

	// Either URL or username/platform must be provided
	if data.URL == "" && data.Username == "" && data.Phone == "" {
		return errors.New("either URL, username, or phone is required")
	}

	// For WhatsApp, phone is required
	if data.Platform == "whatsapp" && data.Phone == "" {
		return errors.New("phone number is required for WhatsApp")
	}

	// If username provided, platform must be specified
	if data.Username != "" && data.Platform == "" {
		return errors.New("platform is required when using username")
	}

	// Validate platform
	if data.Platform != "" && data.Platform != "whatsapp" {
		if _, ok := socialURLTemplates[data.Platform]; !ok {
			return errors.New("unsupported social platform")
		}
	}

	return nil
}

func (g *SocialGenerator) GenerateContent(metadata json.RawMessage) (string, error) {
	var data SocialMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return "", err
	}

	// If direct URL provided, use it
	if data.URL != "" {
		return data.URL, nil
	}

	// Handle WhatsApp specially
	if data.Platform == "whatsapp" {
		phone := strings.ReplaceAll(data.Phone, " ", "")
		phone = strings.TrimPrefix(phone, "+")

		if data.Message != "" {
			return fmt.Sprintf("https://wa.me/%s?text=%s", phone, data.Message), nil
		}
		return fmt.Sprintf("https://wa.me/%s", phone), nil
	}

	// Generate URL from platform + username
	if template, ok := socialURLTemplates[data.Platform]; ok {
		username := strings.TrimPrefix(data.Username, "@")
		return fmt.Sprintf(template, username), nil
	}

	return "", errors.New("unable to generate social URL")
}
