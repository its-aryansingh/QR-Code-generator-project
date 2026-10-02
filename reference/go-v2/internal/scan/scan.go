package scan

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
)

type GeoFacts struct {
	Country string `json:"cc,omitempty"`
	Region  string `json:"rg,omitempty"`
	City    string `json:"ct,omitempty"`
}

type UTMParams struct {
	Source   *string `json:"s,omitempty"`
	Medium   *string `json:"m,omitempty"`
	Campaign *string `json:"c,omitempty"`
}

// ScanEvent defines the JSON payload published to Redis Stream "scans".
type ScanEvent struct {
	ID          string     `json:"id"`
	Timestamp   time.Time  `json:"ts"`
	WorkspaceID string     `json:"ws"`
	QRCodeID    string     `json:"qr"`
	VersionID   *string    `json:"ver"`
	CampaignID  *string    `json:"cmp,omitempty"`
	DomainID    string     `json:"dom"`
	Rule        *string    `json:"rule,omitempty"`
	Outcome     string     `json:"out"`
	Method      string     `json:"m"`
	VisitorHash string     `json:"vh"`
	UserAgent   string     `json:"ua"`
	Datacenter  bool       `json:"dc"`
	Geo         GeoFacts   `json:"geo"`
	Language    *string    `json:"lang"`
	Referrer    *string    `json:"ref,omitempty"`
	UTM         *UTMParams `json:"utm,omitempty"`
}

// SaltForDate derives a deterministic daily salt from the master secret and UTC date.
func SaltForDate(secret []byte, date time.Time) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(date.UTC().Format("2006-01-02")))
	return h.Sum(nil)
}

// ComputeVisitorHash generates a privacy-preserving 16-byte HMAC-SHA256 visitor hash:
// HMAC-SHA256(salt(todayUTC), qrID || 0x00 || ip || 0x00 || ua)[:16]
func ComputeVisitorHash(salt []byte, qrID uuid.UUID, ip, ua string) string {
	h := hmac.New(sha256.New, salt)
	h.Write(qrID[:])
	h.Write([]byte{0x00})
	h.Write([]byte(strings.TrimSpace(ip)))
	h.Write([]byte{0x00})
	h.Write([]byte(strings.TrimSpace(ua)))
	sum := h.Sum(nil)
	return base64.StdEncoding.EncodeToString(sum[:16])
}

var botPatterns = []string{
	// Link preview bots
	"facebookexternalhit", "whatsapp", "telegrambot", "slackbot", "twitterbot",
	"linkedinbot", "discordbot", "applebot", "skypeuripreview",
	// Search engine crawlers
	"googlebot", "bingbot", "baiduspider", "yandexbot", "duckduckbot",
	"slurp", "sogou", "exabot", "ia_archiver",
	// Security scanners
	"safelinks", "proofpoint", "mimecast", "barracuda", "trendmicro",
	"forcepoint", "symantec", "kaspersky",
	// HTTP libraries and tools
	"curl", "wget", "python-requests", "go-http-client", "okhttp",
	"axios", "node-fetch", "httpclient", "postmanruntime",
	// Headless / automation browsers
	"headlesschrome", "phantomjs", "selenium", "puppeteer", "playwright",
}

// BotVerdict indicates whether a request is automated.
type BotVerdict struct {
	IsBot  bool   `json:"is_bot"`
	Reason string `json:"reason,omitempty"`
}

// ClassifyBot evaluates request attributes to detect automation/crawlers.
func ClassifyBot(method, ua string, datacenter bool) BotVerdict {
	// Rule 1: HEAD requests are previews/healthchecks
	if strings.ToUpper(method) == "HEAD" {
		return BotVerdict{IsBot: true, Reason: "head_request"}
	}

	trimmedUA := strings.TrimSpace(ua)
	// Rule 2: Missing UA
	if trimmedUA == "" {
		return BotVerdict{IsBot: true, Reason: "ua_missing"}
	}

	lowerUA := strings.ToLower(trimmedUA)
	// Rule 3: Known crawler / bot signatures
	for _, pattern := range botPatterns {
		if strings.Contains(lowerUA, pattern) {
			return BotVerdict{IsBot: true, Reason: "ua_bot"}
		}
	}

	// Rule 4: Datacenter IP presenting as desktop/generic browser
	if datacenter {
		return BotVerdict{IsBot: true, Reason: "datacenter"}
	}

	return BotVerdict{IsBot: false}
}

// ExtractReferrerHost parses and returns only the hostname of a Referer header.
func ExtractReferrerHost(referer string) *string {
	clean := strings.TrimSpace(referer)
	if clean == "" {
		return nil
	}
	u, err := url.Parse(clean)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	return &host
}

// ExtractPrimaryLanguage extracts the primary 2-letter language code from Accept-Language.
func ExtractPrimaryLanguage(acceptLang string) string {
	if acceptLang == "" {
		return "en"
	}
	parts := strings.Split(acceptLang, ",")
	first := strings.TrimSpace(parts[0])
	langParts := strings.Split(first, "-")
	return strings.ToLower(langParts[0])
}

// NewScanEvent constructs a standardized ScanEvent with UUIDv7 ID and ISO timestamp.
func NewScanEvent(
	now time.Time,
	wsID, qrID, verID, domID string,
	cmpID, rule *string,
	outcome, method, vh, ua string,
	dc bool,
	geo GeoFacts,
	lang string,
	ref *string,
	utm *UTMParams,
) *ScanEvent {
	ev := &ScanEvent{
		ID:          idgen.New().String(),
		Timestamp:   now.UTC(),
		WorkspaceID: wsID,
		QRCodeID:    qrID,
		CampaignID:  cmpID,
		DomainID:    domID,
		Rule:        rule,
		Outcome:     outcome,
		Method:      method,
		VisitorHash: vh,
		UserAgent:   ua,
		Datacenter:  dc,
		Geo:         geo,
		Referrer:    ref,
		UTM:         utm,
	}
	if verID != "" {
		ev.VersionID = &verID
	}
	if lang != "" {
		ev.Language = &lang
	}
	return ev
}

// ParseRequestFacts extracts facts from an incoming HTTP request.
func ParseRequestFacts(r *http.Request) (ip, ua, lang string, ref *string) {
	// IP extraction (Cloudflare CF-Connecting-IP preferred)
	ip = r.Header.Get("CF-Connecting-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
		if ip != "" {
			parts := strings.Split(ip, ",")
			ip = strings.TrimSpace(parts[0])
		}
	}
	if ip == "" {
		ip = r.RemoteAddr
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}
	}

	ua = r.UserAgent()
	lang = ExtractPrimaryLanguage(r.Header.Get("Accept-Language"))
	ref = ExtractReferrerHost(r.Referer())
	return
}
