package handlers

import (
	"encoding/json"
	"net/http"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/auth"
	"github.com/amerfu/pllm/internal/models"
	"github.com/amerfu/pllm/internal/middleware"
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
	h.sendResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "Token refresh not yet implemented",
	})
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

func (h *AuthHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	h.sendResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "List groups not yet implemented",
	})
}

func (h *AuthHandler) JoinGroup(w http.ResponseWriter, r *http.Request) {
	h.sendResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "Join group not yet implemented",
	})
}

func (h *AuthHandler) LeaveGroup(w http.ResponseWriter, r *http.Request) {
	h.sendResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "Leave group not yet implemented",
	})
}

func (h *AuthHandler) sendResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
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

	json.NewEncoder(w).Encode(errorData)
}