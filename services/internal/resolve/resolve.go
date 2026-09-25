package resolve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
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

// Resolver manages LRU and Redis caching with DB fallback and singleflight deduplication.
type Resolver struct {
	lru   *lru.Cache[string, *ResolvedLink]
	rdb   *redis.Client
	sf    singleflight.Group
	fetch func(ctx context.Context, domainID uuid.UUID, code string) (*ResolvedLink, error)
}

func NewResolver(
	lruSize int,
	rdb *redis.Client,
	fetchFunc func(ctx context.Context, domainID uuid.UUID, code string) (*ResolvedLink, error),
) (*Resolver, error) {
	c, err := lru.New[string, *ResolvedLink](lruSize)
	if err != nil {
		return nil, fmt.Errorf("init lru: %w", err)
	}

	return &Resolver{
		lru:   c,
		rdb:   rdb,
		fetch: fetchFunc,
	}, nil
}

func cacheKey(domainID uuid.UUID, code string) string {
	return fmt.Sprintf("link:v1:%s:%s", domainID.String(), code)
}

// Resolve looks up a short code across LRU, Redis, and Database.
func (r *Resolver) Resolve(ctx context.Context, domainID uuid.UUID, code string) (*ResolvedLink, error) {
	key := cacheKey(domainID, code)

	// 1. Check in-process LRU cache
	if link, ok := r.lru.Get(key); ok {
		if link.NextChangeAt == nil || time.Now().UTC().Before(*link.NextChangeAt) {
			return link, nil
		}
		// Stale due to scheduled version activation
		r.lru.Remove(key)
	}

	// 2. Singleflight DB / Redis lookups
	v, err, _ := r.sf.Do(key, func() (interface{}, error) {
		// Check Redis
		if r.rdb != nil {
			val, err := r.rdb.Get(ctx, key).Bytes()
			if err == nil {
				var link ResolvedLink
				if err := json.Unmarshal(val, &link); err == nil {
					if link.NextChangeAt == nil || time.Now().UTC().Before(*link.NextChangeAt) {
						r.lru.Add(key, &link)
						return &link, nil
					}
				}
			}
		}

		// Fallback to fetch (Postgres DB)
		link, err := r.fetch(ctx, domainID, code)
		if err != nil {
			return nil, err
		}
		if link == nil {
			return nil, ErrNotFound
		}

		link.CachedAt = time.Now().UTC()

		// Warm Redis cache (TTL default 30 min, or clamped by next_change_at)
		if r.rdb != nil {
			ttl := 30 * time.Minute
			if link.NextChangeAt != nil {
				remaining := time.Until(*link.NextChangeAt)
				if remaining > 0 && remaining < ttl {
					ttl = remaining
				}
			}
			bytes, _ := json.Marshal(link)
			_ = r.rdb.Set(ctx, key, bytes, ttl).Err()
		}

		// Warm in-process LRU
		r.lru.Add(key, link)
		return link, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(*ResolvedLink), nil
}

// Invalidate purges cache entries from LRU and Redis.
func (r *Resolver) Invalidate(ctx context.Context, domainID uuid.UUID, code string) {
	key := cacheKey(domainID, code)
	r.lru.Remove(key)
	if r.rdb != nil {
		_ = r.rdb.Del(ctx, key).Err()
	}
}
