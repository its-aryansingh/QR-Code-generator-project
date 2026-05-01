package generators

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// MeCardGenerator generates QR content in MECARD format
type MeCardGenerator struct{}

type MeCardMetadata struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

func (g *MeCardGenerator) ValidateMetadata(metadata json.RawMessage) error {
	var data MeCardMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return err
	}

	if data.Name == "" && data.Phone == "" {
		return errors.New("name or phone is required for meCard")
	}

	return nil
}

func (g *MeCardGenerator) GenerateContent(metadata json.RawMessage) (string, error) {
	var data MeCardMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return "", err
	}

	// Build MECARD format: MECARD:N:Name;TEL:Phone;EMAIL:Email;;
	var parts []string

	if data.Name != "" {
		parts = append(parts, fmt.Sprintf("N:%s", escapeValue(data.Name)))
	}

	if data.Phone != "" {
		phone := strings.ReplaceAll(data.Phone, " ", "")
		parts = append(parts, fmt.Sprintf("TEL:%s", phone))
	}

	if data.Email != "" {
		parts = append(parts, fmt.Sprintf("EMAIL:%s", data.Email))
	}

	return "MECARD:" + strings.Join(parts, ";") + ";;", nil
}

// escapeValue escapes special characters in MECARD values
func escapeValue(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ":", "\\:")
	return s
}
