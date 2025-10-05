package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/amerfu/pllm/internal/services/monitoring/metrics"
	"github.com/amerfu/pllm/internal/services/llm/models"
	"go.uber.org/zap"
)

type AdminHandler struct {
	logger         *zap.Logger
	modelManager   *models.ModelManager
	metricsEmitter *metrics.MetricEventEmitter
}

func NewAdminHandler(logger *zap.Logger, modelManager *models.ModelManager) *AdminHandler {
	return &AdminHandler{
		logger:       logger,
		modelManager: modelManager,
	}
}

func NewAdminHandlerWithMetrics(logger *zap.Logger, modelManager *models.ModelManager, metricsEmitter *metrics.MetricEventEmitter) *AdminHandler {
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

