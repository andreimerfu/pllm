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

type ImagesHandler struct {
	logger         *zap.Logger
	modelManager   *models.ModelManager
	metricsEmitter *services.MetricEventEmitter
}

func NewImagesHandler(logger *zap.Logger, modelManager *models.ModelManager) *ImagesHandler {
	return &ImagesHandler{
		logger:       logger,
		modelManager: modelManager,
	}
}

func NewImagesHandlerWithMetrics(logger *zap.Logger, modelManager *models.ModelManager, metricsEmitter *services.MetricEventEmitter) *ImagesHandler {
	return &ImagesHandler{
		logger:         logger,
		modelManager:   modelManager,
		metricsEmitter: metricsEmitter,
	}
}

// GenerateImage creates images from a text prompt
// @Summary Generate image
// @Description Creates an image given a prompt
// @Tags Images
// @Accept json
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param request body providers.ImageRequest true "Image generation request"
// @Success 200 {object} providers.ImageResponse
// @Failure 400 {object} providers.ErrorResponse
// @Failure 401 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Router /images/generations [post]
func (h *ImagesHandler) GenerateImage(w http.ResponseWriter, r *http.Request) {
	var request providers.ImageRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if request.Prompt == "" {
		h.sendError(w, http.StatusBadRequest, "Prompt is required")
		return
	}
	
	// Set default model if not specified
	if request.Model == "" {
		request.Model = "dall-e-3"
	}

	// Get model instance and provider
	instance, err := h.modelManager.GetBestInstanceAdaptive(r.Context(), request.Model)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Model not available: %s", err.Error()))
		return
	}
	provider := instance.Provider

	// Call image generation endpoint
	response, err := provider.ImageGeneration(r.Context(), &request)
	if err != nil {
		h.logger.Error("Image generation failed", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode image response", zap.Error(err))
	}
}

// EditImage creates an edited or extended image given an original image and a prompt
// @Summary Edit image
// @Description Creates an edited or extended image given an original image and a prompt
// @Tags Images
// @Accept multipart/form-data
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param image formData file true "The image to edit. Must be a valid PNG file, less than 4MB, and square."
// @Param prompt formData string true "A text description of the desired image(s)"
// @Param mask formData file false "An additional image whose fully transparent areas indicate where image should be edited"
// @Param model formData string false "The model to use for image generation"
// @Param n formData integer false "The number of images to generate. Must be between 1 and 10."
// @Param size formData string false "The size of the generated images"
// @Success 200 {object} providers.ImageResponse
// @Failure 400 {object} providers.ErrorResponse
// @Failure 401 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Router /images/edits [post]
func (h *ImagesHandler) EditImage(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Image editing not yet implemented")
}

// CreateImageVariation creates a variation of a given image
// @Summary Create image variation
// @Description Creates a variation of a given image
// @Tags Images
// @Accept multipart/form-data
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param image formData file true "The image to use as the basis for the variation(s). Must be a valid PNG file, less than 4MB, and square."
// @Param model formData string false "The model to use for image generation"
// @Param n formData integer false "The number of images to generate. Must be between 1 and 10."
// @Param size formData string false "The size of the generated images"
// @Success 200 {object} providers.ImageResponse
// @Failure 400 {object} providers.ErrorResponse
// @Failure 401 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Router /images/variations [post]
func (h *ImagesHandler) CreateImageVariation(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Image variation not yet implemented")
}

func (h *ImagesHandler) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(providers.ErrorResponse{
		Error: providers.APIError{
			Message: message,
			Type:    "invalid_request_error",
		},
	}); err != nil {
		h.logger.Error("Failed to encode images error response", zap.Error(err))
	}
}