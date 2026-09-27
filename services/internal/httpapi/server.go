// Package httpapi is the control-plane HTTP API (cmd/api).
package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/abuse"
	"github.com/its-aryansingh/qrit/services/internal/access"
	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/config"
	"github.com/its-aryansingh/qrit/services/internal/email"
	"github.com/its-aryansingh/qrit/services/internal/envelope"
	"github.com/its-aryansingh/qrit/services/internal/flags"
	"github.com/its-aryansingh/qrit/services/internal/idempotency"
	"github.com/its-aryansingh/qrit/services/internal/netutil"
	"github.com/its-aryansingh/qrit/services/internal/plans"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
	"github.com/its-aryansingh/qrit/services/internal/shortcode"
	"github.com/its-aryansingh/qrit/services/internal/sso"
	"github.com/its-aryansingh/qrit/services/internal/urlsafety"
	"github.com/its-aryansingh/qrit/services/internal/workspace"
)

// Deps are the collaborators the API needs. Optional ones may be nil.
type Deps struct {
	Config  *config.Config
	Pool    *pgxpool.Pool
	Redis   *redis.Client
	Tokens  *auth.TokenManager
	Email   email.Sender
	Safety  urlsafety.SafetyClient
	Grants  GrantSource // nil → role bindings + org roles (access.Engine)
	Gate    AccessGate  // nil → organisation identity gate
	Limiter abuse.VelocityLimiter
}

type Server struct {
	cfg        *config.Config
	pool       *pgxpool.Pool
	q          *dbgen.Queries
	rdb        *redis.Client
	tm         *auth.TokenManager
	mail       email.Sender
	safety     urlsafety.SafetyClient
	grants     GrantSource
	gate       AccessGate
	limiter    abuse.VelocityLimiter
	ipResolver *netutil.ClientIPResolver

	platformDomainID uuid.UUID
	domainMu         sync.RWMutex
	domainHosts      map[uuid.UUID]string

	provisioner   Provisioner
	sessionPolicy SessionPolicy
	auditor       Auditor
	identity      IdentityHooks
	entitle       Entitlements
	versionGate   VersionGate
	memberHooks   MemberHooks
	sessions      *sessionCache

	access  *access.Engine
	plans   *plans.Service
	flags   *flags.Flags
	keyring *envelope.Keyring
	// streamHTTP delivers SIEM stream tests (SSRF-guarded).
	streamHTTP *http.Client

	ssoHTTP *http.Client
	oidc    *sso.OIDC
	dns     sso.TXTResolver
	polis   *sso.Polis

	// extension points registered by enterprise modules
	mounts       []func(r chi.Router)
	wsMounts     []func(r chi.Router)
	publicMounts []func(r chi.Router)
	orgMounts    []func(r chi.Router)
}

func New(d Deps) (*Server, error) {
	trusted, err := d.Config.TrustedProxies()
	if err != nil {
		return nil, err
	}
	s := &Server{
		cfg: d.Config, pool: d.Pool, q: dbgen.New(d.Pool), rdb: d.Redis, tm: d.Tokens,
		mail: d.Email, safety: d.Safety, grants: d.Grants, gate: d.Gate, limiter: d.Limiter,
		ipResolver:  netutil.NewClientIPResolver(trusted),
		domainHosts: map[uuid.UUID]string{},
		sessions:    newSessionCache(20 * time.Second),
	}
	s.access = access.NewEngine(d.Pool, d.Redis)
	s.plans = plans.NewService(d.Pool)
	s.flags = flags.New(d.Pool)
	master, err := d.Config.EncryptionKey()
	if err != nil {
		return nil, err
	}
	if s.keyring, err = envelope.NewKeyring(d.Pool, master); err != nil {
		return nil, err
	}
	s.initSSO()
	s.streamHTTP = netutil.SafeHTTPClient(s.cfg.IsLocal(), 15*time.Second)
	s.identity = coreIdentity{s: s}
	s.versionGate = approvalGate{s: s}
	if s.grants == nil {
		s.grants = s.access
	}
	if s.gate == nil {
		s.gate = identityGate{s: s}
	}
	if s.entitle == nil {
		s.entitle = planEntitlementsV2{p: s.plans}
	}
	if s.limiter == nil {
		if d.Redis != nil {
			s.limiter = abuse.NewRedisVelocityLimiter(d.Redis)
		} else {
			s.limiter = abuse.NewMemoryVelocityLimiter()
		}
	}
	if s.safety == nil {
		s.safety = urlsafety.NewFakeSafetyClient()
	}
	return s, nil
}

// Access exposes the permission engine (call Invalidate after grant changes).
func (s *Server) Access() *access.Engine { return s.access }

// Keyring exposes per-organisation envelope encryption.
func (s *Server) Keyring() *envelope.Keyring { return s.keyring }

// Plans exposes effective entitlements.
func (s *Server) Plans() *plans.Service { return s.plans }

// Pool exposes the database pool for enterprise modules.
func (s *Server) Pool() *pgxpool.Pool { return s.pool }

// Queries exposes the query set for enterprise modules.
func (s *Server) Queries() *dbgen.Queries { return s.q }

// MountAuthenticated registers routes under /v1 that require a signed-in principal.
func (s *Server) MountAuthenticated(fn func(r chi.Router)) { s.mounts = append(s.mounts, fn) }

// MountWorkspace registers routes under /v1/workspaces/{ws} (workspace context + grants applied).
func (s *Server) MountWorkspace(fn func(r chi.Router)) { s.wsMounts = append(s.wsMounts, fn) }

// MountPublic registers unauthenticated routes under /v1.
func (s *Server) MountPublic(fn func(r chi.Router)) { s.publicMounts = append(s.publicMounts, fn) }

// Init performs startup work: ensure the platform short domain exists.
func (s *Server) Init(ctx context.Context) error {
	host := strings.ToLower(strings.TrimSpace(s.cfg.PlatformShortDomain))
	d, err := s.q.EnsurePlatformDomain(ctx, dbgen.EnsurePlatformDomainParams{ID: idgen.New(), Hostname: host})
	if err != nil {
		return fmt.Errorf("ensure platform domain %q: %w", host, err)
	}
	s.platformDomainID = d.ID
	s.domainHosts[d.ID] = d.Hostname
	return nil
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(s.clientIPMiddleware)
	r.Use(accessLog)
	r.Use(securityHeaders)
	r.Use(s.cors)
	r.Use(auth.AuthenticateMiddleware(s.tm))
	r.Use(s.sessionGuard)
	r.Use(s.mfaGate)
	r.Use(auth.CSRFMiddleware)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", s.handleReady)
	s.scimRoutes(r)

	v1 := func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", s.handleRegister)
			r.Post("/login", s.handleLogin)
			r.Post("/logout", s.handleLogout)
			r.Post("/refresh", s.handleRefresh)
			r.Post("/verify-email", s.handleVerifyEmail)
			r.Post("/password/forgot", s.handleForgotPassword)
			r.Post("/password/reset", s.handleResetPassword)
			r.Post("/sso/start", s.handleSSOStart)
			r.Get("/sso/callback", s.handleSSOCallback)
			r.With(auth.RequireAuth).Post("/resend-verification", s.handleResendVerification)
			r.With(auth.RequireAuth).Get("/me", s.handleMe)
		})
		r.Get("/invites/{token}", s.handleInvitePreview)
		for _, m := range s.publicMounts {
			m(r)
		}

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth)
			r.Get("/me", s.handleMe)
			r.Patch("/me", s.handleUpdateMe)
			r.Post("/me/password", s.handleChangePassword)
			r.Get("/me/sessions", s.handleListSessions)
			r.Delete("/me/sessions/{id}", s.handleRevokeSession)
			r.Post("/invites/{token}/accept", s.handleInviteAccept)
			s.orgRoutes(r)
			s.mfaRoutes(r)
			r.Get("/me/approvals", s.handleMyApprovals)
			for _, m := range s.mounts {
				m(r)
			}

			r.Route("/workspaces", func(r chi.Router) {
				r.Get("/", s.handleListWorkspaces)
				r.Post("/", s.handleCreateWorkspace)
				r.Route("/{ws}", func(r chi.Router) {
					r.Use(s.workspaceContext)
					r.Get("/", s.handleGetWorkspace)
					r.Get("/entitlements", s.handleGetEntitlements)
					r.With(authz.RequireWorkspaceWide(authz.WorkspaceUpdate)).Patch("/", s.handleUpdateWorkspace)
					r.With(authz.RequireWorkspaceWide(authz.WorkspaceDelete)).Delete("/", s.handleDeleteWorkspace)
					r.Post("/leave", s.handleLeaveWorkspace)
					r.With(authz.RequireWorkspaceWide(authz.BillingManage)).Post("/transfer-ownership", s.handleTransferOwnership)

					r.Get("/members", s.handleListMembers)
					r.With(authz.RequireWorkspaceWide(authz.MemberManage)).Patch("/members/{userId}", s.handleUpdateMember)
					r.With(authz.RequireWorkspaceWide(authz.MemberManage)).Delete("/members/{userId}", s.handleRemoveMember)
					r.With(authz.RequireWorkspaceWide(authz.MemberManage)).Get("/invites", s.handleListInvites)
					r.With(authz.RequireWorkspaceWide(authz.MemberManage), idempotency.Middleware(s.q)).Post("/invites", s.handleCreateInvite)
					r.With(authz.RequireWorkspaceWide(authz.MemberManage)).Delete("/invites/{id}", s.handleRevokeInvite)

					r.Route("/qr-codes", func(r chi.Router) {
						r.With(authz.Require(authz.QRRead)).Get("/", s.handleListQRCodes)
						r.With(authz.Require(authz.QRCreate), idempotency.Middleware(s.q)).Post("/", s.handleCreateQRCode)
						r.With(authz.Require(authz.QRDelete)).Post("/{id}/restore", s.handleRestoreQRCode)
						r.Route("/{id}", func(r chi.Router) {
							r.With(authz.Require(authz.QRRead)).Get("/", s.handleGetQRCode)
							r.With(authz.Require(authz.QRUpdate)).Patch("/", s.handleUpdateQRCode)
							r.With(authz.Require(authz.QRDelete)).Delete("/", s.handleDeleteQRCode)
							r.With(authz.Require(authz.QRUpdate)).Post("/pause", s.handleSetStatus("paused"))
							r.With(authz.Require(authz.QRUpdate)).Post("/resume", s.handleSetStatus("active"))
							r.With(authz.Require(authz.QRUpdate)).Post("/archive", s.handleSetStatus("archived"))
							r.With(authz.Require(authz.QRUpdate)).Post("/unarchive", s.handleSetStatus("active"))
							r.With(authz.Require(authz.QRRead)).Get("/versions", s.handleListVersions)
							r.With(authz.Require(authz.QRDestinationUpdate), idempotency.Middleware(s.q)).Post("/versions", s.handleCreateVersion)
							r.With(authz.Require(authz.QRDestinationUpdate)).Post("/versions/{versionId}/restore", s.handleRestoreVersion)
							r.With(authz.Require(authz.QRDestinationUpdate)).Delete("/versions/{versionId}", s.handleCancelScheduledVersion)
							r.With(authz.Require(authz.QRRead)).Post("/resolve-preview", s.handleResolvePreview)
						})
					})
					s.organizeRoutes(r)
					s.analyticsRoutes(r)
					s.approvalRoutes(r)
					s.governanceWSRoutes(r)
					r.With(authz.RequireWorkspaceWide(authz.WorkspaceRead)).Get("/policies", s.handleGetWorkspacePolicy)
					r.With(authz.RequireWorkspaceWide(authz.PolicyManage)).Put("/policies", s.handlePutWorkspacePolicy)
					r.With(authz.Require(authz.AuditRead)).Get("/audit-logs", s.handleWorkspaceAuditLogs)
					for _, m := range s.wsMounts {
						m(r)
					}
				})
			})
		})
	}
	r.Route("/v1", v1)
	r.Route("/api/v1", v1)
	return r
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// hostForDomain returns the hostname of a short-link domain (cached).
func (s *Server) hostForDomain(ctx context.Context, id uuid.UUID) string {
	s.domainMu.RLock()
	h, ok := s.domainHosts[id]
	s.domainMu.RUnlock()
	if ok {
		return h
	}
	d, err := s.q.GetDomainByID(ctx, id)
	if err != nil {
		return s.cfg.PlatformShortDomain
	}
	s.domainMu.Lock()
	s.domainHosts[id] = d.Hostname
	s.domainMu.Unlock()
	return d.Hostname
}

func (s *Server) shortURL(host, code string) string {
	scheme := "https"
	if strings.EqualFold(host, s.cfg.PlatformShortDomain) && s.cfg.PlatformShortDomainScheme != "" {
		scheme = s.cfg.PlatformShortDomainScheme
	}
	return scheme + "://" + host + "/" + code
}

func (s *Server) encodedPayload(host, code string) string {
	scheme := "HTTPS"
	if strings.EqualFold(host, s.cfg.PlatformShortDomain) && s.cfg.PlatformShortDomainScheme == "http" {
		scheme = "HTTP"
	}
	return strings.ToUpper(scheme + "://" + host + "/" + code)
}

// InvalidateLink drops a resolved link from Redis and tells every redirect instance to evict it.
func (s *Server) InvalidateLink(ctx context.Context, domainID uuid.UUID, code string) {
	if s.rdb == nil || code == "" {
		return
	}
	code = shortcode.Normalise(code)
	key := fmt.Sprintf("link:v1:%s:%s", domainID, code)
	if err := s.rdb.Del(ctx, key).Err(); err != nil {
		slog.Warn("link cache delete failed", "error", err)
	}
	if err := s.rdb.Publish(ctx, "qr:invalidate", domainID.String()+":"+code).Err(); err != nil {
		slog.Warn("link invalidation publish failed", "error", err)
	}
}

// inTx runs fn in a transaction with a Queries bound to it.
func (s *Server) inTx(ctx context.Context, fn func(q *dbgen.Queries, tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Inside a workspace request, pin the transaction to that tenant so row-level security
	// (migration 00002) rejects any cross-workspace read or write.
	if ws, ok := workspace.GetWorkspace(ctx); ok && ws != nil {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.workspace_id', $1, true)`, ws.ID.String()); err != nil {
			return err
		}
	}
	if err := fn(s.q.WithTx(tx), tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
