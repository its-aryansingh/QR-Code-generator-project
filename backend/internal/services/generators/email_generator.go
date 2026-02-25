package generators

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// EmailGenerator generates QR content for email type
type EmailGenerator struct{}

type EmailMetadata struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (g *EmailGenerator) ValidateMetadata(metadata json.RawMessage) error {
	var data EmailMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return err
	}

	if data.To == "" {
		return errors.New("email address is required")
	}

	// Basic email validation
	if !strings.Contains(data.To, "@") {
		return errors.New("invalid email address format")
	}

	return nil
}

func (g *EmailGenerator) GenerateContent(metadata json.RawMessage) (string, error) {
	var data EmailMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return "", err
	}

	// Format: mailto:email@example.com?subject=Hello&body=World
	mailto := fmt.Sprintf("mailto:%s", data.To)

	var params []string
	if data.Subject != "" {
		params = append(params, "subject="+url.QueryEscape(data.Subject))
	}
	if data.Body != "" {
		params = append(params, "body="+url.QueryEscape(data.Body))
	}

	if len(params) > 0 {
		mailto += "?" + strings.Join(params, "&")
	}

	return mailto, nil
}

// SMSGenerator generates QR content for SMS type
type SMSGenerator struct{}

type SMSMetadata struct {
	Phone   string `json:"phone"`
	Message string `json:"message"`
}

func (g *SMSGenerator) ValidateMetadata(metadata json.RawMessage) error {
	var data SMSMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return err
	}

	if data.Phone == "" {
		return errors.New("phone number is required")
	}

	return nil
}

func (g *SMSGenerator) GenerateContent(metadata json.RawMessage) (string, error) {
	var data SMSMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return "", err
	}

	// Format: sms:+1234567890?body=Hello
	phone := strings.ReplaceAll(data.Phone, " ", "")
	sms := fmt.Sprintf("sms:%s", phone)

	if data.Message != "" {
		sms += "?body=" + url.QueryEscape(data.Message)
	}

	return sms, nil
}
