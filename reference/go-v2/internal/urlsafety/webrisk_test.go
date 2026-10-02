package urlsafety

import (
	"context"
	"testing"
)

func TestWebRiskClientFallback(t *testing.T) {
	client := NewWebRiskClient("") // empty API key triggers fallback
	ctx := context.Background()

	verdict, err := client.Check(ctx, "https://google.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != VerdictSafe {
		t.Errorf("expected VerdictSafe, got %v", verdict)
	}
}
