package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/amerfu/pllm/internal/services"
	"github.com/amerfu/pllm/internal/services/models"
	"github.com/amerfu/pllm/internal/services/providers"
	"go.uber.org/zap"
)

type ModelsHandler struct {
	logger         *zap.Logger
	modelManager   *models.ModelManager
	metricsEmitter *services.MetricEventEmitter
}

func NewModelsHandler(logger *zap.Logger, modelManager *models.ModelManager) *ModelsHandler {
	return &ModelsHandler{
		logger:       logger,
		modelManager: modelManager,
	}
}

func NewModelsHandlerWithMetrics(logger *zap.Logger, modelManager *models.ModelManager, metricsEmitter *services.MetricEventEmitter) *ModelsHandler {
	return &ModelsHandler{
		logger:         logger,
		modelManager:   modelManager,
		metricsEmitter: metricsEmitter,
	}
}

// ListModels lists available models
// @Summary List available models
// @Description Lists all available models from configured providers
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

	response := map[string]interface{}{
		"object": "list",
		"data":   models,
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