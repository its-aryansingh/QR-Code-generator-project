package redirect

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// SaltStore hands out the daily visitor-hash salt: 32 random bytes per UTC day, agreed
// across instances through Redis (SET NX, 48 h TTL) and never written anywhere else.
// After 48 hours the salt is gone, so visitor hashes can no longer be linked to an IP.
type SaltStore struct {
	rdb   *redis.Client
	mu    sync.Mutex
	cache map[string][]byte
}

func NewSaltStore(rdb *redis.Client) *SaltStore {
	return &SaltStore{rdb: rdb, cache: map[string][]byte{}}
}

// For returns the salt of t's UTC day.
func (s *SaltStore) For(ctx context.Context, t time.Time) []byte {
	day := t.UTC().Format("2006-01-02")
	s.mu.Lock()
	if b, ok := s.cache[day]; ok {
		s.mu.Unlock()
		return b
	}
	s.mu.Unlock()

	fresh := make([]byte, 32)
	_, _ = rand.Read(fresh)
	salt := fresh
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		key := "salt:" + day
		if err := s.rdb.SetNX(ctx, key, base64.StdEncoding.EncodeToString(fresh), 48*time.Hour).Err(); err != nil {
			slog.Warn("salt: redis unavailable, using an instance-local salt", "error", err)
		} else if v, err := s.rdb.Get(ctx, key).Result(); err == nil {
			if b, err := base64.StdEncoding.DecodeString(v); err == nil && len(b) == 32 {
				salt = b
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.cache[day]; ok {
		return b
	}
	// Keep only today and yesterday.
	for d := range s.cache {
		if d < t.UTC().Add(-24*time.Hour).Format("2006-01-02") {
			delete(s.cache, d)
		}
	}
	s.cache[day] = salt
	return salt
}
