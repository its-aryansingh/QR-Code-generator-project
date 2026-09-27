package billing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/its-aryansingh/qrit/services/internal/invoicing"
)

// RazorpayLinks creates Razorpay Payment Links for INR invoices and verifies Razorpay
// webhooks (https://razorpay.com/docs/api/payments/payment-links/).
type RazorpayLinks struct {
	KeyID         string
	KeySecret     string
	WebhookSecret string
	BaseURL       string // default https://api.razorpay.com
	CallbackURL   string // where the payer returns after paying (the invoices page)
	HTTP          *http.Client
}

func (r RazorpayLinks) Configured() bool { return r.KeyID != "" && r.KeySecret != "" }

func (r RazorpayLinks) base() string {
	if r.BaseURL != "" {
		return strings.TrimRight(r.BaseURL, "/")
	}
	return "https://api.razorpay.com"
}

// CreateLink creates a link for the invoice total; reference_id is the invoice number
// (unique per account), notes carry the invoice id for the webhook.
func (r RazorpayLinks) CreateLink(ctx context.Context, inv invoicing.Invoice) (string, string, error) {
	body := map[string]any{
		"amount": inv.TotalMinor, "currency": inv.Currency, "accept_partial": false,
		"reference_id": inv.Number, "description": "Invoice " + inv.Number,
		"expire_by":       time.Now().AddDate(0, 3, 0).Unix(),
		"customer":        map[string]any{"name": inv.Buyer.LegalName, "email": inv.Buyer.Email},
		"notify":          map[string]any{"email": inv.Buyer.Email != "", "sms": false},
		"reminder_enable": true,
		"notes":           map[string]any{"invoice_id": inv.ID.String(), "org_id": inv.OrgID.String()},
	}
	if r.CallbackURL != "" {
		body["callback_url"], body["callback_method"] = r.CallbackURL, "get"
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.base()+"/v1/payment_links", bytes.NewReader(b))
	if err != nil {
		return "", "", err
	}
	req.SetBasicAuth(r.KeyID, r.KeySecret)
	req.Header.Set("Content-Type", "application/json")
	client := r.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if res.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("razorpay payment link: %d %s", res.StatusCode, strings.TrimSpace(string(rb)))
	}
	var out struct {
		ID       string `json:"id"`
		ShortURL string `json:"short_url"`
	}
	if err := json.Unmarshal(rb, &out); err != nil || out.ID == "" {
		return "", "", errors.New("razorpay payment link: unexpected response")
	}
	return out.ID, out.ShortURL, nil
}

// VerifyWebhook checks X-Razorpay-Signature (hex HMAC-SHA256 of the raw body).
func VerifyWebhook(body []byte, signature, secret string) bool {
	if secret == "" || signature == "" {
		return false
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hmac.Equal([]byte(hex.EncodeToString(m.Sum(nil))), []byte(strings.TrimSpace(signature)))
}

// PaidLink is the part of a payment_link.paid event we act on.
type PaidLink struct {
	EventID   string
	LinkID    string
	InvoiceID string
	PaymentID string
	Amount    int64
}

// ParsePaymentLinkPaid extracts a payment_link.paid event (ok=false for other events).
func ParsePaymentLinkPaid(body []byte) (PaidLink, bool, error) {
	var ev struct {
		Event   string `json:"event"`
		Payload struct {
			PaymentLink struct {
				Entity struct {
					ID          string            `json:"id"`
					ReferenceID string            `json:"reference_id"`
					Status      string            `json:"status"`
					Notes       map[string]string `json:"notes"`
				} `json:"entity"`
			} `json:"payment_link"`
			Payment struct {
				Entity struct {
					ID     string `json:"id"`
					Amount int64  `json:"amount"`
				} `json:"entity"`
			} `json:"payment"`
		} `json:"payload"`
		CreatedAt int64 `json:"created_at"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		return PaidLink{}, false, err
	}
	if ev.Event != "payment_link.paid" {
		return PaidLink{}, false, nil
	}
	e := ev.Payload.PaymentLink.Entity
	return PaidLink{EventID: e.ID + ":" + ev.Payload.Payment.Entity.ID, LinkID: e.ID, InvoiceID: e.Notes["invoice_id"],
		PaymentID: ev.Payload.Payment.Entity.ID, Amount: ev.Payload.Payment.Entity.Amount}, true, nil
}
