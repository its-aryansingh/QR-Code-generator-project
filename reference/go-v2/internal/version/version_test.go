package version_test

import (
	"strings"
	"testing"

	"github.com/its-aryansingh/qrit/services/internal/version"
)

func TestAppendUTM(t *testing.T) {
	dest := "https://example.com/shop?existing=1"
	utm := version.UTMConfig{
		Source:   "qr_poster",
		Medium:   "print",
		Campaign: "summer_launch",
	}

	got, err := version.AppendUTM(dest, utm)
	if err != nil {
		t.Fatalf("AppendUTM failed: %v", err)
	}

	if !strings.Contains(got, "existing=1") {
		t.Errorf("expected existing query param preserved: %s", got)
	}
	if !strings.Contains(got, "utm_source=qr_poster") {
		t.Errorf("expected utm_source: %s", got)
	}
	if !strings.Contains(got, "utm_medium=print") {
		t.Errorf("expected utm_medium: %s", got)
	}
	if !strings.Contains(got, "utm_campaign=summer_launch") {
		t.Errorf("expected utm_campaign: %s", got)
	}
}

func TestValidateVersionInput(t *testing.T) {
	if err := version.ValidateVersionInput(version.DestinationKindURL, "https://example.com"); err != nil {
		t.Errorf("valid URL version rejected: %v", err)
	}

	if err := version.ValidateVersionInput(version.DestinationKindURL, "   "); err != version.ErrMissingDestinationURL {
		t.Errorf("expected ErrMissingDestinationURL, got %v", err)
	}

	if err := version.ValidateVersionInput("invalid_kind", ""); err != version.ErrInvalidDestinationKind {
		t.Errorf("expected ErrInvalidDestinationKind, got %v", err)
	}
}
