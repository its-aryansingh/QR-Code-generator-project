package version

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type DestinationKind string

const (
	DestinationKindURL        DestinationKind = "url"
	DestinationKindHostedPage DestinationKind = "hosted_page"
)

var (
	ErrMissingDestinationURL = errors.New("destination_url is required when destination_kind is url")
	ErrInvalidDestinationKind = errors.New("destination_kind must be 'url' or 'hosted_page'")
)

type UTMConfig struct {
	Source   string `json:"source,omitempty"`
	Medium   string `json:"medium,omitempty"`
	Campaign string `json:"campaign,omitempty"`
	Term     string `json:"term,omitempty"`
	Content  string `json:"content,omitempty"`
}

type QRVersion struct {
	ID              uuid.UUID       `json:"id"`
	QRCodeID        uuid.UUID       `json:"qr_code_id"`
	VersionNo       int             `json:"version_no"`
	DestinationKind DestinationKind `json:"destination_kind"`
	DestinationURL  *string         `json:"destination_url,omitempty"`
	HostedPage      json.RawMessage `json:"hosted_page,omitempty"`
	Rules           json.RawMessage `json:"rules"`
	UTM             json.RawMessage `json:"utm"`
	EffectiveAt     time.Time       `json:"effective_at"`
	ChangeNote      *string         `json:"change_note,omitempty"`
	CreatedBy       *uuid.UUID      `json:"created_by,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// AppendUTM merges non-empty UTM parameters into destination URL query string.
func AppendUTM(rawURL string, utm UTMConfig) (string, error) {
	if utm.Source == "" && utm.Medium == "" && utm.Campaign == "" && utm.Term == "" && utm.Content == "" {
		return rawURL, nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, err
	}

	q := u.Query()
	if utm.Source != "" {
		q.Set("utm_source", utm.Source)
	}
	if utm.Medium != "" {
		q.Set("utm_medium", utm.Medium)
	}
	if utm.Campaign != "" {
		q.Set("utm_campaign", utm.Campaign)
	}
	if utm.Term != "" {
		q.Set("utm_term", utm.Term)
	}
	if utm.Content != "" {
		q.Set("utm_content", utm.Content)
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ValidateVersionInput checks basic invariants on new version parameters.
func ValidateVersionInput(kind DestinationKind, destURL string) error {
	switch kind {
	case DestinationKindURL:
		if strings.TrimSpace(destURL) == "" {
			return ErrMissingDestinationURL
		}
		return nil
	case DestinationKindHostedPage:
		return nil
	default:
		return ErrInvalidDestinationKind
	}
}
