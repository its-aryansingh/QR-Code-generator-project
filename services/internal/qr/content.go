package qr

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ErrTextTooLong   = errors.New("text exceeds 1000 characters")
	ErrInvalidVPA    = errors.New("invalid UPI VPA address")
	ErrInvalidGTIN   = errors.New("invalid GS1 GTIN format or checksum")
	ErrInvalidLatLon = errors.New("invalid latitude or longitude")
)

var vpaRegex = regexp.MustCompile(`^[a-zA-Z0-9.\-_]{2,256}@[a-zA-Z]{2,64}$`)

// EncodeURL normalises and returns the URL string.
func EncodeURL(raw string) string {
	return strings.TrimSpace(raw)
}

// EncodeText returns text as-is up to 1000 characters.
func EncodeText(text string) (string, error) {
	if len(text) > 1000 {
		return "", ErrTextTooLong
	}
	return text, nil
}

// EncodeEmail formats mailto:{to}?subject=...&body=...
func EncodeEmail(to, subject, body string) string {
	to = strings.TrimSpace(to)
	params := url.Values{}
	if subject != "" {
		params.Set("subject", subject)
	}
	if body != "" {
		params.Set("body", body)
	}
	qs := params.Encode()
	if qs != "" {
		return fmt.Sprintf("mailto:%s?%s", to, qs)
	}
	return fmt.Sprintf("mailto:%s", to)
}

// EncodePhone formats tel:{e164}
func EncodePhone(e164 string) string {
	clean := strings.TrimSpace(e164)
	return fmt.Sprintf("tel:%s", clean)
}

// EncodeSMS formats SMSTO:{e164}:{message}
func EncodeSMS(e164, message string) string {
	cleanPhone := strings.TrimSpace(e164)
	return fmt.Sprintf("SMSTO:%s:%s", cleanPhone, message)
}

// EncodeWhatsApp formats https://wa.me/{digits}?text={urlenc}
func EncodeWhatsApp(phone, text string) string {
	reg := regexp.MustCompile(`[^0-9]+`)
	digits := reg.ReplaceAllString(phone, "")
	if text != "" {
		return fmt.Sprintf("https://wa.me/%s?text=%s", digits, url.QueryEscape(text))
	}
	return fmt.Sprintf("https://wa.me/%s", digits)
}

// EscapeWiFi escapes special characters (\ ; , : ") with a backslash.
func EscapeWiFi(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		switch b {
		case '\\', ';', ',', ':', '"':
			sb.WriteByte('\\')
			sb.WriteByte(b)
		default:
			sb.WriteByte(b)
		}
	}
	return sb.String()
}

// EncodeWiFi formats WIFI:T:{auth};S:{ssid};P:{password};H:{hidden};;
func EncodeWiFi(ssid, password, authType string, hidden bool) string {
	auth := strings.ToUpper(strings.TrimSpace(authType))
	if auth == "" || auth == "NONE" {
		auth = "nopass"
	}
	hiddenStr := "false"
	if hidden {
		hiddenStr = "true"
	}

	escapedSSID := EscapeWiFi(ssid)
	escapedPass := EscapeWiFi(password)

	return fmt.Sprintf("WIFI:T:%s;S:%s;P:%s;H:%s;;", auth, escapedSSID, escapedPass, hiddenStr)
}

// EscapeVCard escapes commas, semicolons, and backslashes with backslash.
func EscapeVCard(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		switch b {
		case '\\', ';', ',':
			sb.WriteByte('\\')
			sb.WriteByte(b)
		case '\n':
			sb.WriteString("\\n")
		case '\r':
			// skip CR inside property values
		default:
			sb.WriteByte(b)
		}
	}
	return sb.String()
}

type VCardData struct {
	FirstName  string
	LastName   string
	Company    string
	Title      string
	Phone      string
	Email      string
	Website    string
	Street     string
	City       string
	State      string
	PostalCode string
	Country    string
	Note       string
}

// EncodeVCard formats a vCard 3.0 string with CRLF line endings.
func EncodeVCard(data VCardData) string {
	var sb strings.Builder
	crlf := "\r\n"

	sb.WriteString("BEGIN:VCARD" + crlf)
	sb.WriteString("VERSION:3.0" + crlf)

	fullName := strings.TrimSpace(data.FirstName + " " + data.LastName)
	if fullName != "" {
		sb.WriteString("FN:" + EscapeVCard(fullName) + crlf)
	}

	sb.WriteString(fmt.Sprintf("N:%s;%s;;;%s",
		EscapeVCard(data.LastName),
		EscapeVCard(data.FirstName),
		crlf,
	))

	if data.Company != "" {
		sb.WriteString("ORG:" + EscapeVCard(data.Company) + crlf)
	}
	if data.Title != "" {
		sb.WriteString("TITLE:" + EscapeVCard(data.Title) + crlf)
	}
	if data.Phone != "" {
		sb.WriteString("TEL;TYPE=CELL:" + EscapeVCard(data.Phone) + crlf)
	}
	if data.Email != "" {
		sb.WriteString("EMAIL;TYPE=INTERNET:" + EscapeVCard(data.Email) + crlf)
	}
	if data.Website != "" {
		sb.WriteString("URL:" + EscapeVCard(data.Website) + crlf)
	}
	if data.Street != "" || data.City != "" || data.State != "" || data.PostalCode != "" || data.Country != "" {
		sb.WriteString(fmt.Sprintf("ADR;TYPE=WORK:;;%s;%s;%s;%s;%s%s",
			EscapeVCard(data.Street),
			EscapeVCard(data.City),
			EscapeVCard(data.State),
			EscapeVCard(data.PostalCode),
			EscapeVCard(data.Country),
			crlf,
		))
	}
	if data.Note != "" {
		sb.WriteString("NOTE:" + EscapeVCard(data.Note) + crlf)
	}

	sb.WriteString("END:VCARD" + crlf)
	return sb.String()
}

type EventData struct {
	Title       string
	Description string
	Location    string
	Start       time.Time
	End         time.Time
}

// EncodeEvent formats an iCalendar VEVENT with UTC timestamps.
func EncodeEvent(evt EventData) string {
	crlf := "\r\n"
	utcFormat := "20060102T150405Z"

	var sb strings.Builder
	sb.WriteString("BEGIN:VCALENDAR" + crlf)
	sb.WriteString("VERSION:2.0" + crlf)
	sb.WriteString("BEGIN:VEVENT" + crlf)
	if evt.Title != "" {
		sb.WriteString("SUMMARY:" + EscapeVCard(evt.Title) + crlf)
	}
	if evt.Description != "" {
		sb.WriteString("DESCRIPTION:" + EscapeVCard(evt.Description) + crlf)
	}
	if evt.Location != "" {
		sb.WriteString("LOCATION:" + EscapeVCard(evt.Location) + crlf)
	}
	sb.WriteString("DTSTART:" + evt.Start.UTC().Format(utcFormat) + crlf)
	sb.WriteString("DTEND:" + evt.End.UTC().Format(utcFormat) + crlf)
	sb.WriteString("END:VEVENT" + crlf)
	sb.WriteString("END:VCALENDAR" + crlf)
	return sb.String()
}

// EncodeUPI formats upi://pay?pa={vpa}&pn={name}&am={amount}&cu=INR&tn={note}
func EncodeUPI(vpa, name string, amount *float64, note string) (string, error) {
	vpa = strings.TrimSpace(vpa)
	if !vpaRegex.MatchString(vpa) {
		return "", ErrInvalidVPA
	}

	params := url.Values{}
	params.Set("pa", vpa)
	if name != "" {
		params.Set("pn", name)
	}
	if amount != nil && *amount > 0 {
		params.Set("am", strconv.FormatFloat(*amount, 'f', 2, 64))
	}
	params.Set("cu", "INR")
	if note != "" {
		params.Set("tn", note)
	}

	return fmt.Sprintf("upi://pay?%s", params.Encode()), nil
}

// EncodeLocation formats geo:{lat},{lng}
func EncodeLocation(lat, lng float64) (string, error) {
	if lat < -90.0 || lat > 90.0 || lng < -180.0 || lng > 180.0 {
		return "", ErrInvalidLatLon
	}
	return fmt.Sprintf("geo:%.6f,%.6f", lat, lng), nil
}

// ValidateAndPadGTIN14 validates a GTIN (8, 12, 13, or 14 digits) with mod-10 check digit and returns 14-digit string.
func ValidateAndPadGTIN14(raw string) (string, error) {
	digits := strings.TrimSpace(raw)
	for _, r := range digits {
		if r < '0' || r > '9' {
			return "", ErrInvalidGTIN
		}
	}
	switch len(digits) {
	case 8, 12, 13:
		digits = fmt.Sprintf("%014s", digits)
	case 14:
		// already 14 digits
	default:
		return "", ErrInvalidGTIN
	}

	// Validate GS1 Mod-10 check digit
	// Weights from right to left (excluding check digit): 3, 1, 3, 1, ...
	sum := 0
	for i := 0; i < 13; i++ {
		d := int(digits[i] - '0')
		weight := 3
		if i%2 == 0 {
			weight = 3 // For 14-digit, position 0 is weight 3
		} else {
			weight = 1
		}
		sum += d * weight
	}

	checkDigit := (10 - (sum % 10)) % 10
	if checkDigit != int(digits[13]-'0') {
		return "", ErrInvalidGTIN
	}

	return digits, nil
}

// FormatGS1Payload creates the canonical uppercase GS1 digital link payload URL:
// HTTPS://{HOST}/01/{GTIN14}
func FormatGS1Payload(host, gtin14 string) string {
	return strings.ToUpper(fmt.Sprintf("https://%s/01/%s", strings.TrimRight(host, "/"), gtin14))
}
