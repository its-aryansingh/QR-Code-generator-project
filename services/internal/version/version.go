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

// AppendUTM adds the configured UTM parameters to the destination, but only those the
// destination does not already carry (the destination's own tagging wins). The original
// query string order and the fragment are preserved.
func AppendUTM(rawURL string, utm UTMConfig) (string, error) {
	if utm.Source == "" && utm.Medium == "" && utm.Campaign == "" && utm.Term == "" && utm.Content == "" {
		return rawURL, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return rawURL, nil
	}
	existing := u.Query()
	var add []string
	for _, kv := range [][2]string{{"utm_source", utm.Source}, {"utm_medium", utm.Medium},
		{"utm_campaign", utm.Campaign}, {"utm_term", utm.Term}, {"utm_content", utm.Content}} {
		if kv[1] == "" || existing.Has(kv[0]) {
			continue
		}
		add = append(add, kv[0]+"="+url.QueryEscape(kv[1]))
	}
	if len(add) == 0 {
		return rawURL, nil
	}
	if u.RawQuery == "" {
		u.RawQuery = strings.Join(add, "&")
	} else {
		u.RawQuery += "&" + strings.Join(add, "&")
	}
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
