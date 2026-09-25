package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/audit"
	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/config"
	"github.com/its-aryansingh/qrit/services/internal/email"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/idempotency"
	"github.com/its-aryansingh/qrit/services/internal/platform/crypto"
	"github.com/its-aryansingh/qrit/services/internal/platform/db"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/httpx"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/obs"
	"github.com/its-aryansingh/qrit/services/internal/qr"
	"github.com/its-aryansingh/qrit/services/internal/rbac"
	"github.com/its-aryansingh/qrit/services/internal/scan"
	"github.com/its-aryansingh/qrit/services/internal/shortcode"
	"github.com/its-aryansingh/qrit/services/internal/urlsafety"
	"github.com/its-aryansingh/qrit/services/internal/version"
	"github.com/its-aryansingh/qrit/services/internal/workspace"
)

type App struct {
	cfg          *config.Config
	pool         *pgxpool.Pool
	queries      *dbgen.Queries
	tm           *auth.TokenManager
	emailSender  email.Sender
	safetyClient urlsafety.SafetyClient
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	obs.InitLogger(cfg.LogLevel)

	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	tm, err := auth.NewTokenManager(cfg.JWTKeyID, nil)
	if err != nil {
		slog.Error("failed to initialize token manager", "error", err)
		os.Exit(1)
	}

	app := &App{
		cfg:          cfg,
		pool:         pool,
		queries:      dbgen.New(pool),
		tm:           tm,
		emailSender:  &email.ConsoleSender{},
		safetyClient: urlsafety.NewFakeSafetyClient(),
	}

	router := app.routes()

	httpServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("api service listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	slog.Info("api service gracefully stopped")
}

func (a *App) routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)
	r.Use(auth.AuthenticateMiddleware(a.tm))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mountV1 := func(r chi.Router) {
		// Public Auth routes
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", a.handleRegister)
			r.Post("/login", a.handleLogin)
			r.Post("/logout", a.handleLogout)
			r.Post("/refresh", a.handleRefresh)

			// Authenticated user profile
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAuth)
				r.Get("/me", a.handleMe)
			})
		})

		// Workspaces
		r.Route("/workspaces", func(r chi.Router) {
			r.Use(auth.RequireAuth)

			r.Get("/", a.handleListWorkspaces)
			r.Post("/", a.handleCreateWorkspace)

			r.Route("/{ws}", func(r chi.Router) {
				r.Use(a.workspaceContextMiddleware)

				r.Get("/", a.handleGetWorkspace)
				r.Get("/entitlements", a.handleGetEntitlements)

				r.With(rbac.Require(rbac.PermManage)).Patch("/", a.handleUpdateWorkspace)
				r.With(rbac.Require(rbac.PermView)).Get("/members", a.handleListMembers)
				r.With(rbac.Require(rbac.PermManageMembers)).Post("/invites", a.handleCreateInvite)

				// QR codes inside workspace
				r.Route("/qr-codes", func(r chi.Router) {
					r.Use(idempotency.Middleware(a.queries))

					r.With(rbac.Require(rbac.PermView)).Get("/", a.handleListQRCodes)
					r.With(rbac.Require(rbac.PermCreateEdit)).Post("/", a.handleCreateQRCode)

					r.Route("/{id}", func(r chi.Router) {
						r.With(rbac.Require(rbac.PermView)).Get("/", a.handleGetQRCode)
						r.With(rbac.Require(rbac.PermCreateEdit)).Patch("/", a.handleUpdateQRCode)
						r.With(rbac.Require(rbac.PermManage)).Delete("/", a.handleDeleteQRCode)

						r.With(rbac.Require(rbac.PermCreateEdit)).Post("/pause", a.handleSetStatus("paused"))
						r.With(rbac.Require(rbac.PermCreateEdit)).Post("/resume", a.handleSetStatus("active"))
						r.With(rbac.Require(rbac.PermCreateEdit)).Post("/archive", a.handleSetStatus("archived"))

						// Versions
						r.With(rbac.Require(rbac.PermView)).Get("/versions", a.handleListVersions)
						r.With(rbac.Require(rbac.PermCreateEdit)).Post("/versions", a.handleCreateVersion)
					})
				})
			})
		})
	}

	r.Route("/v1", mountV1)
	r.Route("/api/v1", mountV1)

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// workspaceContextMiddleware resolves {ws} param (UUID or slug), verifies membership, and sets Workspace & Role.
func (a *App) workspaceContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsParam := chi.URLParam(r, "ws")
		p, ok := auth.GetPrincipal(r.Context())
		if !ok {
			apierr.Render(w, apierr.Unauthorized("authentication required"))
			return
		}

		var wsRow dbgen.Workspace
		var err error

		wsUUID, parseErr := uuid.Parse(wsParam)
		if parseErr == nil {
			wsRow, err = a.queries.GetWorkspaceByID(r.Context(), wsUUID)
		} else {
			wsRow, err = a.queries.GetWorkspaceBySlug(r.Context(), wsParam)
		}

		if err != nil {
			// Cross-tenant or non-existent returns 404
			apierr.Render(w, apierr.NotFound("workspace not found"))
			return
		}

		// Verify membership
		member, err := a.queries.GetWorkspaceMember(r.Context(), dbgen.GetWorkspaceMemberParams{
			WorkspaceID: wsRow.ID,
			UserID:      p.UserID,
		})
		if err != nil {
			// Non-member returns 404 so existence does not leak
			apierr.Render(w, apierr.NotFound("workspace not found"))
			return
		}

		ws := &workspace.Workspace{
			ID:        wsRow.ID,
			Name:      wsRow.Name,
			Slug:      wsRow.Slug,
			OwnerID:   wsRow.OwnerID,
			PlanID:    wsRow.PlanID,
			Timezone:  wsRow.Timezone,
			CreatedAt: wsRow.CreatedAt,
			UpdatedAt: wsRow.UpdatedAt,
		}

		ctx := workspace.WithWorkspace(r.Context(), ws)
		ctx = rbac.WithRole(ctx, rbac.Role(member.Role))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// --- Auth Handlers ---

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Render(w, apierr.BadRequest("invalid_json", "malformed request body"))
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		apierr.Render(w, apierr.BadRequest("invalid_email", "valid email is required"))
		return
	}

	if err := auth.ValidatePasswordStrength(req.Password); err != nil {
		apierr.Render(w, apierr.BadRequest("weak_password", err.Error()))
		return
	}

	hash, err := crypto.HashPassword(req.Password, crypto.UserPasswordConfig)
	if err != nil {
		apierr.Render(w, apierr.Internal("failed to hash password"))
		return
	}

	userID := idgen.NewUUID()
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.Split(req.Email, "@")[0]
	}

	u, err := a.queries.CreateUser(r.Context(), dbgen.CreateUserParams{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: &hash,
		Name:         name,
		Locale:       "en",
		Timezone:     "UTC",
		IsStaff:      false,
	})
	if err != nil {
		apierr.Render(w, apierr.Conflict("email_exists", "an account with this email already exists"))
		return
	}

	// Create default personal workspace
	wsID := idgen.NewUUID()
	wsSlug := workspace.Slugify(name)
	if wsSlug == "" {
		wsSlug = "workspace-" + userID.String()[:8]
	}

	ws, err := a.queries.CreateWorkspace(r.Context(), dbgen.CreateWorkspaceParams{
		ID:       wsID,
		Name:     name + "'s Workspace",
		Slug:     wsSlug,
		OwnerID:  userID,
		PlanID:   "free",
		Timezone: "UTC",
	})
	if err != nil {
		apierr.Render(w, apierr.Internal("failed to create default workspace"))
		return
	}

	// Add owner membership
	_ = a.queries.AddWorkspaceMember(r.Context(), dbgen.AddWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		UserID:      userID,
		Role:        string(rbac.RoleOwner),
	})

	// Create session
	sessID := idgen.NewUUID()
	famID := idgen.NewUUID()
	plainRefresh, refreshHash, _ := auth.GenerateRandomToken(32)
	ip, ua, _, _ := scan.ParseRequestFacts(r)
	ipPrefix := audit.TruncateIPToPrefix(ip)

	_, err = a.queries.CreateSession(r.Context(), dbgen.CreateSessionParams{
		ID:               sessID,
		UserID:           userID,
		FamilyID:         famID,
		RefreshTokenHash: refreshHash,
		UserAgent:        &ua,
		IpPrefix:         ipPrefix,
		ExpiresAt:        time.Now().UTC().Add(auth.RefreshTokenDuration),
	})
	if err != nil {
		apierr.Render(w, apierr.Internal("failed to create session"))
		return
	}

	accessToken, _ := a.tm.CreateAccessToken(userID, sessID)
	csrfToken, _, _ := auth.GenerateRandomToken(16)

	auth.SetAuthCookies(w, accessToken, plainRefresh, csrfToken, a.cfg.CookieSecure)

	httpx.JSON(w, http.StatusCreated, map[string]interface{}{
		"user": map[string]interface{}{
			"id":    u.ID,
			"email": u.Email,
			"name":  u.Name,
		},
		"workspace": map[string]interface{}{
			"id":   ws.ID,
			"slug": ws.Slug,
			"name": ws.Name,
		},
		"token": accessToken,
	})
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Render(w, apierr.BadRequest("invalid_json", "malformed request body"))
		return
	}

	u, err := a.queries.GetUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil || u.PasswordHash == nil {
		apierr.Render(w, apierr.Unauthorized("invalid email or password"))
		return
	}

	ok, err := crypto.VerifyPassword(req.Password, *u.PasswordHash)
	if err != nil || !ok {
		apierr.Render(w, apierr.Unauthorized("invalid email or password"))
		return
	}

	// Create session
	sessID := idgen.NewUUID()
	famID := idgen.NewUUID()
	plainRefresh, refreshHash, _ := auth.GenerateRandomToken(32)
	ip, ua, _, _ := scan.ParseRequestFacts(r)
	ipPrefix := audit.TruncateIPToPrefix(ip)

	_, err = a.queries.CreateSession(r.Context(), dbgen.CreateSessionParams{
		ID:               sessID,
		UserID:           u.ID,
		FamilyID:         famID,
		RefreshTokenHash: refreshHash,
		UserAgent:        &ua,
		IpPrefix:         ipPrefix,
		ExpiresAt:        time.Now().UTC().Add(auth.RefreshTokenDuration),
	})
	if err != nil {
		apierr.Render(w, apierr.Internal("failed to create session"))
		return
	}

	accessToken, _ := a.tm.CreateAccessToken(u.ID, sessID)
	csrfToken, _, _ := auth.GenerateRandomToken(16)

	auth.SetAuthCookies(w, accessToken, plainRefresh, csrfToken, a.cfg.CookieSecure)

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":    u.ID,
			"email": u.Email,
			"name":  u.Name,
		},
		"token": accessToken,
	})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.GetPrincipal(r.Context())
	if ok {
		_ = a.queries.RevokeSession(r.Context(), p.SessionID)
	}
	auth.ClearAuthCookies(w, a.cfg.CookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var tokenStr string
	if cookie, err := r.Cookie(auth.RefreshCookieName); err == nil && cookie.Value != "" {
		tokenStr = cookie.Value
	}
	if tokenStr == "" {
		tokenStr = r.Header.Get("X-Refresh-Token")
	}
	if tokenStr == "" {
		apierr.Render(w, apierr.Unauthorized("refresh token required"))
		return
	}

	tokenHash := auth.HashToken(tokenStr)
	sess, err := a.queries.GetSessionByHash(r.Context(), tokenHash)
	if err != nil {
		apierr.Render(w, apierr.Unauthorized("invalid refresh token"))
		return
	}

	// Reuse detection: if already replaced or revoked, revoke family!
	if sess.ReplacedBy.Valid || sess.RevokedAt.Valid {
		_ = a.queries.RevokeSessionFamily(r.Context(), sess.FamilyID)
		auth.ClearAuthCookies(w, a.cfg.CookieSecure)
		apierr.Render(w, apierr.Unauthorized("refresh token reuse detected; all sessions revoked"))
		return
	}

	// Rotate session
	newSessID := idgen.NewUUID()
	plainRefresh, newHash, _ := auth.GenerateRandomToken(32)
	ip, ua, _, _ := scan.ParseRequestFacts(r)
	ipPrefix := audit.TruncateIPToPrefix(ip)

	_, err = a.queries.CreateSession(r.Context(), dbgen.CreateSessionParams{
		ID:               newSessID,
		UserID:           sess.UserID,
		FamilyID:         sess.FamilyID,
		RefreshTokenHash: newHash,
		UserAgent:        &ua,
		IpPrefix:         ipPrefix,
		ExpiresAt:        time.Now().UTC().Add(auth.RefreshTokenDuration),
	})
	if err != nil {
		apierr.Render(w, apierr.Internal("failed to rotate session"))
		return
	}

	_, _ = a.queries.RotateSession(r.Context(), dbgen.RotateSessionParams{
		ID:         sess.ID,
		ReplacedBy: pgtype.UUID{Bytes: newSessID, Valid: true},
	})

	accessToken, _ := a.tm.CreateAccessToken(sess.UserID, newSessID)
	csrfToken, _, _ := auth.GenerateRandomToken(16)

	auth.SetAuthCookies(w, accessToken, plainRefresh, csrfToken, a.cfg.CookieSecure)

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"token": accessToken,
	})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.GetPrincipal(r.Context())
	u, err := a.queries.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		apierr.Render(w, apierr.NotFound("user not found"))
		return
	}

	workspaces, _ := a.queries.ListWorkspacesForUser(r.Context(), p.UserID)

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":         u.ID,
			"email":      u.Email,
			"name":       u.Name,
			"avatar_url": u.AvatarUrl,
			"locale":     u.Locale,
			"timezone":   u.Timezone,
		},
		"workspaces": workspaces,
	})
}

// --- Workspace Handlers ---

func (a *App) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.GetPrincipal(r.Context())
	list, err := a.queries.ListWorkspacesForUser(r.Context(), p.UserID)
	if err != nil {
		apierr.Render(w, apierr.Internal("failed to list workspaces"))
		return
	}
	httpx.JSON(w, http.StatusOK, list)
}

func (a *App) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.GetPrincipal(r.Context())

	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Render(w, apierr.BadRequest("invalid_json", "malformed request"))
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		apierr.Render(w, apierr.BadRequest("missing_name", "workspace name is required"))
		return
	}

	slug := req.Slug
	if slug == "" {
		slug = workspace.Slugify(name)
	}
	if err := workspace.ValidateSlug(slug); err != nil {
		apierr.Render(w, apierr.BadRequest("invalid_slug", err.Error()))
		return
	}

	wsID := idgen.NewUUID()
	ws, err := a.queries.CreateWorkspace(r.Context(), dbgen.CreateWorkspaceParams{
		ID:       wsID,
		Name:     name,
		Slug:     slug,
		OwnerID:  p.UserID,
		PlanID:   "free",
		Timezone: "UTC",
	})
	if err != nil {
		apierr.Render(w, apierr.Conflict("slug_exists", "a workspace with this slug already exists"))
		return
	}

	_ = a.queries.AddWorkspaceMember(r.Context(), dbgen.AddWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		UserID:      p.UserID,
		Role:        string(rbac.RoleOwner),
	})

	httpx.JSON(w, http.StatusCreated, ws)
}

func (a *App) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, _ := workspace.GetWorkspace(r.Context())
	role, _ := rbac.GetRole(r.Context())
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"workspace": ws,
		"role":      role,
	})
}

func (a *App) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, _ := workspace.GetWorkspace(r.Context())
	var req struct {
		Name     *string `json:"name"`
		Timezone *string `json:"timezone"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		ws.Name = strings.TrimSpace(*req.Name)
	}
	if req.Timezone != nil && strings.TrimSpace(*req.Timezone) != "" {
		ws.Timezone = strings.TrimSpace(*req.Timezone)
	}

	httpx.JSON(w, http.StatusOK, ws)
}

func (a *App) handleGetEntitlements(w http.ResponseWriter, r *http.Request) {
	ws, _ := workspace.GetWorkspace(r.Context())
	limits := entitlements.GetLimits(ws.PlanID)
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"plan":   ws.PlanID,
		"limits": limits,
	})
}

func (a *App) handleListMembers(w http.ResponseWriter, r *http.Request) {
	ws, _ := workspace.GetWorkspace(r.Context())
	members, err := a.queries.ListWorkspaceMembers(r.Context(), ws.ID)
	if err != nil {
		apierr.Render(w, apierr.Internal("failed to list members"))
		return
	}
	httpx.JSON(w, http.StatusOK, members)
}

func (a *App) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	ws, _ := workspace.GetWorkspace(r.Context())
	p, _ := auth.GetPrincipal(r.Context())

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Render(w, apierr.BadRequest("invalid_json", "malformed request"))
		return
	}

	invitedRole := rbac.Role(strings.ToLower(req.Role))
	if err := workspace.ValidateInviteRole(invitedRole); err != nil {
		apierr.Render(w, apierr.BadRequest("invalid_role", err.Error()))
		return
	}

	plainToken, tokenHash, _ := workspace.GenerateInviteToken()
	inviteID := idgen.NewUUID()
	expiresAt := time.Now().UTC().Add(workspace.InviteDuration)

	invite, err := a.queries.CreateInvite(r.Context(), dbgen.CreateInviteParams{
		ID:          inviteID,
		WorkspaceID: ws.ID,
		Email:       strings.ToLower(strings.TrimSpace(req.Email)),
		Role:        string(invitedRole),
		TokenHash:   tokenHash,
		InvitedBy:   p.UserID,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		apierr.Render(w, apierr.Conflict("invite_exists", "an active invite already exists for this email"))
		return
	}

	_ = a.emailSender.SendInvite(r.Context(), invite.Email, "Team Admin", ws.Name, plainToken, a.cfg.AppBaseURL)

	httpx.JSON(w, http.StatusCreated, invite)
}

// --- QR Code Handlers ---

func (a *App) handleListQRCodes(w http.ResponseWriter, r *http.Request) {
	ws, _ := workspace.GetWorkspace(r.Context())
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")

	var searchPtr, statusPtr *string
	if search != "" {
		searchPtr = &search
	}
	if status != "" {
		statusPtr = &status
	}

	codes, err := a.queries.ListQRCodes(r.Context(), dbgen.ListQRCodesParams{
		WorkspaceID: ws.ID,
		Search:      searchPtr,
		Status:      statusPtr,
		RowLimit:    100,
	})
	if err != nil {
		apierr.Render(w, apierr.Internal("failed to list qr codes"))
		return
	}

	httpx.JSON(w, http.StatusOK, codes)
}

type CreateQRRequest struct {
	Name           string          `json:"name"`
	Mode           string          `json:"mode"`
	ContentType    string          `json:"content_type"`
	DestinationURL string          `json:"destination_url"`
	Design         *qr.DesignV1    `json:"design"`
	StaticPayload  string          `json:"static_payload"`
	StaticContent  json.RawMessage `json:"static_content"`
}

func (a *App) handleCreateQRCode(w http.ResponseWriter, r *http.Request) {
	ws, _ := workspace.GetWorkspace(r.Context())
	p, _ := auth.GetPrincipal(r.Context())

	var req CreateQRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Render(w, apierr.BadRequest("invalid_json", "malformed request"))
		return
	}

	mode := qr.Mode(strings.ToLower(strings.TrimSpace(req.Mode)))
	if mode != qr.ModeDynamic && mode != qr.ModeStatic {
		mode = qr.ModeDynamic
	}

	// Dynamic code limits
	if mode == qr.ModeDynamic {
		count, err := a.queries.CountActiveDynamicQRCodes(r.Context(), ws.ID)
		if err == nil {
			if err := entitlements.CheckDynamicCodeLimit(ws.PlanID, int(count)); err != nil {
				apierr.Render(w, apierr.LimitReached(err.Error()))
				return
			}
		}
	}

	// Validate design
	design := qr.DefaultDesign()
	if req.Design != nil {
		design = *req.Design
	}
	designBytes, designHash, err := qr.CanonicalDesignJSON(&design)
	if err != nil {
		apierr.Render(w, apierr.BadRequest("invalid_design", err.Error()))
		return
	}

	// Domain
	domains, err := a.queries.ListActiveDomains(r.Context())
	var domainID uuid.UUID
	if err == nil && len(domains) > 0 {
		domainID = domains[0].ID
	} else {
		domainID = idgen.NewUUID()
	}

	// Generate Crockford Base32 short code
	code, err := shortcode.Generate()
	if err != nil {
		apierr.Render(w, apierr.Internal("failed to generate short code"))
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "QR " + code
	}

	qrID := idgen.NewUUID()
	contentType := req.ContentType
	if contentType == "" {
		contentType = "url"
	}

	var staticPayload *string
	if req.StaticPayload != "" {
		staticPayload = &req.StaticPayload
	}

	qrCode, err := a.queries.CreateQRCode(r.Context(), dbgen.CreateQRCodeParams{
		ID:            qrID,
		WorkspaceID:   ws.ID,
		DomainID:      pgtype.UUID{Bytes: domainID, Valid: true},
		ShortCode:     &code,
		Mode:          string(mode),
		ContentType:   contentType,
		Name:          name,
		Status:        string(qr.StatusActive),
		Design:        designBytes,
		DesignHash:    []byte(designHash),
		StaticPayload: staticPayload,
		StaticContent: req.StaticContent,
	})
	if err != nil {
		apierr.Render(w, apierr.Internal("failed to create qr code"))
		return
	}

	// Create initial version for dynamic code
	if mode == qr.ModeDynamic && req.DestinationURL != "" {
		// Validate destination URL
		policy := urlsafety.Policy{
			RequireHTTPS:    false,
			AllowShorteners: false,
			OwnHosts:        []string{a.cfg.PlatformShortDomain},
		}
		normURL, err := urlsafety.Validate(req.DestinationURL, policy)
		if err != nil {
			apierr.Render(w, apierr.BadRequest("invalid_destination", err.Error()))
			return
		}

		verID := idgen.NewUUID()
		ver, err := a.queries.CreateQRVersion(r.Context(), dbgen.CreateQRVersionParams{
			ID:              verID,
			QrCodeID:        qrID,
			VersionNo:       1,
			DestinationKind: string(version.DestinationKindURL),
			DestinationUrl:  &normURL,
			HostedPage:      nil,
			Rules:           json.RawMessage("[]"),
			Utm:             json.RawMessage("{}"),
			EffectiveAt:     time.Now().UTC(),
			CreatedBy:       pgtype.UUID{Bytes: p.UserID, Valid: true},
		})
		if err == nil {
			_, _ = a.queries.UpdateQRCode(r.Context(), dbgen.UpdateQRCodeParams{
				ID:               qrID,
				WorkspaceID:      ws.ID,
				CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
			})
		}
	}

	// Write audit log
	_ = audit.Record(r.Context(), a.queries, audit.Entry{
		WorkspaceID: &ws.ID,
		ActorType:   audit.ActorUser,
		ActorID:     &p.UserID,
		Action:      "qr.created",
		TargetType:  "qr_code",
		TargetID:    &qrID,
	})

	httpx.JSON(w, http.StatusCreated, qrCode)
}

func (a *App) handleGetQRCode(w http.ResponseWriter, r *http.Request) {
	ws, _ := workspace.GetWorkspace(r.Context())
	idStr := chi.URLParam(r, "id")
	qrID, err := uuid.Parse(idStr)
	if err != nil {
		apierr.Render(w, apierr.NotFound("qr code not found"))
		return
	}

	code, err := a.queries.GetQRCode(r.Context(), dbgen.GetQRCodeParams{
		ID:          qrID,
		WorkspaceID: ws.ID,
	})
	if err != nil {
		apierr.Render(w, apierr.NotFound("qr code not found"))
		return
	}

	var latestVersion *dbgen.QrVersion
	if ver, err := a.queries.GetLatestQRVersion(r.Context(), qrID); err == nil {
		latestVersion = &ver
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"qr":      code,
		"version": latestVersion,
	})
}

func (a *App) handleUpdateQRCode(w http.ResponseWriter, r *http.Request) {
	ws, _ := workspace.GetWorkspace(r.Context())
	idStr := chi.URLParam(r, "id")
	qrID, err := uuid.Parse(idStr)
	if err != nil {
		apierr.Render(w, apierr.NotFound("qr code not found"))
		return
	}

	var req struct {
		Name   *string      `json:"name"`
		Design *qr.DesignV1 `json:"design"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Render(w, apierr.BadRequest("invalid_json", "malformed request"))
		return
	}

	var designBytes []byte
	var designHash string
	if req.Design != nil {
		var err error
		designBytes, designHash, err = qr.CanonicalDesignJSON(req.Design)
		if err != nil {
			apierr.Render(w, apierr.BadRequest("invalid_design", err.Error()))
			return
		}
	}

	updated, err := a.queries.UpdateQRCode(r.Context(), dbgen.UpdateQRCodeParams{
		ID:          qrID,
		WorkspaceID: ws.ID,
		Name:        req.Name,
		Design:      designBytes,
		DesignHash:  []byte(designHash),
	})
	if err != nil {
		apierr.Render(w, apierr.NotFound("qr code not found"))
		return
	}

	httpx.JSON(w, http.StatusOK, updated)
}

func (a *App) handleSetStatus(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, _ := workspace.GetWorkspace(r.Context())
		idStr := chi.URLParam(r, "id")
		qrID, err := uuid.Parse(idStr)
		if err != nil {
			apierr.Render(w, apierr.NotFound("qr code not found"))
			return
		}

		updated, err := a.queries.UpdateQRCode(r.Context(), dbgen.UpdateQRCodeParams{
			ID:          qrID,
			WorkspaceID: ws.ID,
			Status:      &status,
		})
		if err != nil {
			apierr.Render(w, apierr.NotFound("qr code not found"))
			return
		}

		httpx.JSON(w, http.StatusOK, updated)
	}
}

func (a *App) handleDeleteQRCode(w http.ResponseWriter, r *http.Request) {
	ws, _ := workspace.GetWorkspace(r.Context())
	idStr := chi.URLParam(r, "id")
	qrID, err := uuid.Parse(idStr)
	if err != nil {
		apierr.Render(w, apierr.NotFound("qr code not found"))
		return
	}

	if err := a.queries.SoftDeleteQRCode(r.Context(), dbgen.SoftDeleteQRCodeParams{
		ID:          qrID,
		WorkspaceID: ws.ID,
	}); err != nil {
		apierr.Render(w, apierr.NotFound("qr code not found"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleListVersions(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	qrID, err := uuid.Parse(idStr)
	if err != nil {
		apierr.Render(w, apierr.NotFound("qr code not found"))
		return
	}

	versions, err := a.queries.ListQRVersions(r.Context(), qrID)
	if err != nil {
		apierr.Render(w, apierr.Internal("failed to list versions"))
		return
	}
	httpx.JSON(w, http.StatusOK, versions)
}

func (a *App) handleCreateVersion(w http.ResponseWriter, r *http.Request) {
	ws, _ := workspace.GetWorkspace(r.Context())
	p, _ := auth.GetPrincipal(r.Context())
	idStr := chi.URLParam(r, "id")
	qrID, err := uuid.Parse(idStr)
	if err != nil {
		apierr.Render(w, apierr.NotFound("qr code not found"))
		return
	}

	var req struct {
		DestinationURL string `json:"destination_url"`
		ChangeNote     string `json:"change_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Render(w, apierr.BadRequest("invalid_json", "malformed request"))
		return
	}

	policy := urlsafety.Policy{
		RequireHTTPS:    false,
		AllowShorteners: false,
		OwnHosts:        []string{a.cfg.PlatformShortDomain},
	}
	normURL, err := urlsafety.Validate(req.DestinationURL, policy)
	if err != nil {
		apierr.Render(w, apierr.BadRequest("invalid_destination", err.Error()))
		return
	}

	latest, err := a.queries.GetLatestQRVersion(r.Context(), qrID)
	nextVerNo := int32(1)
	if err == nil {
		nextVerNo = latest.VersionNo + 1
	}

	verID := idgen.NewUUID()
	var changeNotePtr *string
	if req.ChangeNote != "" {
		changeNotePtr = &req.ChangeNote
	}

	ver, err := a.queries.CreateQRVersion(r.Context(), dbgen.CreateQRVersionParams{
		ID:              verID,
		QrCodeID:        qrID,
		VersionNo:       nextVerNo,
		DestinationKind: string(version.DestinationKindURL),
		DestinationUrl:  &normURL,
		HostedPage:      nil,
		Rules:           json.RawMessage("[]"),
		Utm:             json.RawMessage("{}"),
		EffectiveAt:     time.Now().UTC(),
		ChangeNote:      changeNotePtr,
		CreatedBy:       pgtype.UUID{Bytes: p.UserID, Valid: true},
	})
	if err != nil {
		apierr.Render(w, apierr.Internal("failed to create version"))
		return
	}

	_, _ = a.queries.UpdateQRCode(r.Context(), dbgen.UpdateQRCodeParams{
		ID:               qrID,
		WorkspaceID:      ws.ID,
		CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
	})

	httpx.JSON(w, http.StatusCreated, ver)
}
