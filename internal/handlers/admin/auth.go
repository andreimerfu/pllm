package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/auth"
)

type AuthHandler struct {
	baseHandler
	masterKeyService *auth.MasterKeyService
	db               *gorm.DB
}

func NewAuthHandler(logger *zap.Logger, masterKeyService *auth.MasterKeyService, db *gorm.DB) *AuthHandler {
	return &AuthHandler{
		baseHandler:      baseHandler{logger: logger},
		masterKeyService: masterKeyService,
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