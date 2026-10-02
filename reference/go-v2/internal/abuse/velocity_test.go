package abuse

import (
	"context"
	"testing"
	"time"
)

func TestMemoryVelocityLimiter(t *testing.T) {
	ctx := context.Background()
	limiter := NewMemoryVelocityLimiter()

	key := "test:user:123"
	limit := 3
	window := 200 * time.Millisecond

	// First 3 should succeed
	for i := 1; i <= limit; i++ {
		allowed, current, _, err := limiter.Allow(ctx, key, limit, window)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Fatalf("expected call %d to be allowed", i)
		}
		if current != i {
			t.Fatalf("expected count %d, got %d", i, current)
		}
	}

	// 4th call within window should be rejected
	allowed, current, retryAfter, err := limiter.Allow(ctx, key, limit, window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("expected call beyond limit to be blocked")
	}
	if current != limit {
		t.Fatalf("expected current count %d, got %d", limit, current)
	}
	if retryAfter <= 0 {
		t.Fatalf("expected positive retryAfter, got %v", retryAfter)
	}

	// Wait for window to expire
	time.Sleep(window + 20*time.Millisecond)

	// Call after window should be allowed again
	allowed, current, _, err = limiter.Allow(ctx, key, limit, window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatalf("expected call after window to be allowed")
	}
	if current != 1 {
		t.Fatalf("expected reset count 1, got %d", current)
	}
}
