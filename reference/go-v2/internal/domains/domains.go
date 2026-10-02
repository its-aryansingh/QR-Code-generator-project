package domains

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"golang.org/x/net/idna"
)

var (
	ErrInvalidDomain  = errors.New("invalid domain name")
	ErrReservedDomain = errors.New("domain is reserved by the platform")
)

type DomainStatus string

const (
	StatusPendingVerification DomainStatus = "pending_verification"
	StatusActive              DomainStatus = "active"
	StatusFailed              DomainStatus = "failed"
)

type CustomDomain struct {
	ID                string       `json:"id"`
	WorkspaceID       string       `json:"workspace_id"`
	Domain            string       `json:"domain"`
	Status            DomainStatus `json:"status"`
	VerificationToken string       `json:"verification_token"`
	CreatedAt         time.Time    `json:"created_at"`
	VerifiedAt        *time.Time   `json:"verified_at,omitempty"`
}

type Provider interface {
	CreateCustomHostname(ctx context.Context, domain string) error
	CheckCustomHostname(ctx context.Context, domain string) (bool, error)
}

type MockProvider struct{}

func (m *MockProvider) CreateCustomHostname(ctx context.Context, domain string) error {
	return nil
}

func (m *MockProvider) CheckCustomHostname(ctx context.Context, domain string) (bool, error) {
	return true, nil
}

// CleanDomain normalises an input domain into lowercase ASCII punycode without schemes or ports.
func CleanDomain(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "http://")
	cleaned = strings.TrimPrefix(cleaned, "https://")
	if idx := strings.Index(cleaned, "/"); idx != -1 {
		cleaned = cleaned[:idx]
	}
	if idx := strings.Index(cleaned, ":"); idx != -1 {
		cleaned = cleaned[:idx]
	}

	cleaned = strings.ToLower(cleaned)
	if cleaned == "" || strings.Contains(cleaned, "..") {
		return "", ErrInvalidDomain
	}

	punycode, err := idna.Lookup.ToASCII(cleaned)
	if err != nil {
		return "", ErrInvalidDomain
	}

	parts := strings.Split(punycode, ".")
	if len(parts) < 2 {
		return "", ErrInvalidDomain
	}

	// Disallow platform reserved domains
	if punycode == "qrit.io" || punycode == "qr.example.com" || strings.HasSuffix(punycode, ".qrit.io") {
		return "", ErrReservedDomain
	}

	return punycode, nil
}

// VerifyDNSTXT checks if the expected TXT token exists at _qrit-challenge.<domain>.
func VerifyDNSTXT(ctx context.Context, domain, expectedToken string, resolver *net.Resolver) (bool, error) {
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	recordName := "_qrit-challenge." + domain
	records, err := resolver.LookupTXT(ctx, recordName)
	if err != nil {
		return false, err
	}

	for _, r := range records {
		if strings.TrimSpace(r) == strings.TrimSpace(expectedToken) {
			return true, nil
		}
	}
	return false, nil
}
