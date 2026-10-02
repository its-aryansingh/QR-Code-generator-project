package email

import (
	"strings"
	"testing"
)

func TestEmailTemplates(t *testing.T) {
	// EN Verification
	enVerify := RenderVerificationEmail(LangEN, "https://qrit.io/verify?t=123")
	if !strings.Contains(enVerify.Subject, "Verify your QRit account") {
		t.Fatalf("unexpected EN subject: %s", enVerify.Subject)
	}
	if !strings.Contains(enVerify.HTML, "https://qrit.io/verify?t=123") {
		t.Fatalf("EN HTML missing link")
	}

	// HI Verification
	hiVerify := RenderVerificationEmail(LangHI, "https://qrit.io/verify?t=123")
	if !strings.Contains(hiVerify.Subject, "खाता सत्यापन") {
		t.Fatalf("unexpected HI subject: %s", hiVerify.Subject)
	}
	if !strings.Contains(hiVerify.Text, "स्वागत") {
		t.Fatalf("HI text missing Hindi greeting")
	}

	// EN Invite
	enInvite := RenderInviteEmail(LangEN, "Aryan", "Engineering", "https://qrit.io/invite/456")
	if !strings.Contains(enInvite.Subject, "Aryan invited you to join Engineering") {
		t.Fatalf("unexpected invite subject: %s", enInvite.Subject)
	}

	// Safety Alert
	alert := RenderSafetyAlertEmail(LangEN, "Summer Standee", "SUMMER1", "malware detected")
	if !strings.Contains(alert.Subject, "[Security Alert]") {
		t.Fatalf("unexpected alert subject: %s", alert.Subject)
	}
	if !strings.Contains(alert.HTML, "malware detected") {
		t.Fatalf("alert HTML missing reason")
	}
}
