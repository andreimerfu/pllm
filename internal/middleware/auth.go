package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
	
	"github.com/amerfu/pllm/internal/auth"
	"github.com/amerfu/pllm/internal/models"
	"github.com/amerfu/pllm/internal/services/virtualkey"
)

type contextKey string

const (
	UserContextKey        contextKey = "user"
	VirtualKeyContextKey  contextKey = "virtual_key"
	TeamContextKey        contextKey = "team"
	AuthTypeContextKey    contextKey = "auth_type"
)

type AuthType string

const (
	AuthTypeMasterKey   AuthType = "master_key"
	AuthTypeVirtualKey  AuthType = "virtual_key"
	AuthTypeJWT         AuthType = "jwt"
	AuthTypeNone        AuthType = "none"
)

type AuthMiddleware struct {
	logger         *zap.Logger
	authService    *auth.AuthService
	keyService     *virtualkey.VirtualKeyService
	masterKey      string
	requireAuth    bool
}

type AuthConfig struct {
	Logger      *zap.Logger
	AuthService *auth.AuthService
	KeyService  *virtualkey.VirtualKeyService
	MasterKey   string
	RequireAuth bool
}

func NewAuthMiddleware(config *AuthConfig) *AuthMiddleware {
	return &AuthMiddleware{
		logger:      config.Logger,
		authService: config.AuthService,
		keyService:  config.KeyService,
		masterKey:   config.MasterKey,
		requireAuth: config.RequireAuth,
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

		// Check for authentication
		authType, authData, err := m.extractAuth(r)
		if err != nil {
			if m.requireAuth {
				m.sendError(w, http.StatusUnauthorized, "Invalid authentication")
				return
			}
			// Continue without auth if not required
			ctx := context.WithValue(r.Context(), AuthTypeContextKey, AuthTypeNone)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Process based on auth type
		switch authType {
		case AuthTypeMasterKey:
			if authData != m.masterKey {
				m.sendError(w, http.StatusUnauthorized, "Invalid master key")
				return
			}
			ctx := context.WithValue(r.Context(), AuthTypeContextKey, AuthTypeMasterKey)
			next.ServeHTTP(w, r.WithContext(ctx))

		case AuthTypeVirtualKey:
			key, err := m.keyService.ValidateKey(r.Context(), authData)
			if err != nil {
				m.sendError(w, http.StatusUnauthorized, err.Error())
				return
			}
			ctx := context.WithValue(r.Context(), AuthTypeContextKey, AuthTypeVirtualKey)
			ctx = context.WithValue(ctx, VirtualKeyContextKey, key)
			if key.UserID != nil {
				ctx = context.WithValue(ctx, UserContextKey, key.UserID)
			}
			if key.TeamID != nil {
				ctx = context.WithValue(ctx, TeamContextKey, key.TeamID)
			}
			next.ServeHTTP(w, r.WithContext(ctx))

		case AuthTypeJWT:
			claims, err := m.authService.ValidateToken(authData)
			if err != nil {
				m.sendError(w, http.StatusUnauthorized, "Invalid token")
				return
			}
			ctx := context.WithValue(r.Context(), AuthTypeContextKey, AuthTypeJWT)
			ctx = context.WithValue(ctx, UserContextKey, claims.UserID)
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

		// Check virtual key team access
		if authType == AuthTypeVirtualKey {
			key := r.Context().Value(VirtualKeyContextKey).(*models.VirtualKey)
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

			// Check virtual key model access
			if authType == AuthTypeVirtualKey {
				key := r.Context().Value(VirtualKeyContextKey).(*models.VirtualKey)
				err := m.keyService.CheckModelAccess(r.Context(), key, model)
				if err != nil {
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
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 {
			// Assume it's a raw API key
			if strings.HasPrefix(authHeader, "sk-") {
				return AuthTypeVirtualKey, authHeader, nil
			}
			return "", "", nil
		}

		scheme := strings.ToLower(parts[0])
		credentials := parts[1]

		switch scheme {
		case "bearer":
			// Check if it's a virtual key or JWT
			if strings.HasPrefix(credentials, "sk-") {
				return AuthTypeVirtualKey, credentials, nil
			}
			return AuthTypeJWT, credentials, nil
		case "basic":
			// Could be master key
			if credentials == m.masterKey {
				return AuthTypeMasterKey, credentials, nil
			}
		}
	}

	// Check X-API-Key header
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		if apiKey == m.masterKey {
			return AuthTypeMasterKey, apiKey, nil
		}
		if strings.HasPrefix(apiKey, "sk-") {
			return AuthTypeVirtualKey, apiKey, nil
		}
	}

	// Check query parameter (for SSE connections)
	apiKey = r.URL.Query().Get("api_key")
	if apiKey != "" {
		if apiKey == m.masterKey {
			return AuthTypeMasterKey, apiKey, nil
		}
		if strings.HasPrefix(apiKey, "sk-") {
			return AuthTypeVirtualKey, apiKey, nil
		}
	}

	return "", "", nil
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

func GetVirtualKey(ctx context.Context) (*models.VirtualKey, bool) {
	key, ok := ctx.Value(VirtualKeyContextKey).(*models.VirtualKey)
	return key, ok
}

func GetTeamID(ctx context.Context) (uuid.UUID, bool) {
	teamID, ok := ctx.Value(TeamContextKey).(uuid.UUID)
	return teamID, ok
}

func IsMasterKey(ctx context.Context) bool {
	return GetAuthType(ctx) == AuthTypeMasterKey
}