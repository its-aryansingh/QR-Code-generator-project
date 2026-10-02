package webhooks

import (
	"net"
	"testing"
	"time"
)

func TestWebhookSignatureGenerationAndVerification(t *testing.T) {
	secret := "whsec_supersecret123"
	payload := []byte(`{"event":"scan.created","data":{"qr_id":"qr-1","scans":42}}`)
	now := time.Now()

	sig := GenerateSignature(payload, secret, now)
	if sig == "" {
		t.Fatalf("expected non-empty signature header")
	}

	// Valid signature
	ok := VerifySignature(payload, sig, secret, 5*time.Minute)
	if !ok {
		t.Fatalf("expected signature verification to succeed")
	}

	// Tampered payload
	tampered := []byte(`{"event":"scan.created","data":{"qr_id":"qr-1","scans":43}}`)
	if VerifySignature(tampered, sig, secret, 5*time.Minute) {
		t.Fatalf("expected signature verification to fail for tampered payload")
	}

	// Invalid secret
	if VerifySignature(payload, sig, "wrong_secret", 5*time.Minute) {
		t.Fatalf("expected verification to fail with wrong secret")
	}
}

func TestSSRFChecks(t *testing.T) {
	privateIPs := []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.169.254",
		"::1",
	}

	for _, ipStr := range privateIPs {
		ip := net.ParseIP(ipStr)
		if !IsPrivateIP(ip) {
			t.Errorf("expected %s to be classified as private IP", ipStr)
		}
	}

	publicIPs := []string{
		"8.8.8.8",
		"1.1.1.1",
		"142.250.190.46",
	}

	for _, ipStr := range publicIPs {
		ip := net.ParseIP(ipStr)
		if IsPrivateIP(ip) {
			t.Errorf("expected %s to NOT be classified as private IP", ipStr)
		}
	}
}
