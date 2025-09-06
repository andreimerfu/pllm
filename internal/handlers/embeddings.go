package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/amerfu/pllm/internal/services"
	"github.com/amerfu/pllm/internal/services/models"
	"github.com/amerfu/pllm/internal/services/providers"
	"go.uber.org/zap"
)

type EmbeddingsHandler struct {
	logger         *zap.Logger
	modelManager   *models.ModelManager
	metricsEmitter *services.MetricEventEmitter
}

func NewEmbeddingsHandler(logger *zap.Logger, modelManager *models.ModelManager) *EmbeddingsHandler {
	return &EmbeddingsHandler{
		logger:       logger,
		modelManager: modelManager,
	}
}

func NewEmbeddingsHandlerWithMetrics(logger *zap.Logger, modelManager *models.ModelManager, metricsEmitter *services.MetricEventEmitter) *EmbeddingsHandler {
	return &EmbeddingsHandler{
		logger:         logger,
		modelManager:   modelManager,
		metricsEmitter: metricsEmitter,
	}
}

// Embeddings creates embeddings for text input
// @Summary Create embeddings
// @Description Creates an embedding vector representing the input text
// @Tags Embeddings
// @Accept json
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param request body providers.EmbeddingsRequest true "Embeddings request"
// @Success 200 {object} providers.EmbeddingsResponse
// @Failure 400 {object} providers.ErrorResponse
// @Failure 401 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Router /embeddings [post]
func (h *EmbeddingsHandler) Embeddings(w http.ResponseWriter, r *http.Request) {
	var request providers.EmbeddingsRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get model instance and provider
	instance, err := h.modelManager.GetBestInstanceAdaptive(r.Context(), request.Model)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Model not available: %s", err.Error()))
		return
	}
	provider := instance.Provider

	// Call embeddings endpoint
	response, err := provider.Embeddings(r.Context(), &request)
	if err != nil {
		h.logger.Error("Embeddings request failed", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode embeddings response", zap.Error(err))
	}
}

func (h *EmbeddingsHandler) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(providers.ErrorResponse{
		Error: providers.APIError{
			Message: message,
			Type:    "invalid_request_error",
		},
	}); err != nil {
		h.logger.Error("Failed to encode embeddings error response", zap.Error(err))
	}
}