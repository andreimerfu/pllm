package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	
	"github.com/amerfu/pllm/internal/middleware"
	"github.com/amerfu/pllm/internal/models"
	"github.com/amerfu/pllm/internal/services/team"
	"github.com/amerfu/pllm/internal/services/virtualkey"
)

type KeyHandler struct {
	baseHandler
	keyService  *virtualkey.VirtualKeyService
	teamService *team.TeamService
}

func NewKeyHandler(logger *zap.Logger, keyService *virtualkey.VirtualKeyService, teamService *team.TeamService) *KeyHandler {
	return &KeyHandler{
		baseHandler: baseHandler{logger: logger},
		keyService:  keyService,
		teamService: teamService,
	}
}

// GenerateKey creates a new virtual key
func (h *KeyHandler) GenerateKey(w http.ResponseWriter, r *http.Request) {
	var req models.VirtualKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get creator ID from context
	var creatorID uuid.UUID
	if userID, ok := middleware.GetUserID(r.Context()); ok {
		creatorID = userID
	} else if middleware.IsMasterKey(r.Context()) {
		creatorID = uuid.New() // System user
	} else {
		h.sendError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// If not master key, ensure user can only create keys for themselves or their teams
	if !middleware.IsMasterKey(r.Context()) {
		if req.UserID != nil && *req.UserID != creatorID {
			h.sendError(w, http.StatusForbidden, "Cannot create keys for other users")
			return
		}
		// Check if user is member of the team if TeamID is provided
		if req.TeamID != nil {
			if err := h.validateTeamAccess(r.Context(), creatorID, *req.TeamID); err != nil {
				h.sendError(w, http.StatusForbidden, "Not a member of the specified team")
				return
			}
		}
	}

	key, err := h.keyService.CreateKey(r.Context(), &req, creatorID)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return the full key only on creation
	response := map[string]interface{}{
		"key":        key.Key,
		"key_id":     key.ID,
		"name":       key.Name,
		"created_at": key.CreatedAt,
		"expires_at": key.ExpiresAt,
		"metadata":   key.Metadata,
	}

	h.sendJSON(w, http.StatusCreated, response)
}

// ValidateKey validates a virtual key
func (h *KeyHandler) ValidateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	key, err := h.keyService.ValidateKey(r.Context(), req.Key)
	if err != nil {
		switch err {
		case virtualkey.ErrKeyNotFound:
			h.sendError(w, http.StatusNotFound, "Key not found")
		case virtualkey.ErrKeyExpired:
			h.sendError(w, http.StatusUnauthorized, "Key expired")
		case virtualkey.ErrKeyRevoked:
			h.sendError(w, http.StatusUnauthorized, "Key revoked")
		case virtualkey.ErrKeyInactive:
			h.sendError(w, http.StatusUnauthorized, "Key inactive")
		case virtualkey.ErrBudgetExceeded:
			h.sendError(w, http.StatusPaymentRequired, "Budget exceeded")
		default:
			h.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Don't return the actual key value in response
	response := map[string]interface{}{
		"valid":      true,
		"key_id":     key.ID,
		"name":       key.Name,
		"user_id":    key.UserID,
		"team_id":    key.TeamID,
		"expires_at": key.ExpiresAt,
		"metadata":   key.Metadata,
	}

	h.sendJSON(w, http.StatusOK, response)
}

// GetKey gets a key by ID
func (h *KeyHandler) GetKey(w http.ResponseWriter, r *http.Request) {
	keyID, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid key ID")
		return
	}

	key, err := h.keyService.GetKey(r.Context(), keyID)
	if err != nil {
		if err == virtualkey.ErrKeyNotFound {
			h.sendError(w, http.StatusNotFound, "Key not found")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Check permissions
	if !middleware.IsMasterKey(r.Context()) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			h.sendError(w, http.StatusForbidden, "Access denied")
			return
		}
		if err := h.validateKeyAccess(r.Context(), userID, key); err != nil {
			h.sendError(w, http.StatusForbidden, "Access denied")
			return
		}
	}

	// Don't return the actual key value
	key.Key = "sk-****" + key.Key[len(key.Key)-4:]

	h.sendJSON(w, http.StatusOK, key)
}

// UpdateKey updates a key
func (h *KeyHandler) UpdateKey(w http.ResponseWriter, r *http.Request) {
	keyID, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid key ID")
		return
	}

	// Check permissions
	key, err := h.keyService.GetKey(r.Context(), keyID)
	if err != nil {
		if err == virtualkey.ErrKeyNotFound {
			h.sendError(w, http.StatusNotFound, "Key not found")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if !middleware.IsMasterKey(r.Context()) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			h.sendError(w, http.StatusForbidden, "Access denied")
			return
		}
		if err := h.validateKeyAccess(r.Context(), userID, key); err != nil {
			h.sendError(w, http.StatusForbidden, "Access denied")
			return
		}
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Don't allow updating certain fields
	delete(updates, "key")
	delete(updates, "id")
	delete(updates, "created_at")
	delete(updates, "user_id")
	delete(updates, "team_id")

	updatedKey, err := h.keyService.UpdateKey(r.Context(), keyID, updates)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Don't return the actual key value
	updatedKey.Key = "sk-****" + updatedKey.Key[len(updatedKey.Key)-4:]

	h.sendJSON(w, http.StatusOK, updatedKey)
}

// RevokeKey revokes a key
func (h *KeyHandler) RevokeKey(w http.ResponseWriter, r *http.Request) {
	keyID, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid key ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Reason = "Revoked by user"
	}

	// Check permissions
	key, err := h.keyService.GetKey(r.Context(), keyID)
	if err != nil {
		if err == virtualkey.ErrKeyNotFound {
			h.sendError(w, http.StatusNotFound, "Key not found")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var revokerID uuid.UUID
	if userID, ok := middleware.GetUserID(r.Context()); ok {
		if !middleware.IsMasterKey(r.Context()) {
			if err := h.validateKeyAccess(r.Context(), userID, key); err != nil {
				h.sendError(w, http.StatusForbidden, "Access denied")
				return
			}
		}
		revokerID = userID
	} else if middleware.IsMasterKey(r.Context()) {
		revokerID = uuid.New() // System user
	} else {
		h.sendError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	if err := h.keyService.RevokeKey(r.Context(), keyID, revokerID, req.Reason); err != nil {
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]string{"message": "Key revoked successfully"})
}

// DeleteKey deletes a key
func (h *KeyHandler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	keyID, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid key ID")
		return
	}

	// Check permissions
	key, err := h.keyService.GetKey(r.Context(), keyID)
	if err != nil {
		if err == virtualkey.ErrKeyNotFound {
			h.sendError(w, http.StatusNotFound, "Key not found")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if !middleware.IsMasterKey(r.Context()) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			h.sendError(w, http.StatusForbidden, "Access denied")
			return
		}
		if err := h.validateKeyAccess(r.Context(), userID, key); err != nil {
			h.sendError(w, http.StatusForbidden, "Access denied")
			return
		}
	}

	if err := h.keyService.DeleteKey(r.Context(), keyID); err != nil {
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]string{"message": "Key deleted successfully"})
}

// ListKeys lists keys
func (h *KeyHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	
	offset := (page - 1) * limit

	var userID, teamID *uuid.UUID
	
	// Parse filters
	if uid := r.URL.Query().Get("user_id"); uid != "" {
		if id, err := uuid.Parse(uid); err == nil {
			userID = &id
		}
	}
	if tid := r.URL.Query().Get("team_id"); tid != "" {
		if id, err := uuid.Parse(tid); err == nil {
			teamID = &id
		}
	}

	// If not master key, restrict to user's own keys
	if !middleware.IsMasterKey(r.Context()) {
		if uid, ok := middleware.GetUserID(r.Context()); ok {
			userID = &uid
		}
	}

	keys, total, err := h.keyService.ListKeys(r.Context(), userID, teamID, limit, offset)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Mask the actual key values
	maskedKeys := make([]map[string]interface{}, len(keys))
	for i, key := range keys {
		maskedKeys[i] = map[string]interface{}{
			"id":            key.ID,
			"key":           "sk-****" + key.Key[len(key.Key)-4:],
			"name":          key.Name,
			"user_id":       key.UserID,
			"team_id":       key.TeamID,
			"is_active":     key.IsActive,
			"max_budget":    key.MaxBudget,
			"current_spend": key.CurrentSpend,
			"tpm":           key.TPM,
			"rpm":           key.RPM,
			"usage_count":   key.UsageCount,
			"total_tokens":  key.TotalTokens,
			"created_at":    key.CreatedAt,
			"last_used_at":  key.LastUsedAt,
			"expires_at":    key.ExpiresAt,
			"tags":          key.Tags,
		}
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"keys":  maskedKeys,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetKeyStats gets key statistics
func (h *KeyHandler) GetKeyStats(w http.ResponseWriter, r *http.Request) {
	keyID, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid key ID")
		return
	}

	// Check permissions
	key, err := h.keyService.GetKey(r.Context(), keyID)
	if err != nil {
		if err == virtualkey.ErrKeyNotFound {
			h.sendError(w, http.StatusNotFound, "Key not found")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if !middleware.IsMasterKey(r.Context()) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			h.sendError(w, http.StatusForbidden, "Access denied")
			return
		}
		if err := h.validateKeyAccess(r.Context(), userID, key); err != nil {
			h.sendError(w, http.StatusForbidden, "Access denied")
			return
		}
	}

	stats, err := h.keyService.GetKeyStats(r.Context(), keyID)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, stats)
}

// TemporaryBudgetIncrease temporarily increases a key's budget
func (h *KeyHandler) TemporaryBudgetIncrease(w http.ResponseWriter, r *http.Request) {
	keyID, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid key ID")
		return
	}

	var req struct {
		Amount   float64 `json:"amount"`
		Duration int     `json:"duration"` // in seconds
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Check permissions (only master key or key owner)
	key, err := h.keyService.GetKey(r.Context(), keyID)
	if err != nil {
		if err == virtualkey.ErrKeyNotFound {
			h.sendError(w, http.StatusNotFound, "Key not found")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if !middleware.IsMasterKey(r.Context()) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok || (key.UserID != nil && *key.UserID != userID) {
			h.sendError(w, http.StatusForbidden, "Access denied")
			return
		}
	}

	duration := time.Duration(req.Duration) * time.Second
	if err := h.keyService.TemporaryBudgetIncrease(r.Context(), keyID, req.Amount, duration); err != nil {
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "Budget temporarily increased",
		"amount":     req.Amount,
		"expires_in": req.Duration,
	})
}

// validateTeamAccess checks if user has access to the team
func (h *KeyHandler) validateTeamAccess(ctx context.Context, userID, teamID uuid.UUID) error {
	isMember, err := h.teamService.IsTeamMember(ctx, teamID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return fmt.Errorf("user is not a member of the team")
	}
	return nil
}

// validateKeyAccess checks if user has access to the key
func (h *KeyHandler) validateKeyAccess(ctx context.Context, userID uuid.UUID, key *models.VirtualKey) error {
	// User can access their own keys
	if key.UserID != nil && *key.UserID == userID {
		return nil
	}
	
	// User can access team keys if they're a team member
	if key.TeamID != nil {
		return h.validateTeamAccess(ctx, userID, *key.TeamID)
	}
	
	return fmt.Errorf("access denied")
}