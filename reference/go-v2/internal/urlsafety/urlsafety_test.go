package urlsafety_test

import (
	"context"
	"strings"
	"testing"

	"github.com/its-aryansingh/qrit/services/internal/urlsafety"
)

func TestValidateTable(t *testing.T) {
	defaultPolicy := urlsafety.Policy{
		RequireHTTPS:    false,
		AllowShorteners: false,
		OwnHosts:        []string{"qrit.io", "qr.it"},
		AllowedHosts:    nil,
	}

	tests := []struct {
		name        string
		raw         string
		policy      urlsafety.Policy
		wantCode    string // empty if valid
		wantURLPart string
	}{
		// 1-10: Standard valid web URLs
		{"simple https", "https://example.com", defaultPolicy, "", "https://example.com"},
		{"simple http", "http://example.com", defaultPolicy, "", "http://example.com"},
		{"with path", "https://example.com/some/path", defaultPolicy, "", "https://example.com/some/path"},
		{"with query", "https://example.com/path?utm_source=qr&foo=bar", defaultPolicy, "", "https://example.com/path?utm_source=qr&foo=bar"},
		{"with fragment", "https://example.com/path#section-1", defaultPolicy, "", "https://example.com/path#section-1"},
		{"with port", "https://example.com:8443/test", defaultPolicy, "", "https://example.com:8443/test"},
		{"uppercase scheme and host", "HTTPS://EXAMPLE.COM/Page", defaultPolicy, "", "https://example.com/Page"},
		{"subdomain", "https://blog.sub.example.com", defaultPolicy, "", "https://blog.sub.example.com"},
		{"deep nested path", "https://api.github.com/repos/org/repo/pulls/123", defaultPolicy, "", "https://api.github.com/repos/org/repo/pulls/123"},
		{"numeric ip", "https://93.184.216.34/test", defaultPolicy, "", "https://93.184.216.34/test"},

		// 11-20: Allowed non-web schemes
		{"mailto basic", "mailto:user@example.com", defaultPolicy, "", "mailto:user@example.com"},
		{"mailto with subject", "mailto:user@example.com?subject=Hello", defaultPolicy, "", "mailto:user@example.com?subject=Hello"},
		{"tel standard", "tel:+1234567890", defaultPolicy, "", "tel:+1234567890"},
		{"tel with dashes", "tel:555-123-4567", defaultPolicy, "", "tel:555-123-4567"},
		{"sms standard", "sms:+19876543210", defaultPolicy, "", "sms:+19876543210"},
		{"sms with body", "sms:+19876543210?body=Hi", defaultPolicy, "", "sms:+19876543210?body=Hi"},
		{"geo coordinates", "geo:37.7749,-122.4194", defaultPolicy, "", "geo:37.7749,-122.4194"},
		{"whatsapp deep link", "whatsapp://send?phone=1234567890", defaultPolicy, "", "whatsapp://send?phone=1234567890"},
		{"instagram profile", "instagram://user?username=example", defaultPolicy, "", "instagram://user?username=example"},
		{"spotify link", "spotify:track:6rqhFgbbKwnb9MLmUQDhG6", defaultPolicy, "", "spotify:track:6rqhFgbbKwnb9MLmUQDhG6"},
		{"upi pay link", "upi://pay?pa=merchant@upi&pn=Store&am=100", defaultPolicy, "", "upi://pay?pa=merchant@upi&pn=Store&am=100"},

		// 21-30: Disallowed and dangerous schemes
		{"javascript basic", "javascript:alert(1)", defaultPolicy, "scheme_not_allowed", ""},
		{"javascript upper", "JAVASCRIPT:alert(document.cookie)", defaultPolicy, "scheme_not_allowed", ""},
		{"data uri html", "data:text/html,<script>alert(1)</script>", defaultPolicy, "scheme_not_allowed", ""},
		{"data uri base64", "data:text/plain;base64,SGVsbG8=", defaultPolicy, "scheme_not_allowed", ""},
		{"vbscript", "vbscript:msgbox(1)", defaultPolicy, "scheme_not_allowed", ""},
		{"file scheme", "file:///etc/passwd", defaultPolicy, "scheme_not_allowed", ""},
		{"blob scheme", "blob:https://example.com/uuid", defaultPolicy, "scheme_not_allowed", ""},
		{"intent scheme", "intent://scan/#Intent;scheme=zxing;end", defaultPolicy, "scheme_not_allowed", ""},
		{"about blank", "about:blank", defaultPolicy, "scheme_not_allowed", ""},
		{"chrome settings", "chrome://settings", defaultPolicy, "scheme_not_allowed", ""},

		// 31-40: Malformed, empty, control characters, userinfo
		{"empty string", "", defaultPolicy, "invalid_url", ""},
		{"spaces only", "   ", defaultPolicy, "invalid_url", ""},
		{"missing scheme", "www.example.com", defaultPolicy, "invalid_url", ""},
		{"missing host", "https://", defaultPolicy, "missing_host", ""},
		{"embedded tab", "https://example.com/path\twithtab", defaultPolicy, "invalid_url", ""},
		{"embedded newline", "https://example.com/path\nwithnewline", defaultPolicy, "invalid_url", ""},
		{"embedded null byte", "https://example.com/path\x00null", defaultPolicy, "invalid_url", ""},
		{"embedded space", "https://example.com/path with space", defaultPolicy, "invalid_url", ""},
		{"userinfo user only", "https://admin@example.com", defaultPolicy, "userinfo_not_allowed", ""},
		{"userinfo user and pass", "https://user:password@example.com/login", defaultPolicy, "userinfo_not_allowed", ""},
		{"userinfo deception", "https://paypal.com@evil.com", defaultPolicy, "userinfo_not_allowed", ""},

		// 41-48: Shorteners
		{"bit.ly shortener blocked", "https://bit.ly/3XYZ123", defaultPolicy, "shortener_not_allowed", ""},
		{"t.co shortener blocked", "https://t.co/abcde", defaultPolicy, "shortener_not_allowed", ""},
		{"tinyurl blocked", "https://tinyurl.com/something", defaultPolicy, "shortener_not_allowed", ""},
		{"goo.gl blocked", "https://goo.gl/maps/123", defaultPolicy, "shortener_not_allowed", ""},
		{"is.gd blocked", "https://is.gd/xyz", defaultPolicy, "shortener_not_allowed", ""},
		{"subdomain of bitly blocked", "https://custom.bit.ly/code", defaultPolicy, "shortener_not_allowed", ""},
		{"shortener allowed with policy", "https://bit.ly/3XYZ123", urlsafety.Policy{AllowShorteners: true}, "", "https://bit.ly/3XYZ123"},
		{"tinyurl allowed with policy", "https://tinyurl.com/abc", urlsafety.Policy{AllowShorteners: true}, "", "https://tinyurl.com/abc"},

		// 49-54: Own domain protection
		{"own root domain", "https://qrit.io/ABC1234", defaultPolicy, "own_domain", ""},
		{"own subdomain", "https://links.qrit.io/XYZ", defaultPolicy, "own_domain", ""},
		{"second own domain", "https://qr.it/foo", defaultPolicy, "own_domain", ""},
		{"own domain case variation", "https://QRIT.IO/admin", defaultPolicy, "own_domain", ""},
		{"not own domain but prefix", "https://qrit.io.attacker.com", defaultPolicy, "", "https://qrit.io.attacker.com"},
		{"not own domain substring", "https://notqrit.io", defaultPolicy, "", "https://notqrit.io"},

		// 55-60: HTTPS enforcement & AllowedHosts policy
		{"http allowed by default", "http://myblog.com", defaultPolicy, "", "http://myblog.com"},
		{"http blocked when HTTPS required", "http://myblog.com", urlsafety.Policy{RequireHTTPS: true}, "https_required", ""},
		{"https passes when HTTPS required", "https://myblog.com", urlsafety.Policy{RequireHTTPS: true}, "", "https://myblog.com"},
		{"allowed hosts exact match", "https://brand.com/products", urlsafety.Policy{AllowedHosts: []string{"brand.com"}}, "", "https://brand.com/products"},
		{"allowed hosts wildcard match", "https://shop.brand.com/item", urlsafety.Policy{AllowedHosts: []string{"*.brand.com"}}, "", "https://shop.brand.com/item"},
		{"allowed hosts blocked", "https://other.com", urlsafety.Policy{AllowedHosts: []string{"brand.com"}}, "host_not_allowed", ""},

		// 61-68: IDN & Confusable detection
		{"IDN german umlaut", "https://münchen.de", defaultPolicy, "", "https://xn--mnchen-3ya.de"},
		{"mixed script cyrillic a in paypal", "https://pаypal.com", defaultPolicy, "confusable_host", ""},
		{"mixed script greek o", "https://gοogle.com", defaultPolicy, "confusable_host", ""},
		{"pure cyrillic domain", "https://президент.рф", defaultPolicy, "", "https://xn--d1abbgf6aiiy.xn--p1ai"},
		{"length exceeding limit", "https://example.com/" + strings.Repeat("a", 2040), defaultPolicy, "too_long", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, err := urlsafety.Validate(tt.raw, tt.policy)
			if tt.wantCode != "" {
				if err == nil {
					t.Fatalf("Validate(%q) expected error code %q, got nil (normalized: %s)", tt.raw, tt.wantCode, normalized)
				}
				valErr, ok := err.(*urlsafety.ValidationError)
				if !ok {
					t.Fatalf("Validate(%q) expected *ValidationError, got %T (%v)", tt.raw, err, err)
				}
				if valErr.Code != tt.wantCode {
					t.Errorf("Validate(%q) code = %q, want %q", tt.raw, valErr.Code, tt.wantCode)
				}
			} else {
				if err != nil {
					t.Fatalf("Validate(%q) unexpected error: %v", tt.raw, err)
				}
				if tt.wantURLPart != "" && !strings.Contains(normalized, tt.wantURLPart) {
					t.Errorf("Validate(%q) = %q; want containing %q", tt.raw, normalized, tt.wantURLPart)
				}
			}
		})
	}
}

func TestFakeSafetyClient(t *testing.T) {
	ctx := context.Background()
	client := urlsafety.NewFakeSafetyClient()

	client.UnsafeURLs["https://malware.evil.com"] = true

	verdict, err := client.Check(ctx, "https://example.com")
	if err != nil || verdict != urlsafety.VerdictSafe {
		t.Errorf("expected safe, got %v, err: %v", verdict, err)
	}

	verdict, err = client.Check(ctx, "https://malware.evil.com")
	if err != nil || verdict != urlsafety.VerdictUnsafe {
		t.Errorf("expected unsafe, got %v, err: %v", verdict, err)
	}
}
