package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/amerfu/pllm/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UserHandler handles user management endpoints
type UserHandler struct {
	logger *zap.Logger
	db     *gorm.DB
}

// NewUserHandler creates a new user handler
func NewUserHandler(logger *zap.Logger, db *gorm.DB) *UserHandler {
	return &UserHandler{
		logger: logger,
		db:     db,
	}
}

// UserResponse extends the User model with provider icon information
type UserResponse struct {
	models.User
	ProviderIcon string `json:"provider_icon"`
}

// ListUsers returns all users
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	var users []models.User

	query := h.db.Model(&models.User{}).
		Preload("Teams").
		Preload("Keys")

	// Filter by role if provided
	if role := r.URL.Query().Get("role"); role != "" {
		query = query.Where("role = ?", role)
	}

	// Filter by active status
	if active := r.URL.Query().Get("active"); active != "" {
		if active == "true" {
			query = query.Where("is_active = ?", true)
		} else if active == "false" {
			query = query.Where("is_active = ?", false)
		}
	}

	// Filter by team if provided
	if teamID := r.URL.Query().Get("team_id"); teamID != "" {
		query = query.Joins("JOIN team_members ON team_members.user_id = users.id").
			Where("team_members.team_id = ?", teamID)
	}

	if err := query.Find(&users).Error; err != nil {
		h.logger.Error("Failed to list users", zap.Error(err))
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	// Transform users to include provider icons
	var userResponses []UserResponse
	for _, user := range users {
		userResponses = append(userResponses, UserResponse{
			User:         user,
			ProviderIcon: getProviderIcon(user.ExternalProvider),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userResponses)
}

// GetUser returns a specific user
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	var user models.User
	if err := h.db.Preload("Teams.Team").
		Preload("Keys").
		First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			h.logger.Error("Failed to get user", zap.Error(err))
			http.Error(w, "Failed to get user", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// CreateUser creates a new user (usually auto-provisioned from Dex)
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email     string   `json:"email"`
		Username  string   `json:"username"`
		FirstName string   `json:"first_name"`
		LastName  string   `json:"last_name"`
		Role      string   `json:"role"`
		TeamIDs   []string `json:"team_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Email == "" || req.Username == "" {
		http.Error(w, "Email and username are required", http.StatusBadRequest)
		return
	}

	// Set default role if not provided
	if req.Role == "" {
		req.Role = string(models.RoleUser)
	}

	user := &models.User{
		Email:     req.Email,
		Username:  req.Username,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      models.UserRole(req.Role),
		IsActive:  true,
	}

	// Start transaction
	tx := h.db.Begin()

	if err := tx.Create(user).Error; err != nil {
		tx.Rollback()
		h.logger.Error("Failed to create user", zap.Error(err))
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Add to teams if specified
	for _, teamIDStr := range req.TeamIDs {
		teamID, err := uuid.Parse(teamIDStr)
		if err != nil {
			continue
		}

		member := &models.TeamMember{
			TeamID: teamID,
			UserID: user.ID,
			Role:   models.TeamRoleMember,
		}

		if err := tx.Create(member).Error; err != nil {
			h.logger.Warn("Failed to add user to team",
				zap.String("team_id", teamIDStr),
				zap.Error(err))
		}
	}

	if err := tx.Commit().Error; err != nil {
		h.logger.Error("Failed to commit transaction", zap.Error(err))
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// UpdateUser updates a user
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	var user models.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			h.logger.Error("Failed to find user", zap.Error(err))
			http.Error(w, "Failed to find user", http.StatusInternalServerError)
		}
		return
	}

	var req struct {
		FirstName        *string  `json:"first_name"`
		LastName         *string  `json:"last_name"`
		Role             *string  `json:"role"`
		IsActive         *bool    `json:"is_active"`
		MaxBudget        *float64 `json:"max_budget"`
		BudgetDuration   *string  `json:"budget_duration"`
		AllowedModels    []string `json:"allowed_models"`
		BlockedModels    []string `json:"blocked_models"`
		TPM              *int     `json:"tpm"`
		RPM              *int     `json:"rpm"`
		MaxParallelCalls *int     `json:"max_parallel_calls"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Update fields if provided
	if req.FirstName != nil {
		user.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		user.LastName = *req.LastName
	}
	if req.Role != nil {
		user.Role = models.UserRole(*req.Role)
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.MaxBudget != nil {
		user.MaxBudget = *req.MaxBudget
	}
	if req.BudgetDuration != nil {
		user.BudgetDuration = models.BudgetPeriod(*req.BudgetDuration)
		// Reset budget period if changed
		user.BudgetResetAt = calculateNextReset(user.BudgetDuration)
	}
	if len(req.AllowedModels) > 0 {
		user.AllowedModels = req.AllowedModels
	}
	if len(req.BlockedModels) > 0 {
		user.BlockedModels = req.BlockedModels
	}
	if req.TPM != nil {
		user.TPM = *req.TPM
	}
	if req.RPM != nil {
		user.RPM = *req.RPM
	}
	if req.MaxParallelCalls != nil {
		user.MaxParallelCalls = *req.MaxParallelCalls
	}

	if err := h.db.Save(&user).Error; err != nil {
		h.logger.Error("Failed to update user", zap.Error(err))
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// DeleteUser deletes a user
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	// Soft delete by setting is_active to false
	result := h.db.Model(&models.User{}).
		Where("id = ?", userID).
		Update("is_active", false)

	if result.Error != nil {
		h.logger.Error("Failed to delete user", zap.Error(result.Error))
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetUserStats returns usage statistics for a user
func (h *UserHandler) GetUserStats(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	// Get user to ensure they exist
	var user models.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			h.logger.Error("Failed to find user", zap.Error(err))
			http.Error(w, "Failed to find user", http.StatusInternalServerError)
		}
		return
	}

	// Calculate usage statistics
	var stats struct {
		TotalRequests   int64      `json:"total_requests"`
		TotalTokens     int64      `json:"total_tokens"`
		TotalCost       float64    `json:"total_cost"`
		CurrentSpend    float64    `json:"current_spend"`
		MaxBudget       float64    `json:"max_budget"`
		BudgetRemaining float64    `json:"budget_remaining"`
		BudgetResetAt   time.Time  `json:"budget_reset_at"`
		LastUsedAt      *time.Time `json:"last_used_at"`
		ActiveKeys      int64      `json:"active_keys"`
		Teams           int64      `json:"teams"`
	}

	// Get usage stats
	h.db.Model(&models.Usage{}).
		Where("user_id = ?", userID).
		Select("COUNT(*) as total_requests, SUM(total_tokens) as total_tokens, SUM(cost) as total_cost").
		Scan(&stats)

	// Get last usage time
	var lastUsage models.Usage
	if err := h.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&lastUsage).Error; err == nil {
		stats.LastUsedAt = &lastUsage.CreatedAt
	}

	// Get active keys count
	h.db.Model(&models.Key{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Count(&stats.ActiveKeys)

	// Get teams count
	h.db.Model(&models.TeamMember{}).
		Where("user_id = ?", userID).
		Count(&stats.Teams)

	// Set budget info
	stats.CurrentSpend = user.CurrentSpend
	stats.MaxBudget = user.MaxBudget
	stats.BudgetRemaining = user.MaxBudget - user.CurrentSpend
	stats.BudgetResetAt = user.BudgetResetAt

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// ResetUserBudget resets a user's budget
func (h *UserHandler) ResetUserBudget(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	var user models.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			h.logger.Error("Failed to find user", zap.Error(err))
			http.Error(w, "Failed to find user", http.StatusInternalServerError)
		}
		return
	}

	// Reset budget
	user.CurrentSpend = 0
	user.BudgetResetAt = calculateNextReset(user.BudgetDuration)

	if err := h.db.Save(&user).Error; err != nil {
		h.logger.Error("Failed to reset user budget", zap.Error(err))
		http.Error(w, "Failed to reset budget", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Budget reset successfully",
		"user_id":    userID,
		"next_reset": user.BudgetResetAt,
	})
}

// Helper function to calculate next budget reset time
func calculateNextReset(period models.BudgetPeriod) time.Time {
	now := time.Now()
	switch period {
	case models.BudgetPeriodDaily:
		return now.Add(24 * time.Hour)
	case models.BudgetPeriodWeekly:
		return now.Add(7 * 24 * time.Hour)
	case models.BudgetPeriodMonthly:
		return now.AddDate(0, 1, 0)
	case models.BudgetPeriodYearly:
		return now.AddDate(1, 0, 0)
	default:
		return now.AddDate(0, 1, 0) // Default to monthly
	}
}

// getProviderIcon returns an icon identifier for the OAuth provider
func getProviderIcon(provider string) string {
	switch provider {
	case "github":
		return "github"
	case "google":
		return "google"
	case "microsoft":
		return "microsoft"
	case "gitlab":
		return "gitlab"
	case "ldap":
		return "ldap"
	case "local":
		return "key" // Key icon for local accounts
	default:
		return "user" // Generic user icon
	}
}
