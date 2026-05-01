package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"

	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/internal/repository"
	"github.com/qrapp/backend/internal/services"
	"github.com/qrapp/backend/pkg/utils"
)

// PaymentHandler handles payment/subscription endpoints
type PaymentHandler struct {
	stripeService *services.StripeService
	userRepo      repository.UserRepository
}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler(stripeService *services.StripeService, userRepo repository.UserRepository) *PaymentHandler {
	return &PaymentHandler{
		stripeService: stripeService,
		userRepo:      userRepo,
	}
}

// CreateCheckoutRequest for creating checkout session
type CreateCheckoutRequest struct {
	Plan          string `json:"plan" binding:"required"`
	BillingPeriod string `json:"billing_period"` // "monthly" or "yearly"
}

// CreateCheckout creates a Stripe checkout session
// POST /api/v1/payments/checkout
func (h *PaymentHandler) CreateCheckout(c *gin.Context) {
	if !h.stripeService.IsConfigured() {
		utils.InternalError(c, "Payment system not configured")
		return
	}

	var req CreateCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	// Get user
	userIDStr, _ := c.Get("userID")
	userID, _ := uuid.Parse(userIDStr.(string))

	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		utils.NotFound(c, "User not found")
		return
	}

	// Create Stripe customer if not exists
	if user.StripeCustomerID == "" {
		customer, err := h.stripeService.CreateCustomer(user.Email, user.Name)
		if err != nil {
			utils.InternalError(c, "Failed to create customer")
			return
		}
		user.StripeCustomerID = customer.ID
		h.userRepo.Update(user)
	}

	// Default billing period
	if req.BillingPeriod == "" {
		req.BillingPeriod = "monthly"
	}

	// Create checkout session
	session, err := h.stripeService.CreateCheckoutSession(user.StripeCustomerID, req.Plan, req.BillingPeriod)
	if err != nil {
		if err == services.ErrInvalidPlan {
			utils.BadRequest(c, "Invalid plan")
			return
		}
		utils.InternalError(c, "Failed to create checkout session")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"checkout_url": session.URL,
		"session_id":   session.ID,
	})
}

// GetSubscription retrieves current subscription status
// GET /api/v1/payments/subscription
func (h *PaymentHandler) GetSubscription(c *gin.Context) {
	userIDStr, _ := c.Get("userID")
	userID, _ := uuid.Parse(userIDStr.(string))

	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		utils.NotFound(c, "User not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"plan":                 user.Plan,
			"subscription_status":  user.SubscriptionStatus,
			"subscription_ends_at": user.SubscriptionEndsAt,
			"can_use_api":          user.CanUseAPI(),
			"api_calls_limit":      user.GetAPICallsLimit(),
			"dynamic_qr_limit":     user.GetDynamicQRLimit(),
		},
	})
}

// CancelSubscription cancels at period end
// POST /api/v1/payments/cancel
func (h *PaymentHandler) CancelSubscription(c *gin.Context) {
	if !h.stripeService.IsConfigured() {
		utils.InternalError(c, "Payment system not configured")
		return
	}

	userIDStr, _ := c.Get("userID")
	userID, _ := uuid.Parse(userIDStr.(string))

	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		utils.NotFound(c, "User not found")
		return
	}

	if user.StripeSubscriptionID == "" {
		utils.BadRequest(c, "No active subscription")
		return
	}

	_, err = h.stripeService.CancelSubscription(user.StripeSubscriptionID)
	if err != nil {
		utils.InternalError(c, "Failed to cancel subscription")
		return
	}

	user.SubscriptionStatus = "canceling"
	h.userRepo.Update(user)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subscription will be canceled at period end",
	})
}

// Webhook handles Stripe webhooks
// POST /api/v1/payments/webhook
func (h *PaymentHandler) Webhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	signature := c.GetHeader("Stripe-Signature")
	event, err := h.stripeService.VerifyWebhook(payload, signature)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid signature"})
		return
	}

	// Handle different event types
	switch event.Type {
	case services.EventCheckoutComplete:
		h.handleCheckoutComplete(event)
	case services.EventSubscriptionUpdated:
		h.handleSubscriptionUpdated(event)
	case services.EventSubscriptionDeleted:
		h.handleSubscriptionDeleted(event)
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

func (h *PaymentHandler) handleCheckoutComplete(event *stripe.Event) {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		log.Printf("payment: failed to parse checkout.session.completed: %v", err)
		return
	}
	if sess.Customer == nil || sess.Subscription == nil {
		return
	}
	user, err := h.userRepo.FindByStripeCustomerID(sess.Customer.ID)
	if err != nil {
		log.Printf("payment: user not found for customer %s: %v", sess.Customer.ID, err)
		return
	}
	user.StripeSubscriptionID = sess.Subscription.ID
	user.SubscriptionStatus = "active"
	// Determine plan from line items metadata or subscription lookup
	if sess.Metadata != nil {
		if plan, ok := sess.Metadata["plan"]; ok && plan != "" {
			user.Plan = plan
		}
	}
	if err := h.userRepo.Update(user); err != nil {
		log.Printf("payment: failed to update user after checkout: %v", err)
	}
}

func (h *PaymentHandler) handleSubscriptionUpdated(event *stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		log.Printf("payment: failed to parse subscription.updated: %v", err)
		return
	}
	if sub.Customer == nil {
		return
	}
	user, err := h.userRepo.FindByStripeCustomerID(sub.Customer.ID)
	if err != nil {
		log.Printf("payment: user not found for customer %s: %v", sub.Customer.ID, err)
		return
	}

	user.StripeSubscriptionID = sub.ID
	user.SubscriptionStatus = string(sub.Status)

	// Map price ID to plan
	if len(sub.Items.Data) > 0 {
		priceID := sub.Items.Data[0].Price.ID
		user.Plan = services.GetPlanFromPriceID(priceID)
	}

	// Set expiry for cancel-at-period-end subscriptions
	if sub.CancelAtPeriodEnd && sub.CurrentPeriodEnd > 0 {
		t := time.Unix(sub.CurrentPeriodEnd, 0)
		user.SubscriptionEndsAt = &t
	}

	if err := h.userRepo.Update(user); err != nil {
		log.Printf("payment: failed to update user after subscription update: %v", err)
	}
}

func (h *PaymentHandler) handleSubscriptionDeleted(event *stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		log.Printf("payment: failed to parse subscription.deleted: %v", err)
		return
	}
	if sub.Customer == nil {
		return
	}
	user, err := h.userRepo.FindByStripeCustomerID(sub.Customer.ID)
	if err != nil {
		log.Printf("payment: user not found for customer %s: %v", sub.Customer.ID, err)
		return
	}

	user.Plan = models.PlanFree
	user.StripeSubscriptionID = ""
	user.SubscriptionStatus = "canceled"
	user.SubscriptionEndsAt = nil

	if err := h.userRepo.Update(user); err != nil {
		log.Printf("payment: failed to downgrade user after subscription deleted: %v", err)
	}
}

// GetPlans returns available subscription plans
// GET /api/v1/payments/plans
func (h *PaymentHandler) GetPlans(c *gin.Context) {
	plans := []gin.H{
		{
			"id":        models.PlanFree,
			"name":      "Free",
			"price":     0,
			"features":  []string{"10 QR codes/day", "Basic types", "PNG export"},
			"is_active": true,
		},
		{
			"id":            models.PlanStarter,
			"name":          "Starter",
			"price_monthly": 9,
			"price_yearly":  86,
			"features":      []string{"Unlimited static QR", "10 dynamic QR", "All types", "SVG export", "Basic analytics"},
			"is_active":     true,
		},
		{
			"id":            models.PlanPro,
			"name":          "Pro",
			"price_monthly": 29,
			"price_yearly":  278,
			"features":      []string{"Unlimited static QR", "100 dynamic QR", "All types", "PDF export", "Advanced analytics", "API access"},
			"is_active":     true,
			"popular":       true,
		},
		{
			"id":            models.PlanEnterprise,
			"name":          "Enterprise",
			"price_monthly": 0, // Custom
			"features":      []string{"Unlimited everything", "Custom domain", "SSO", "Dedicated support"},
			"is_active":     true,
			"contact":       true,
		},
	}

	utils.Success(c, plans)
}
