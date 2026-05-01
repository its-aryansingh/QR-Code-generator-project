package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a token bucket rate limiter per key
type RateLimiter struct {
	mu       sync.RWMutex
	buckets  map[string]*bucket
	rate     int           // tokens per interval
	interval time.Duration // refill interval
	burst    int           // max tokens
}

type bucket struct {
	tokens    int
	lastCheck time.Time
}

// NewRateLimiter creates a new rate limiter
// rate: requests per interval
// interval: time window
// burst: max burst size
func NewRateLimiter(rate int, interval time.Duration, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		interval: interval,
		burst:    burst,
	}

	// Cleanup stale buckets every 5 minutes
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			rl.cleanup()
		}
	}()

	return rl
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, b := range rl.buckets {
		if now.Sub(b.lastCheck) > 10*time.Minute {
			delete(rl.buckets, key)
		}
	}
}

// Allow checks if the request is within rate limits
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[key]
	if !exists {
		rl.buckets[key] = &bucket{
			tokens:    rl.burst - 1,
			lastCheck: now,
		}
		return true
	}

	// Add tokens based on elapsed time
	elapsed := now.Sub(b.lastCheck)
	tokensToAdd := int(elapsed/rl.interval) * rl.rate
	b.tokens += tokensToAdd
	if b.tokens > rl.burst {
		b.tokens = rl.burst
	}
	b.lastCheck = now

	if b.tokens <= 0 {
		return false
	}

	b.tokens--
	return true
}

// APIRateLimit middleware applies rate limiting to API endpoints
// Uses API key or user ID as the key
func APIRateLimit(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use API key or IP as the rate limit key
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.ClientIP()
		}

		if !limiter.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success":     false,
				"error":       "Rate limit exceeded. Please retry after a moment.",
				"retry_after": 60,
			})
			return
		}

		c.Next()
	}
}

// TierBasedRateLimit creates rate limiters for different plan tiers
type TierBasedRateLimit struct {
	Free       *RateLimiter
	Pro        *RateLimiter
	Enterprise *RateLimiter
}

// NewTierBasedRateLimit creates rate limiters for each tier
func NewTierBasedRateLimit() *TierBasedRateLimit {
	return &TierBasedRateLimit{
		Free:       NewRateLimiter(10, time.Minute, 10),   // 10 req/min
		Pro:        NewRateLimiter(60, time.Minute, 60),   // 60 req/min (1/sec)
		Enterprise: NewRateLimiter(300, time.Minute, 300), // 300 req/min (5/sec)
	}
}
