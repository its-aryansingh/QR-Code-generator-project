package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/config"
	"github.com/its-aryansingh/qrit/services/internal/email"
	"github.com/its-aryansingh/qrit/services/internal/urlsafety"
)

// Integration tests run against a migrated database:
//   QRIT_TEST_DATABASE_URL=postgres://... QRIT_TEST_REDIS_URL=redis://localhost:6379/15 go test ./internal/httpapi
type harness struct {
	t      *testing.T
	srv    *Server
	ts     *httptest.Server
	pool   *pgxpool.Pool
	rdb    *redis.Client
	mail   *email.MemorySender
	safety *urlsafety.FakeSafetyClient
}

func newHarness(t *testing.T, configure ...func(*Server)) *harness {
	t.Helper()
	dsn := os.Getenv("QRIT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("QRIT_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var rdb *redis.Client
	if u := os.Getenv("QRIT_TEST_REDIS_URL"); u != "" {
		opt, err := redis.ParseURL(u)
		if err != nil {
			t.Fatal(err)
		}
		rdb = redis.NewClient(opt)
		t.Cleanup(func() { _ = rdb.Close() })
		if err := rdb.FlushDB(ctx).Err(); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{AppEnv: "test", DatabaseURL: dsn, AppBaseURL: "http://app.test",
		PlatformShortDomain: "qr.test", PlatformShortDomainScheme: "https", JWTKeyID: "test"}
	key, _ := cfg.JWTPrivateKey()
	tm, _ := auth.NewTokenManager("test", key)
	mail := email.NewMemorySender()
	safety := urlsafety.NewFakeSafetyClient()
	s, err := New(Deps{Config: cfg, Pool: pool, Redis: rdb, Tokens: tm, Email: mail, Safety: safety})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Init(ctx); err != nil {
		t.Fatal(err)
	}
	for _, c := range configure {
		c(s)
	}
	ts := httptest.NewServer(s.Routes())
	t.Cleanup(ts.Close)
	return &harness{t: t, srv: s, ts: ts, pool: pool, rdb: rdb, mail: mail, safety: safety}
}

// exec runs SQL as the test's superuser (fixtures, plan changes).
func (h *harness) exec(sql string, args ...any) {
	h.t.Helper()
	if _, err := h.pool.Exec(context.Background(), sql, args...); err != nil {
		h.t.Fatalf("exec %q: %v", sql, err)
	}
}

type client struct {
	h     *harness
	http  *http.Client
	token string // bearer override
}

func (h *harness) client() *client {
	jar, _ := cookiejar.New(nil)
	return &client{h: h, http: &http.Client{Jar: jar, Timeout: 20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

type resp struct {
	Status int
	Header http.Header
	Body   []byte
}

func (r resp) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		t.Fatalf("decode %d %s: %v", r.Status, r.Body, err)
	}
	return m
}

func (c *client) do(method, path string, body any, hdr ...string) resp {
	c.h.t.Helper()
	var rd io.Reader
	if body != nil {
		switch b := body.(type) {
		case string:
			rd = strings.NewReader(b)
		default:
			buf, _ := json.Marshal(b)
			rd = bytes.NewReader(buf)
		}
	}
	req, _ := http.NewRequest(method, c.h.ts.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for _, ck := range c.http.Jar.Cookies(req.URL) {
		if ck.Name == auth.CSRFCookieName {
			req.Header.Set(auth.CSRFHeaderName, ck.Value)
		}
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return resp{Status: res.StatusCode, Header: res.Header, Body: b}
}

func (c *client) must(status int, method, path string, body any, hdr ...string) map[string]any {
	c.h.t.Helper()
	r := c.do(method, path, body, hdr...)
	if r.Status != status {
		c.h.t.Fatalf("%s %s: want %d got %d: %s", method, path, status, r.Status, r.Body)
	}
	if len(r.Body) == 0 {
		return nil
	}
	return r.json(c.h.t)
}

var uniq = time.Now().UnixNano()

func uniqueEmail(prefix string) string {
	return prefix + "+" + strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", "") + "@example.com"
}

// register signs up a new user and returns the client plus the created workspace id.
func (h *harness) register(name string) (*client, string, string) {
	c := h.client()
	em := uniqueEmail(name)
	m := c.must(201, "POST", "/v1/auth/register", map[string]any{"email": em, "password": "correct horse battery", "name": name})
	ws := m["workspace"].(map[string]any)["id"].(string)
	return c, ws, em
}

var tokenInLink = regexp.MustCompile(`token=([0-9a-f]+)`)

func dig(m map[string]any, path ...string) any {
	var cur any = m
	for _, p := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[p]
	}
	return cur
}

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// setPlan changes the plan of the workspace's organisation (plans live on the org).
func (h *harness) setPlan(ws, plan string) {
	h.t.Helper()
	h.exec(`UPDATE organizations SET plan_id = $2 WHERE id = (SELECT org_id FROM workspaces WHERE id = $1)`, ws, plan)
	h.srv.plans.Invalidate()
}
