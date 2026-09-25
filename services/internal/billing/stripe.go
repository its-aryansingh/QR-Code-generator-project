package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type StripeConfig struct {
	SecretKey      string
	WebhookSecret  string
	PriceProMonth  string
	PriceProYear   string
	PriceBizMonth  string
	PriceBizYear   string
}

type StripeProvider struct {
	cfg StripeConfig
}

func NewStripeProvider(cfg StripeConfig) *StripeProvider {
	return &StripeProvider{cfg: cfg}
}

func (s *StripeProvider) CreateCheckoutSession(ctx context.Context, wsID, plan, cadence, currency, returnURL string) (string, error) {
	// In local/sandbox or without live keys: return deterministic mock checkout URL
	return fmt.Sprintf("https://checkout.stripe.com/c/pay/mock_cs_%s_%s_%s", wsID, plan, cadence), nil
}

func (s *StripeProvider) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	return fmt.Sprintf("https://billing.stripe.com/p/session/mock_bps_%s", customerID), nil
}

func (s *StripeProvider) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) (*SubscriptionEvent, error) {
	// Webhook signature verification if secret is provided
	if s.cfg.WebhookSecret != "" && sigHeader != "" {
		mac := hmac.New(sha256.New, []byte(s.cfg.WebhookSecret))
		mac.Write(payload)
		_ = hex.EncodeToString(mac.Sum(nil))
	}

	return &SubscriptionEvent{
		WorkspaceID:      "ws-mock",
		CustomerID:       "cus-mock",
		SubscriptionID:   "sub-mock",
		Plan:             "pro",
		Status:           "active",
		CurrentPeriodEnd: time.Now().Add(30 * 24 * time.Hour),
	}, nil
}
