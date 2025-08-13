package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type AuthHandler struct {
	baseHandler
	masterKey string
}

func NewAuthHandler(logger *zap.Logger, masterKey string) *AuthHandler {
	return &AuthHandler{
		baseHandler: baseHandler{logger: logger},
		masterKey:   masterKey,
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

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Check if master key matches
	if req.MasterKey == "" {
		h.sendError(w, http.StatusBadRequest, "Master key is required")
		return
	}

	// Default master key if not configured
	expectedKey := h.masterKey
	if expectedKey == "" {
		expectedKey = "sk-pllm-test-key-2024"
	}

	if req.MasterKey != expectedKey {
		h.logger.Warn("Invalid master key attempt",
			zap.String("provided_key_prefix", req.MasterKey[:min(10, len(req.MasterKey))]),
			zap.String("expected_key_prefix", expectedKey[:min(10, len(expectedKey))]),
		)
		h.sendError(w, http.StatusUnauthorized, "Invalid master key")
		return
	}

	// For now, return the master key as token (in production, generate JWT)
	response := LoginResponse{
		Success: true,
		Token:   req.MasterKey,
		Message: "Login successful",
	}

	h.sendJSON(w, http.StatusOK, response)
}

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

	// Default master key if not configured
	expectedKey := h.masterKey
	if expectedKey == "" {
		expectedKey = "sk-pllm-test-key-2024"
	}

	if token != expectedKey {
		h.sendError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Return success with some user info
	response := map[string]interface{}{
		"valid": true,
		"user": map[string]interface{}{
			"id":    "admin",
			"role":  "admin",
			"email": "admin@pllm.local",
		},
		"expires_at": time.Now().Add(24 * time.Hour).Unix(),
	}

	h.sendJSON(w, http.StatusOK, response)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}