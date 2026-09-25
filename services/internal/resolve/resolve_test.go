package resolve_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/its-aryansingh/qrit/services/internal/resolve"
)

func TestEvaluateState(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	fallback := "https://example.com/fallback"
	dest := "https://example.com/destination"

	baseLink := func() *resolve.ResolvedLink {
		return &resolve.ResolvedLink{
			QRCodeID:    uuid.New(),
			WorkspaceID: uuid.New(),
			Status:      "active",
			Safety:      "safe",
			FallbackURL: &fallback,
			Version: &resolve.ResolvedVersion{
				URL: dest,
			},
		}
	}

	// 1. Blocked
	l := baseLink()
	l.Status = "blocked"
	outcome, target := resolve.EvaluateState(l, now)
	if outcome != resolve.OutcomeBlocked || target != "" {
		t.Errorf("blocked link expected (OutcomeBlocked, \"\"), got (%s, %s)", outcome, target)
	}

	// 2. Paused with fallback
	l = baseLink()
	l.Status = "paused"
	outcome, target = resolve.EvaluateState(l, now)
	if outcome != resolve.OutcomePaused || target != fallback {
		t.Errorf("paused link expected fallback, got (%s, %s)", outcome, target)
	}

	// 3. Not started
	l = baseLink()
	future := now.Add(2 * time.Hour)
	l.StartsAt = &future
	outcome, target = resolve.EvaluateState(l, now)
	if outcome != resolve.OutcomeNotStarted || target != fallback {
		t.Errorf("not started link expected fallback, got (%s, %s)", outcome, target)
	}

	// 4. Expired
	l = baseLink()
	past := now.Add(-2 * time.Hour)
	l.ExpiresAt = &past
	outcome, target = resolve.EvaluateState(l, now)
	if outcome != resolve.OutcomeExpired || target != fallback {
		t.Errorf("expired link expected fallback, got (%s, %s)", outcome, target)
	}

	// 5. Scan limit reached
	l = baseLink()
	limit := int64(100)
	l.ScanLimit = &limit
	l.TotalScans = 100
	outcome, target = resolve.EvaluateState(l, now)
	if outcome != resolve.OutcomeLimitReached || target != fallback {
		t.Errorf("limit reached link expected fallback, got (%s, %s)", outcome, target)
	}

	// 6. Password required
	l = baseLink()
	l.HasPassword = true
	outcome, target = resolve.EvaluateState(l, now)
	if outcome != resolve.OutcomePasswordRequired || target != "" {
		t.Errorf("password required link expected password prompt, got (%s, %s)", outcome, target)
	}

	// 7. Active success
	l = baseLink()
	outcome, target = resolve.EvaluateState(l, now)
	if outcome != resolve.OutcomeActive || target != dest {
		t.Errorf("active link expected (%s, %s), got (%s, %s)", resolve.OutcomeActive, dest, outcome, target)
	}
}

func TestResolverLRUAndFetch(t *testing.T) {
	domainID := uuid.New()
	code := "7K9M2X1"
	fetchCount := 0

	r, err := resolve.NewResolver(100, nil, func(ctx context.Context, d uuid.UUID, c string) (*resolve.ResolvedLink, error) {
		fetchCount++
		return &resolve.ResolvedLink{
			QRCodeID: uuid.New(),
			DomainID: d,
			Status:   "active",
		}, nil
	})
	if err != nil {
		t.Fatalf("NewResolver failed: %v", err)
	}

	// First call -> fetches from callback
	link1, err := r.Resolve(context.Background(), domainID, code)
	if err != nil || link1 == nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if fetchCount != 1 {
		t.Errorf("fetchCount = %d; want 1", fetchCount)
	}

	// Second call -> should hit in-process LRU without fetching again
	link2, err := r.Resolve(context.Background(), domainID, code)
	if err != nil || link2 == nil {
		t.Fatalf("Resolve 2 failed: %v", err)
	}
	if fetchCount != 1 {
		t.Errorf("fetchCount = %d after LRU hit; want 1", fetchCount)
	}

	// Invalidate -> next call fetches again
	r.Invalidate(context.Background(), domainID, code)
	_, _ = r.Resolve(context.Background(), domainID, code)
	if fetchCount != 2 {
		t.Errorf("fetchCount = %d after invalidate; want 2", fetchCount)
	}
}
