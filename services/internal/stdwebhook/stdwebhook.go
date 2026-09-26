// Package stdwebhook signs and verifies payloads per the Standard Webhooks specification
// (https://www.standardwebhooks.com): headers webhook-id, webhook-timestamp and
// webhook-signature = "v1,<base64 HMAC-SHA256(key, id.timestamp.body)>", with secrets
// formatted "whsec_<base64 key>".
package stdwebhook

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const prefix = "whsec_"

// NewSecret returns a fresh secret (24 random bytes).
func NewSecret() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return prefix + base64.StdEncoding.EncodeToString(b)
}

func key(secret string) ([]byte, error) {
	if !strings.HasPrefix(secret, prefix) {
		return nil, errors.New("stdwebhook: secret must start with whsec_")
	}
	return base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, prefix))
}

// Sign returns the webhook-signature header value.
func Sign(secret, msgID string, ts time.Time, body []byte) (string, error) {
	k, err := key(secret)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, k)
	mac.Write([]byte(msgID + "." + strconv.FormatInt(ts.Unix(), 10) + "."))
	mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

// SetHeaders signs body and sets the three Standard Webhooks headers on req.
func SetHeaders(h http.Header, secret, msgID string, ts time.Time, body []byte) error {
	sig, err := Sign(secret, msgID, ts, body)
	if err != nil {
		return err
	}
	h.Set("webhook-id", msgID)
	h.Set("webhook-timestamp", strconv.FormatInt(ts.Unix(), 10))
	h.Set("webhook-signature", sig)
	return nil
}

// Verify checks headers against body within tolerance (receivers and tests).
func Verify(secret string, h http.Header, body []byte, tolerance time.Duration, now time.Time) error {
	id, tsRaw, sigs := h.Get("webhook-id"), h.Get("webhook-timestamp"), h.Get("webhook-signature")
	if id == "" || tsRaw == "" || sigs == "" {
		return errors.New("stdwebhook: missing headers")
	}
	sec, err := strconv.ParseInt(tsRaw, 10, 64)
	if err != nil {
		return errors.New("stdwebhook: bad timestamp")
	}
	ts := time.Unix(sec, 0)
	if now.Sub(ts) > tolerance || ts.Sub(now) > tolerance {
		return errors.New("stdwebhook: timestamp outside tolerance")
	}
	want, err := Sign(secret, id, ts, body)
	if err != nil {
		return err
	}
	for _, s := range strings.Fields(sigs) { // several signatures during secret rotation
		if hmac.Equal([]byte(s), []byte(want)) {
			return nil
		}
	}
	return errors.New("stdwebhook: signature mismatch")
}
