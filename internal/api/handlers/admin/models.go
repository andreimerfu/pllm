package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/core/models"
	modelService "github.com/amerfu/pllm/internal/services/integrations/model"
	llmModels "github.com/amerfu/pllm/internal/services/llm/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ModelCRUDHandler handles CRUD operations for model instances.
type ModelCRUDHandler struct {
	baseHandler
	db           *gorm.DB
	modelManager *llmModels.ModelManager
	service      *modelService.Service
}

// NewModelCRUDHandler creates a new ModelCRUDHandler.
func NewModelCRUDHandler(logger *zap.Logger, db *gorm.DB, modelManager *llmModels.ModelManager) *ModelCRUDHandler {
	return &ModelCRUDHandler{
		baseHandler:  baseHandler{logger: logger},
		db:           db,
		modelManager: modelManager,
		service:      modelService.NewService(db, logger),
	}
}

// CreateModelRequest is the request body for creating a user model.
type CreateModelRequest struct {
	ModelName          string                    `json:"model_name"`
	InstanceName       string                    `json:"instance_name,omitempty"`
	Provider           models.ProviderConfigJSON `json:"provider"`
	ModelInfo          models.ModelInfoJSON      `json:"model_info"`
	RPM                int                       `json:"rpm,omitempty"`
	TPM                int                       `json:"tpm,omitempty"`
	Priority           int                       `json:"priority,omitempty"`
	Weight             float64                   `json:"weight,omitempty"`
	InputCostPerToken  float64                   `json:"input_cost_per_token,omitempty"`
	OutputCostPerToken float64                   `json:"output_cost_per_token,omitempty"`
	TimeoutSeconds     int                       `json:"timeout_seconds,omitempty"`
	Tags               []string                  `json:"tags,omitempty"`
	Enabled            *bool                     `json:"enabled,omitempty"`
}

// modelResponse is the response format for a single model in the list.
type modelResponse struct {
	ID                 string                     `json:"id"`
	ModelName          string                     `json:"model_name"`
	InstanceName       string                     `json:"instance_name,omitempty"`
	Source             string                     `json:"source"`
	Provider           *models.ProviderConfigJSON `json:"provider,omitempty"`
	ModelInfo          *models.ModelInfoJSON      `json:"model_info,omitempty"`
	OwnedBy            string                     `json:"owned_by,omitempty"`
	RPM                int                        `json:"rpm,omitempty"`
	TPM                int                        `json:"tpm,omitempty"`
	Priority           int                        `json:"priority,omitempty"`
	Weight             float64                    `json:"weight,omitempty"`
	InputCostPerToken  float64                    `json:"input_cost_per_token,omitempty"`
	OutputCostPerToken float64                    `json:"output_cost_per_token,omitempty"`
	TimeoutSeconds     int                        `json:"timeout_seconds,omitempty"`
	Tags               []string                   `json:"tags,omitempty"`
	Enabled            bool                       `json:"enabled"`
	CreatedAt          string                     `json:"created_at,omitempty"`
	CreatedByID        *uuid.UUID                 `json:"created_by_id,omitempty"`
}

// ListModels returns all models (system + user) with source information.
func (h *ModelCRUDHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	var allModels []modelResponse

	// 1. System models from the model manager
	systemModels := h.modelManager.GetDetailedModelInfo()
	for _, sm := range systemModels {
		resp := modelResponse{
			ID:        sm.ID,
			ModelName: sm.ID,
			Source:    sm.Source,
			OwnedBy:  sm.OwnedBy,
			Enabled:  true,
		}
		if resp.Source == "" {
			resp.Source = "system"
		}

		// Add tags from model manager
		if tags := h.modelManager.GetModelTags(sm.ID); len(tags) > 0 {
			resp.Tags = tags
		}

		allModels = append(allModels, resp)
	}

	// 2. User models from the database
	userModels, err := h.service.ListUserModels()
	if err != nil {
		h.logger.Error("Failed to list user models", zap.Error(err))
		// Continue with just system models
	} else {
		for _, um := range userModels {
			// Mask the API key
			maskedProvider := um.ProviderConfig
			if maskedProvider.APIKey != "" {
				maskedProvider.APIKey = "********"
			}
			if maskedProvider.APISecret != "" {
				maskedProvider.APISecret = "********"
			}
			if maskedProvider.AWSAccessKeyID != "" {
				maskedProvider.AWSAccessKeyID = "********"
			}
			if maskedProvider.AWSSecretAccessKey != "" {
				maskedProvider.AWSSecretAccessKey = "********"
			}

			resp := modelResponse{
				ID:                 um.ID.String(),
				ModelName:          um.ModelName,
				InstanceName:       um.InstanceName,
				Source:             "user",
				Provider:           &maskedProvider,
				ModelInfo:          &um.ModelInfoConfig,
				OwnedBy:           um.ProviderConfig.Type,
				RPM:                um.RPM,
				TPM:                um.TPM,
				Priority:           um.Priority,
				Weight:             um.Weight,
				InputCostPerToken:  um.InputCostPerToken,
				OutputCostPerToken: um.OutputCostPerToken,
				TimeoutSeconds:     um.TimeoutSeconds,
				Tags:               []string(um.Tags),
				Enabled:            um.Enabled,
				CreatedAt:          um.CreatedAt.Format("2006-01-02T15:04:05Z"),
				CreatedByID:        um.CreatedByID,
			}
			allModels = append(allModels, resp)
		}
	}

	// Deduplicate models that appear in both registry and database.
	// Database entries (appended last) have full provider details and should
	// take precedence over registry entries for the same model_name.
	modelMap := make(map[string]modelResponse)
	var order []string
	for _, m := range allModels {
		if _, exists := modelMap[m.ModelName]; !exists {
			order = append(order, m.ModelName)
		}
		modelMap[m.ModelName] = m // later entries (DB) overwrite earlier ones (registry)
	}
	deduped := make([]modelResponse, 0, len(order))
	for _, name := range order {
		deduped = append(deduped, modelMap[name])
	}

	h.sendResponse(w, http.StatusOK, map[string]interface{}{
		"models": deduped,
		"total":  len(deduped),
	})
}

// CreateModel creates a new user model, persists it to DB, and adds it to the registry.
func (h *ModelCRUDHandler) CreateModel(w http.ResponseWriter, r *http.Request) {
	var req CreateModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if req.ModelName == "" {
		h.sendError(w, http.StatusBadRequest, "model_name is required")
		return
	}
	if req.Provider.Type == "" {
		h.sendError(w, http.StatusBadRequest, "provider.type is required")
		return
	}
	if req.Provider.Model == "" {
		h.sendError(w, http.StatusBadRequest, "provider.model is required")
		return
	}

	// Provider-specific validation
	if err := validateProviderConfig(req.Provider); err != nil {
		h.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	um := &models.UserModel{
		ModelName:          req.ModelName,
		InstanceName:       req.InstanceName,
		ProviderConfig:     req.Provider,
		ModelInfoConfig:    req.ModelInfo,
		RPM:                req.RPM,
		TPM:                req.TPM,
		Priority:           req.Priority,
		Weight:             req.Weight,
		InputCostPerToken:  req.InputCostPerToken,
		OutputCostPerToken: req.OutputCostPerToken,
		TimeoutSeconds:     req.TimeoutSeconds,
		Tags:               models.StringArrayJSON(req.Tags),
		Enabled:            enabled,
	}

	if err := h.service.CreateUserModel(um); err != nil {
		// Check for unique constraint violation (SQLSTATE 23505)
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			h.sendError(w, http.StatusConflict, "A model with this name already exists")
			return
		}
		h.logger.Error("Failed to create user model", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to create model: "+err.Error())
		return
	}

	// Add to model registry
	instance := h.service.ConvertToModelInstance(*um)
	if err := h.modelManager.AddInstance(instance); err != nil {
		h.logger.Warn("Failed to add model to registry (will be loaded on restart)",
			zap.String("model", um.ModelName),
			zap.Error(err))
	}

	h.sendResponse(w, http.StatusCreated, um)
}

// GetModel returns a single model by ID (supports both system and user models).
func (h *ModelCRUDHandler) GetModel(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "modelID")
	if modelID == "" {
		h.sendError(w, http.StatusBadRequest, "model ID is required")
		return
	}

	// Try to parse as UUID (user model)
	if id, err := uuid.Parse(modelID); err == nil {
		um, err := h.service.GetUserModel(id)
		if err != nil {
			h.sendError(w, http.StatusNotFound, "Model not found")
			return
		}

		// Mask secrets
		maskedProvider := um.ProviderConfig
		if maskedProvider.APIKey != "" {
			maskedProvider.APIKey = "********"
		}
		if maskedProvider.APISecret != "" {
			maskedProvider.APISecret = "********"
		}
		if maskedProvider.AWSAccessKeyID != "" {
			maskedProvider.AWSAccessKeyID = "********"
		}
		if maskedProvider.AWSSecretAccessKey != "" {
			maskedProvider.AWSSecretAccessKey = "********"
		}

		resp := modelResponse{
			ID:                 um.ID.String(),
			ModelName:          um.ModelName,
			InstanceName:       um.InstanceName,
			Source:             "user",
			Provider:           &maskedProvider,
			ModelInfo:          &um.ModelInfoConfig,
			OwnedBy:           um.ProviderConfig.Type,
			RPM:                um.RPM,
			TPM:                um.TPM,
			Priority:           um.Priority,
			Weight:             um.Weight,
			InputCostPerToken:  um.InputCostPerToken,
			OutputCostPerToken: um.OutputCostPerToken,
			TimeoutSeconds:     um.TimeoutSeconds,
			Tags:               []string(um.Tags),
			Enabled:            um.Enabled,
			CreatedAt:          um.CreatedAt.Format("2006-01-02T15:04:05Z"),
			CreatedByID:        um.CreatedByID,
		}
		h.sendResponse(w, http.StatusOK, resp)
		return
	}

	// System model - look up by model name
	systemModels := h.modelManager.GetDetailedModelInfo()
	for _, sm := range systemModels {
		if sm.ID == modelID {
			resp := modelResponse{
				ID:        sm.ID,
				ModelName: sm.ID,
				Source:    "system",
				OwnedBy:  sm.OwnedBy,
				Enabled:  true,
			}
			if tags := h.modelManager.GetModelTags(sm.ID); len(tags) > 0 {
				resp.Tags = tags
			}
			h.sendResponse(w, http.StatusOK, resp)
			return
		}
	}

	h.sendError(w, http.StatusNotFound, "Model not found")
}

// UpdateModel updates a user model. Returns 403 for system models.
func (h *ModelCRUDHandler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "modelID")
	if modelID == "" {
		h.sendError(w, http.StatusBadRequest, "model ID is required")
		return
	}

	// Check if it's a UUID (user model)
	id, err := uuid.Parse(modelID)
	if err != nil {
		// Not a UUID - must be a system model
		h.sendError(w, http.StatusForbidden, "System models are managed via config.yaml")
		return
	}

	// Verify it's a user model
	existing, err := h.service.GetUserModel(id)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "Model not found")
		return
	}

	var req CreateModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Build updates map
	updates := map[string]interface{}{}
	if req.ModelName != "" {
		updates["model_name"] = req.ModelName
	}
	if req.InstanceName != "" {
		updates["instance_name"] = req.InstanceName
	}
	if req.Provider.Type != "" {
		// Merge with existing provider config to preserve secrets that
		// aren't being changed. The frontend sends empty strings for
		// masked fields (API keys, etc.) — keep the existing value.
		merged := existing.ProviderConfig
		merged.Type = req.Provider.Type
		if req.Provider.Model != "" {
			merged.Model = req.Provider.Model
		}
		if req.Provider.APIKey != "" {
			merged.APIKey = req.Provider.APIKey
		}
		if req.Provider.APISecret != "" {
			merged.APISecret = req.Provider.APISecret
		}
		if req.Provider.BaseURL != "" {
			merged.BaseURL = req.Provider.BaseURL
		}
		if req.Provider.APIVersion != "" {
			merged.APIVersion = req.Provider.APIVersion
		}
		if req.Provider.OrgID != "" {
			merged.OrgID = req.Provider.OrgID
		}
		if req.Provider.ProjectID != "" {
			merged.ProjectID = req.Provider.ProjectID
		}
		if req.Provider.Region != "" {
			merged.Region = req.Provider.Region
		}
		if req.Provider.Location != "" {
			merged.Location = req.Provider.Location
		}
		if req.Provider.AzureDeployment != "" {
			merged.AzureDeployment = req.Provider.AzureDeployment
		}
		if req.Provider.AzureEndpoint != "" {
			merged.AzureEndpoint = req.Provider.AzureEndpoint
		}
		if req.Provider.AWSAccessKeyID != "" {
			merged.AWSAccessKeyID = req.Provider.AWSAccessKeyID
		}
		if req.Provider.AWSSecretAccessKey != "" {
			merged.AWSSecretAccessKey = req.Provider.AWSSecretAccessKey
		}
		if req.Provider.AWSRegionName != "" {
			merged.AWSRegionName = req.Provider.AWSRegionName
		}
		if req.Provider.VertexProject != "" {
			merged.VertexProject = req.Provider.VertexProject
		}
		if req.Provider.VertexLocation != "" {
			merged.VertexLocation = req.Provider.VertexLocation
		}
		if req.Provider.ReasoningEffort != "" {
			merged.ReasoningEffort = req.Provider.ReasoningEffort
		}
		updates["provider_config"] = merged
	}
	if req.ModelInfo.Mode != "" || req.ModelInfo.SupportsStreaming || req.ModelInfo.SupportsFunctions || req.ModelInfo.SupportsVision {
		updates["model_info_config"] = req.ModelInfo
	}
	if req.RPM != 0 {
		updates["rpm"] = req.RPM
	}
	if req.TPM != 0 {
		updates["tpm"] = req.TPM
	}
	if req.Priority != 0 {
		updates["priority"] = req.Priority
	}
	if req.Weight != 0 {
		updates["weight"] = req.Weight
	}
	if req.InputCostPerToken != 0 {
		updates["input_cost_per_token"] = req.InputCostPerToken
	}
	if req.OutputCostPerToken != 0 {
		updates["output_cost_per_token"] = req.OutputCostPerToken
	}
	if req.TimeoutSeconds != 0 {
		updates["timeout_seconds"] = req.TimeoutSeconds
	}
	if req.Tags != nil {
		updates["tags"] = models.StringArrayJSON(req.Tags)
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	updated, err := h.service.UpdateUserModel(id, updates)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to update model: "+err.Error())
		return
	}

	// Update in registry: remove old, add new
	oldInstanceID := existing.ID.String()
	newInstance := h.service.ConvertToModelInstance(*updated)
	if err := h.modelManager.UpdateInstance(oldInstanceID, newInstance); err != nil {
		h.logger.Warn("Failed to update model in registry",
			zap.String("model", updated.ModelName),
			zap.Error(err))
	}

	h.sendResponse(w, http.StatusOK, updated)
}

// DeleteModel deletes a model (user or system) from the registry and database.
func (h *ModelCRUDHandler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "modelID")
	if modelID == "" {
		h.sendError(w, http.StatusBadRequest, "model ID is required")
		return
	}

	// Try as UUID first (user model stored in DB)
	if id, err := uuid.Parse(modelID); err == nil {
		// Delete from DB
		if err := h.service.DeleteUserModel(id); err != nil {
			h.sendError(w, http.StatusNotFound, "Model not found: "+err.Error())
			return
		}
	}

	// Remove from registry (works for both user and system models)
	if err := h.modelManager.RemoveInstance(modelID); err != nil {
		h.logger.Warn("Failed to remove model from registry",
			zap.String("id", modelID),
			zap.Error(err))
	}

	h.sendResponse(w, http.StatusOK, map[string]string{
		"message": "Model deleted successfully",
	})
}

// TestConnectionRequest is the request body for testing provider connectivity.
type TestConnectionRequest struct {
	Provider models.ProviderConfigJSON `json:"provider"`
}

// TestConnectionResponse is the response body for test-connection.
type TestConnectionResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Provider string `json:"provider"`
	Latency  string `json:"latency,omitempty"`
}

var testConnEnvVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// expandEnvVars expands environment variable references in the format ${VAR_NAME}.
func expandEnvVars(s string) string {
	return testConnEnvVarRegex.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1]
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match
	})
}

// convertProviderConfigToParams converts a ProviderConfigJSON to config.ProviderParams
// with environment variable expansion on secret fields.
func convertProviderConfigToParams(p models.ProviderConfigJSON) config.ProviderParams {
	return config.ProviderParams{
		Type:               p.Type,
		Model:              p.Model,
		APIKey:             expandEnvVars(p.APIKey),
		APISecret:          expandEnvVars(p.APISecret),
		BaseURL:            p.BaseURL,
		APIVersion:         p.APIVersion,
		OrgID:              p.OrgID,
		ProjectID:          p.ProjectID,
		Region:             p.Region,
		Location:           p.Location,
		AzureDeployment:    p.AzureDeployment,
		AzureEndpoint:      p.AzureEndpoint,
		AWSAccessKeyID:     expandEnvVars(p.AWSAccessKeyID),
		AWSSecretAccessKey: expandEnvVars(p.AWSSecretAccessKey),
		AWSRegionName:      p.AWSRegionName,
		VertexProject:      p.VertexProject,
		VertexLocation:     p.VertexLocation,
		ReasoningEffort:    p.ReasoningEffort,
	}
}

// TestConnection tests provider connectivity without persisting anything.
func (h *ModelCRUDHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	var req TestConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendResponse(w, http.StatusOK, TestConnectionResponse{
			Success:  false,
			Message:  "Invalid request body: " + err.Error(),
			Provider: "",
		})
		return
	}

	if req.Provider.Type == "" {
		h.sendResponse(w, http.StatusOK, TestConnectionResponse{
			Success:  false,
			Message:  "Provider type is required",
			Provider: "",
		})
		return
	}

	// Validate provider config
	if err := validateProviderConfig(req.Provider); err != nil {
		h.sendResponse(w, http.StatusOK, TestConnectionResponse{
			Success:  false,
			Message:  err.Error(),
			Provider: req.Provider.Type,
		})
		return
	}

	// Convert to ProviderParams with env var expansion
	params := convertProviderConfigToParams(req.Provider)

	// Create a temporary provider instance
	provider, err := h.modelManager.CreateProvider(params)
	if err != nil {
		h.sendResponse(w, http.StatusOK, TestConnectionResponse{
			Success:  false,
			Message:  "Failed to initialize provider: " + err.Error(),
			Provider: req.Provider.Type,
		})
		return
	}

	// Run health check with 10s timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	start := time.Now()
	err = provider.HealthCheck(ctx)
	latency := time.Since(start)

	if err != nil {
		h.sendResponse(w, http.StatusOK, TestConnectionResponse{
			Success:  false,
			Message:  "Health check failed: " + err.Error(),
			Provider: req.Provider.Type,
			Latency:  latency.Round(time.Millisecond).String(),
		})
		return
	}

	h.sendResponse(w, http.StatusOK, TestConnectionResponse{
		Success:  true,
		Message:  "Connection successful",
		Provider: req.Provider.Type,
		Latency:  latency.Round(time.Millisecond).String(),
	})
}

// GetModelsHealth returns aggregated health check results for all models.
func (h *ModelCRUDHandler) GetModelsHealth(w http.ResponseWriter, r *http.Request) {
	healthStore := h.modelManager.GetHealthStore()
	modelNames := h.modelManager.GetAvailableModels()

	if healthStore != nil {
		healthData, err := healthStore.GetAllModelsHealth(r.Context(), modelNames)
		if err != nil {
			h.logger.Error("Failed to get models health from Redis", zap.Error(err))
			h.sendError(w, http.StatusInternalServerError, "Failed to retrieve health data")
			return
		}
		h.sendResponse(w, http.StatusOK, map[string]interface{}{
			"models": healthData,
			"total":  len(healthData),
		})
		return
	}

	// Fallback: in-memory health from healthTracker (lite mode or no Redis)
	stats := h.modelManager.GetModelStats()
	h.sendResponse(w, http.StatusOK, map[string]interface{}{
		"models": stats["health"],
		"total":  len(modelNames),
	})
}

// validateProviderConfig checks provider-specific required fields.
func validateProviderConfig(p models.ProviderConfigJSON) error {
	switch p.Type {
	case "anthropic":
		if p.APIKey == "" {
			return fmt.Errorf("API key is required for Anthropic")
		}
	case "azure":
		if p.BaseURL == "" && p.AzureEndpoint == "" {
			return fmt.Errorf("endpoint URL is required for Azure OpenAI (set base_url or azure_endpoint)")
		}
	case "bedrock":
		if p.AWSAccessKeyID == "" || p.AWSSecretAccessKey == "" {
			return fmt.Errorf("AWS access key ID and secret access key are required for Bedrock")
		}
	case "vertex":
		if p.APIKey == "" {
			return fmt.Errorf("service account credentials (api_key) are required for Vertex AI")
		}
	case "openrouter":
		if p.APIKey == "" {
			return fmt.Errorf("API key is required for OpenRouter")
		}
	case "openai":
		// OpenAI doesn't strictly require an API key at construction time
		// (it's used in requests), but we warn if missing
	default:
		return fmt.Errorf("unsupported provider type: %s", p.Type)
	}
	return nil
}
