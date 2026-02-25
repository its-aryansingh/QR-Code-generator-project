package generators

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// VCardGenerator generates QR content for vCard (contact) type
type VCardGenerator struct{}

type VCardMetadata struct {
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Mobile     string `json:"mobile"`
	Company    string `json:"company"`
	JobTitle   string `json:"job_title"`
	Website    string `json:"website"`
	Address    string `json:"address"`
	City       string `json:"city"`
	State      string `json:"state"`
	Country    string `json:"country"`
	PostalCode string `json:"postal_code"`
	Note       string `json:"note"`
}

func (g *VCardGenerator) ValidateMetadata(metadata json.RawMessage) error {
	var data VCardMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return err
	}

	if data.FirstName == "" && data.LastName == "" {
		return errors.New("at least first name or last name is required")
	}

	return nil
}

func (g *VCardGenerator) GenerateContent(metadata json.RawMessage) (string, error) {
	var data VCardMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return "", err
	}

	var lines []string

	// vCard 3.0 format
	lines = append(lines, "BEGIN:VCARD")
	lines = append(lines, "VERSION:3.0")

	// Full name
	fullName := strings.TrimSpace(data.FirstName + " " + data.LastName)
	lines = append(lines, fmt.Sprintf("FN:%s", fullName))
	lines = append(lines, fmt.Sprintf("N:%s;%s;;;", data.LastName, data.FirstName))

	// Organization
	if data.Company != "" {
		lines = append(lines, fmt.Sprintf("ORG:%s", data.Company))
	}

	// Title
	if data.JobTitle != "" {
		lines = append(lines, fmt.Sprintf("TITLE:%s", data.JobTitle))
	}

	// Phone numbers
	if data.Phone != "" {
		lines = append(lines, fmt.Sprintf("TEL;TYPE=WORK,VOICE:%s", data.Phone))
	}
	if data.Mobile != "" {
		lines = append(lines, fmt.Sprintf("TEL;TYPE=CELL,VOICE:%s", data.Mobile))
	}

	// Email
	if data.Email != "" {
		lines = append(lines, fmt.Sprintf("EMAIL;TYPE=INTERNET:%s", data.Email))
	}

	// Website
	if data.Website != "" {
		lines = append(lines, fmt.Sprintf("URL:%s", data.Website))
	}

	// Address
	if data.Address != "" || data.City != "" || data.Country != "" {
		// ADR;TYPE=WORK:;;street;city;state;postal;country
		addr := fmt.Sprintf("ADR;TYPE=WORK:;;%s;%s;%s;%s;%s",
			data.Address, data.City, data.State, data.PostalCode, data.Country)
		lines = append(lines, addr)
	}

	// Note
	if data.Note != "" {
		lines = append(lines, fmt.Sprintf("NOTE:%s", data.Note))
	}

	lines = append(lines, "END:VCARD")

	return strings.Join(lines, "\n"), nil
}
