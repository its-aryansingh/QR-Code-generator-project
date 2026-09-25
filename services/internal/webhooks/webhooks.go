package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrSSRFBlocked     = errors.New("webhook destination IP is forbidden (private or loopback address)")
	ErrInvalidSignature = errors.New("invalid webhook signature")
)

type WebhookConfig struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	URL         string    `json:"url"`
	Secret      string    `json:"secret"`
	Events      []string  `json:"events"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// GenerateSignature generates standard t={timestamp},v1={hex} signature header.
func GenerateSignature(payload []byte, secret string, now time.Time) string {
	ts := strconv.FormatInt(now.Unix(), 10)
	toSign := fmt.Sprintf("%s.%s", ts, string(payload))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(toSign))
	sig := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("t=%s,v1=%s", ts, sig)
}

// VerifySignature validates a webhook signature header against secret with timestamp tolerance.
func VerifySignature(payload []byte, header, secret string, tolerance time.Duration) bool {
	parts := strings.Split(header, ",")
	var tsStr, sigStr string
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			if kv[0] == "t" {
				tsStr = kv[1]
			} else if kv[0] == "v1" {
				sigStr = kv[1]
			}
		}
	}

	if tsStr == "" || sigStr == "" {
		return false
	}

	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return false
	}

	now := time.Now().Unix()
	if tolerance > 0 && (now-ts > int64(tolerance.Seconds()) || ts-now > int64(tolerance.Seconds())) {
		return false
	}

	toSign := fmt.Sprintf("%s.%s", tsStr, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(toSign))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sigStr), []byte(expectedSig))
}

// IsPrivateIP checks if an IP belongs to private, loopback, or link-local CIDRs.
func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

// DeliverWebhook delivers payload via HTTP POST with SSRF protection.
func DeliverWebhook(ctx context.Context, targetURL string, payload []byte, secret string, client *http.Client) (int, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return 0, err
	}

	if u.Scheme != "https" && u.Scheme != "http" {
		return 0, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	// SSRF Check: resolve hostname and ensure no private IPs
	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		return 0, fmt.Errorf("lookup host: %w", err)
	}

	for _, ip := range ips {
		if IsPrivateIP(ip) {
			return 0, ErrSSRFBlocked
		}
	}

	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}

	sig := GenerateSignature(payload, secret, time.Now())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-QRit-Signature", sig)
	req.Header.Set("User-Agent", "QRit-Webhook/2.0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}
