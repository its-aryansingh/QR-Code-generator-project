package generators

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// BusinessGenerator generates QR content for business information
type BusinessGenerator struct{}

type BusinessMetadata struct {
	CompanyName string `json:"company_name"`
	Industry    string `json:"industry"`
	Address     string `json:"address"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Website     string `json:"website"`
	Hours       string `json:"hours"`
}

func (g *BusinessGenerator) ValidateMetadata(metadata json.RawMessage) error {
	var data BusinessMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return err
	}

	if data.CompanyName == "" {
		return errors.New("company name is required for business QR")
	}

	return nil
}

func (g *BusinessGenerator) GenerateContent(metadata json.RawMessage) (string, error) {
	var data BusinessMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return "", err
	}

	// If website is provided, use it as the primary URL
	if data.Website != "" {
		return data.Website, nil
	}

	// Otherwise, build a data URL with business info as vCard-like format
	var parts []string
	parts = append(parts, "BEGIN:VCARD")
	parts = append(parts, "VERSION:3.0")
	parts = append(parts, fmt.Sprintf("ORG:%s", data.CompanyName))

	if data.Industry != "" {
		parts = append(parts, fmt.Sprintf("TITLE:%s", data.Industry))
	}

	if data.Address != "" {
		parts = append(parts, fmt.Sprintf("ADR:;;%s", strings.ReplaceAll(data.Address, ",", ";")))
	}

	if data.Phone != "" {
		phone := strings.ReplaceAll(data.Phone, " ", "")
		parts = append(parts, fmt.Sprintf("TEL:%s", phone))
	}

	if data.Email != "" {
		parts = append(parts, fmt.Sprintf("EMAIL:%s", data.Email))
	}

	if data.Hours != "" {
		parts = append(parts, fmt.Sprintf("NOTE:Hours: %s", data.Hours))
	}

	parts = append(parts, "END:VCARD")

	return strings.Join(parts, "\n"), nil
}

// CouponGenerator generates QR content for coupons
type CouponGenerator struct{}

type CouponMetadata struct {
	Code        string `json:"code"`
	Discount    string `json:"discount"`
	Description string `json:"description"`
	Expiry      string `json:"expiry"`
	Terms       string `json:"terms"`
}

func (g *CouponGenerator) ValidateMetadata(metadata json.RawMessage) error {
	var data CouponMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return err
	}

	if data.Code == "" {
		return errors.New("coupon code is required")
	}

	return nil
}

func (g *CouponGenerator) GenerateContent(metadata json.RawMessage) (string, error) {
	var data CouponMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return "", err
	}

	// Build a structured coupon URL with query params
	// This creates a data-driven URL that can be parsed by apps
	params := url.Values{}
	params.Set("code", data.Code)

	if data.Discount != "" {
		params.Set("discount", data.Discount)
	}

	if data.Description != "" {
		params.Set("desc", data.Description)
	}

	if data.Expiry != "" {
		params.Set("expiry", data.Expiry)
	}

	if data.Terms != "" {
		params.Set("terms", data.Terms)
	}

	// Return coupon code with formatted text for direct scanning
	content := fmt.Sprintf("COUPON:%s", data.Code)
	if data.Discount != "" {
		content += fmt.Sprintf("\nDiscount: %s", data.Discount)
	}
	if data.Description != "" {
		content += fmt.Sprintf("\n%s", data.Description)
	}
	if data.Expiry != "" {
		content += fmt.Sprintf("\nValid until: %s", data.Expiry)
	}
	if data.Terms != "" {
		content += fmt.Sprintf("\nTerms: %s", data.Terms)
	}

	return content, nil
}
