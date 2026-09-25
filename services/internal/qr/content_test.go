package qr_test

import (
	"strings"
	"testing"
	"time"

	"github.com/its-aryansingh/qrit/services/internal/qr"
)

func TestEncodeEmail(t *testing.T) {
	got := qr.EncodeEmail("user@example.com", "Hello World", "Body text with spaces & symbols!")
	want := "mailto:user@example.com?body=Body+text+with+spaces+%26+symbols%21&subject=Hello+World"
	if got != want {
		t.Errorf("EncodeEmail() = %q; want %q", got, want)
	}
}

func TestEncodePhoneAndSMS(t *testing.T) {
	phone := qr.EncodePhone("+15551234567")
	if phone != "tel:+15551234567" {
		t.Errorf("EncodePhone() = %q", phone)
	}

	sms := qr.EncodeSMS("+15551234567", "Hello there!")
	if sms != "SMSTO:+15551234567:Hello there!" {
		t.Errorf("EncodeSMS() = %q", sms)
	}
}

func TestEncodeWhatsApp(t *testing.T) {
	got := qr.EncodeWhatsApp("+1 (555) 123-4567", "Check this out")
	want := "https://wa.me/15551234567?text=Check+this+out"
	if got != want {
		t.Errorf("EncodeWhatsApp() = %q; want %q", got, want)
	}
}

func TestEncodeWiFi(t *testing.T) {
	got := qr.EncodeWiFi("My;Network:\"Guest\"", "P@ss\\word,123", "WPA", true)
	want := "WIFI:T:WPA;S:My\\;Network\\:\\\"Guest\\\";P:P@ss\\\\word\\,123;H:true;;"
	if got != want {
		t.Errorf("EncodeWiFi() = %q; want %q", got, want)
	}
}

func TestEncodeVCard(t *testing.T) {
	card := qr.VCardData{
		FirstName: "John",
		LastName:  "Doe, Jr.",
		Company:   "Acme; Inc.",
		Title:     "Lead Engineer",
		Phone:     "+15551234567",
		Email:     "john.doe@example.com",
		City:      "New York",
		Country:   "USA",
		Note:      "Line 1\nLine 2",
	}

	res := qr.EncodeVCard(card)
	if !strings.HasPrefix(res, "BEGIN:VCARD\r\nVERSION:3.0\r\n") {
		t.Errorf("vcard header invalid: %q", res)
	}
	if !strings.HasSuffix(res, "END:VCARD\r\n") {
		t.Errorf("vcard footer invalid: %q", res)
	}
	if !strings.Contains(res, "N:Doe\\, Jr.;John;;;\r\n") {
		t.Errorf("vcard name escaping failed: %q", res)
	}
	if !strings.Contains(res, "ORG:Acme\\; Inc.\r\n") {
		t.Errorf("vcard org escaping failed: %q", res)
	}
	if !strings.Contains(res, "NOTE:Line 1\\nLine 2\r\n") {
		t.Errorf("vcard note escaping failed: %q", res)
	}
}

func TestEncodeEvent(t *testing.T) {
	start := time.Date(2026, 10, 19, 10, 30, 0, 0, time.UTC)
	end := time.Date(2026, 10, 19, 11, 30, 0, 0, time.UTC)

	evt := qr.EventData{
		Title:       "Sprint Planning",
		Description: "Plan tasks for Q4",
		Location:    "Room 101",
		Start:       start,
		End:         end,
	}

	res := qr.EncodeEvent(evt)
	if !strings.Contains(res, "DTSTART:20261019T103000Z\r\n") {
		t.Errorf("DTSTART format invalid: %q", res)
	}
	if !strings.Contains(res, "DTEND:20261019T113000Z\r\n") {
		t.Errorf("DTEND format invalid: %q", res)
	}
	if !strings.Contains(res, "SUMMARY:Sprint Planning\r\n") {
		t.Errorf("SUMMARY format invalid: %q", res)
	}
}

func TestEncodeUPI(t *testing.T) {
	amt := 150.50
	got, err := qr.EncodeUPI("merchant@okaxis", "Coffee Shop", &amt, "Latte")
	if err != nil {
		t.Fatalf("EncodeUPI failed: %v", err)
	}
	if !strings.Contains(got, "pa=merchant%40okaxis") || !strings.Contains(got, "am=150.50") {
		t.Errorf("EncodeUPI() = %q", got)
	}

	_, err = qr.EncodeUPI("invalid-vpa-without-at", "Shop", nil, "")
	if err == nil {
		t.Errorf("expected error for invalid VPA")
	}
}

func TestEncodeLocation(t *testing.T) {
	loc, err := qr.EncodeLocation(37.774929, -122.419416)
	if err != nil {
		t.Fatalf("EncodeLocation failed: %v", err)
	}
	if loc != "geo:37.774929,-122.419416" {
		t.Errorf("EncodeLocation() = %q", loc)
	}

	_, err = qr.EncodeLocation(95.0, 10.0)
	if err == nil {
		t.Errorf("expected error for out of bounds latitude")
	}
}

func TestGS1GTIN(t *testing.T) {
	// Valid GTIN-13: 4006381333931 -> pad to 14: 04006381333931
	gtin14, err := qr.ValidateAndPadGTIN14("4006381333931")
	if err != nil {
		t.Fatalf("ValidateAndPadGTIN14 failed: %v", err)
	}
	if gtin14 != "04006381333931" {
		t.Errorf("padded gtin = %q; want 04006381333931", gtin14)
	}

	payload := qr.FormatGS1Payload("qr.brand.com", gtin14)
	if payload != "HTTPS://QR.BRAND.COM/01/04006381333931" {
		t.Errorf("FormatGS1Payload() = %q", payload)
	}

	// Invalid check digit
	_, err = qr.ValidateAndPadGTIN14("4006381333932")
	if err == nil {
		t.Errorf("expected error for invalid check digit")
	}
}
