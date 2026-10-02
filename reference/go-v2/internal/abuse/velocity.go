package abuse

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// VelocityLimiter tracks and enforces creation velocity to mitigate spam.
type VelocityLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, current int, retryAfter time.Duration, err error)
}

// MemoryVelocityLimiter is an in-memory sliding window limiter.
type MemoryVelocityLimiter struct {
	mu      sync.Mutex
	buckets map[string][]time.Time
}

func NewMemoryVelocityLimiter() *MemoryVelocityLimiter {
	return &MemoryVelocityLimiter{
		buckets: make(map[string][]time.Time),
	}
}

func (m *MemoryVelocityLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	timestamps := m.buckets[key]
	valid := make([]time.Time, 0, len(timestamps))
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= limit {
		oldest := valid[0]
		retryAfter := oldest.Add(window).Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
		m.buckets[key] = valid
		return false, len(valid), retryAfter, nil
	}

	valid = append(valid, now)
	m.buckets[key] = valid
	return true, len(valid), 0, nil
}

// RedisVelocityLimiter uses Redis sorted sets for distributed velocity enforcement.
type RedisVelocityLimiter struct {
	client *redis.Client
}

func NewRedisVelocityLimiter(client *redis.Client) *RedisVelocityLimiter {
	return &RedisVelocityLimiter{client: client}
}

func (r *RedisVelocityLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	if r.client == nil {
		return true, 1, 0, nil
	}

	now := time.Now()
	nowUnixMs := now.UnixMilli()
	cutoffUnixMs := now.Add(-window).UnixMilli()
	redisKey := fmt.Sprintf("velocity:%s", key)

	pipe := r.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "-inf", fmt.Sprintf("%d", cutoffUnixMs))
	countCmd := pipe.ZCard(ctx, redisKey)
	pipe.Expire(ctx, redisKey, window*2)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, 0, 0, err
	}

	current := int(countCmd.Val())
	if current >= limit {
		return false, current, window, nil
	}

	member := fmt.Sprintf("%d-%d", nowUnixMs, time.Now().UnixNano()%1000)
	err = r.client.ZAdd(ctx, redisKey, redis.Z{Score: float64(nowUnixMs), Member: member}).Err()
	if err != nil {
		return true, current, 0, err
	}

	return true, current + 1, 0, nil
}
