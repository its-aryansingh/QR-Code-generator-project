package analytics

import (
	"context"
	"testing"
	"time"
)

func TestAnalyticsService(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	now := time.Now().UTC()
	filters := FilterParams{
		WorkspaceID: "ws-123",
		From:        now.Add(-7 * 24 * time.Hour),
		To:          now,
	}

	summary, err := svc.GetSummary(ctx, filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary == nil {
		t.Fatalf("expected non-nil summary")
	}

	ts, err := svc.GetTimeSeries(ctx, filters, "day")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ts) == 0 {
		t.Fatalf("expected non-empty timeseries")
	}

	hm, err := svc.GetHeatmap(ctx, filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hm) != 7*24 {
		t.Fatalf("expected 168 heatmap cells, got %d", len(hm))
	}
}
