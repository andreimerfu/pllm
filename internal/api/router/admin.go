package router

import (
	"log"
	"net/http"

	"github.com/amerfu/pllm/internal/core/auth"
	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/api/handlers"
	"github.com/amerfu/pllm/internal/api/handlers/admin"
	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	"github.com/amerfu/pllm/internal/services/data/budget"
	"github.com/amerfu/pllm/internal/services/integrations/guardrails"
	"github.com/amerfu/pllm/internal/services/integrations/team"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type AdminRouterConfig struct {
	Config              *config.Config
	Logger              *zap.Logger
	DB                  *gorm.DB
	AuthService         *auth.AuthService
	MasterKeyService    *auth.MasterKeyService
	BudgetService       budget.Service
	GuardrailsExecutor  *guardrails.Executor
	ModelManager        interface {
		GetModelStats() map[string]interface{}
	}
}

// NewAdminSubRouter creates admin routes to be mounted on the main router
// This doesn't include middleware as it will inherit from the parent router
func NewAdminSubRouter(cfg *AdminRouterConfig) http.Handler {
	r := chi.NewRouter()

	// Initialize services
	teamService := team.NewTeamService(cfg.DB)

	// Initialize handlers
	authHandler := admin.NewAuthHandler(cfg.Logger, cfg.MasterKeyService, cfg.AuthService, cfg.DB)
	oauthHandler := admin.NewOAuthHandler(
		cfg.Logger,
		cfg.DB,
		"http://dex:5556/dex",
		"pllm-web",
		"pllm-web-secret",
	)
	userHandler := admin.NewUserHandler(cfg.Logger, cfg.DB)
	teamHandler := admin.NewTeamHandler(cfg.Logger, teamService, cfg.DB, cfg.BudgetService)
	keyHandler := admin.NewKeyHandler(cfg.Logger, cfg.DB, cfg.BudgetService)
	analyticsHandler := admin.NewAnalyticsHandler(cfg.Logger, cfg.DB, cfg.ModelManager)
	systemHandler := admin.NewSystemHandler(cfg.Logger, cfg.DB)
	dashboardHandler := handlers.NewDashboardHandler(cfg.DB, cfg.Logger)
	guardrailsHandler := admin.NewGuardrailsHandler(cfg.Logger, cfg.Config, cfg.GuardrailsExecutor)

	// Initialize auth middleware
	authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthConfig{
		Logger:           cfg.Logger,
		AuthService:      cfg.AuthService,
		MasterKeyService: cfg.MasterKeyService,
		RequireAuth:      false, // Allow public endpoints
	})

	// Auth endpoints (public - no auth required)
	r.Post("/auth/login", authHandler.Login)
	r.Post("/auth/master-key", authHandler.MasterKeyLogin) // Master key authentication
	r.Get("/auth/validate", authHandler.Validate)
	r.Get("/auth/config", systemHandler.GetAuthConfig) // Get available auth options
	r.Post("/auth/token", oauthHandler.TokenExchange)
	r.Get("/auth/userinfo", oauthHandler.UserInfo)

	// Permission endpoint (requires authentication)
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.Get("/auth/permissions", authHandler.GetPermissions)
	})

	// Stats endpoint (commonly accessed by dashboard)
	r.Get("/stats", analyticsHandler.GetStats)
	r.Get("/dashboard", analyticsHandler.GetDashboard)

	// Dashboard metrics endpoints
	r.Get("/dashboard/metrics", dashboardHandler.GetDashboardMetrics)
	r.Get("/dashboard/models/{model}", dashboardHandler.GetModelMetrics)
	r.Get("/dashboard/models/{model}/trends", dashboardHandler.GetModelTrends)
	r.Get("/dashboard/usage-trends", dashboardHandler.GetUsageTrends)

	// Protected admin routes - require authentication and admin role
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.Use(authMiddleware.RequireAdmin)

		// User management
		r.Route("/users", func(r chi.Router) {
			r.Get("/", userHandler.ListUsers)
			r.Post("/", userHandler.CreateUser)
			r.Get("/{userID}", userHandler.GetUser)
			r.Put("/{userID}", userHandler.UpdateUser)
			r.Delete("/{userID}", userHandler.DeleteUser)
			r.Get("/{userID}/stats", userHandler.GetUserStats)
			r.Post("/{userID}/reset-budget", userHandler.ResetUserBudget)
		})

		// Team management
		r.Route("/teams", func(r chi.Router) {
			r.Get("/", teamHandler.ListTeams)
			r.Post("/", teamHandler.CreateTeam)
			r.Get("/{teamID}", teamHandler.GetTeam)
			r.Put("/{teamID}", teamHandler.UpdateTeam)
			r.Delete("/{teamID}", teamHandler.DeleteTeam)
			r.Post("/{teamID}/members", teamHandler.AddMember)
			r.Put("/{teamID}/members/{memberID}", teamHandler.UpdateMember)
			r.Delete("/{teamID}/members/{memberID}", teamHandler.RemoveMember)
			r.Get("/{teamID}/stats", teamHandler.GetTeamStats)
		})

		// Virtual Keys management
		r.Route("/keys", func(r chi.Router) {
			r.Get("/", keyHandler.ListKeys)
			r.Post("/", keyHandler.CreateKey)
			r.Post("/validate", keyHandler.ValidateKey)
			r.Get("/{keyID}", keyHandler.GetKey)
			r.Put("/{keyID}", keyHandler.UpdateKey)
			r.Delete("/{keyID}", keyHandler.DeleteKey)
			r.Post("/{keyID}/revoke", keyHandler.RevokeKey)
			r.Get("/{keyID}/stats", keyHandler.GetKeyStats)
			r.Get("/{keyID}/usage", keyHandler.GetKeyUsage)
		})

		// Analytics
		r.Route("/analytics", func(r chi.Router) {
			r.Get("/budget", analyticsHandler.GetBudgetSummary)
			r.Get("/user-breakdown", analyticsHandler.GetUserBreakdown)
			r.Get("/team-user-breakdown", analyticsHandler.GetTeamUserBreakdown)
			r.Get("/usage", analyticsHandler.GetUsage)
			r.Get("/usage/hourly", analyticsHandler.GetHourlyUsage)
			r.Get("/usage/daily", analyticsHandler.GetDailyUsage)
			r.Get("/usage/monthly", analyticsHandler.GetMonthlyUsage)
			r.Get("/costs", analyticsHandler.GetCosts)
			r.Get("/costs/breakdown", analyticsHandler.GetCostBreakdown)
			r.Get("/performance", analyticsHandler.GetPerformance)
			r.Get("/errors", analyticsHandler.GetErrors)
			r.Get("/cache", analyticsHandler.GetCacheStats)
			// Historical metrics endpoints
			r.Get("/historical/model-health", analyticsHandler.GetHistoricalModelHealth)
			r.Get("/historical/system-metrics", analyticsHandler.GetHistoricalSystemMetrics)
			r.Get("/historical/model-latencies", analyticsHandler.GetHistoricalModelLatencies)
		})

		// System management
		r.Route("/system", func(r chi.Router) {
			r.Get("/config", systemHandler.GetConfig)
			r.Put("/config", systemHandler.UpdateConfig)
			r.Get("/health", systemHandler.GetSystemHealth)
			r.Get("/logs", systemHandler.GetLogs)
			r.Get("/audit", systemHandler.GetAuditLogs)
			r.Post("/cache/clear", systemHandler.ClearCache)
			r.Post("/maintenance", systemHandler.SetMaintenance)
		})

		// Settings
		r.Route("/settings", func(r chi.Router) {
			r.Get("/", systemHandler.GetSettings)
			r.Put("/", systemHandler.UpdateSettings)
			r.Get("/rate-limits", systemHandler.GetRateLimits)
			r.Put("/rate-limits", systemHandler.UpdateRateLimits)
			r.Get("/cache", systemHandler.GetCacheSettings)
			r.Put("/cache", systemHandler.UpdateCacheSettings)
		})

		// Guardrails management
		r.Route("/guardrails", func(r chi.Router) {
			r.Get("/", guardrailsHandler.ListGuardrails)
			r.Get("/{guardrailName}", guardrailsHandler.GetGuardrail)
			r.Get("/{guardrailName}/stats", guardrailsHandler.GetGuardrailStats)
			r.Get("/{guardrailName}/health", guardrailsHandler.CheckGuardrailHealth)
			r.Post("/test", guardrailsHandler.TestGuardrail)
		})
	})

	// User self-service routes (authenticated users, not necessarily admin)
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)

		r.Route("/user", func(r chi.Router) {
			// Current user profile
			r.Get("/profile", func(w http.ResponseWriter, r *http.Request) {
				_, ok := middleware.GetUserID(r.Context())
				if !ok {
					http.Error(w, "User not found", http.StatusUnauthorized)
					return
				}
				// Profile endpoint - to be implemented
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotImplemented)
				if _, err := w.Write([]byte(`{"error": "Profile endpoint not yet implemented"}`)); err != nil {
					log.Printf("Failed to write profile error response: %v", err)
				}
			})

			// User's own keys
			r.Get("/keys", func(w http.ResponseWriter, r *http.Request) {
				userID, _ := middleware.GetUserID(r.Context())
				q := r.URL.Query()
				q.Add("user_id", userID.String())
				r.URL.RawQuery = q.Encode()
				keyHandler.ListKeys(w, r)
			})
			r.Post("/keys", keyHandler.CreateKey)

			// User's teams
			r.Get("/teams", func(w http.ResponseWriter, r *http.Request) {
				teamHandler.ListTeams(w, r)
			})
		})
	})

	return r
}

// NewAdminRouter creates a standalone admin router with its own middleware
// This is used when running admin API on a separate port
func NewAdminRouter(cfg *AdminRouterConfig) http.Handler {
	r := chi.NewRouter()

	// Basic middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.Logger(cfg.Logger))

	// CORS for admin UI
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.Config.CORS.AllowedOrigins,
		AllowedMethods:   cfg.Config.CORS.AllowedMethods,
		AllowedHeaders:   cfg.Config.CORS.AllowedHeaders,
		ExposedHeaders:   cfg.Config.CORS.ExposedHeaders,
		AllowCredentials: cfg.Config.CORS.AllowCredentials,
		MaxAge:           cfg.Config.CORS.MaxAge,
	}))

	// Initialize services
	teamService := team.NewTeamService(cfg.DB)
	// Budget service could be used for budget handlers if needed
	// Note: Not starting the budget service monitor to avoid excessive logging
	// It will be properly initialized when authentication is implemented
	// _ = budget.NewBudgetService(&budget.BudgetConfig{
	//     DB:            cfg.DB,
	//     CheckInterval: 5 * time.Minute,
	// })

	// Initialize handlers
	// authHandler := admin.NewAuthHandler(cfg.Logger, cfg.MasterKey) // Will be used when auth endpoints are enabled
	teamHandler := admin.NewTeamHandler(cfg.Logger, teamService, cfg.DB, cfg.BudgetService)
	keyHandler := admin.NewKeyHandler(cfg.Logger, cfg.DB, cfg.BudgetService)
	analyticsHandler := admin.NewAnalyticsHandler(cfg.Logger, cfg.DB, cfg.ModelManager)
	systemHandler := admin.NewSystemHandler(cfg.Logger, cfg.DB)
	guardrailsHandler := admin.NewGuardrailsHandler(cfg.Logger, cfg.Config, cfg.GuardrailsExecutor)

	// Initialize auth middleware
	authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthConfig{
		Logger:           cfg.Logger,
		AuthService:      cfg.AuthService,
		MasterKeyService: cfg.MasterKeyService,
		RequireAuth:      false, // Allow public endpoints
	})

	// Health check (public)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status": "ok", "service": "admin"}`)); err != nil {
			log.Printf("Failed to write admin health response: %v", err)
		}
	})

	// Public authentication endpoints
	// Authentication endpoints handled by auth service
	// r.Post("/api/auth/login", authHandler.Login)
	// r.Post("/api/auth/sso/callback", authHandler.LoginSSO)
	// r.Post("/api/auth/refresh", authHandler.RefreshToken)

	// Virtual key validation (requires any auth)
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.Post("/api/keys/validate", keyHandler.ValidateKey)
	})

	// Protected admin routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.Use(authMiddleware.RequireAdmin)

		r.Route("/api/admin", func(r chi.Router) {
			// Dashboard
			r.Get("/dashboard", analyticsHandler.GetDashboard)
			r.Get("/stats", analyticsHandler.GetStats)

			// User management - Not implemented yet
			// Users are managed through Teams in the LiteLLM model

			// Team management
			r.Route("/teams", func(r chi.Router) {
				r.Get("/", teamHandler.ListTeams)
				r.Post("/", teamHandler.CreateTeam)
				r.Get("/{teamID}", teamHandler.GetTeam)
				r.Put("/{teamID}", teamHandler.UpdateTeam)
				r.Delete("/{teamID}", teamHandler.DeleteTeam)
				r.Post("/{teamID}/members", teamHandler.AddMember)
				r.Put("/{teamID}/members/{memberID}", teamHandler.UpdateMember)
				r.Delete("/{teamID}/members/{memberID}", teamHandler.RemoveMember)
				r.Get("/{teamID}/stats", teamHandler.GetTeamStats)
			})

			// Virtual Keys management
			r.Route("/keys", func(r chi.Router) {
				r.Get("/", keyHandler.ListKeys)
				r.Post("/", keyHandler.CreateKey)
				r.Get("/{keyID}", keyHandler.GetKey)
				r.Put("/{keyID}", keyHandler.UpdateKey)
				r.Delete("/{keyID}", keyHandler.DeleteKey)
				r.Post("/{keyID}/revoke", keyHandler.RevokeKey)
				r.Get("/{keyID}/stats", keyHandler.GetKeyStats)
				r.Get("/{keyID}/usage", keyHandler.GetKeyUsage)
			})

			// Analytics
			r.Route("/analytics", func(r chi.Router) {
				r.Get("/budget", analyticsHandler.GetBudgetSummary)
				r.Get("/user-breakdown", analyticsHandler.GetUserBreakdown)
				r.Get("/team-user-breakdown", analyticsHandler.GetTeamUserBreakdown)
				r.Get("/usage", analyticsHandler.GetUsage)
				r.Get("/usage/hourly", analyticsHandler.GetHourlyUsage)
				r.Get("/usage/daily", analyticsHandler.GetDailyUsage)
				r.Get("/usage/monthly", analyticsHandler.GetMonthlyUsage)
				r.Get("/costs", analyticsHandler.GetCosts)
				r.Get("/costs/breakdown", analyticsHandler.GetCostBreakdown)
				r.Get("/performance", analyticsHandler.GetPerformance)
				r.Get("/errors", analyticsHandler.GetErrors)
				r.Get("/cache", analyticsHandler.GetCacheStats)
				// Historical metrics endpoints
				r.Get("/historical/model-health", analyticsHandler.GetHistoricalModelHealth)
				r.Get("/historical/system-metrics", analyticsHandler.GetHistoricalSystemMetrics)
				r.Get("/historical/model-latencies", analyticsHandler.GetHistoricalModelLatencies)
			})

			// System management
			r.Route("/system", func(r chi.Router) {
				r.Get("/config", systemHandler.GetConfig)
				r.Put("/config", systemHandler.UpdateConfig)
				r.Get("/health", systemHandler.GetSystemHealth)
				r.Get("/logs", systemHandler.GetLogs)
				r.Get("/audit", systemHandler.GetAuditLogs)
				r.Post("/cache/clear", systemHandler.ClearCache)
				r.Post("/maintenance", systemHandler.SetMaintenance)
				r.Get("/backup", systemHandler.CreateBackup)
				r.Post("/restore", systemHandler.RestoreBackup)
			})

			// Settings
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", systemHandler.GetSettings)
				r.Put("/", systemHandler.UpdateSettings)
				r.Get("/rate-limits", systemHandler.GetRateLimits)
				r.Put("/rate-limits", systemHandler.UpdateRateLimits)
				r.Get("/cache", systemHandler.GetCacheSettings)
				r.Put("/cache", systemHandler.UpdateCacheSettings)
			})

			// Guardrails management
			r.Route("/guardrails", func(r chi.Router) {
				r.Get("/", guardrailsHandler.ListGuardrails)
				r.Get("/{guardrailName}", guardrailsHandler.GetGuardrail)
				r.Get("/{guardrailName}/stats", guardrailsHandler.GetGuardrailStats)
				r.Get("/{guardrailName}/health", guardrailsHandler.CheckGuardrailHealth)
				r.Post("/test", guardrailsHandler.TestGuardrail)
			})
		})
	})

	// User self-service routes (authenticated users, not necessarily admin)
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)

		r.Route("/api/user", func(r chi.Router) {
			// Current user profile
			r.Get("/profile", func(w http.ResponseWriter, r *http.Request) {
				_, ok := middleware.GetUserID(r.Context())
				if !ok {
					http.Error(w, "User not found", http.StatusUnauthorized)
					return
				}
				// Profile endpoint - to be implemented
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotImplemented)
				if _, err := w.Write([]byte(`{"error": "Profile endpoint not yet implemented"}`)); err != nil {
					log.Printf("Failed to write profile error response: %v", err)
				}
			})

			// User's own keys
			r.Get("/keys", func(w http.ResponseWriter, r *http.Request) {
				userID, _ := middleware.GetUserID(r.Context())
				q := r.URL.Query()
				q.Add("user_id", userID.String())
				r.URL.RawQuery = q.Encode()
				keyHandler.ListKeys(w, r)
			})
			r.Post("/keys", keyHandler.CreateKey)

			// User's teams
			r.Get("/teams", func(w http.ResponseWriter, r *http.Request) {
				teamHandler.ListTeams(w, r)
			})
		})
	})

	// Serve admin UI static files
	r.Group(func(r chi.Router) {
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			// Serve the React admin UI
			// This will be configured to serve the built React app
			http.ServeFile(w, r, "./web/admin/dist/index.html")
		})
	})

	return r
}
