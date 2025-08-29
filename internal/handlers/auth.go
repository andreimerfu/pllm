package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/auth"
	"github.com/amerfu/pllm/internal/middleware"
	"github.com/amerfu/pllm/internal/models"
)

type AuthHandler struct {
	logger           *zap.Logger
	authService      *auth.AuthService
	masterKeyService *auth.MasterKeyService
	db               *gorm.DB
}

func NewAuthHandler(logger *zap.Logger, authService *auth.AuthService, masterKeyService *auth.MasterKeyService, db *gorm.DB) *AuthHandler {
	return &AuthHandler{
		logger:           logger,
		authService:      authService,
		masterKeyService: masterKeyService,
		db:               db,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	h.sendResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "Registration not yet implemented",
	})
}

// Login initiates Dex OAuth flow or validates master key
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Check if this is a Dex OAuth callback
	code := r.URL.Query().Get("code")
	if code != "" {
		// Handle Dex OAuth callback
		loginResp, err := h.authService.LoginWithDex(r.Context(), code)
		if err != nil {
			h.sendError(w, http.StatusUnauthorized, "Authentication failed", err)
			return
		}
		h.sendResponse(w, http.StatusOK, loginResp)
		return
	}

	// For non-Dex requests, return auth URL
	h.sendResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "Direct login not supported - use Dex OAuth flow",
	})
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request", err)
		return
	}

	if req.RefreshToken == "" {
		h.sendError(w, http.StatusBadRequest, "Missing refresh token", nil)
		return
	}

	// Use the auth service to refresh the token
	loginResp, err := h.authService.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Token refresh failed", err)
		return
	}

	h.sendResponse(w, http.StatusOK, loginResp)
}

func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	h.sendResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "Get profile not yet implemented",
	})
}

func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	h.sendResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "Update profile not yet implemented",
	})
}

// ChangePassword is not supported - passwords are handled by Dex
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusBadRequest, "Password changes not supported", fmt.Errorf("passwords are managed by Dex OAuth provider"))
}

// ListAPIKeys returns API keys for the authenticated user
func (h *AuthHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		h.sendError(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	var keys []models.Key
	err := h.db.Where("user_id = ? AND is_active = ?", userID, true).Find(&keys).Error
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to fetch keys", err)
		return
	}

	h.sendResponse(w, http.StatusOK, map[string]interface{}{
		"keys": keys,
	})
}

// CreateAPIKey creates a new API key for the authenticated user
func (h *AuthHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		h.sendError(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	var req models.KeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request", err)
		return
	}

	// Generate key
	keyValue, keyHash, err := models.GenerateKey(models.KeyTypeAPI)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to generate key", err)
		return
	}

	// Create key record
	key := &models.Key{
		Key:              keyValue,
		KeyHash:          keyHash,
		Name:             req.Name,
		Type:             models.KeyTypeAPI,
		UserID:           &userID,
		IsActive:         true,
		MaxBudget:        req.MaxBudget,
		BudgetDuration:   req.BudgetDuration,
		TPM:              req.TPM,
		RPM:              req.RPM,
		MaxParallelCalls: req.MaxParallelCalls,
		AllowedModels:    req.AllowedModels,
		BlockedModels:    req.BlockedModels,
		Scopes:           req.Scopes,
		Tags:             req.Tags,
		CreatedBy:        &userID,
	}

	if err := h.db.Create(key).Error; err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to create key", err)
		return
	}

	// Create audit entry
	auditEntry := &models.Audit{
		EventType:    models.AuditEventKeyCreate,
		EventAction:  "api_key_create",
		EventResult:  models.AuditResultSuccess,
		UserID:       &userID,
		KeyID:        &key.ID,
		ResourceType: "key",
		ResourceID:   &key.ID,
		Message:      "API key created",
	}
	h.db.Create(auditEntry)

	response := &models.KeyResponse{
		Key:      *key,
		KeyValue: keyValue, // Only returned on creation
	}

	h.sendResponse(w, http.StatusCreated, response)
}

// DeleteAPIKey revokes an API key
func (h *AuthHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		h.sendError(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	// Get key ID from URL path
	keyIDStr := r.URL.Path[len("/api/keys/"):]
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid key ID", err)
		return
	}

	var key models.Key
	err = h.db.Where("id = ? AND user_id = ?", keyID, userID).First(&key).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.sendError(w, http.StatusNotFound, "Key not found", nil)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Database error", err)
		}
		return
	}

	// Revoke the key
	key.Revoke(userID, "Deleted by user")
	if err := h.db.Save(&key).Error; err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to revoke key", err)
		return
	}

	// Create audit entry
	auditEntry := &models.Audit{
		EventType:    models.AuditEventKeyRevoke,
		EventAction:  "api_key_delete",
		EventResult:  models.AuditResultSuccess,
		UserID:       &userID,
		KeyID:        &key.ID,
		ResourceType: "key",
		ResourceID:   &key.ID,
		Message:      "API key deleted by user",
	}
	h.db.Create(auditEntry)

	h.sendResponse(w, http.StatusOK, map[string]string{
		"message": "API key deleted successfully",
	})
}

func (h *AuthHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	h.sendResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "Get usage not yet implemented",
	})
}

func (h *AuthHandler) GetDailyUsage(w http.ResponseWriter, r *http.Request) {
	h.sendResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "Get daily usage not yet implemented",
	})
}

func (h *AuthHandler) GetMonthlyUsage(w http.ResponseWriter, r *http.Request) {
	h.sendResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "Get monthly usage not yet implemented",
	})
}

// GetBudgetStatus returns the user's budget status and team budgets
func (h *AuthHandler) GetBudgetStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		h.sendError(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	// Get user with budget data
	var user models.User
	err := h.db.First(&user, "id = ?", userID).Error
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to fetch user budget data", err)
		return
	}

	// Get user's teams with their budget data
	var teams []models.Team
	err = h.db.Raw(`
		SELECT t.id, t.name, t.max_budget, t.current_spend, t.budget_duration, t.budget_reset_at
		FROM teams t
		JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = ? AND t.is_active = true
	`, userID).Scan(&teams).Error
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to fetch team budget data", err)
		return
	}

	// Get user's active API keys with budget data
	var keys []models.Key
	err = h.db.Where("user_id = ? AND is_active = true", userID).Find(&keys).Error
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to fetch key budget data", err)
		return
	}

	// Format response
	response := map[string]interface{}{
		"user": map[string]interface{}{
			"id":              user.ID,
			"email":           user.Email,
			"max_budget":      user.MaxBudget,
			"current_spend":   user.CurrentSpend,
			"budget_duration": user.BudgetDuration,
			"budget_reset_at": user.BudgetResetAt,
			"should_alert":    user.ShouldAlertBudget(),
			"is_exceeded":     user.IsBudgetExceeded(),
		},
		"teams": make([]map[string]interface{}, 0),
		"keys":  make([]map[string]interface{}, 0),
	}

	// Add team budget data
	for _, team := range teams {
		usagePercent := 0.0
		if team.MaxBudget > 0 {
			usagePercent = (team.CurrentSpend / team.MaxBudget) * 100
		}

		response["teams"] = append(response["teams"].([]map[string]interface{}), map[string]interface{}{
			"id":              team.ID,
			"name":            team.Name,
			"max_budget":      team.MaxBudget,
			"current_spend":   team.CurrentSpend,
			"budget_duration": team.BudgetDuration,
			"budget_reset_at": team.BudgetResetAt,
			"usage_percent":   usagePercent,
			"should_alert":    team.ShouldAlertBudget(),
			"is_exceeded":     team.IsBudgetExceeded(),
		})
	}

	// Add key budget data
	for _, key := range keys {
		usagePercent := 0.0
		if key.MaxBudget != nil && *key.MaxBudget > 0 {
			usagePercent = (key.CurrentSpend / *key.MaxBudget) * 100
		}

		response["keys"] = append(response["keys"].([]map[string]interface{}), map[string]interface{}{
			"id":              key.ID,
			"name":            key.Name,
			"max_budget":      key.MaxBudget,
			"current_spend":   key.CurrentSpend,
			"budget_duration": key.BudgetDuration,
			"budget_reset_at": key.BudgetResetAt,
			"usage_percent":   usagePercent,
			"usage_count":     key.UsageCount,
			"total_cost":      key.TotalCost,
		})
	}

	h.sendResponse(w, http.StatusOK, response)
}

func (h *AuthHandler) sendResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

func (h *AuthHandler) sendError(w http.ResponseWriter, status int, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	errorData := map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"code":    status,
		},
	}

	if err != nil {
		h.logger.Error("Request error", zap.Error(err), zap.String("message", message), zap.Int("status", status))
		errorData["error"].(map[string]interface{})["details"] = err.Error()
	}

	if err := json.NewEncoder(w).Encode(errorData); err != nil {
		log.Printf("Failed to encode JSON error response: %v", err)
	}
}

// GetUserTeams returns the teams that the user belongs to
func (h *AuthHandler) GetUserTeams(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		h.sendError(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	// Get user's teams
	var teams []models.Team
	err := h.db.Raw(`
		SELECT t.id, t.name, t.description, t.max_budget, t.current_spend, t.budget_duration, t.budget_reset_at, t.is_active
		FROM teams t
		JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = ? AND t.is_active = true
		ORDER BY t.name
	`, userID).Scan(&teams).Error
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to fetch user teams", err)
		return
	}

	// Format response
	teamList := make([]map[string]interface{}, 0, len(teams))
	for _, team := range teams {
		usagePercent := 0.0
		if team.MaxBudget > 0 {
			usagePercent = (team.CurrentSpend / team.MaxBudget) * 100
		}

		teamList = append(teamList, map[string]interface{}{
			"id":              team.ID,
			"name":            team.Name,
			"description":     team.Description,
			"max_budget":      team.MaxBudget,
			"current_spend":   team.CurrentSpend,
			"budget_duration": team.BudgetDuration,
			"budget_reset_at": team.BudgetResetAt,
			"usage_percent":   usagePercent,
			"should_alert":    team.ShouldAlertBudget(),
			"is_exceeded":     team.IsBudgetExceeded(),
			"is_active":       team.IsActive,
		})
	}

	response := map[string]interface{}{
		"teams": teamList,
		"count": len(teamList),
	}

	h.sendResponse(w, http.StatusOK, response)
}
