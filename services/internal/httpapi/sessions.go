package httpapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/auth"
)

// sessionCache remembers recently validated sessions so each request does not hit the DB.
// Revocations on this instance evict immediately; other instances notice within ttl.
type sessionCache struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[uuid.UUID]time.Time // session id -> cache expiry
}

func newSessionCache(ttl time.Duration) *sessionCache {
	return &sessionCache{ttl: ttl, m: map[uuid.UUID]time.Time{}}
}

func (c *sessionCache) valid(id uuid.UUID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	exp, ok := c.m[id]
	if ok && time.Now().Before(exp) {
		return true
	}
	delete(c.m, id)
	return false
}

func (c *sessionCache) put(id uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) > 50000 { // bound memory: drop everything; entries are cheap to rebuild
		c.m = map[uuid.UUID]time.Time{}
	}
	c.m[id] = time.Now().Add(c.ttl)
}

func (c *sessionCache) evict(id uuid.UUID) {
	c.mu.Lock()
	delete(c.m, id)
	c.mu.Unlock()
}

func (c *sessionCache) evictAll() {
	c.mu.Lock()
	c.m = map[uuid.UUID]time.Time{}
	c.mu.Unlock()
}

// sessionGuard drops the principal of an access token whose session was revoked or expired
// (logout, password reset, "sign out other devices", SCIM deprovisioning). Without it a
// stolen access token would stay usable for its full lifetime.
func (s *Server) sessionGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.GetPrincipal(r.Context())
		if !ok || p.IsAPIKey() || p.SessionID == uuid.Nil {
			next.ServeHTTP(w, r)
			return
		}
		if s.sessions.valid(p.SessionID) {
			next.ServeHTTP(w, r)
			return
		}
		sess, err := s.q.GetSessionByID(r.Context(), p.SessionID)
		if err != nil || sess.RevokedAt.Valid || time.Now().After(sess.ExpiresAt) || sess.UserID != p.UserID {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), nil)))
			return
		}
		if _, err := s.q.GetUserByID(r.Context(), p.UserID); err != nil { // deleted user
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), nil)))
			return
		}
		s.sessions.put(p.SessionID)
		next.ServeHTTP(w, r)
	})
}
