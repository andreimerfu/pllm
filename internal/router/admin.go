package router

import (
	"net/http"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/handlers/admin"
	"github.com/amerfu/pllm/internal/middleware"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
)

func NewAdminRouter(cfg *config.Config, logger *zap.Logger) http.Handler {
	r := chi.NewRouter()
	
	// Basic middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.Logger(logger))
	
	// CORS for admin UI
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		ExposedHeaders:   cfg.CORS.ExposedHeaders,
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           cfg.CORS.MaxAge,
	}))
	
	// Initialize admin handlers
	userHandler := admin.NewUserHandler(logger)
	groupHandler := admin.NewGroupHandler(logger)
	providerHandler := admin.NewProviderHandler(logger)
	budgetHandler := admin.NewBudgetHandler(logger)
	analyticsHandler := admin.NewAnalyticsHandler(logger)
	systemHandler := admin.NewSystemHandler(logger)
	
	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok", "service": "admin"}`))
	})
	
	// Admin authentication
	r.Post("/api/admin/login", userHandler.AdminLogin)
	r.Post("/api/admin/refresh", userHandler.RefreshToken)
	
	// Protected admin routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.AdminAuth(cfg.JWT.SecretKey))
		
		r.Route("/api/admin", func(r chi.Router) {
			// Dashboard
			r.Get("/dashboard", analyticsHandler.GetDashboard)
			r.Get("/stats", analyticsHandler.GetStats)
			
			// User management
			r.Route("/users", func(r chi.Router) {
				r.Get("/", userHandler.ListUsers)
				r.Post("/", userHandler.CreateUser)
				r.Get("/{user_id}", userHandler.GetUser)
				r.Put("/{user_id}", userHandler.UpdateUser)
				r.Delete("/{user_id}", userHandler.DeleteUser)
				r.Post("/{user_id}/activate", userHandler.ActivateUser)
				r.Post("/{user_id}/deactivate", userHandler.DeactivateUser)
				r.Post("/{user_id}/reset-password", userHandler.ResetPassword)
				r.Get("/{user_id}/api-keys", userHandler.GetUserAPIKeys)
				r.Get("/{user_id}/usage", userHandler.GetUserUsage)
			})
			
			// Group management
			r.Route("/groups", func(r chi.Router) {
				r.Get("/", groupHandler.ListGroups)
				r.Post("/", groupHandler.CreateGroup)
				r.Get("/{group_id}", groupHandler.GetGroup)
				r.Put("/{group_id}", groupHandler.UpdateGroup)
				r.Delete("/{group_id}", groupHandler.DeleteGroup)
				r.Get("/{group_id}/members", groupHandler.GetMembers)
				r.Post("/{group_id}/members", groupHandler.AddMember)
				r.Delete("/{group_id}/members/{user_id}", groupHandler.RemoveMember)
				r.Put("/{group_id}/members/{user_id}/role", groupHandler.UpdateMemberRole)
				r.Get("/{group_id}/usage", groupHandler.GetGroupUsage)
				r.Put("/{group_id}/settings", groupHandler.UpdateSettings)
			})
			
			// Provider management
			r.Route("/providers", func(r chi.Router) {
				r.Get("/", providerHandler.ListProviders)
				r.Post("/", providerHandler.CreateProvider)
				r.Get("/{provider_id}", providerHandler.GetProvider)
				r.Put("/{provider_id}", providerHandler.UpdateProvider)
				r.Delete("/{provider_id}", providerHandler.DeleteProvider)
				r.Post("/{provider_id}/test", providerHandler.TestProvider)
				r.Get("/{provider_id}/models", providerHandler.GetProviderModels)
				r.Post("/{provider_id}/models", providerHandler.AddModel)
				r.Put("/{provider_id}/models/{model_id}", providerHandler.UpdateModel)
				r.Delete("/{provider_id}/models/{model_id}", providerHandler.DeleteModel)
				r.Get("/{provider_id}/health", providerHandler.GetHealth)
			})
			
			// Budget management
			r.Route("/budgets", func(r chi.Router) {
				r.Get("/", budgetHandler.ListBudgets)
				r.Post("/", budgetHandler.CreateBudget)
				r.Get("/{budget_id}", budgetHandler.GetBudget)
				r.Put("/{budget_id}", budgetHandler.UpdateBudget)
				r.Delete("/{budget_id}", budgetHandler.DeleteBudget)
				r.Post("/{budget_id}/reset", budgetHandler.ResetBudget)
				r.Get("/alerts", budgetHandler.GetAlerts)
			})
			
			// API Keys management
			r.Route("/api-keys", func(r chi.Router) {
				r.Get("/", userHandler.ListAllAPIKeys)
				r.Get("/{key_id}", userHandler.GetAPIKey)
				r.Put("/{key_id}", userHandler.UpdateAPIKey)
				r.Delete("/{key_id}", userHandler.DeleteAPIKey)
				r.Post("/{key_id}/activate", userHandler.ActivateAPIKey)
				r.Post("/{key_id}/deactivate", userHandler.DeactivateAPIKey)
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