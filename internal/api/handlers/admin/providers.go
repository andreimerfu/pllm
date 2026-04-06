package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
	providerService "github.com/amerfu/pllm/internal/services/integrations/provider"
)

// ProviderHandler handles CRUD operations for provider profiles.
type ProviderHandler struct {
	baseHandler
	db      *gorm.DB
	service *providerService.Service
}

// NewProviderHandler creates a new ProviderHandler.
func NewProviderHandler(logger *zap.Logger, db *gorm.DB) *ProviderHandler {
	return &ProviderHandler{
		baseHandler: baseHandler{logger: logger},
		db:          db,
		service:     providerService.NewService(db, logger),
	}
}

// CreateProviderProfileRequest is the request body for creating a provider profile.
type CreateProviderProfileRequest struct {
	Name   string                           `json:"name"`
	Type   string                           `json:"type"`
	Config models.ProviderProfileConfigJSON `json:"config"`
}

// providerProfileResponse is the response format for a provider profile.
type providerProfileResponse struct {
	ID         string                           `json:"id"`
	Name       string                           `json:"name"`
	Type       string                           `json:"type"`
	Config     models.ProviderProfileConfigJSON `json:"config"`
	ModelCount int64                            `json:"model_count"`
	CreatedAt  string                           `json:"created_at"`
	UpdatedAt  string                           `json:"updated_at"`
}

var validProviderTypes = map[string]bool{
	"openai":      true,
	"anthropic":   true,
	"azure":       true,
	"bedrock":     true,
	"vertex":      true,
	"openrouter":  true,
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}

func toProviderProfileResponse(p models.ProviderProfile, modelCount int64) providerProfileResponse {
	maskedConfig := p.Config
	maskedConfig.APIKey = maskSecret(p.Config.APIKey)
	maskedConfig.OAuthToken = maskSecret(p.Config.OAuthToken)
	maskedConfig.AWSAccessKeyID = maskSecret(p.Config.AWSAccessKeyID)
	maskedConfig.AWSSecretAccessKey = maskSecret(p.Config.AWSSecretAccessKey)

	return providerProfileResponse{
		ID:         p.ID.String(),
		Name:       p.Name,
		Type:       p.Type,
		Config:     maskedConfig,
		ModelCount: modelCount,
		CreatedAt:  p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// ListProviders returns all provider profiles.
func (h *ProviderHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.service.List()
	if err != nil {
		h.logger.Error("Failed to list provider profiles", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to list provider profiles: "+err.Error())
		return
	}

	responses := make([]providerProfileResponse, 0, len(profiles))
	for _, p := range profiles {
		count, err := h.service.GetModelCount(p.ID)
		if err != nil {
			h.logger.Warn("Failed to get model count for provider profile",
				zap.String("id", p.ID.String()),
				zap.Error(err))
		}
		responses = append(responses, toProviderProfileResponse(p, count))
	}

	h.sendResponse(w, http.StatusOK, map[string]interface{}{
		"providers": responses,
		"total":     len(responses),
	})
}

// CreateProvider creates a new provider profile.
func (h *ProviderHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req CreateProviderProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		h.sendError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Type == "" {
		h.sendError(w, http.StatusBadRequest, "type is required")
		return
	}
	if !validProviderTypes[req.Type] {
		h.sendError(w, http.StatusBadRequest, "invalid provider type: must be one of openai, anthropic, azure, bedrock, vertex, openrouter")
		return
	}

	profile := &models.ProviderProfile{
		Name:   req.Name,
		Type:   req.Type,
		Config: req.Config,
	}

	if err := h.service.Create(profile); err != nil {
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			h.sendError(w, http.StatusConflict, "A provider profile with this name already exists")
			return
		}
		h.logger.Error("Failed to create provider profile", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to create provider profile: "+err.Error())
		return
	}

	h.sendResponse(w, http.StatusCreated, toProviderProfileResponse(*profile, 0))
}

// GetProvider returns a single provider profile by ID.
func (h *ProviderHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "providerID")
	if providerID == "" {
		h.sendError(w, http.StatusBadRequest, "provider ID is required")
		return
	}

	id, err := uuid.Parse(providerID)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "invalid provider ID format")
		return
	}

	profile, err := h.service.Get(id)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "Provider profile not found")
		return
	}

	count, err := h.service.GetModelCount(id)
	if err != nil {
		h.logger.Warn("Failed to get model count for provider profile",
			zap.String("id", id.String()),
			zap.Error(err))
	}

	h.sendResponse(w, http.StatusOK, toProviderProfileResponse(*profile, count))
}

// UpdateProvider updates an existing provider profile.
func (h *ProviderHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "providerID")
	if providerID == "" {
		h.sendError(w, http.StatusBadRequest, "provider ID is required")
		return
	}

	id, err := uuid.Parse(providerID)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "invalid provider ID format")
		return
	}

	// Verify it exists first
	existing, err := h.service.Get(id)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "Provider profile not found")
		return
	}

	var req CreateProviderProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	updates := map[string]interface{}{}

	if strings.TrimSpace(req.Name) != "" {
		updates["name"] = req.Name
	}
	if req.Type != "" {
		if !validProviderTypes[req.Type] {
			h.sendError(w, http.StatusBadRequest, "invalid provider type: must be one of openai, anthropic, azure, bedrock, vertex, openrouter")
			return
		}
		updates["type"] = req.Type
	}

	// Merge config: preserve existing secrets when incoming fields are empty
	mergedConfig := existing.Config
	if req.Config.APIKey != "" {
		mergedConfig.APIKey = req.Config.APIKey
	}
	if req.Config.BaseURL != "" {
		mergedConfig.BaseURL = req.Config.BaseURL
	}
	if req.Config.OAuthToken != "" {
		mergedConfig.OAuthToken = req.Config.OAuthToken
	}
	if req.Config.AzureEndpoint != "" {
		mergedConfig.AzureEndpoint = req.Config.AzureEndpoint
	}
	if req.Config.AzureDeployment != "" {
		mergedConfig.AzureDeployment = req.Config.AzureDeployment
	}
	if req.Config.APIVersion != "" {
		mergedConfig.APIVersion = req.Config.APIVersion
	}
	if req.Config.AWSRegionName != "" {
		mergedConfig.AWSRegionName = req.Config.AWSRegionName
	}
	if req.Config.AWSAccessKeyID != "" {
		mergedConfig.AWSAccessKeyID = req.Config.AWSAccessKeyID
	}
	if req.Config.AWSSecretAccessKey != "" {
		mergedConfig.AWSSecretAccessKey = req.Config.AWSSecretAccessKey
	}
	if req.Config.VertexProject != "" {
		mergedConfig.VertexProject = req.Config.VertexProject
	}
	if req.Config.VertexLocation != "" {
		mergedConfig.VertexLocation = req.Config.VertexLocation
	}
	updates["config"] = mergedConfig

	updated, err := h.service.Update(id, updates)
	if err != nil {
		h.logger.Error("Failed to update provider profile", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to update provider profile: "+err.Error())
		return
	}

	count, err := h.service.GetModelCount(id)
	if err != nil {
		h.logger.Warn("Failed to get model count for provider profile",
			zap.String("id", id.String()),
			zap.Error(err))
	}

	h.sendResponse(w, http.StatusOK, toProviderProfileResponse(*updated, count))
}

// DeleteProvider deletes a provider profile by ID.
func (h *ProviderHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "providerID")
	if providerID == "" {
		h.sendError(w, http.StatusBadRequest, "provider ID is required")
		return
	}

	id, err := uuid.Parse(providerID)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "invalid provider ID format")
		return
	}

	if err := h.service.Delete(id); err != nil {
		// Check if it's a reference conflict
		if strings.Contains(err.Error(), "models reference it") {
			// Extract count from error message for the response
			count, _ := h.service.GetModelCount(id)
			h.sendResponse(w, http.StatusConflict, map[string]interface{}{
				"error":       err.Error(),
				"model_count": count,
			})
			return
		}
		if strings.Contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, "Provider profile not found")
			return
		}
		h.logger.Error("Failed to delete provider profile", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to delete provider profile: "+err.Error())
		return
	}

	h.sendResponse(w, http.StatusOK, map[string]string{
		"message": "Provider profile deleted successfully",
	})
}

// TestProvider tests provider connectivity (stub — delegates to model test-connection).
func (h *ProviderHandler) TestProvider(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Test provider")
}

// GetProviderModels returns models associated with a provider profile.
func (h *ProviderHandler) GetProviderModels(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Get provider models")
}

// AddModel adds a model to a provider profile.
func (h *ProviderHandler) AddModel(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Add model")
}

// UpdateModel updates a model within a provider profile.
func (h *ProviderHandler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Update model")
}

// DeleteModel removes a model from a provider profile.
func (h *ProviderHandler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Delete model")
}

// GetHealth returns health status for a provider profile.
func (h *ProviderHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Get health")
}
