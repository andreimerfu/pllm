package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/amerfu/pllm/internal/middleware"
	"github.com/amerfu/pllm/internal/services"
	"github.com/amerfu/pllm/internal/services/models"
	"github.com/amerfu/pllm/internal/services/providers"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type LLMHandler struct {
	logger         *zap.Logger
	modelManager   *models.ModelManager
	metricsEmitter *services.MetricEventEmitter
}

func NewLLMHandler(logger *zap.Logger, modelManager *models.ModelManager) *LLMHandler {
	return &LLMHandler{
		logger:       logger,
		modelManager: modelManager,
	}
}

func NewLLMHandlerWithMetrics(logger *zap.Logger, modelManager *models.ModelManager, metricsEmitter *services.MetricEventEmitter) *LLMHandler {
	return &LLMHandler{
		logger:         logger,
		modelManager:   modelManager,
		metricsEmitter: metricsEmitter,
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
		h.sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Debug logging
	h.logger.Info("Received chat completion request",
		zap.String("model", request.Model),
		zap.Int("num_messages", len(request.Messages)),
		zap.Bool("stream", request.Stream))
	
	for i, msg := range request.Messages {
		h.logger.Info("Message content",
			zap.Int("index", i),
			zap.String("role", msg.Role),
			zap.String("content_type", fmt.Sprintf("%T", msg.Content)))
	}

	// Populate metrics context if available
	if metricsCtx, ok := r.Context().Value(middleware.MetricsContextKey).(*middleware.MetricsContext); ok && metricsCtx != nil {
		metricsCtx.ModelName = request.Model
		// Extract user/team info from auth context if available
		// These will be populated by the auth middleware in the context
	}

	// Track request start for adaptive routing
	h.modelManager.RecordRequestStart(request.Model)

	// Get best instance for the model
	// Use adaptive routing for better high-load handling
	startTime := time.Now()
	instance, err := h.modelManager.GetBestInstanceAdaptive(r.Context(), request.Model)
	if err != nil {
		// Record failure for adaptive components
		h.modelManager.RecordRequestEnd(request.Model, time.Since(startTime), false, err)
		h.logger.Error("Failed to get model instance",
			zap.String("model", request.Model),
			zap.Error(err))
		h.sendError(w, http.StatusServiceUnavailable, "No instance available for model: "+request.Model)
		return
	}

	h.logger.Info("Selected instance for request",
		zap.String("requested_model", request.Model),
		zap.String("instance_id", instance.Config.ID),
		zap.String("provider_model", instance.Config.Provider.Model),
		zap.Bool("stream", request.Stream))

	// Handle streaming
	if request.Stream {
		h.logger.Info("Routing to streaming handler")
		h.handleStreamingChat(w, r, &request, instance, startTime)
		return
	}

	// Create a copy of the request with the provider's actual model name
	// Users call with their custom model name (e.g., "my-gpt-4")
	// But we need to send the actual provider model name (e.g., "gpt-4")
	providerRequest := request
	providerRequest.Model = instance.Config.Provider.Model

	// Forward request to provider
	response, err := instance.Provider.ChatCompletion(r.Context(), &providerRequest)
	latency := time.Since(startTime)
	latencyMs := latency.Milliseconds()

	if err != nil {
		instance.RecordError(err)
		// Record failure for adaptive components
		h.modelManager.RecordRequestEnd(request.Model, latency, false, err)
		h.logger.Error("Provider request failed", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Provider request failed")
		return
	}

	// Record successful request
	totalTokens := int32(response.Usage.TotalTokens)
	instance.RecordRequest(totalTokens, latencyMs)
	// Record success for adaptive components
	h.modelManager.RecordRequestEnd(request.Model, latency, true, nil)

	// Emit detailed metrics if metrics emitter is available
	if h.metricsEmitter != nil && middleware.GetMetricsContext(r.Context()) != nil {
		// Calculate cost (simple estimation - could be moved to a proper cost calculator)
		estimatedCost := float64(response.Usage.TotalTokens) * 0.001 // $0.001 per token estimate

		middleware.EmitDetailedResponse(r.Context(), h.metricsEmitter,
			int64(response.Usage.TotalTokens), int64(response.Usage.PromptTokens),
			int64(response.Usage.CompletionTokens), estimatedCost, false)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode LLM response", zap.Error(err))
	}
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
	models := h.modelManager.GetDetailedModelInfo()

	response := map[string]interface{}{
		"object": "list",
		"data":   models,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode embeddings response", zap.Error(err))
	}
}

func (h *LLMHandler) GetModel(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Get model endpoint not yet implemented")
}

// UploadFile handles file uploads for chat attachments
// @Summary Upload file
// @Description Upload a file for use in chat messages
// @Tags Files
// @Accept multipart/form-data
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param file formData file true "File to upload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} providers.ErrorResponse
// @Failure 413 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Router /files [post]
func (h *LLMHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (limit to 32MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to parse form: "+err.Error())
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "No file provided or invalid file")
		return
	}
	defer func() { _ = file.Close() }()

	// Validate file size (max 10MB)
	if fileHeader.Size > 10<<20 {
		h.sendError(w, http.StatusRequestEntityTooLarge, "File too large (max 10MB)")
		return
	}

	// Validate file type (images only for now)
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		// Try to detect from extension
		contentType = http.DetectContentType(make([]byte, 512))
	}
	
	if !strings.HasPrefix(contentType, "image/") {
		h.sendError(w, http.StatusBadRequest, "Only image files are supported")
		return
	}

	// Generate unique filename
	fileID := fmt.Sprintf("%d_%s", time.Now().Unix(), fileHeader.Filename)
	uploadDir := "./uploads"
	
	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		h.logger.Error("Failed to create upload directory", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to create upload directory")
		return
	}

	// Save file to disk
	filepath := fmt.Sprintf("%s/%s", uploadDir, fileID)
	dst, err := os.Create(filepath)
	if err != nil {
		h.logger.Error("Failed to create file", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to save file")
		return
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, file); err != nil {
		h.logger.Error("Failed to write file", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to save file")
		return
	}

	// Return file info
	response := map[string]interface{}{
		"id":       fileID,
		"filename": fileHeader.Filename,
		"size":     fileHeader.Size,
		"type":     contentType,
		"url":      fmt.Sprintf("/files/%s", fileID),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode upload response", zap.Error(err))
	}
}

func (h *LLMHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "List files not yet implemented")
}

// GetFile serves uploaded files
// @Summary Get file
// @Description Serve an uploaded file
// @Tags Files
// @Param fileID path string true "File ID"
// @Success 200 {file} binary
// @Failure 404 {object} providers.ErrorResponse
// @Router /files/{fileID} [get]
func (h *LLMHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	// Extract file ID from URL parameter
	fileID := chi.URLParam(r, "fileID")
	if fileID == "" {
		h.sendError(w, http.StatusBadRequest, "File ID required")
		return
	}

	// Basic security: prevent directory traversal
	if strings.Contains(fileID, "..") || strings.Contains(fileID, "/") {
		h.sendError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	filepath := fmt.Sprintf("./uploads/%s", fileID)
	
	// Check if file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		h.sendError(w, http.StatusNotFound, "File not found")
		return
	}

	// Serve the file
	http.ServeFile(w, r, filepath)
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

// ModelStats returns detailed statistics about model performance
// @Summary Get model performance statistics
// @Description Returns detailed performance metrics for all models including latency, health scores, and circuit breaker states
// @Tags Admin
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /admin/models/stats [get]
func (h *LLMHandler) ModelStats(w http.ResponseWriter, r *http.Request) {
	stats := h.modelManager.GetModelStats()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		h.logger.Error("Failed to encode model stats response", zap.Error(err))
	}
}

// handleStreamingChat handles streaming chat completion requests
func (h *LLMHandler) handleStreamingChat(w http.ResponseWriter, r *http.Request, request *providers.ChatRequest, instance *models.ModelInstance, startTime time.Time) {
	// Populate metrics context if available
	if metricsCtx, ok := r.Context().Value(middleware.MetricsContextKey).(*middleware.MetricsContext); ok && metricsCtx != nil {
		metricsCtx.ModelName = request.Model
		// Extract user/team info from auth context if available
		// These will be populated by the auth middleware in the context
	}

	// Debug: write type to header
	w.Header().Set("X-Debug-Writer-Type", fmt.Sprintf("%T", w))

	h.logger.Info("Starting streaming request",
		zap.String("model", request.Model),
		zap.String("provider_model", instance.Config.Provider.Model),
		zap.String("writer_type", fmt.Sprintf("%T", w)))

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable Nginx buffering

	// Our middleware ensures w is a StreamingResponseWriter which implements Flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		// Add debug info to error response
		errMsg := fmt.Sprintf("Streaming not supported - Writer type: %T", w)
		h.logger.Error(errMsg,
			zap.String("writer_type", fmt.Sprintf("%T", w)),
			zap.String("model", request.Model))
		// Send error response
		h.sendError(w, http.StatusInternalServerError, errMsg)
		return
	}

	// Create a copy of the request with the provider's actual model name
	providerRequest := *request
	providerRequest.Model = instance.Config.Provider.Model

	// Get streaming response from provider
	streamChan, err := instance.Provider.ChatCompletionStream(r.Context(), &providerRequest)
	if err != nil {
		instance.RecordError(err)
		h.modelManager.RecordRequestEnd(request.Model, time.Since(startTime), false, err)
		h.logger.Error("Failed to start streaming",
			zap.String("model", request.Model),
			zap.Error(err))
		// Send error in SSE format
		_, _ = fmt.Fprintf(w, "data: {\"error\": {\"message\": \"%s\"}}\n\n", err.Error())
		flusher.Flush()
		return
	}

	// Stream responses to client
	tokenCount := 0
	chunkCount := 0
	h.logger.Debug("Starting to read from stream channel")

	for chunk := range streamChan {
		chunkCount++
		// Count tokens (approximate)
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != nil {
			if content, ok := chunk.Choices[0].Delta.Content.(string); ok {
				// Rough token estimation: 1 token per 4 characters
				tokenCount += len(content) / 4
				if tokenCount == 0 {
					tokenCount = 1
				}
			}
		}

		// Convert chunk to JSON
		data, err := json.Marshal(chunk)
		if err != nil {
			h.logger.Error("Failed to marshal streaming chunk", zap.Error(err))
			continue
		}

		// Write SSE formatted data
		_, err = fmt.Fprintf(w, "data: %s\n\n", data)
		if err != nil {
			// Client disconnected
			h.logger.Debug("Client disconnected during streaming",
				zap.String("model", request.Model))
			break
		}

		// Flush immediately for real-time streaming
		flusher.Flush()

		if chunkCount == 1 {
			h.logger.Debug("First chunk sent successfully")
		}
	}

	// Send final [DONE] marker
	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	// Record successful streaming request
	latency := time.Since(startTime)
	latencyMs := latency.Milliseconds()
	instance.RecordRequest(int32(tokenCount), latencyMs)
	h.modelManager.RecordRequestEnd(request.Model, latency, true, nil)

	// Emit detailed metrics for streaming request if metrics emitter is available
	if h.metricsEmitter != nil && middleware.GetMetricsContext(r.Context()) != nil {
		// Calculate cost (simple estimation for streaming - could be more sophisticated)
		estimatedCost := float64(tokenCount) * 0.001 // $0.001 per token estimate

		middleware.EmitDetailedResponse(r.Context(), h.metricsEmitter,
			int64(tokenCount), int64(tokenCount), // For streaming, we estimate input ≈ output for simplicity
			int64(tokenCount), estimatedCost, false)
	}

	h.logger.Info("Streaming request completed",
		zap.String("model", request.Model),
		zap.Int("chunks_sent", chunkCount),
		zap.Int("estimated_tokens", tokenCount),
		zap.Int64("latency_ms", latencyMs))
}

func (h *LLMHandler) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(providers.ErrorResponse{
		Error: providers.APIError{
			Message: message,
			Type:    "invalid_request_error",
		},
	}); err != nil {
		h.logger.Error("Failed to encode LLM error response", zap.Error(err))
	}
}
