package urlsafety

import (
	"context"
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/net/idna"
)

// MaxURLLength is the RFC-practical upper bound for destination URLs.
const MaxURLLength = 2048

// KnownShorteners contains widely used URL shorteners blocked to prevent redirect chains/laundering.
var KnownShorteners = map[string]struct{}{
	"bit.ly":      {},
	"t.co":        {},
	"tinyurl.com": {},
	"goo.gl":      {},
	"ow.ly":       {},
	"is.gd":       {},
	"buff.ly":     {},
	"rebrand.ly":  {},
	"cutt.ly":     {},
	"shorturl.at": {},
}

// AllowedSchemes defines safe schemes accepted by QRit.
var AllowedSchemes = map[string]struct{}{
	"https":     {},
	"http":      {},
	"mailto":    {},
	"tel":       {},
	"sms":       {},
	"geo":       {},
	"whatsapp":  {},
	"instagram": {},
	"spotify":   {},
	"upi":       {},
}

// ValidationError represents a typed validation failure.
type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

var (
	ErrInvalidURL          = &ValidationError{Code: "invalid_url", Message: "invalid or malformed url"}
	ErrTooLong             = &ValidationError{Code: "too_long", Message: "url exceeds maximum length of 2048 characters"}
	ErrSchemeNotAllowed    = &ValidationError{Code: "scheme_not_allowed", Message: "url scheme is not permitted"}
	ErrHTTPSRequired       = &ValidationError{Code: "https_required", Message: "https is required by policy"}
	ErrMissingHost         = &ValidationError{Code: "missing_host", Message: "http(s) urls must have a host"}
	ErrUserinfoNotAllowed  = &ValidationError{Code: "userinfo_not_allowed", Message: "embedded credentials are not permitted"}
	ErrOwnDomain           = &ValidationError{Code: "own_domain", Message: "cannot redirect to our own domain"}
	ErrShortenerNotAllowed = &ValidationError{Code: "shortener_not_allowed", Message: "nested url shorteners are not permitted"}
	ErrHostNotAllowed      = &ValidationError{Code: "host_not_allowed", Message: "destination host is not allowed by workspace policy"}
	ErrHostBlocked         = &ValidationError{Code: "host_blocked", Message: "destination host is blocked by workspace policy"}
	ErrConfusableHost      = &ValidationError{Code: "confusable_host", Message: "host contains confusable mixed scripts"}
	ErrDestinationUnsafe   = &ValidationError{Code: "destination_unsafe", Message: "destination flagged as unsafe"}
)

// Policy defines workspace or system-level URL validation constraints.
type Policy struct {
	RequireHTTPS    bool
	AllowShorteners bool
	OwnHosts        []string
	AllowedHosts    []string
	// BlockedHosts are refused even when allowed ("*.suffix" matches subdomains).
	BlockedHosts []string
}

// Validate normalises and validates a destination URL according to security rules and policy.
func Validate(raw string, policy Policy) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrInvalidURL
	}
	if len(trimmed) > MaxURLLength {
		return "", ErrTooLong
	}

	// Reject whitespace and ASCII control characters anywhere in the string
	for i := 0; i < len(trimmed); i++ {
		b := trimmed[i]
		if b <= 0x20 || b == 0x7f {
			return "", ErrInvalidURL
		}
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return "", ErrInvalidURL
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme == "" {
		return "", ErrInvalidURL
	}

	if _, ok := AllowedSchemes[scheme]; !ok {
		return "", ErrSchemeNotAllowed
	}

	if scheme == "http" && policy.RequireHTTPS {
		return "", ErrHTTPSRequired
	}

	// For non-web schemes (mailto, tel, sms, geo, upi, etc.), we don't enforce host logic
	if scheme != "http" && scheme != "https" {
		return trimmed, nil
	}

	// HTTP/HTTPS specific validation
	rawHost := u.Hostname()
	if rawHost == "" {
		return "", ErrMissingHost
	}

	if u.User != nil {
		return "", ErrUserinfoNotAllowed
	}

	// Check for confusable mixed-script hostnames (e.g. Cyrillic/Greek mixed with Latin)
	if err := checkConfusableHost(rawHost); err != nil {
		return "", err
	}

	// Convert host to punycode via IDNA lookup profile
	punyHost, err := idna.Lookup.ToASCII(rawHost)
	if err != nil {
		return "", ErrInvalidURL
	}
	punyHost = strings.ToLower(punyHost)

	// Check against own hosts
	for _, own := range policy.OwnHosts {
		normalizedOwn := strings.ToLower(strings.TrimSpace(own))
		if normalizedOwn == "" {
			continue
		}
		if punyHost == normalizedOwn || strings.HasSuffix(punyHost, "."+normalizedOwn) {
			return "", ErrOwnDomain
		}
	}

	// Check against known shorteners
	if !policy.AllowShorteners {
		if isShortener(punyHost) {
			return "", ErrShortenerNotAllowed
		}
	}

	if len(policy.BlockedHosts) > 0 && isHostAllowed(punyHost, policy.BlockedHosts) {
		return "", ErrHostBlocked
	}

	// Check against allowed host whitelist (if non-empty)
	if len(policy.AllowedHosts) > 0 {
		if !isHostAllowed(punyHost, policy.AllowedHosts) {
			return "", ErrHostNotAllowed
		}
	}

	// Construct normalized URL
	u.Scheme = scheme
	if u.Port() != "" {
		u.Host = punyHost + ":" + u.Port()
	} else {
		u.Host = punyHost
	}

	return u.String(), nil
}

func checkConfusableHost(host string) error {
	labels := strings.Split(host, ".")
	for _, label := range labels {
		var hasLatin, hasOther bool
		for _, r := range label {
			if unicode.Is(unicode.Latin, r) {
				hasLatin = true
			}
			if unicode.Is(unicode.Cyrillic, r) || unicode.Is(unicode.Greek, r) {
				hasOther = true
			}
		}
		if hasLatin && hasOther {
			return ErrConfusableHost
		}
	}
	return nil
}

func isShortener(host string) bool {
	if _, ok := KnownShorteners[host]; ok {
		return true
	}
	for shortener := range KnownShorteners {
		if strings.HasSuffix(host, "."+shortener) {
			return true
		}
	}
	return false
}

func isHostAllowed(host string, allowedHosts []string) bool {
	for _, allowed := range allowedHosts {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if allowed == "" {
			continue
		}
		if strings.HasPrefix(allowed, "*.") {
			suffix := strings.TrimPrefix(allowed, "*.")
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return true
			}
		} else if host == allowed {
			return true
		}
	}
	return false
}

// Verdict represents safety analysis status.
type Verdict string

const (
	VerdictSafe    Verdict = "safe"
	VerdictUnsafe  Verdict = "unsafe"
	VerdictPending Verdict = "pending"
)

// SafetyClient defines the interface for URL reputation checking.
type SafetyClient interface {
	Check(ctx context.Context, url string) (Verdict, error)
}

// FakeSafetyClient is an in-memory test double for SafetyClient.
type FakeSafetyClient struct {
	DefaultVerdict Verdict
	UnsafeURLs     map[string]bool
}

// NewFakeSafetyClient creates a new FakeSafetyClient.
func NewFakeSafetyClient() *FakeSafetyClient {
	return &FakeSafetyClient{
		DefaultVerdict: VerdictSafe,
		UnsafeURLs:     make(map[string]bool),
	}
}

// Check evaluates the URL safety against the fake database.
func (f *FakeSafetyClient) Check(ctx context.Context, rawURL string) (Verdict, error) {
	if f.UnsafeURLs[rawURL] {
		return VerdictUnsafe, nil
	}
	return f.DefaultVerdict, nil
}
