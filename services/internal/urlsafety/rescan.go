package urlsafety

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RescanItem represents a QR code target scheduled for safety evaluation.
type RescanItem struct {
	CodeID         uuid.UUID
	WorkspaceID    uuid.UUID
	ShortCode      string
	DestinationURL string
}

// RescanResult describes the evaluation result for a RescanItem.
type RescanResult struct {
	CodeID         uuid.UUID
	WorkspaceID    uuid.UUID
	ShortCode      string
	DestinationURL string
	Verdict        Verdict
	IsFlagged      bool
	Reason         string
}

// RescanHandler callback invoked when an unsafe URL is discovered.
type RescanHandler func(ctx context.Context, res RescanResult) error

// Rescanner evaluates batches of QR code destination URLs.
type Rescanner struct {
	safetyClient SafetyClient
	onFlagged    RescanHandler
}

// NewRescanner creates a new background safety rescanner.
func NewRescanner(client SafetyClient, onFlagged RescanHandler) *Rescanner {
	return &Rescanner{
		safetyClient: client,
		onFlagged:    onFlagged,
	}
}

// RescanBatch processes a batch of items against the URL reputation client.
func (r *Rescanner) RescanBatch(ctx context.Context, items []RescanItem) ([]RescanResult, error) {
	results := make([]RescanResult, 0, len(items))

	for _, item := range items {
		if item.DestinationURL == "" {
			continue
		}

		verdict, err := r.safetyClient.Check(ctx, item.DestinationURL)
		if err != nil {
			// Do not block processing batch on transient provider error
			results = append(results, RescanResult{
				CodeID:         item.CodeID,
				WorkspaceID:    item.WorkspaceID,
				ShortCode:      item.ShortCode,
				DestinationURL: item.DestinationURL,
				Verdict:        VerdictPending,
				IsFlagged:      false,
				Reason:         fmt.Sprintf("check error: %v", err),
			})
			continue
		}

		flagged := verdict == VerdictUnsafe
		res := RescanResult{
			CodeID:         item.CodeID,
			WorkspaceID:    item.WorkspaceID,
			ShortCode:      item.ShortCode,
			DestinationURL: item.DestinationURL,
			Verdict:        verdict,
			IsFlagged:      flagged,
		}

		if flagged {
			res.Reason = "destination URL detected as malware, phishing, or harmful content by automated safety rescan"
			if r.onFlagged != nil {
				_ = r.onFlagged(ctx, res)
			}
		}

		results = append(results, res)
	}

	return results, nil
}

// NextRescanDelay returns an interval to avoid hammering the reputation API.
func NextRescanDelay(batchSize int) time.Duration {
	if batchSize <= 0 {
		return 10 * time.Millisecond
	}
	return time.Duration(100/batchSize) * time.Millisecond
}
