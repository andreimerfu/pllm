package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/amerfu/pllm/internal/core/auth"
	"github.com/amerfu/pllm/internal/core/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RBACMiddleware handles role-based access control
type RBACMiddleware struct {
	logger      *zap.Logger
	db          *gorm.DB
	permService *auth.PermissionService
}

// NewRBACMiddleware creates a new RBAC middleware
func NewRBACMiddleware(logger *zap.Logger, db *gorm.DB) *RBACMiddleware {
	return &RBACMiddleware{
		logger:      logger,
		db:          db,
		permService: auth.NewPermissionService(),
	}
}

// RequirePermission checks if the user has a specific permission
func (m *RBACMiddleware) RequirePermission(permission auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context
			userID, ok := GetUserID(r.Context())
			if !ok {
				m.unauthorized(w, "User not authenticated")
				return
			}

			// Load user with teams
			var user models.User
			if err := m.db.Preload("Teams").First(&user, "id = ?", userID).Error; err != nil {
				m.logger.Error("Failed to load user", zap.Error(err))
				m.unauthorized(w, "User not found")
				return
			}

			// Check permission
			if !m.permService.HasPermission(&user, permission) {
				m.logger.Warn("Permission denied",
					zap.String("user_id", userID.String()),
					zap.String("permission", string(permission)))
				m.forbidden(w, "Insufficient permissions")
				return
			}

			// Add permission service to context
			ctx := auth.WithPermissionService(r.Context(), m.permService)
			ctx = auth.WithUserPermissions(ctx, m.permService.GetUserPermissions(&user))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireTeamPermission checks if the user has a permission within a team context
func (m *RBACMiddleware) RequireTeamPermission(permission auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context
			userID, ok := GetUserID(r.Context())
			if !ok {
				m.unauthorized(w, "User not authenticated")
				return
			}

			// Get team ID from URL
			teamIDStr := chi.URLParam(r, "teamID")
			if teamIDStr == "" {
				// Try to get from query params
				teamIDStr = r.URL.Query().Get("team_id")
			}

			if teamIDStr == "" {
				m.badRequest(w, "Team ID required")
				return
			}

			teamID, err := uuid.Parse(teamIDStr)
			if err != nil {
				m.badRequest(w, "Invalid team ID")
				return
			}

			// Load user with teams
			var user models.User
			if err := m.db.Preload("Teams").First(&user, "id = ?", userID).Error; err != nil {
				m.logger.Error("Failed to load user", zap.Error(err))
				m.unauthorized(w, "User not found")
				return
			}

			// Check team permission
			if !m.permService.HasTeamPermission(&user, teamID, permission) {
				m.logger.Warn("Team permission denied",
					zap.String("user_id", userID.String()),
					zap.String("team_id", teamIDStr),
					zap.String("permission", string(permission)))
				m.forbidden(w, "Insufficient team permissions")
				return
			}

			// Add permission service to context
			ctx := auth.WithPermissionService(r.Context(), m.permService)
			ctx = auth.WithUserPermissions(ctx, m.permService.GetUserPermissions(&user))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole checks if the user has a specific role
func (m *RBACMiddleware) RequireRole(roles ...models.UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context
			userID, ok := GetUserID(r.Context())
			if !ok {
				m.unauthorized(w, "User not authenticated")
				return
			}

			// Load user
			var user models.User
			if err := m.db.First(&user, "id = ?", userID).Error; err != nil {
				m.logger.Error("Failed to load user", zap.Error(err))
				m.unauthorized(w, "User not found")
				return
			}

			// Check if user has one of the required roles
			hasRole := false
			for _, role := range roles {
				if user.Role == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				m.logger.Warn("Role requirement not met",
					zap.String("user_id", userID.String()),
					zap.String("user_role", string(user.Role)))
				m.forbidden(w, "Insufficient role privileges")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireTeamRole checks if the user has a specific role in the team
func (m *RBACMiddleware) RequireTeamRole(roles ...models.TeamRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context
			userID, ok := GetUserID(r.Context())
			if !ok {
				m.unauthorized(w, "User not authenticated")
				return
			}

			// Get team ID from URL
			teamIDStr := chi.URLParam(r, "teamID")
			if teamIDStr == "" {
				teamIDStr = r.URL.Query().Get("team_id")
			}

			if teamIDStr == "" {
				m.badRequest(w, "Team ID required")
				return
			}

			teamID, err := uuid.Parse(teamIDStr)
			if err != nil {
				m.badRequest(w, "Invalid team ID")
				return
			}

			// Check team membership and role
			var member models.TeamMember
			err = m.db.Where("user_id = ? AND team_id = ?", userID, teamID).First(&member).Error
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					m.forbidden(w, "Not a member of this team")
				} else {
					m.logger.Error("Failed to check team membership", zap.Error(err))
					m.internalError(w, "Failed to verify team membership")
				}
				return
			}

			// Check if user has one of the required roles
			hasRole := false
			for _, role := range roles {
				if member.Role == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				m.logger.Warn("Team role requirement not met",
					zap.String("user_id", userID.String()),
					zap.String("team_id", teamIDStr),
					zap.String("member_role", string(member.Role)))
				m.forbidden(w, "Insufficient team role privileges")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireSelfOrAdmin allows access if user is accessing their own resource or is an admin
func (m *RBACMiddleware) RequireSelfOrAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get authenticated user from context
			authUserID, ok := GetUserID(r.Context())
			if !ok {
				m.unauthorized(w, "User not authenticated")
				return
			}

			// Get target user ID from URL
			targetUserIDStr := chi.URLParam(r, "userID")
			if targetUserIDStr == "" {
				targetUserIDStr = chi.URLParam(r, "id")
			}

			if targetUserIDStr != "" {
				targetUserID, err := uuid.Parse(targetUserIDStr)
				if err != nil {
					m.badRequest(w, "Invalid user ID")
					return
				}

				// Check if same user
				if authUserID == targetUserID {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check if admin
			var user models.User
			if err := m.db.First(&user, "id = ?", authUserID).Error; err != nil {
				m.logger.Error("Failed to load user", zap.Error(err))
				m.unauthorized(w, "User not found")
				return
			}

			if user.Role != models.RoleAdmin {
				m.forbidden(w, "Can only access your own resources")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// EnforceBudget checks if the user/team/key has budget available
func (m *RBACMiddleware) EnforceBudget() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context
			userID, ok := GetUserID(r.Context())
			if !ok {
				m.unauthorized(w, "User not authenticated")
				return
			}

			// Load user with budget info
			var user models.User
			if err := m.db.First(&user, "id = ?", userID).Error; err != nil {
				m.logger.Error("Failed to load user", zap.Error(err))
				m.unauthorized(w, "User not found")
				return
			}

			// Check user budget
			if user.MaxBudget > 0 && user.CurrentSpend >= user.MaxBudget {
				m.logger.Warn("User budget exceeded",
					zap.String("user_id", userID.String()),
					zap.Float64("current_spend", user.CurrentSpend),
					zap.Float64("max_budget", user.MaxBudget))
				m.paymentRequired(w, "Budget limit exceeded")
				return
			}

			// TODO: Check team budgets if applicable
			// TODO: Check key budgets if applicable

			next.ServeHTTP(w, r)
		})
	}
}

// Helper methods for error responses

func (m *RBACMiddleware) unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error": message,
		"code":  "unauthorized",
	}); err != nil {
		m.logger.Error("Failed to encode RBAC error response", zap.Error(err))
	}
}

func (m *RBACMiddleware) forbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error": message,
		"code":  "forbidden",
	}); err != nil {
		m.logger.Error("Failed to encode forbidden response", zap.Error(err))
	}
}

func (m *RBACMiddleware) badRequest(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error": message,
		"code":  "bad_request",
	}); err != nil {
		m.logger.Error("Failed to encode bad request response", zap.Error(err))
	}
}

func (m *RBACMiddleware) internalError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error": message,
		"code":  "internal_error",
	}); err != nil {
		m.logger.Error("Failed to encode internal error response", zap.Error(err))
	}
}

func (m *RBACMiddleware) paymentRequired(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error": message,
		"code":  "payment_required",
	}); err != nil {
		m.logger.Error("Failed to encode payment required response", zap.Error(err))
	}
}
