package version

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSchedulerOffline(t *testing.T) {
	scheduler := NewScheduler(nil, nil)
	ctx := context.Background()

	qrID := uuid.New()
	verID := uuid.New()

	err := scheduler.ActivateScheduledVersion(ctx, qrID, verID, "testcode")
	if err != nil {
		t.Fatalf("unexpected error in offline activation: %v", err)
	}

	count, err := scheduler.CheckDueVersions(ctx, time.Now())
	if err != nil {
		t.Fatalf("unexpected error checking due versions: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 count in offline mode, got %d", count)
	}
}
