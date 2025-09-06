package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/amerfu/pllm/internal/services"
	"github.com/amerfu/pllm/internal/services/models"
	"github.com/amerfu/pllm/internal/services/providers"
	"go.uber.org/zap"
)

type AdminHandler struct {
	logger         *zap.Logger
	modelManager   *models.ModelManager
	metricsEmitter *services.MetricEventEmitter
}

func NewAdminHandler(logger *zap.Logger, modelManager *models.ModelManager) *AdminHandler {
	return &AdminHandler{
		logger:       logger,
		modelManager: modelManager,
	}
}

func NewAdminHandlerWithMetrics(logger *zap.Logger, modelManager *models.ModelManager, metricsEmitter *services.MetricEventEmitter) *AdminHandler {
	return &AdminHandler{
		logger:         logger,
		modelManager:   modelManager,
		metricsEmitter: metricsEmitter,
	}
}

// ModelStats returns detailed statistics about model performance
// @Summary Get model performance statistics
// @Description Returns detailed performance metrics for all models including latency, health scores, and circuit breaker states
// @Tags Admin
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /admin/models/stats [get]
func (h *AdminHandler) ModelStats(w http.ResponseWriter, r *http.Request) {
	stats := h.modelManager.GetModelStats()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		h.logger.Error("Failed to encode model stats response", zap.Error(err))
	}
}

func (h *AdminHandler) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(providers.ErrorResponse{
		Error: providers.APIError{
			Message: message,
			Type:    "invalid_request_error",
		},
	}); err != nil {
		h.logger.Error("Failed to encode admin error response", zap.Error(err))
	}
}