package billing

import (
	"context"
	"testing"
)

func TestBillingProviders(t *testing.T) {
	ctx := context.Background()

	// Stripe test
	sp := NewStripeProvider(StripeConfig{})
	csURL, err := sp.CreateCheckoutSession(ctx, "ws-1", "pro", "month", "usd", "https://qrit.io")
	if err != nil || csURL == "" {
		t.Fatalf("stripe checkout URL failed: %v", err)
	}

	ev, err := sp.HandleWebhook(ctx, []byte(`{"type":"customer.subscription.updated"}`), "")
	if err != nil || ev.Plan != "pro" {
		t.Fatalf("stripe webhook handling failed: %v", err)
	}

	// Razorpay test
	rp := NewRazorpayProvider(RazorpayConfig{})
	rzpURL, err := rp.CreateCheckoutSession(ctx, "ws-1", "pro", "month", "inr", "https://qrit.io")
	if err != nil || rzpURL == "" {
		t.Fatalf("razorpay checkout URL failed: %v", err)
	}

	ev2, err := rp.HandleWebhook(ctx, []byte(`{"event":"subscription.activated"}`), "")
	if err != nil || ev2.Plan != "pro" {
		t.Fatalf("razorpay webhook handling failed: %v", err)
	}

	// Service downgrade offline test
	svc := NewService(nil, sp)
	err = svc.HandleDowngrade(ctx, "ws-1", 10)
	if err != nil {
		t.Fatalf("downgrade handling failed: %v", err)
	}
}
