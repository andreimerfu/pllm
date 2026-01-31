package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/services/monitoring/metrics"
	"github.com/amerfu/pllm/internal/services/llm/models"
	"github.com/amerfu/pllm/internal/services/llm/providers"
	"go.uber.org/zap"
)

type ModelsHandler struct {
	logger         *zap.Logger
	modelManager   *models.ModelManager
	pricingManager *config.ModelPricingManager
	metricsEmitter *metrics.MetricEventEmitter
}

func NewModelsHandler(logger *zap.Logger, modelManager *models.ModelManager, pricingManager *config.ModelPricingManager) *ModelsHandler {
	return &ModelsHandler{
		logger:         logger,
		modelManager:   modelManager,
		pricingManager: pricingManager,
	}
}

func NewModelsHandlerWithMetrics(logger *zap.Logger, modelManager *models.ModelManager, pricingManager *config.ModelPricingManager, metricsEmitter *metrics.MetricEventEmitter) *ModelsHandler {
	return &ModelsHandler{
		logger:         logger,
		modelManager:   modelManager,
		pricingManager: pricingManager,
		metricsEmitter: metricsEmitter,
	}
}

// ListModels lists available models
// @Summary List available models
// @Description Lists all available models from configured providers with pricing information
// @Tags Models
// @Accept json
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} providers.ErrorResponse
// @Router /models [get]
func (h *ModelsHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	models := h.modelManager.GetDetailedModelInfo()

	// Enhance models with pricing information
	enhancedModels := make([]map[string]interface{}, 0, len(models))
	for _, model := range models {
		// Convert ModelInfo struct to map with pricing
		modelMap := map[string]interface{}{
			"id":       model.ID,
			"object":   model.Object,
			"owned_by": model.OwnedBy,
		}

		// Add source field if available
		if model.Source != "" {
			modelMap["source"] = model.Source
		}

		// Add pricing information if available
		if h.pricingManager != nil && model.ID != "" {
			if pricingInfo := h.pricingManager.GetModelInfo(model.ID); pricingInfo != nil {
				// Add all pricing fields to the model
				for key, value := range pricingInfo {
					modelMap[key] = value
				}
			}
		}
		
		// Add tags from model manager if available
		if tags := h.modelManager.GetModelTags(model.ID); len(tags) > 0 {
			modelMap["tags"] = tags
		}
		enhancedModels = append(enhancedModels, modelMap)
	}

	response := map[string]interface{}{
		"object": "list",
		"data":   enhancedModels,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode models response", zap.Error(err))
	}
}

// GetModel retrieves a specific model
// @Summary Get model
// @Description Retrieves details about a specific model
// @Tags Models
// @Accept json
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param model path string true "Model ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} providers.ErrorResponse
// @Failure 404 {object} providers.ErrorResponse
// @Router /models/{model} [get]
func (h *ModelsHandler) GetModel(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Get model endpoint not yet implemented")
}

func (h *ModelsHandler) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(providers.ErrorResponse{
		Error: providers.APIError{
			Message: message,
			Type:    "invalid_request_error",
		},
	}); err != nil {
		h.logger.Error("Failed to encode models error response", zap.Error(err))
	}
}