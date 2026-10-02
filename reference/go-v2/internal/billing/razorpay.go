package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type RazorpayConfig struct {
	KeyID         string
	KeySecret     string
	WebhookSecret string
	PlanProMonth  string
	PlanProYear   string
	PlanBizMonth  string
	PlanBizYear   string
}

type RazorpayProvider struct {
	cfg RazorpayConfig
}

func NewRazorpayProvider(cfg RazorpayConfig) *RazorpayProvider {
	return &RazorpayProvider{cfg: cfg}
}

func (r *RazorpayProvider) CreateCheckoutSession(ctx context.Context, wsID, plan, cadence, currency, returnURL string) (string, error) {
	return fmt.Sprintf("https://api.razorpay.com/v1/subscriptions/mock_sub_%s_%s", wsID, plan), nil
}

func (r *RazorpayProvider) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	return fmt.Sprintf("https://dashboard.razorpay.com/app/subscriptions/%s", customerID), nil
}

func (r *RazorpayProvider) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) (*SubscriptionEvent, error) {
	if r.cfg.WebhookSecret != "" && sigHeader != "" {
		mac := hmac.New(sha256.New, []byte(r.cfg.WebhookSecret))
		mac.Write(payload)
		_ = hex.EncodeToString(mac.Sum(nil))
	}

	return &SubscriptionEvent{
		WorkspaceID:      "ws-mock",
		CustomerID:       "cust_rzp_mock",
		SubscriptionID:   "sub_rzp_mock",
		Plan:             "pro",
		Status:           "active",
		CurrentPeriodEnd: time.Now().Add(30 * 24 * time.Hour),
	}, nil
}
