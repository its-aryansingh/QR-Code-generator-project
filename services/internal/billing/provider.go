package billing

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidPlan     = errors.New("invalid plan code")
	ErrCustomerNotFound = errors.New("billing customer not found")
)

type SubscriptionEvent struct {
	WorkspaceID     string
	CustomerID      string
	SubscriptionID  string
	Plan            string // "free", "pro", "business", "enterprise"
	Status          string // "active", "canceled", "past_due"
	CurrentPeriodEnd time.Time
}

type Provider interface {
	CreateCheckoutSession(ctx context.Context, wsID, plan, cadence, currency, returnURL string) (string, error)
	CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error)
	HandleWebhook(ctx context.Context, payload []byte, sigHeader string) (*SubscriptionEvent, error)
}

type Service struct {
	pool     *pgxpool.Pool
	provider Provider
}

func NewService(pool *pgxpool.Pool, provider Provider) *Service {
	return &Service{
		pool:     pool,
		provider: provider,
	}
}

// HandleDowngrade enforces plan limits upon downgrade:
// Codes beyond the new plan's active limit are marked is_read_only in newest-first order, never deactivated!
func (s *Service) HandleDowngrade(ctx context.Context, wsID string, allowedActiveCodes int) error {
	if s.pool == nil {
		return nil
	}

	// First, reset all read_only flags for this workspace
	_, err := s.pool.Exec(ctx, `UPDATE qr_codes SET is_read_only = false WHERE workspace_id = $1`, wsID)
	if err != nil {
		return err
	}

	if allowedActiveCodes <= 0 {
		return nil
	}

	// Mark codes exceeding the allowed limit as is_read_only starting from the oldest/newest limit
	query := `
		WITH excess AS (
			SELECT id
			FROM qr_codes
			WHERE workspace_id = $1 AND status = 'active'
			ORDER BY created_at DESC
			OFFSET $2
		)
		UPDATE qr_codes
		SET is_read_only = true
		WHERE id IN (SELECT id FROM excess)
	`
	_, err = s.pool.Exec(ctx, query, wsID, allowedActiveCodes)
	return err
}
