package router

import (
	"net/http"

	"github.com/amerfu/pllm/internal/auth"
	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/handlers/admin"
	"github.com/amerfu/pllm/internal/middleware"
	"github.com/amerfu/pllm/internal/services/team"
	"github.com/amerfu/pllm/internal/services/virtualkey"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type AdminRouterConfig struct {
	Config       *config.Config
	Logger       *zap.Logger
	DB           *gorm.DB
	AuthService  *auth.AuthService
	MasterKey    string
	ModelManager interface {
		GetModelStats() map[string]interface{}
	}
}

// NewAdminSubRouter creates admin routes to be mounted on the main router
// This doesn't include middleware as it will inherit from the parent router
func NewAdminSubRouter(cfg *AdminRouterConfig) http.Handler {
	r := chi.NewRouter()
	
	// Initialize services
	keyService := virtualkey.NewVirtualKeyService(cfg.DB)
	teamService := team.NewTeamService(cfg.DB)
	
	// Initialize handlers
	authHandler := admin.NewAuthHandler(cfg.Logger, cfg.MasterKey)
	oauthHandler := admin.NewOAuthHandler(
		cfg.Logger,
		"http://dex:5556/dex",
		"pllm-web",
		"pllm-web-secret",
	)
	teamHandler := admin.NewTeamHandler(cfg.Logger, teamService)
	keyHandler := admin.NewKeyHandler(cfg.Logger, keyService, teamService)
	budgetHandler := admin.NewBudgetHandler(cfg.Logger)
	analyticsHandler := admin.NewAnalyticsHandler(cfg.Logger, cfg.ModelManager)
	systemHandler := admin.NewSystemHandler(cfg.Logger)
	
	// Auth endpoints (public - no auth required)
	r.Post("/auth/login", authHandler.Login)
	r.Get("/auth/validate", authHandler.Validate)
	r.Post("/auth/token", oauthHandler.TokenExchange)
	r.Get("/auth/userinfo", oauthHandler.UserInfo)
	
	// Stats endpoint (commonly accessed by dashboard)
	r.Get("/stats", analyticsHandler.GetStats)
	r.Get("/dashboard", analyticsHandler.GetDashboard)
	
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
		r.Post("/generate", keyHandler.GenerateKey)
		r.Post("/validate", keyHandler.ValidateKey)
		r.Get("/{keyID}", keyHandler.GetKey)
		r.Put("/{keyID}", keyHandler.UpdateKey)
		r.Delete("/{keyID}", keyHandler.DeleteKey)
		r.Post("/{keyID}/revoke", keyHandler.RevokeKey)
		r.Get("/{keyID}/stats", keyHandler.GetKeyStats)
		r.Post("/{keyID}/budget-increase", keyHandler.TemporaryBudgetIncrease)
	})
	
	// Budget management
	r.Route("/budgets", func(r chi.Router) {
		r.Get("/", budgetHandler.ListBudgets)
		r.Post("/", budgetHandler.CreateBudget)
		r.Get("/{budgetID}", budgetHandler.GetBudget)
		r.Put("/{budgetID}", budgetHandler.UpdateBudget)
		r.Delete("/{budgetID}", budgetHandler.DeleteBudget)
		r.Post("/{budgetID}/reset", budgetHandler.ResetBudget)
		r.Get("/alerts", budgetHandler.GetAlerts)
	})
	
	// Analytics
	r.Route("/analytics", func(r chi.Router) {
		r.Get("/usage", analyticsHandler.GetUsage)
		r.Get("/usage/hourly", analyticsHandler.GetHourlyUsage)
		r.Get("/usage/daily", analyticsHandler.GetDailyUsage)
		r.Get("/usage/monthly", analyticsHandler.GetMonthlyUsage)
		r.Get("/costs", analyticsHandler.GetCosts)
		r.Get("/costs/breakdown", analyticsHandler.GetCostBreakdown)
		r.Get("/performance", analyticsHandler.GetPerformance)
		r.Get("/errors", analyticsHandler.GetErrors)
		r.Get("/cache", analyticsHandler.GetCacheStats)
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
	keyService := virtualkey.NewVirtualKeyService(cfg.DB)
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
	teamHandler := admin.NewTeamHandler(cfg.Logger, teamService)
	keyHandler := admin.NewKeyHandler(cfg.Logger, keyService, teamService)
	budgetHandler := admin.NewBudgetHandler(cfg.Logger)
	analyticsHandler := admin.NewAnalyticsHandler(cfg.Logger, cfg.ModelManager)
	systemHandler := admin.NewSystemHandler(cfg.Logger)

	// Initialize auth middleware
	authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthConfig{
		Logger:      cfg.Logger,
		AuthService: cfg.AuthService,
		KeyService:  keyService,
		MasterKey:   cfg.MasterKey,
		RequireAuth: false, // Allow public endpoints
	})

	// Health check (public)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok", "service": "admin"}`))
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
				r.Post("/generate", keyHandler.GenerateKey)
				r.Get("/{keyID}", keyHandler.GetKey)
				r.Put("/{keyID}", keyHandler.UpdateKey)
				r.Delete("/{keyID}", keyHandler.DeleteKey)
				r.Post("/{keyID}/revoke", keyHandler.RevokeKey)
				r.Get("/{keyID}/stats", keyHandler.GetKeyStats)
				r.Post("/{keyID}/budget/increase", keyHandler.TemporaryBudgetIncrease)
			})

			// Budget management
			r.Route("/budgets", func(r chi.Router) {
				r.Get("/", budgetHandler.ListBudgets)
				r.Post("/", budgetHandler.CreateBudget)
				r.Get("/{budgetID}", budgetHandler.GetBudget)
				r.Put("/{budgetID}", budgetHandler.UpdateBudget)
				r.Delete("/{budgetID}", budgetHandler.DeleteBudget)
				r.Post("/{budgetID}/reset", budgetHandler.ResetBudget)
				r.Get("/alerts", budgetHandler.GetAlerts)
			})

			// Analytics
			r.Route("/analytics", func(r chi.Router) {
				r.Get("/usage", analyticsHandler.GetUsage)
				r.Get("/usage/hourly", analyticsHandler.GetHourlyUsage)
				r.Get("/usage/daily", analyticsHandler.GetDailyUsage)
				r.Get("/usage/monthly", analyticsHandler.GetMonthlyUsage)
				r.Get("/costs", analyticsHandler.GetCosts)
				r.Get("/costs/breakdown", analyticsHandler.GetCostBreakdown)
				r.Get("/performance", analyticsHandler.GetPerformance)
				r.Get("/errors", analyticsHandler.GetErrors)
				r.Get("/cache", analyticsHandler.GetCacheStats)
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
				w.Write([]byte(`{"error": "Profile endpoint not yet implemented"}`))
			})

			// User's own keys
			r.Get("/keys", func(w http.ResponseWriter, r *http.Request) {
				userID, _ := middleware.GetUserID(r.Context())
				r.URL.Query().Add("user_id", userID.String())
				keyHandler.ListKeys(w, r)
			})
			r.Post("/keys", keyHandler.GenerateKey)

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
