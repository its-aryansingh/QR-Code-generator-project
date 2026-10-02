package ingest

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/scan"
)

func msgFor(t *testing.T, mut func(*scan.ScanEvent)) redis.XMessage {
	t.Helper()
	ver := uuid.NewString()
	lang := "hi"
	ev := scan.ScanEvent{
		ID: uuid.NewString(), Timestamp: time.Now().UTC(), WorkspaceID: uuid.NewString(), QRCodeID: uuid.NewString(),
		VersionID: &ver, DomainID: uuid.NewString(), Outcome: "redirect", Method: "GET",
		VisitorHash: base64.StdEncoding.EncodeToString(make([]byte, 16)),
		UserAgent:   "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 Version/17.4 Mobile/15E148 Safari/604.1",
		Geo:         scan.GeoFacts{Country: "in", Region: "UP", City: "Noida"}, Language: &lang,
	}
	if mut != nil {
		mut(&ev)
	}
	b, _ := json.Marshal(ev)
	return redis.XMessage{ID: "1-0", Values: map[string]any{"e": string(b)}}
}

func TestDecode(t *testing.T) {
	c := NewConsumer(nil, nil, Config{Consumer: "t"}, nil)
	now := time.Now().UTC()

	r, reason := c.decode(msgFor(t, nil), now)
	if reason != "" {
		t.Fatalf("valid event rejected: %s", reason)
	}
	if !r.Counted() || r.IsBot || *r.DeviceType != "mobile" || *r.Country != "IN" || *r.City != "Noida" || r.VersionID == nil {
		t.Fatalf("enrichment: %+v", r)
	}

	head, _ := c.decode(msgFor(t, func(e *scan.ScanEvent) { e.Method = "HEAD" }), now)
	if !head.IsBot || *head.BotReason != "head_request" || head.Counted() {
		t.Fatalf("HEAD must be a bot hit: %+v", head)
	}
	bot, _ := c.decode(msgFor(t, func(e *scan.ScanEvent) { e.UserAgent = "WhatsApp/2.23.20.0 A" }), now)
	if !bot.IsBot {
		t.Fatal("link-preview bot not detected")
	}
	noVer, reason := c.decode(msgFor(t, func(e *scan.ScanEvent) { e.VersionID = nil; e.Outcome = "not_started" }), now)
	if reason != "" || noVer.VersionID != nil || noVer.Counted() {
		t.Fatalf("no-version event: %s %+v", reason, noVer)
	}

	bad := map[string]func(*scan.ScanEvent){
		"invalid_id":              func(e *scan.ScanEvent) { e.ID = "nope" },
		"invalid_outcome":         func(e *scan.ScanEvent) { e.Outcome = "hacked" },
		"timestamp_out_of_window": func(e *scan.ScanEvent) { e.Timestamp = now.Add(-8 * 24 * time.Hour) },
		"invalid_visitor_hash":    func(e *scan.ScanEvent) { e.VisitorHash = "abc" },
	}
	for want, mut := range bad {
		if _, got := c.decode(msgFor(t, mut), now); got != want {
			t.Fatalf("want %s got %q", want, got)
		}
	}
	if _, got := c.decode(redis.XMessage{ID: "1-0", Values: map[string]any{"e": "{"}}, now); got != "invalid_json" {
		t.Fatalf("bad json: %s", got)
	}
}
