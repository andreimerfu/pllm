package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/amerfu/pllm/internal/services/models"
	"github.com/amerfu/pllm/internal/services/providers"
	"go.uber.org/zap"
)

type LLMHandler struct {
	logger       *zap.Logger
	modelManager *models.ModelManager
}

func NewLLMHandler(logger *zap.Logger, modelManager *models.ModelManager) *LLMHandler {
	return &LLMHandler{
		logger:       logger,
		modelManager: modelManager,
	}
}

// ChatCompletions creates a chat completion
// @Summary Create chat completion
// @Description Creates a completion for the chat messages
// @Tags Chat
// @Accept json
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param request body providers.ChatRequest true "Chat completion request"
// @Success 200 {object} providers.ChatResponse
// @Failure 400 {object} providers.ErrorResponse
// @Failure 401 {object} providers.ErrorResponse
// @Failure 429 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Failure 503 {object} providers.ErrorResponse
// @Router /chat/completions [post]
func (h *LLMHandler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	var request providers.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.logger.Error("Failed to decode request body", zap.Error(err))
		h.sendError(w, http.StatusBadRequest, "Invalid request body: " + err.Error())
		return
	}

	// Get best instance for the model
	startTime := time.Now()
	instance, err := h.modelManager.GetBestInstance(r.Context(), request.Model)
	if err != nil {
		h.logger.Error("Failed to get model instance", zap.Error(err))
		h.sendError(w, http.StatusServiceUnavailable, "No instance available for model: "+request.Model)
		return
	}

	// Handle streaming
	if request.Stream {
		// TODO: Implement streaming
		h.sendError(w, http.StatusNotImplemented, "Streaming not yet implemented")
		return
	}

	// Forward request to provider
	response, err := instance.Provider.ChatCompletion(r.Context(), &request)
	latencyMs := time.Since(startTime).Milliseconds()
	
	if err != nil {
		instance.RecordError(err)
		h.logger.Error("Provider request failed", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Provider request failed")
		return
	}
	
	// Record successful request
	totalTokens := int32(response.Usage.TotalTokens)
	instance.RecordRequest(totalTokens, latencyMs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *LLMHandler) Completions(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Completions endpoint not yet implemented")
}

func (h *LLMHandler) Embeddings(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Embeddings endpoint not yet implemented")
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
func (h *LLMHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	models := h.modelManager.ListModels()
	
	response := map[string]interface{}{
		"object": "list",
		"data":   models,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *LLMHandler) GetModel(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Get model endpoint not yet implemented")
}

func (h *LLMHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "File upload not yet implemented")
}

func (h *LLMHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "List files not yet implemented")
}

func (h *LLMHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Get file not yet implemented")
}

func (h *LLMHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Delete file not yet implemented")
}

func (h *LLMHandler) GenerateImage(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Image generation not yet implemented")
}

func (h *LLMHandler) EditImage(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Image editing not yet implemented")
}

func (h *LLMHandler) CreateImageVariation(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Image variation not yet implemented")
}

func (h *LLMHandler) CreateTranscription(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Transcription not yet implemented")
}

func (h *LLMHandler) CreateTranslation(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Translation not yet implemented")
}

func (h *LLMHandler) CreateSpeech(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Speech synthesis not yet implemented")
}

func (h *LLMHandler) CreateModeration(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Moderation not yet implemented")
}

func (h *LLMHandler) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(providers.ErrorResponse{
		Error: providers.APIError{
			Message: message,
			Type:    "invalid_request_error",
		},
	})
}