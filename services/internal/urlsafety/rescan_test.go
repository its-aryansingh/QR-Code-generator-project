package urlsafety

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestRescanBatch(t *testing.T) {
	ctx := context.Background()

	client := NewFakeSafetyClient()
	client.UnsafeURLs["https://malicious-phishing.com/login"] = true

	var flaggedItems []RescanResult
	var mu sync.Mutex

	handler := func(ctx context.Context, res RescanResult) error {
		mu.Lock()
		defer mu.Unlock()
		flaggedItems = append(flaggedItems, res)
		return nil
	}

	rescanner := NewRescanner(client, handler)

	items := []RescanItem{
		{
			CodeID:         uuid.New(),
			WorkspaceID:    uuid.New(),
			ShortCode:      "SAFE01",
			DestinationURL: "https://mycompany.com",
		},
		{
			CodeID:         uuid.New(),
			WorkspaceID:    uuid.New(),
			ShortCode:      "BAD001",
			DestinationURL: "https://malicious-phishing.com/login",
		},
		{
			CodeID:         uuid.New(),
			WorkspaceID:    uuid.New(),
			ShortCode:      "SAFE02",
			DestinationURL: "https://google.com",
		},
	}

	results, err := rescanner.RescanBatch(ctx, items)
	if err != nil {
		t.Fatalf("RescanBatch failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// First should be safe
	if results[0].IsFlagged || results[0].Verdict != VerdictSafe {
		t.Fatalf("expected item 0 to be safe, got %v", results[0])
	}

	// Second should be flagged
	if !results[1].IsFlagged || results[1].Verdict != VerdictUnsafe {
		t.Fatalf("expected item 1 to be flagged as unsafe, got %v", results[1])
	}

	// Third should be safe
	if results[2].IsFlagged || results[2].Verdict != VerdictSafe {
		t.Fatalf("expected item 2 to be safe, got %v", results[2])
	}

	// Check onFlagged handler invocation
	mu.Lock()
	defer mu.Unlock()
	if len(flaggedItems) != 1 {
		t.Fatalf("expected exactly 1 flagged item in callback, got %d", len(flaggedItems))
	}
	if flaggedItems[0].ShortCode != "BAD001" {
		t.Fatalf("expected BAD001 to be flagged, got %s", flaggedItems[0].ShortCode)
	}
}
