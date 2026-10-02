package resolve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

var (
	ErrNotFound = errors.New("short code not found")
)

type Outcome string

const (
	OutcomeActive           Outcome = "active"
	OutcomeBlocked          Outcome = "blocked"
	OutcomePaused           Outcome = "paused"
	OutcomeNotStarted       Outcome = "not_started"
	OutcomeExpired          Outcome = "expired"
	OutcomeLimitReached     Outcome = "limit_reached"
	OutcomePasswordRequired Outcome = "password_required"
)

type ResolvedVersion struct {
	ID    uuid.UUID       `json:"id"`
	No    int             `json:"no"`
	Kind  string          `json:"kind"`
	URL   string          `json:"url"`
	Rules json.RawMessage `json:"rules"`
	UTM   json.RawMessage `json:"utm"`
	// HostedPage is the page JSON when Kind is hosted_page (rendered by the redirect service).
	HostedPage json.RawMessage `json:"hp,omitempty"`
}

type ResolvedLink struct {
	QRCodeID     uuid.UUID        `json:"qr"`
	WorkspaceID  uuid.UUID        `json:"ws"`
	DomainID     uuid.UUID        `json:"dom"`
	CampaignID   *uuid.UUID       `json:"cmp,omitempty"`
	Status       string           `json:"status"`
	Safety       string           `json:"safety"`
	StartsAt     *time.Time       `json:"starts_at,omitempty"`
	ExpiresAt    *time.Time       `json:"expires_at,omitempty"`
	ScanLimit    *int64           `json:"scan_limit,omitempty"`
	TotalScans   int64            `json:"total_scans"`
	HasPassword  bool             `json:"has_password"`
	PasswordHash *string          `json:"pw_hash,omitempty"`
	FallbackURL  *string          `json:"fallback_url,omitempty"`
	Timezone     string           `json:"tz"`
	Version      *ResolvedVersion `json:"ver,omitempty"`
	NextChangeAt *time.Time       `json:"next_change_at,omitempty"`
	CachedAt     time.Time        `json:"cached_at"`
}

// EvaluateState checks all sequential lifecycle gates for a resolved link.
func EvaluateState(link *ResolvedLink, now time.Time) (Outcome, string) {
	fallback := ""
	if link.FallbackURL != nil {
		fallback = *link.FallbackURL
	}

	// 1. Blocked
	if link.Status == "blocked" || link.Safety == "blocked" {
		return OutcomeBlocked, ""
	}

	// 2. Paused or archived
	if link.Status == "paused" || link.Status == "archived" {
		return OutcomePaused, fallback
	}

	// 3. Not started
	if link.StartsAt != nil && now.Before(*link.StartsAt) {
		return OutcomeNotStarted, fallback
	}

	// 4. Expired
	if link.ExpiresAt != nil && !now.Before(*link.ExpiresAt) {
		return OutcomeExpired, fallback
	}

	// 5. Scan limit
	if link.ScanLimit != nil && link.TotalScans >= *link.ScanLimit {
		return OutcomeLimitReached, fallback
	}

	// 6. Password required
	if link.HasPassword {
		return OutcomePasswordRequired, ""
	}

	// 7. Active
	destURL := ""
	if link.Version != nil {
		destURL = link.Version.URL
	}
	return OutcomeActive, destURL
}

// ErrUnavailable means neither Redis nor Postgres could answer and no last-known-good
// entry exists: the redirect serves a 503 "try again" page.
var ErrUnavailable = errors.New("link store unavailable")

const (
	redisTTL     = 10 * time.Minute
	negativeTTL  = 60 * time.Second
	lastGoodTTL  = 24 * time.Hour
	negativeMark = "-"
	// InvalidateChannel carries "{domain_id}:{code}" whenever a code changes.
	InvalidateChannel = "qr:invalidate"
)

// FetchFunc loads a link from Postgres; it returns (nil, nil) when the code does not exist.
type FetchFunc func(ctx context.Context, domainID uuid.UUID, code string) (*ResolvedLink, error)

// Resolver resolves (domain, code) through three tiers — in-process LRU (short TTL),
// Redis (10 min, clamped to the next scheduled change) and Postgres — collapsing
// concurrent misses with singleflight. Unknown codes are negative-cached for 60 s.
// A 24 h last-known-good copy keeps known codes working if Redis and Postgres both fail.
type Resolver struct {
	lru      *expirable.LRU[string, *ResolvedLink]
	negative *expirable.LRU[string, struct{}]
	lastGood *expirable.LRU[string, *ResolvedLink]
	rdb      *redis.Client
	sf       singleflight.Group
	fetch    FetchFunc
}

// NewResolver builds a resolver with a 30 s in-process TTL.
func NewResolver(lruSize int, rdb *redis.Client, fetch FetchFunc) (*Resolver, error) {
	return NewResolverTTL(lruSize, 30*time.Second, rdb, fetch)
}

// NewResolverTTL builds a resolver whose in-process entries expire after ttl (LRU_TTL).
func NewResolverTTL(lruSize int, ttl time.Duration, rdb *redis.Client, fetch FetchFunc) (*Resolver, error) {
	if lruSize <= 0 {
		return nil, fmt.Errorf("lru size must be positive")
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &Resolver{
		lru:      expirable.NewLRU[string, *ResolvedLink](lruSize, nil, ttl),
		negative: expirable.NewLRU[string, struct{}](lruSize/4+1, nil, negativeTTL),
		lastGood: expirable.NewLRU[string, *ResolvedLink](lruSize, nil, lastGoodTTL),
		rdb:      rdb,
		fetch:    fetch,
	}, nil
}

func cacheKey(domainID uuid.UUID, code string) string {
	return fmt.Sprintf("link:v1:%s:%s", domainID.String(), code)
}

func fresh(l *ResolvedLink, now time.Time) bool {
	return l.NextChangeAt == nil || now.Before(*l.NextChangeAt)
}

// Resolve returns the link, ErrNotFound, or ErrUnavailable.
func (r *Resolver) Resolve(ctx context.Context, domainID uuid.UUID, code string) (*ResolvedLink, error) {
	key := cacheKey(domainID, code)
	now := time.Now().UTC()
	if link, ok := r.lru.Get(key); ok {
		if fresh(link, now) {
			return link, nil
		}
		r.lru.Remove(key)
	}
	if _, ok := r.negative.Get(key); ok {
		return nil, ErrNotFound
	}
	v, err, _ := r.sf.Do(key, func() (interface{}, error) {
		redisOK := false
		if r.rdb != nil {
			val, err := r.rdb.Get(ctx, key).Bytes()
			switch {
			case err == nil && string(val) == negativeMark:
				r.negative.Add(key, struct{}{})
				return nil, ErrNotFound
			case err == nil:
				redisOK = true
				var link ResolvedLink
				if json.Unmarshal(val, &link) == nil && fresh(&link, now) {
					r.lru.Add(key, &link)
					r.lastGood.Add(key, &link)
					return &link, nil
				}
			case errors.Is(err, redis.Nil):
				redisOK = true
			}
		}
		link, err := r.fetch(ctx, domainID, code)
		if err != nil {
			if lg, ok := r.lastGood.Get(key); ok {
				return lg, nil
			}
			return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		if link == nil {
			r.negative.Add(key, struct{}{})
			if r.rdb != nil && redisOK {
				_ = r.rdb.Set(ctx, key, negativeMark, negativeTTL).Err()
			}
			return nil, ErrNotFound
		}
		link.CachedAt = time.Now().UTC()
		if r.rdb != nil {
			ttl := redisTTL
			if link.NextChangeAt != nil {
				if remaining := time.Until(*link.NextChangeAt); remaining > 0 && remaining < ttl {
					ttl = remaining
				}
			}
			if b, err := json.Marshal(link); err == nil {
				_ = r.rdb.Set(ctx, key, b, ttl).Err()
			}
		}
		r.lru.Add(key, link)
		r.lastGood.Add(key, link)
		return link, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*ResolvedLink), nil
}

// Evict drops local copies (called for pub/sub invalidations; the API deletes Redis).
func (r *Resolver) Evict(domainID uuid.UUID, code string) {
	key := cacheKey(domainID, code)
	r.lru.Remove(key)
	r.negative.Remove(key)
	r.lastGood.Remove(key)
}

// Invalidate purges local caches and Redis.
func (r *Resolver) Invalidate(ctx context.Context, domainID uuid.UUID, code string) {
	r.Evict(domainID, code)
	if r.rdb != nil {
		_ = r.rdb.Del(ctx, cacheKey(domainID, code)).Err()
	}
}

// Len reports how many live entries the in-process cache holds (readiness signal).
func (r *Resolver) Len() int { return r.lru.Len() }

// Subscribe evicts local entries on "{domain_id}:{code}" messages until ctx ends.
// It reconnects automatically (go-redis PubSub) and returns when ctx is cancelled.
func (r *Resolver) Subscribe(ctx context.Context) {
	if r.rdb == nil {
		return
	}
	sub := r.rdb.Subscribe(ctx, InvalidateChannel)
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			i := strings.LastIndex(msg.Payload, ":")
			if i <= 0 {
				continue
			}
			if id, err := uuid.Parse(msg.Payload[:i]); err == nil {
				r.Evict(id, msg.Payload[i+1:])
			}
		}
	}
}
