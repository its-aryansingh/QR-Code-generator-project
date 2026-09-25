package realtime

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type MinuteCount struct {
	Minute string `json:"minute"` // YYYYMMDDHHMM
	Count  int64  `json:"count"`
}

type RealtimeService struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *RealtimeService {
	return &RealtimeService{rdb: rdb}
}

func formatMinute(t time.Time) string {
	return t.UTC().Format("200601021504")
}

// RecordScan increments per-minute counters for workspace and specific QR with a 2-hour TTL.
func (s *RealtimeService) RecordScan(ctx context.Context, wsID, qrID string, t time.Time) error {
	minute := formatMinute(t)
	wsKey := fmt.Sprintf("rt:%s:%s", wsID, minute)
	qrKey := fmt.Sprintf("rt:%s:%s:%s", wsID, qrID, minute)

	pipe := s.rdb.Pipeline()
	pipe.Incr(ctx, wsKey)
	pipe.Expire(ctx, wsKey, 2*time.Hour)

	if qrID != "" {
		pipe.Incr(ctx, qrKey)
		pipe.Expire(ctx, qrKey, 2*time.Hour)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetRecentCounts retrieves per-minute counts for a workspace over the last N minutes.
func (s *RealtimeService) GetRecentCounts(ctx context.Context, wsID string, minutes int) ([]MinuteCount, int64, error) {
	if minutes <= 0 {
		minutes = 15
	}
	if minutes > 60 {
		minutes = 60
	}

	now := time.Now().UTC()
	keys := make([]string, minutes)
	minStrs := make([]string, minutes)

	for i := 0; i < minutes; i++ {
		t := now.Add(-time.Duration(i) * time.Minute)
		minStr := formatMinute(t)
		minStrs[i] = minStr
		keys[i] = fmt.Sprintf("rt:%s:%s", wsID, minStr)
	}

	vals, err := s.rdb.MGet(ctx, keys...).Result()
	if err != nil && err != redis.Nil {
		return nil, 0, err
	}

	result := make([]MinuteCount, minutes)
	var total int64 = 0

	for i := 0; i < minutes; i++ {
		var cnt int64 = 0
		if i < len(vals) && vals[i] != nil {
			if sVal, ok := vals[i].(string); ok {
				cnt, _ = strconv.ParseInt(sVal, 10, 64)
			}
		}
		result[minutes-1-i] = MinuteCount{
			Minute: minStrs[i],
			Count:  cnt,
		}
		total += cnt
	}

	return result, total, nil
}
