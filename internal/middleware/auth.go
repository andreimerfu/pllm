package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/auth"
	"github.com/amerfu/pllm/internal/models"
)

type contextKey string

const (
	UserContextKey      contextKey = "user"
	KeyContextKey       contextKey = "key"
	TeamContextKey      contextKey = "team"
	AuthTypeContextKey  contextKey = "auth_type"
	MasterKeyContextKey contextKey = "master_key_context"
)

type AuthType string

const (
	AuthTypeMasterKey AuthType = "master_key"
	AuthTypeAPIKey    AuthType = "api_key"
	AuthTypeJWT       AuthType = "jwt"
	AuthTypeNone      AuthType = "none"
)

type AuthMiddleware struct {
	logger            *zap.Logger
	authService       *auth.AuthService
	cachedAuthService *auth.CachedAuthService
	masterKeyService  *auth.MasterKeyService
	requireAuth       bool
}

type AuthConfig struct {
	Logger           *zap.Logger
	AuthService      *auth.AuthService
	MasterKeyService *auth.MasterKeyService
	RequireAuth      bool
}

func NewAuthMiddleware(config *AuthConfig) *AuthMiddleware {
	// Create cached auth service wrapper
	cachedAuth := auth.NewCachedAuthService(config.AuthService, config.Logger)

	return &AuthMiddleware{
		logger:            config.Logger,
		authService:       config.AuthService,
		cachedAuthService: cachedAuth,
		masterKeyService:  config.MasterKeyService,
		requireAuth:       config.RequireAuth,
	}
}

// Authenticate is the main authentication middleware
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check and metrics
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		// Debug logging
		m.logger.Debug("Authentication middleware",
			zap.String("path", r.URL.Path),
			zap.String("method", r.Method),
			zap.String("auth_header", r.Header.Get("Authorization")))

		// Check for authentication
		authType, authData, err := m.extractAuth(r)
		if err != nil {
			m.logger.Debug("Failed to extract auth", zap.Error(err))
			if m.requireAuth {
				m.sendError(w, http.StatusUnauthorized, "Invalid authentication")
				return
			}
			// Continue without auth if not required
			ctx := context.WithValue(r.Context(), AuthTypeContextKey, AuthTypeNone)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		m.logger.Debug("Extracted auth",
			zap.String("type", string(authType)),
			zap.String("data_prefix", authData[:min(20, len(authData))]))

		// Process based on auth type
		switch authType {
		case AuthTypeMasterKey:
			masterCtx, err := m.masterKeyService.ValidateMasterKey(r.Context(), authData)
			if err != nil {
				m.sendError(w, http.StatusUnauthorized, "Invalid master key")
				return
			}
			ctx := context.WithValue(r.Context(), AuthTypeContextKey, AuthTypeMasterKey)
			ctx = context.WithValue(ctx, MasterKeyContextKey, masterCtx)
			next.ServeHTTP(w, r.WithContext(ctx))

		case AuthTypeAPIKey:
			// Use cached key validation for performance
			key, err := m.cachedAuthService.ValidateKeyCached(r.Context(), authData)
			if err != nil {
				m.sendError(w, http.StatusUnauthorized, err.Error())
				return
			}
			ctx := context.WithValue(r.Context(), AuthTypeContextKey, AuthTypeAPIKey)
			ctx = context.WithValue(ctx, KeyContextKey, key)
			if key.UserID != nil {
				ctx = context.WithValue(ctx, UserContextKey, *key.UserID)
			}
			if key.TeamID != nil {
				ctx = context.WithValue(ctx, TeamContextKey, *key.TeamID)
			}
			next.ServeHTTP(w, r.WithContext(ctx))

		case AuthTypeJWT:
			m.logger.Debug("Validating JWT token")
			// Use cached JWT validation for performance
			cachedClaims, err := m.cachedAuthService.ValidateTokenCached(r.Context(), authData)
			if err != nil {
				m.logger.Debug("JWT validation failed", zap.Error(err))
				m.sendError(w, http.StatusUnauthorized, "Invalid token")
				return
			}
			m.logger.Debug("JWT validation successful", zap.String("user_id", cachedClaims.UserID.String()))
			ctx := context.WithValue(r.Context(), AuthTypeContextKey, AuthTypeJWT)
			ctx = context.WithValue(ctx, UserContextKey, cachedClaims.UserID)
			// Store permissions in context for RBAC
			ctx = context.WithValue(ctx, "permissions", cachedClaims.Permissions)
			next.ServeHTTP(w, r.WithContext(ctx))

		default:
			if m.requireAuth {
				m.sendError(w, http.StatusUnauthorized, "Authentication required")
				return
			}
			ctx := context.WithValue(r.Context(), AuthTypeContextKey, AuthTypeNone)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

// RequireAdmin ensures the request has admin privileges
func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authType := r.Context().Value(AuthTypeContextKey).(AuthType)

		// Master key always has admin access
		if authType == AuthTypeMasterKey {
			next.ServeHTTP(w, r)
			return
		}

		// Check JWT claims for admin role
		if authType == AuthTypeJWT {
			// TODO: Check user role from database
			userID := r.Context().Value(UserContextKey).(uuid.UUID)
			if userID == uuid.Nil {
				m.sendError(w, http.StatusForbidden, "Admin access required")
				return
			}
			// For now, allow all authenticated users
			// In production, check user.Role == models.RoleAdmin
			next.ServeHTTP(w, r)
			return
		}

		m.sendError(w, http.StatusForbidden, "Admin access required")
	})
}

// RequireTeamAccess ensures the user has access to the team
func (m *AuthMiddleware) RequireTeamAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authType := r.Context().Value(AuthTypeContextKey).(AuthType)

		// Master key always has access
		if authType == AuthTypeMasterKey {
			next.ServeHTTP(w, r)
			return
		}

		// Extract team ID from URL
		teamIDStr := strings.TrimPrefix(r.URL.Path, "/api/teams/")
		teamIDStr = strings.Split(teamIDStr, "/")[0]
		teamID, err := uuid.Parse(teamIDStr)
		if err != nil {
			m.sendError(w, http.StatusBadRequest, "Invalid team ID")
			return
		}

		// Check API key team access
		if authType == AuthTypeAPIKey {
			key := r.Context().Value(KeyContextKey).(*models.Key)
			if key.TeamID != nil && *key.TeamID == teamID {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Check JWT user team membership
		if authType == AuthTypeJWT {
			userID := r.Context().Value(UserContextKey).(uuid.UUID)
			// TODO: Check if user is member of team
			_ = userID
			next.ServeHTTP(w, r)
			return
		}

		m.sendError(w, http.StatusForbidden, "Team access required")
	})
}

// RequireModelAccess ensures the request has access to the specified model
func (m *AuthMiddleware) RequireModelAccess(modelExtractor func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authType := r.Context().Value(AuthTypeContextKey).(AuthType)

			// Master key always has access
			if authType == AuthTypeMasterKey {
				next.ServeHTTP(w, r)
				return
			}

			model := modelExtractor(r)
			if model == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check API key model access
			if authType == AuthTypeAPIKey {
				key := r.Context().Value(KeyContextKey).(*models.Key)
				if !key.IsModelAllowed(model) {
					m.sendError(w, http.StatusForbidden, "Model access denied")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractAuth extracts authentication from the request
func (m *AuthMiddleware) extractAuth(r *http.Request) (AuthType, string, error) {
	// Check Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		m.logger.Debug("Found Authorization header", zap.String("header", authHeader[:min(50, len(authHeader))]))
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 {
			// Assume it's a raw API key
			if strings.HasPrefix(authHeader, "sk-") || strings.HasPrefix(authHeader, "pllm_") {
				return AuthTypeAPIKey, authHeader, nil
			}
			return "", "", fmt.Errorf("malformed authorization header")
		}

		scheme := strings.ToLower(parts[0])
		credentials := parts[1]

		switch scheme {
		case "bearer":
			// Check if it's an API key or JWT
			if strings.HasPrefix(credentials, "sk-") || strings.HasPrefix(credentials, "pllm_") {
				m.logger.Debug("Detected API key in Bearer token")
				return AuthTypeAPIKey, credentials, nil
			}
			m.logger.Debug("Detected JWT in Bearer token")
			return AuthTypeJWT, credentials, nil
		case "basic":
			// Could be master key (encoded)
			m.logger.Debug("Detected Basic auth")
			return AuthTypeMasterKey, credentials, nil
		}
	}

	// Check X-API-Key header
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		if m.masterKeyService != nil && m.masterKeyService.IsConfigured() {
			if _, err := m.masterKeyService.ValidateMasterKey(r.Context(), apiKey); err == nil {
				return AuthTypeMasterKey, apiKey, nil
			}
		}
		if strings.HasPrefix(apiKey, "sk-") || strings.HasPrefix(apiKey, "pllm_") {
			return AuthTypeAPIKey, apiKey, nil
		}
	}

	// Check query parameter (for SSE connections)
	apiKey = r.URL.Query().Get("api_key")
	if apiKey != "" {
		if m.masterKeyService != nil && m.masterKeyService.IsConfigured() {
			if _, err := m.masterKeyService.ValidateMasterKey(r.Context(), apiKey); err == nil {
				return AuthTypeMasterKey, apiKey, nil
			}
		}
		if strings.HasPrefix(apiKey, "sk-") || strings.HasPrefix(apiKey, "pllm_") {
			return AuthTypeAPIKey, apiKey, nil
		}
	}

	m.logger.Debug("No authentication found in request")
	return "", "", fmt.Errorf("no authentication found")
}

func (m *AuthMiddleware) sendError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    "authentication_error",
			"code":    statusCode,
		},
	})
}

// Helper functions to extract auth context

func GetAuthType(ctx context.Context) AuthType {
	authType, ok := ctx.Value(AuthTypeContextKey).(AuthType)
	if !ok {
		return AuthTypeNone
	}
	return authType
}

func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserContextKey).(uuid.UUID)
	return userID, ok
}

func GetKey(ctx context.Context) (*models.Key, bool) {
	key, ok := ctx.Value(KeyContextKey).(*models.Key)
	return key, ok
}

func GetTeamID(ctx context.Context) (uuid.UUID, bool) {
	teamID, ok := ctx.Value(TeamContextKey).(uuid.UUID)
	return teamID, ok
}

func IsMasterKey(ctx context.Context) bool {
	return GetAuthType(ctx) == AuthTypeMasterKey
}
