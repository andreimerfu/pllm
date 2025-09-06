package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/amerfu/pllm/internal/services"
	"github.com/amerfu/pllm/internal/services/models"
	"github.com/amerfu/pllm/internal/services/providers"
	"go.uber.org/zap"
)

type ModerationHandler struct {
	logger         *zap.Logger
	modelManager   *models.ModelManager
	metricsEmitter *services.MetricEventEmitter
}

func NewModerationHandler(logger *zap.Logger, modelManager *models.ModelManager) *ModerationHandler {
	return &ModerationHandler{
		logger:       logger,
		modelManager: modelManager,
	}
}

func NewModerationHandlerWithMetrics(logger *zap.Logger, modelManager *models.ModelManager, metricsEmitter *services.MetricEventEmitter) *ModerationHandler {
	return &ModerationHandler{
		logger:         logger,
		modelManager:   modelManager,
		metricsEmitter: metricsEmitter,
	}
}

// CreateModeration classifies if text violates OpenAI's Usage Policies
// @Summary Create moderation
// @Description Classifies if text violates OpenAI's Usage Policies
// @Tags Moderations
// @Accept json
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param request body providers.ModerationRequest true "Moderation request"
// @Success 200 {object} providers.ModerationResponse
// @Failure 400 {object} providers.ErrorResponse
// @Failure 401 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Router /moderations [post]
func (h *ModerationHandler) CreateModeration(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Moderation not yet implemented")
}

func (h *ModerationHandler) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(providers.ErrorResponse{
		Error: providers.APIError{
			Message: message,
			Type:    "invalid_request_error",
		},
	}); err != nil {
		h.logger.Error("Failed to encode moderation error response", zap.Error(err))
	}
}