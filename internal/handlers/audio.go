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

type AudioHandler struct {
	logger         *zap.Logger
	modelManager   *models.ModelManager
	metricsEmitter *services.MetricEventEmitter
}

func NewAudioHandler(logger *zap.Logger, modelManager *models.ModelManager) *AudioHandler {
	return &AudioHandler{
		logger:       logger,
		modelManager: modelManager,
	}
}

func NewAudioHandlerWithMetrics(logger *zap.Logger, modelManager *models.ModelManager, metricsEmitter *services.MetricEventEmitter) *AudioHandler {
	return &AudioHandler{
		logger:         logger,
		modelManager:   modelManager,
		metricsEmitter: metricsEmitter,
	}
}

// CreateTranscription transcribes audio into the input language
// @Summary Create transcription
// @Description Transcribes audio into the input language
// @Tags Audio
// @Accept multipart/form-data
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param file formData file true "The audio file object (not file name) to transcribe"
// @Param model formData string true "ID of the model to use"
// @Param language formData string false "The language of the input audio"
// @Param prompt formData string false "An optional text to guide the model's style"
// @Param response_format formData string false "The format of the transcript output"
// @Param temperature formData number false "The sampling temperature"
// @Success 200 {object} providers.TranscriptionResponse
// @Failure 400 {object} providers.ErrorResponse
// @Failure 401 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Router /audio/transcriptions [post]
func (h *AudioHandler) CreateTranscription(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	err := r.ParseMultipartForm(32 << 20) // 32MB max
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	// Get the uploaded file
	file, _, err := r.FormFile("file")
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Audio file is required")
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			h.logger.Warn("failed to close uploaded file", zap.Error(err))
		}
	}()

	// Get model parameter
	model := r.FormValue("model")
	if model == "" {
		h.sendError(w, http.StatusBadRequest, "Model parameter is required")
		return
	}

	// Build transcription request
	request := &providers.TranscriptionRequest{
		File:           file,
		Model:          model,
		Language:       r.FormValue("language"),
		Prompt:         r.FormValue("prompt"),
		ResponseFormat: r.FormValue("response_format"),
	}

	// Parse optional temperature
	if tempStr := r.FormValue("temperature"); tempStr != "" {
		temp := float32(0.0)
		if _, err := fmt.Sscanf(tempStr, "%f", &temp); err == nil {
			request.Temperature = &temp
		}
	}

	// Get model instance and provider
	instance, err := h.modelManager.GetBestInstanceAdaptive(r.Context(), model)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Model not available: %s", err.Error()))
		return
	}
	provider := instance.Provider

	// Call transcription endpoint
	response, err := provider.AudioTranscription(r.Context(), request)
	if err != nil {
		h.logger.Error("Audio transcription failed", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode transcription response", zap.Error(err))
	}
}

// CreateTranslation translates audio into English
// @Summary Create translation
// @Description Translates audio into English
// @Tags Audio
// @Accept multipart/form-data
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param file formData file true "The audio file object (not file name) to translate"
// @Param model formData string true "ID of the model to use"
// @Param prompt formData string false "An optional text to guide the model's style"
// @Param response_format formData string false "The format of the transcript output"
// @Param temperature formData number false "The sampling temperature"
// @Success 200 {object} providers.TranslationResponse
// @Failure 400 {object} providers.ErrorResponse
// @Failure 401 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Router /audio/translations [post]
func (h *AudioHandler) CreateTranslation(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Translation not yet implemented")
}

// CreateSpeech generates audio from the input text
// @Summary Create speech
// @Description Generates audio from the input text
// @Tags Audio
// @Accept json
// @Produce audio/mpeg
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param request body providers.SpeechRequest true "Speech synthesis request"
// @Success 200 {file} binary "The audio file content"
// @Failure 400 {object} providers.ErrorResponse
// @Failure 401 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Router /audio/speech [post]
func (h *AudioHandler) CreateSpeech(w http.ResponseWriter, r *http.Request) {
	var request providers.SpeechRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if request.Model == "" {
		h.sendError(w, http.StatusBadRequest, "Model is required")
		return
	}
	if request.Input == "" {
		h.sendError(w, http.StatusBadRequest, "Input text is required")
		return
	}
	if request.Voice == "" {
		h.sendError(w, http.StatusBadRequest, "Voice is required")
		return
	}

	// Get model instance and provider
	instance, err := h.modelManager.GetBestInstanceAdaptive(r.Context(), request.Model)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Model not available: %s", err.Error()))
		return
	}
	provider := instance.Provider

	// Call speech endpoint
	audioData, err := provider.AudioSpeech(r.Context(), &request)
	if err != nil {
		h.logger.Error("Audio speech synthesis failed", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Determine content type based on response format
	contentType := "audio/mpeg" // default
	if request.ResponseFormat != "" {
		switch request.ResponseFormat {
		case "mp3":
			contentType = "audio/mpeg"
		case "wav":
			contentType = "audio/wav"
		case "flac":
			contentType = "audio/flac"
		case "opus":
			contentType = "audio/ogg"
		}
	}

	// Return binary audio data
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(audioData)))
	if _, err := w.Write(audioData); err != nil {
		h.logger.Error("failed to write audio data", zap.Error(err))
	}
}

func (h *AudioHandler) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(providers.ErrorResponse{
		Error: providers.APIError{
			Message: message,
			Type:    "invalid_request_error",
		},
	}); err != nil {
		h.logger.Error("Failed to encode audio error response", zap.Error(err))
	}
}