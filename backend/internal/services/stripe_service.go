package services

import (
	"errors"
	"os"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"
	"github.com/stripe/stripe-go/v76/webhook"

	"github.com/qrapp/backend/internal/models"
)

// Stripe errors
var (
	ErrStripeNotConfigured = errors.New("stripe not configured")
	ErrInvalidPlan         = errors.New("invalid plan")
	ErrCustomerNotFound    = errors.New("customer not found")
)

// Price IDs (set these in Stripe dashboard)
var PriceIDs = map[string]struct {
	Monthly string
	Yearly  string
}{
	models.PlanStarter: {
		Monthly: "price_starter_monthly",
		Yearly:  "price_starter_yearly",
	},
	models.PlanPro: {
		Monthly: "price_pro_monthly",
		Yearly:  "price_pro_yearly",
	},
	models.PlanEnterprise: {
		Monthly: "price_enterprise_monthly",
		Yearly:  "price_enterprise_yearly",
	},
}

// StripeService handles Stripe payment operations
type StripeService struct {
	secretKey     string
	webhookSecret string
	successURL    string
	cancelURL     string
}

// NewStripeService creates a new Stripe service
func NewStripeService() *StripeService {
	secretKey := os.Getenv("STRIPE_SECRET_KEY")
	if secretKey != "" {
		stripe.Key = secretKey
	}

	return &StripeService{
		secretKey:     secretKey,
		webhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		successURL:    os.Getenv("STRIPE_SUCCESS_URL"),
		cancelURL:     os.Getenv("STRIPE_CANCEL_URL"),
	}
}

// IsConfigured checks if Stripe is properly configured
func (s *StripeService) IsConfigured() bool {
	return s.secretKey != ""
}

// CreateCustomer creates a Stripe customer for a user
func (s *StripeService) CreateCustomer(email, name string) (*stripe.Customer, error) {
	if !s.IsConfigured() {
		return nil, ErrStripeNotConfigured
	}

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
	}

	return customer.New(params)
}

// CreateCheckoutSession creates a Stripe checkout session for subscription
func (s *StripeService) CreateCheckoutSession(customerID, plan, billingPeriod string) (*stripe.CheckoutSession, error) {
	if !s.IsConfigured() {
		return nil, ErrStripeNotConfigured
	}

	priceInfo, ok := PriceIDs[plan]
	if !ok {
		return nil, ErrInvalidPlan
	}

	priceID := priceInfo.Monthly
	if billingPeriod == "yearly" {
		priceID = priceInfo.Yearly
	}

	successURL := s.successURL
	if successURL == "" {
		successURL = "http://localhost:3000/dashboard?success=true"
	}
	cancelURL := s.cancelURL
	if cancelURL == "" {
		cancelURL = "http://localhost:3000/pricing?canceled=true"
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(successURL + "&session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(cancelURL),
	}

	return session.New(params)
}

// CreatePortalSession creates a customer portal session for managing subscription
func (s *StripeService) CreatePortalSession(customerID, returnURL string) (string, error) {
	if !s.IsConfigured() {
		return "", ErrStripeNotConfigured
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	}

	// Use billing portal session
	sess, err := session.New(&stripe.CheckoutSessionParams{
		Customer:   stripe.String(customerID),
		Mode:       stripe.String("setup"),
		SuccessURL: stripe.String(returnURL),
		CancelURL:  stripe.String(returnURL),
	})
	if err != nil {
		return "", err
	}
	_ = params // Placeholder - would use billing portal API

	return sess.URL, nil
}

// GetSubscription retrieves the active subscription for a customer
func (s *StripeService) GetSubscription(customerID string) (*stripe.Subscription, error) {
	if !s.IsConfigured() {
		return nil, ErrStripeNotConfigured
	}

	params := &stripe.SubscriptionListParams{}
	params.Customer = stripe.String(customerID)
	params.Status = stripe.String(string(stripe.SubscriptionStatusActive))
	params.Limit = stripe.Int64(1)

	iter := subscription.List(params)
	if iter.Next() {
		return iter.Subscription(), nil
	}

	return nil, nil
}

// CancelSubscription cancels a subscription at period end
func (s *StripeService) CancelSubscription(subscriptionID string) (*stripe.Subscription, error) {
	if !s.IsConfigured() {
		return nil, ErrStripeNotConfigured
	}

	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	}

	return subscription.Update(subscriptionID, params)
}

// VerifyWebhook verifies and parses a Stripe webhook
func (s *StripeService) VerifyWebhook(payload []byte, signature string) (*stripe.Event, error) {
	if s.webhookSecret == "" {
		return nil, errors.New("webhook secret not configured")
	}

	event, err := webhook.ConstructEvent(payload, signature, s.webhookSecret)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// WebhookEvent types we care about
const (
	EventCheckoutComplete        = "checkout.session.completed"
	EventSubscriptionUpdated     = "customer.subscription.updated"
	EventSubscriptionDeleted     = "customer.subscription.deleted"
	EventInvoicePaymentFailed    = "invoice.payment_failed"
	EventInvoicePaymentSucceeded = "invoice.payment_succeeded"
)

// GetPlanFromPriceID maps a Stripe price ID to our plan name
func GetPlanFromPriceID(priceID string) string {
	for plan, prices := range PriceIDs {
		if prices.Monthly == priceID || prices.Yearly == priceID {
			return plan
		}
	}
	return models.PlanFree
}
