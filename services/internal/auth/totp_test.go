package auth

import (
	"strings"
	"testing"
	"time"
)

func TestTOTPGenerationAndVerification(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("failed to generate secret: %v", err)
	}

	if len(secret) < 20 {
		t.Fatalf("secret too short: %s", secret)
	}

	now := time.Now()
	code, err := GenerateTOTPCode(secret, now)
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	if len(code) != 6 {
		t.Fatalf("expected 6-digit code, got %s", code)
	}

	// Code must verify at current time
	if !VerifyTOTPCode(secret, code, now) {
		t.Fatalf("failed to verify code at exact time")
	}

	// Code must verify within 30s drift
	if !VerifyTOTPCode(secret, code, now.Add(25*time.Second)) {
		t.Fatalf("failed to verify code within +25s drift")
	}
	if !VerifyTOTPCode(secret, code, now.Add(-25*time.Second)) {
		t.Fatalf("failed to verify code within -25s drift")
	}

	// Code must reject expired window (> 60s)
	if VerifyTOTPCode(secret, code, now.Add(95*time.Second)) {
		t.Fatalf("code should have been rejected beyond tolerance window")
	}

	// Invalid code must fail
	if VerifyTOTPCode(secret, "000000", now) && code != "000000" {
		t.Fatalf("wrong code should not verify")
	}
}

func TestKeyURIGeneration(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	uri := GenerateKeyURI("QRit", "user@example.com", secret)

	if !strings.HasPrefix(uri, "otpauth://totp/QRit:user@example.com?") {
		t.Fatalf("unexpected uri prefix: %s", uri)
	}
	if !strings.Contains(uri, "secret=JBSWY3DPEHPK3PXP") {
		t.Fatalf("uri missing secret: %s", uri)
	}
	if !strings.Contains(uri, "issuer=QRit") {
		t.Fatalf("uri missing issuer: %s", uri)
	}
}

func TestRecoveryCodes(t *testing.T) {
	plain, hashed, err := GenerateRecoveryCodes(10)
	if err != nil {
		t.Fatalf("failed to generate recovery codes: %v", err)
	}

	if len(plain) != 10 || len(hashed) != 10 {
		t.Fatalf("expected 10 codes, got %d and %d", len(plain), len(hashed))
	}

	for i := 0; i < 10; i++ {
		if !VerifyRecoveryCode(hashed[i], plain[i]) {
			t.Fatalf("recovery code %d failed verification", i)
		}
		if VerifyRecoveryCode(hashed[i], "WRONG-CODE") {
			t.Fatalf("wrong recovery code verified successfully")
		}
	}
}
