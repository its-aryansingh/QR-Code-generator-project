package main

import (
	"log"
	"os"

	"io"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/qrapp/backend/internal/config"
	"github.com/qrapp/backend/internal/database"
	"github.com/qrapp/backend/internal/handlers"
	"github.com/qrapp/backend/internal/middleware"
	"github.com/qrapp/backend/internal/repository"
	"github.com/qrapp/backend/internal/services"
)

func main() {
	// Setup logging to file
	f, err := os.OpenFile("server.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(io.MultiWriter(f, os.Stdout))
	gin.DefaultWriter = io.MultiWriter(f, os.Stdout)
	// Load .env file (try multiple locations)
	if err := godotenv.Load(); err != nil {
		if err := godotenv.Load("../../.env"); err != nil {
			log.Println("No .env file found, using environment variables")
		}
	}

	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	qrRepo := repository.NewQRRecordRepository(db)
	scanRepo := repository.NewQRScanRepository(db)
	freeTierRepo := repository.NewFreeTierRepository(db)

	// Initialize services
	authService := services.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTExpiry, cfg.RefreshExpiry)
	qrService := services.NewQRService(qrRepo)
	analyticsService := services.NewAnalyticsService(scanRepo, qrRepo, services.NewIPAPIGeoIPService())

	// Enterprise: Initialize workspace repository & webhook service (needed by qr/redirect handlers)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	webhookService := services.NewWebhookService(workspaceRepo)
	workspaceHandler := handlers.NewWorkspaceHandler(workspaceRepo, userRepo, webhookService)
	bulkHandler := handlers.NewBulkHandler(qrService, workspaceRepo)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService)
	qrHandler := handlers.NewQRHandler(qrService, webhookService)
	publicHandler := handlers.NewPublicHandler(qrService, freeTierRepo, analyticsService)
	redirectHandler := handlers.NewRedirectHandler(qrService, qrRepo, scanRepo, analyticsService, webhookService)
	analyticsHandler := handlers.NewAnalyticsHandler(analyticsService, qrService)

	// Setup Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// Apply CORS middleware
	r.Use(middleware.CORS())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy", "version": "3.0.0"})
	})

	// Dynamic QR redirect (short URL)
	r.GET("/r/:code", redirectHandler.Redirect)

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Public routes (no auth, rate limited)
		public := v1.Group("/public")
		{
			public.POST("/generate", publicHandler.Generate)
			public.GET("/types", publicHandler.GetQRTypes)
			public.GET("/quota", publicHandler.GetRemainingQuota)
			public.GET("/analytics/:code", publicHandler.GetQRAnalytics)
		}

		// Auth routes (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.Refresh)
			auth.POST("/google", authHandler.GoogleLogin)
		}

		// Protected routes
		protected := v1.Group("")
		protected.Use(middleware.AuthRequired(cfg.JWTSecret))
		{
			// User routes
			user := protected.Group("/user")
			{
				user.GET("/profile", authHandler.GetProfile)
				user.PUT("/profile", authHandler.UpdateProfile)
			}

			// QR routes
			qr := protected.Group("/qr")
			{
				qr.POST("/generate", qrHandler.Generate)
				qr.GET("/history", qrHandler.History)
				qr.GET("/:id", qrHandler.GetByID)
				qr.PUT("/:id", qrHandler.Update)
				qr.DELETE("/:id", qrHandler.Delete)
				qr.GET("/:id/analytics", analyticsHandler.GetQRAnalytics)
				qr.GET("/:id/analytics/export", analyticsHandler.ExportAnalytics)
				qr.GET("/:id/scans", analyticsHandler.GetRecentScans)
			}

			// Analytics routes
			analytics := protected.Group("/analytics")
			{
				analytics.GET("/summary", analyticsHandler.GetUserSummary)
			}

			// ==========================================
			// ENTERPRISE: Workspace routes
			// ==========================================
			ws := protected.Group("/workspaces")
			{
				ws.POST("", workspaceHandler.CreateWorkspace)
				ws.GET("", workspaceHandler.GetWorkspaces)
				ws.GET("/:id", workspaceHandler.GetWorkspace)
				ws.PUT("/:id", workspaceHandler.UpdateWorkspace)
				ws.DELETE("/:id", workspaceHandler.DeleteWorkspace)

				// Members
				ws.POST("/:id/members/invite", workspaceHandler.InviteMember)
				ws.GET("/:id/members", workspaceHandler.GetMembers)
				ws.PUT("/:id/members/:memberID/role", workspaceHandler.UpdateMemberRole)
				ws.DELETE("/:id/members/:memberID", workspaceHandler.RemoveMember)

				// Folders
				ws.POST("/:id/folders", workspaceHandler.CreateFolder)
				ws.GET("/:id/folders", workspaceHandler.GetFolders)
				ws.PUT("/:id/folders/:folderID", workspaceHandler.UpdateFolder)
				ws.DELETE("/:id/folders/:folderID", workspaceHandler.DeleteFolder)

				// QR Codes (workspace-scoped)
				ws.GET("/:id/qr", workspaceHandler.GetWorkspaceQRCodes)
				ws.POST("/:id/qr/bulk-move", workspaceHandler.BulkMoveToFolder)
				ws.POST("/:id/qr/bulk-delete", workspaceHandler.BulkDeleteQR)

				// Bulk CSV generation → ZIP download
				ws.POST("/:id/bulk", bulkHandler.BulkCSVGenerate)
				ws.POST("/:id/bulk/preview", bulkHandler.BulkCSVPreview)

				// Audit Logs
				ws.GET("/:id/audit-logs", workspaceHandler.GetAuditLogs)

				// Webhooks
				ws.POST("/:id/webhooks", workspaceHandler.CreateWebhook)
				ws.GET("/:id/webhooks", workspaceHandler.GetWebhooks)
				ws.DELETE("/:id/webhooks/:webhookID", workspaceHandler.DeleteWebhook)
				ws.GET("/:id/webhooks/:webhookID/logs", workspaceHandler.GetWebhookLogs)
				ws.POST("/:id/webhooks/:webhookID/test", workspaceHandler.TestWebhook)

				// Analytics (workspace-scoped)
				ws.GET("/:id/analytics", workspaceHandler.GetWorkspaceAnalytics)
				ws.GET("/:id/analytics/heatmap", workspaceHandler.GetWorkspaceHeatmap)
				ws.GET("/:id/analytics/top-qr", workspaceHandler.GetTopQRCodes)

				// White-labeling & SSO
				whiteLabelHandler := handlers.NewWhiteLabelHandler(workspaceRepo)
				ws.GET("/:id/branding", whiteLabelHandler.GetBranding)
				ws.PUT("/:id/branding", whiteLabelHandler.UpdateBranding)
				ws.GET("/:id/sso", whiteLabelHandler.GetSSOConfig)
				ws.PUT("/:id/sso", whiteLabelHandler.UpdateSSOConfig)

				// Lead Capture Pages
				leadHandler := handlers.NewLeadCaptureHandler(workspaceRepo)
				ws.POST("/:id/pages", leadHandler.CreatePage)
				ws.GET("/:id/pages", leadHandler.GetPages)
				ws.DELETE("/:id/pages/:pageID", leadHandler.DeletePage)
				ws.GET("/:id/leads", leadHandler.GetLeads)

				// Reports & Export
				reportHandler := handlers.NewReportHandler(workspaceRepo)
				ws.GET("/:id/export/analytics", reportHandler.ExportAnalyticsCSV)
				ws.GET("/:id/export/qr-codes", reportHandler.ExportQRCodesCSV)
				ws.GET("/:id/export/leads", reportHandler.ExportLeadsCSV)
				ws.GET("/:id/report", reportHandler.GenerateReport)
			}

			// Invite acceptance (outside workspace group since token-based)
			protected.POST("/invites/:token/accept", workspaceHandler.AcceptInvite)

			// Public lead capture page routes (no auth)
			leadPublicHandler := handlers.NewLeadCaptureHandler(workspaceRepo)
			v1.GET("/pages/:slug", leadPublicHandler.GetPageBySlug)
			v1.POST("/pages/:slug/submit", leadPublicHandler.SubmitLead)
		}

		// API routes (API key auth)
		apiHandler := handlers.NewAPIHandler(qrService, userRepo)
		apiRoutes := v1.Group("/api")
		apiRoutes.Use(middleware.APIKeyAuth(db))
		{
			apiRoutes.POST("/generate", apiHandler.Generate)
			apiRoutes.POST("/bulk", apiHandler.BulkGenerate)
			apiRoutes.GET("/key", apiHandler.GetAPIKey)
		}

		// API key management (JWT auth)
		apiManagement := v1.Group("/api")
		apiManagement.Use(middleware.AuthRequired(cfg.JWTSecret))
		{
			apiManagement.POST("/key/regenerate", apiHandler.RegenerateAPIKey)
		}

		// Payment routes
		stripeService := services.NewStripeService()
		paymentHandler := handlers.NewPaymentHandler(stripeService, userRepo)

		// Public payment routes
		payments := v1.Group("/payments")
		{
			payments.GET("/plans", paymentHandler.GetPlans)
			payments.POST("/webhook", paymentHandler.Webhook) // Stripe webhook
		}

		// Protected payment routes
		paymentsProtected := v1.Group("/payments")
		paymentsProtected.Use(middleware.AuthRequired(cfg.JWTSecret))
		{
			paymentsProtected.POST("/checkout", paymentHandler.CreateCheckout)
			paymentsProtected.GET("/subscription", paymentHandler.GetSubscription)
			paymentsProtected.POST("/cancel", paymentHandler.CancelSubscription)
		}
	}

	// Start server
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 QRApp Server v2.0.0 starting on port %s", port)
	log.Printf("📊 Public API: /api/v1/public/generate (Free: 10 QR/IP/day)")
	log.Printf("🔗 Dynamic QR Redirect: /r/:code")

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
		os.Exit(1)
	}
}
