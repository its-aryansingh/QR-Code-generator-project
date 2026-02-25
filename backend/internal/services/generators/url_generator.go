package generators

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

// URLGenerator generates QR content for URL type
type URLGenerator struct{}

type URLMetadata struct {
	URL string `json:"url"`
}

func (g *URLGenerator) ValidateMetadata(metadata json.RawMessage) error {
	var data URLMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return err
	}

	if data.URL == "" {
		return errors.New("URL is required")
	}

	// Validate URL format
	if !strings.HasPrefix(data.URL, "http://") && !strings.HasPrefix(data.URL, "https://") {
		data.URL = "https://" + data.URL
	}

	if _, err := url.Parse(data.URL); err != nil {
		return errors.New("invalid URL format")
	}

	return nil
}

func (g *URLGenerator) GenerateContent(metadata json.RawMessage) (string, error) {
	var data URLMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return "", err
	}

	// Ensure URL has protocol
	if !strings.HasPrefix(data.URL, "http://") && !strings.HasPrefix(data.URL, "https://") {
		data.URL = "https://" + data.URL
	}

	return data.URL, nil
}

// TextGenerator generates QR content for plain text type
type TextGenerator struct{}

type TextMetadata struct {
	Text string `json:"text"`
}

func (g *TextGenerator) ValidateMetadata(metadata json.RawMessage) error {
	var data TextMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return err
	}

	if data.Text == "" {
		return errors.New("text is required")
	}

	if len(data.Text) > 4000 {
		return errors.New("text exceeds maximum length of 4000 characters")
	}

	return nil
}

func (g *TextGenerator) GenerateContent(metadata json.RawMessage) (string, error) {
	var data TextMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return "", err
	}

	return data.Text, nil
}

// PhoneGenerator generates QR content for phone call type
type PhoneGenerator struct{}

type PhoneMetadata struct {
	Phone string `json:"phone"`
}

func (g *PhoneGenerator) ValidateMetadata(metadata json.RawMessage) error {
	var data PhoneMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return err
	}

	if data.Phone == "" {
		return errors.New("phone number is required")
	}

	return nil
}

func (g *PhoneGenerator) GenerateContent(metadata json.RawMessage) (string, error) {
	var data PhoneMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return "", err
	}

	// Format: tel:+1234567890
	phone := strings.ReplaceAll(data.Phone, " ", "")
	if !strings.HasPrefix(phone, "tel:") {
		phone = "tel:" + phone
	}

	return phone, nil
}
