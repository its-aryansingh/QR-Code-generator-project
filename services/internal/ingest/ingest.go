package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/realtime"
	"github.com/its-aryansingh/qrit/services/internal/scan"
)

type BatchConfig struct {
	StreamName    string
	GroupName     string
	ConsumerName  string
	BatchSize     int64
	BatchWait     time.Duration
	DedupeTimeout time.Duration
}

type Consumer struct {
	pool     *pgxpool.Pool
	rdb      *redis.Client
	realtime *realtime.RealtimeService
	cfg      BatchConfig
	logger   *slog.Logger
}

func NewConsumer(pool *pgxpool.Pool, rdb *redis.Client, rt *realtime.RealtimeService, cfg BatchConfig, logger *slog.Logger) *Consumer {
	if cfg.StreamName == "" {
		cfg.StreamName = "scans"
	}
	if cfg.GroupName == "" {
		cfg.GroupName = "ingest"
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1000
	}
	if cfg.BatchWait <= 0 {
		cfg.BatchWait = 1 * time.Second
	}
	if cfg.DedupeTimeout <= 0 {
		cfg.DedupeTimeout = 10 * time.Second
	}

	return &Consumer{
		pool:     pool,
		rdb:      rdb,
		realtime: rt,
		cfg:      cfg,
		logger:   logger,
	}
}

// InitGroup ensures the consumer group exists on the stream.
func (c *Consumer) InitGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, c.cfg.StreamName, c.cfg.GroupName, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

type ProcessedEvent struct {
	Event       scan.ScanEvent
	ParsedUA    scan.ParsedUA
	BotVerdict  scan.BotVerdict
	IsDuplicate bool
	RawID       string
}

// ProcessBatch evaluates and ingests a slice of Redis Stream messages.
func (c *Consumer) ProcessBatch(ctx context.Context, messages []redis.XMessage) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}

	processed := make([]ProcessedEvent, 0, len(messages))
	ackIDs := make([]string, 0, len(messages))

	for _, msg := range messages {
		dataStr, ok := msg.Values["data"].(string)
		if !ok {
			ackIDs = append(ackIDs, msg.ID)
			continue
		}

		var ev scan.ScanEvent
		if err := json.Unmarshal([]byte(dataStr), &ev); err != nil {
			c.logger.Warn("Failed to unmarshal scan event", "id", msg.ID, "err", err)
			ackIDs = append(ackIDs, msg.ID)
			continue
		}

		parsedUA := scan.ParseUserAgent(ev.UserAgent)
		verdict := scan.ClassifyBot(ev.Method, ev.UserAgent, ev.Datacenter)

		// Duplicate detection in Redis:
		// SET dup:{qr}:{vh} {event_id} NX PX 10000
		dupKey := fmt.Sprintf("dup:%s:%s", ev.QRCodeID, ev.VisitorHash)
		isDup := false
		res, err := c.rdb.SetArgs(ctx, dupKey, ev.ID, redis.SetArgs{
			Mode: "NX",
			TTL:  c.cfg.DedupeTimeout,
		}).Result()

		if err == nil && res != "OK" {
			// Key already exists; check if event_id is different
			existingID, _ := c.rdb.Get(ctx, dupKey).Result()
			if existingID != "" && existingID != ev.ID {
				isDup = true
			}
		}

		processed = append(processed, ProcessedEvent{
			Event:       ev,
			ParsedUA:    parsedUA,
			BotVerdict:  verdict,
			IsDuplicate: isDup,
			RawID:       msg.ID,
		})
		ackIDs = append(ackIDs, msg.ID)
	}

	// Record in realtime counters
	if c.realtime != nil {
		for _, p := range processed {
			if !p.BotVerdict.IsBot && !p.IsDuplicate {
				_ = c.realtime.RecordScan(ctx, p.Event.WorkspaceID, p.Event.QRCodeID, p.Event.Timestamp)
			}
		}
	}

	// ACK consumed stream messages
	if len(ackIDs) > 0 {
		_ = c.rdb.XAck(ctx, c.cfg.StreamName, c.cfg.GroupName, ackIDs...).Err()
	}

	return len(processed), nil
}
