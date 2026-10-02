package httpapi

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/workspace"
)

type ctxKey int

const (
	ctxClientIP ctxKey = iota
	ctxWorkspaceRow
)

// clientIPMiddleware stores the trusted client IP in the context (never logged raw).
func (s *Server) clientIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := s.ipResolver.ClientIP(r)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxClientIP, ip)))
	})
}

func clientIP(r *http.Request) net.IP {
	ip, _ := r.Context().Value(ctxClientIP).(net.IP)
	return ip
}

func clientIPString(r *http.Request) string {
	if ip := clientIP(r); ip != nil {
		return ip.String()
	}
	return ""
}

// accessLog writes one structured line per request without IPs or tokens.
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		route := chi.RouteContext(r.Context())
		pattern := ""
		if route != nil {
			pattern = route.RoutePattern()
		}
		level := slog.LevelInfo
		if ww.Status() >= 500 {
			level = slog.LevelError
		}
		slog.Log(r.Context(), level, "http_request",
			"request_id", middleware.GetReqID(r.Context()),
			"method", r.Method, "route", pattern, "status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds())
	})
}

// cors allows only configured origins, with credentials. No wildcard.
func (s *Server) cors(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range s.cfg.CORSAllowedOrigins {
		if o = strings.TrimSpace(o); o != "" {
			allowed[strings.TrimRight(o, "/")] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[origin] {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, Idempotency-Key, If-Match")
			h.Set("Access-Control-Expose-Headers", "ETag, Location, RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset, Retry-After")
			h.Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions && origin != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeaders applies conservative headers to API responses.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// workspaceContext resolves {ws} (UUID or slug), verifies access, and attaches the workspace
// row, typed workspace and the principal's grants. Non-members get 404 so ids do not leak.
func (s *Server) workspaceContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.GetPrincipal(r.Context())
		if !ok {
			fail(w, apierr.Unauthorized("authentication required"))
			return
		}
		param := chi.URLParam(r, "ws")
		var row dbgen.Workspace
		var err error
		if id, perr := uuid.Parse(param); perr == nil {
			row, err = s.q.GetWorkspaceByID(r.Context(), id)
		} else {
			row, err = s.q.GetWorkspaceBySlug(r.Context(), param)
		}
		if err != nil {
			fail(w, apierr.NotFound("workspace not found"))
			return
		}
		grants, gerr := s.grants.Grants(r.Context(), p, row)
		if gerr != nil {
			slog.Error("resolve grants", "error", gerr)
			fail(w, apierr.Internal("failed to resolve permissions"))
			return
		}
		if grants == nil || (!grants.HasAnywhere(authz.WorkspaceRead) && !grants.IsOwner()) {
			fail(w, apierr.NotFound("workspace not found"))
			return
		}
		if gateErr := s.gate.Check(r, p, row); gateErr != nil {
			fail(w, gateErr)
			return
		}
		ws := &workspace.Workspace{
			ID: row.ID, Name: row.Name, Slug: row.Slug, OwnerID: row.OwnerID,
			PlanID: row.PlanID, Timezone: row.Timezone, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
		ctx := workspace.WithWorkspace(r.Context(), ws)
		ctx = context.WithValue(ctx, ctxWorkspaceRow, row)
		ctx = authz.WithGrants(ctx, grants)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func workspaceRow(r *http.Request) dbgen.Workspace {
	row, _ := r.Context().Value(ctxWorkspaceRow).(dbgen.Workspace)
	return row
}

func principal(r *http.Request) *auth.Principal {
	p, _ := auth.GetPrincipal(r.Context())
	return p
}

// GrantSource resolves a principal's permissions in a workspace.
type GrantSource interface {
	Grants(ctx context.Context, p *auth.Principal, ws dbgen.Workspace) (*authz.Grants, error)
}

// AccessGate applies org-level identity policy (SSO/MFA/IP/session) before handlers run.
type AccessGate interface {
	Check(r *http.Request, p *auth.Principal, ws dbgen.Workspace) error
}

// membershipGrants is the pre-enterprise source: the member's role → system role permissions.
type membershipGrants struct{ q *dbgen.Queries }

func (m membershipGrants) Grants(ctx context.Context, p *auth.Principal, ws dbgen.Workspace) (*authz.Grants, error) {
	mem, err := m.q.GetWorkspaceMember(ctx, dbgen.GetWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: p.UserID})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	g := authz.NewGrants()
	g.Add(authz.SystemRoles[mem.Role], nil)
	g.Sources = append(g.Sources, authz.Source{Kind: "membership", RoleKey: mem.Role})
	return g, nil
}

type noGate struct{}

func (noGate) Check(*http.Request, *auth.Principal, dbgen.Workspace) error { return nil }
