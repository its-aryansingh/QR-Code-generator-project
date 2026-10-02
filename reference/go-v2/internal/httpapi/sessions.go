package httpapi

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
)

// sessionCache remembers recently validated sessions so each request does not hit the DB.
// Revocations on this instance evict immediately; other instances notice within ttl.
type sessionCache struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[uuid.UUID]cachedSession
}

type cachedSession struct {
	sess dbgen.Session
	exp  time.Time
}

func newSessionCache(ttl time.Duration) *sessionCache {
	return &sessionCache{ttl: ttl, m: map[uuid.UUID]cachedSession{}}
}

func (c *sessionCache) get(id uuid.UUID) (dbgen.Session, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cs, ok := c.m[id]
	if ok && time.Now().Before(cs.exp) {
		return cs.sess, true
	}
	delete(c.m, id)
	return dbgen.Session{}, false
}

// valid reports a cached, still-valid session (kept for callers that only need a yes/no).
func (c *sessionCache) valid(id uuid.UUID) bool { _, ok := c.get(id); return ok }

func (c *sessionCache) put(s dbgen.Session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) > 50000 { // bound memory: drop everything; entries are cheap to rebuild
		c.m = map[uuid.UUID]cachedSession{}
	}
	c.m[s.ID] = cachedSession{sess: s, exp: time.Now().Add(c.ttl)}
}

func (c *sessionCache) evict(id uuid.UUID) {
	c.mu.Lock()
	delete(c.m, id)
	c.mu.Unlock()
}

func (c *sessionCache) evictAll() {
	c.mu.Lock()
	c.m = map[uuid.UUID]cachedSession{}
	c.mu.Unlock()
}

type sessionCtxKey struct{}

// currentSession returns the validated session of a user request (false for API keys).
func currentSession(r *http.Request) (dbgen.Session, bool) {
	s, ok := r.Context().Value(sessionCtxKey{}).(dbgen.Session)
	return s, ok
}

// sessionGuard drops the principal of an access token whose session was revoked or expired
// (logout, password reset, "sign out other devices", SCIM deprovisioning). Without it a
// stolen access token would stay usable for its full lifetime. It also keeps last_used_at
// fresh (≤ 1 min stale) for idle-timeout policies.
func (s *Server) sessionGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.GetPrincipal(r.Context())
		if !ok || p.IsAPIKey() || p.SessionID == uuid.Nil {
			next.ServeHTTP(w, r)
			return
		}
		sess, cached := s.sessions.get(p.SessionID)
		if !cached {
			var err error
			sess, err = s.q.GetSessionByID(r.Context(), p.SessionID)
			if err != nil || sess.RevokedAt.Valid || time.Now().After(sess.ExpiresAt) || sess.UserID != p.UserID {
				next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), nil)))
				return
			}
			if _, err := s.q.GetUserByID(r.Context(), p.UserID); err != nil { // deleted user
				next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), nil)))
				return
			}
		}
		prev := sess // idle-timeout checks compare against activity before this request
		if time.Since(sess.LastUsedAt) > time.Minute {
			sess.LastUsedAt = time.Now()
			go func(id uuid.UUID) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = s.q.TouchSession(ctx, id)
			}(sess.ID)
			cached = false
		}
		if !cached {
			s.sessions.put(sess)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionCtxKey{}, prev)))
	})
}
