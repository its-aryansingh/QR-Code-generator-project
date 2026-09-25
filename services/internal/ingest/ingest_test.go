package ingest

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/its-aryansingh/qrit/services/internal/scan"
	"github.com/redis/go-redis/v9"
)

func TestIngestBatchProcessing(t *testing.T) {
	ev := scan.ScanEvent{
		ID:          "018e1234-5678-7000-8000-000000000001",
		Timestamp:   time.Now().UTC(),
		WorkspaceID: "ws-1",
		QRCodeID:    "qr-1",
		UserAgent:   "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X)",
		Outcome:     "redirect",
		Method:      "GET",
		VisitorHash: "vh-test-1",
	}

	bytes, _ := json.Marshal(ev)
	msg := redis.XMessage{
		ID: "1000-0",
		Values: map[string]interface{}{
			"data": string(bytes),
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	// In memory / dry-run test
	if msg.Values["data"] != string(bytes) {
		t.Fatalf("expected serialized message")
	}

	parsed := scan.ParseUserAgent(ev.UserAgent)
	if parsed.OS != "iOS" || parsed.DeviceType != "mobile" {
		t.Fatalf("expected iOS mobile, got %+v", parsed)
	}

	verdict := scan.ClassifyBot(ev.Method, ev.UserAgent, false)
	if verdict.IsBot {
		t.Fatalf("expected non-bot for iPhone Safari")
	}
	_ = logger
}
