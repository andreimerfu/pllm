package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/auth"
	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	"github.com/amerfu/pllm/internal/core/models"
)

type AuthHandler struct {
	baseHandler
	masterKeyService *auth.MasterKeyService
	authService      *auth.AuthService
	db               *gorm.DB
}

func NewAuthHandler(logger *zap.Logger, masterKeyService *auth.MasterKeyService, authService *auth.AuthService, db *gorm.DB) *AuthHandler {
	return &AuthHandler{
		baseHandler:      baseHandler{logger: logger},
		masterKeyService: masterKeyService,
		authService:      authService,
		db:               db,
	}
}

type LoginRequest struct {
	MasterKey string `json:"master_key"`
}

type LoginResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token"`
	Message string `json:"message"`
}

// MasterKeyLogin handles master key authentication for admin access
func (h *AuthHandler) MasterKeyLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Check if master key is provided
	if req.MasterKey == "" {
		h.sendError(w, http.StatusBadRequest, "Master key is required")
		return
	}

	// Validate master key using the service
	masterCtx, err := h.masterKeyService.ValidateMasterKey(r.Context(), req.MasterKey)
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Invalid master key")
		return
	}

	// Generate JWT token for the master key session
	token, err := h.masterKeyService.GenerateAdminToken(masterCtx)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	h.sendJSON(w, http.StatusOK, LoginResponse{
		Success: true,
		Token:   token,
		Message: "Master key authentication successful",
	})
}

// Login is deprecated - users should use Dex OAuth
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// For backward compatibility, check if it's a master key login
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.MasterKey != "" {
		// Redirect to master key login
		h.MasterKeyLogin(w, r)
		return
	}

	h.sendError(w, http.StatusBadRequest, "Password authentication is deprecated. Please use Dex OAuth or master key")
}

// Validate checks if a token is valid
func (h *AuthHandler) Validate(w http.ResponseWriter, r *http.Request) {
	// Get token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		h.sendError(w, http.StatusUnauthorized, "Authorization header required")
		return
	}

	// Remove "Bearer " prefix
	token := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	// Try to validate as master key first
	masterCtx, err := h.masterKeyService.ValidateMasterKey(r.Context(), token)
	if err == nil {
		// It's a valid master key
		response := map[string]interface{}{
			"valid": true,
			"user": map[string]interface{}{
				"id":     "master",
				"role":   "admin",
				"type":   "master_key",
				"scopes": masterCtx.Scopes,
			},
			"expires_at": masterCtx.ValidatedAt.Add(24 * time.Hour).Unix(),
		}
		h.sendJSON(w, http.StatusOK, response)
		return
	}

	// TODO: Validate as JWT token from Dex
	h.sendError(w, http.StatusUnauthorized, "Invalid token")
}

// GetPermissions returns the current user's permissions
func (h *AuthHandler) GetPermissions(w http.ResponseWriter, r *http.Request) {
	// Get authentication context
	authType := middleware.GetAuthType(r.Context())

	var permissions []string
	var role string
	var groups []string

	switch authType {
	case middleware.AuthTypeMasterKey:
		// Master key has all permissions
		permissions = []string{
			"admin.*", "users.*", "teams.*", "keys.*", "models.*", "settings.*", "budget.*",
			"admin.users.read", "admin.users.write", "admin.users.delete",
			"admin.teams.read", "admin.teams.write", "admin.teams.delete",
			"admin.keys.read", "admin.keys.write", "admin.keys.delete",
			"admin.models.read", "admin.models.write",
			"admin.budget.read", "admin.budget.write",
			"admin.settings.read", "admin.settings.write",
		}
		role = "admin"
		groups = []string{"admin"}

	case middleware.AuthTypeJWT:
		// Get user info from JWT
		userID, hasUserID := middleware.GetUserID(r.Context())

		// Check if this is a master key JWT (user ID is the special master key UUID)
		if hasUserID && userID.String() == "00000000-0000-0000-0000-000000000001" {
			// This is a master key JWT - treat it like a master key
			permissions = []string{
				"admin.*", "users.*", "teams.*", "keys.*", "models.*", "settings.*", "budget.*",
				"admin.users.read", "admin.users.write", "admin.users.delete",
				"admin.teams.read", "admin.teams.write", "admin.teams.delete",
				"admin.keys.read", "admin.keys.write", "admin.keys.delete",
				"admin.models.read", "admin.models.write",
				"admin.budget.read", "admin.budget.write",
				"admin.settings.read", "admin.settings.write",
			}
			role = "admin"
			groups = []string{"admin", "master"}
		} else {
			// Regular Dex JWT - look up user and determine permissions
			var user models.User
			if err := h.db.First(&user, "id = ?", userID).Error; err == nil {
				role = string(user.Role)

				// Admin users get the full permission set (same as master key)
				if user.Role == models.RoleAdmin {
					permissions = []string{
						"admin.*", "users.*", "teams.*", "keys.*", "models.*", "settings.*", "budget.*",
						"admin.users.read", "admin.users.write", "admin.users.delete",
						"admin.teams.read", "admin.teams.write", "admin.teams.delete",
						"admin.keys.read", "admin.keys.write", "admin.keys.delete",
						"admin.models.read", "admin.models.write",
						"admin.budget.read", "admin.budget.write",
						"admin.settings.read", "admin.settings.write",
						"admin.audit.read",
						"admin.guardrails.read",
					}
				} else if h.authService != nil && hasUserID {
					userPerms, err := h.authService.GetUserPermissions(r.Context(), userID)
					if err == nil {
						permissions = userPerms
					}
				}

				// Get user's groups/teams
				var teamMembers []models.TeamMember
				if err := h.db.Preload("Team").Where("user_id = ?", userID).Find(&teamMembers).Error; err == nil {
					for _, tm := range teamMembers {
						groups = append(groups, string(tm.Role))
					}
				}
			}
		}

	case middleware.AuthTypeAPIKey:
		// API keys have limited permissions based on key configuration
		key, _ := middleware.GetKey(r.Context())
		if key != nil {
			// Basic permissions for API keys
			permissions = []string{"models:read", "models:use"}
			role = "api_key"
			if key.UserID != nil {
				var user models.User
				if err := h.db.First(&user, "id = ?", *key.UserID).Error; err == nil {
					role = string(user.Role)
				}
			}
		}

	default:
		// No auth or unknown auth type
		permissions = []string{}
		role = "anonymous"
		groups = []string{}
	}

	response := map[string]interface{}{
		"permissions": permissions,
		"role":        role,
		"groups":      groups,
		"auth_type":   string(authType),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode auth response", zap.Error(err))
	}
}
