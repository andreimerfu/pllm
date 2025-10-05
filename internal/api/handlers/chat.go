package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	"github.com/amerfu/pllm/internal/services/monitoring/metrics"
	"github.com/amerfu/pllm/internal/services/llm/models"
	"github.com/amerfu/pllm/internal/services/llm/providers"
	"go.uber.org/zap"
)

type ChatHandler struct {
	logger         *zap.Logger
	modelManager   *models.ModelManager
	metricsEmitter *metrics.MetricEventEmitter
}

func NewChatHandler(logger *zap.Logger, modelManager *models.ModelManager) *ChatHandler {
	return &ChatHandler{
		logger:       logger,
		modelManager: modelManager,
	}
}

func NewChatHandlerWithMetrics(logger *zap.Logger, modelManager *models.ModelManager, metricsEmitter *metrics.MetricEventEmitter) *ChatHandler {
	return &ChatHandler{
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
func (h *ChatHandler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
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
	startTime := time.Now()

	// Execute with automatic failover
	result, err := h.modelManager.ExecuteWithFailover(r.Context(), &models.FailoverRequest{
		ModelName: request.Model,
		ExecuteFunc: func(ctx context.Context, instance *models.ModelInstance) (interface{}, error) {
			// Create a copy of the request with the provider's actual model name
			providerRequest := request
			providerRequest.Model = instance.Config.Provider.Model

			// Handle streaming separately
			if request.Stream {
				// For streaming, we return a special marker that tells the handler to stream
				return map[string]interface{}{
					"__streaming__": true,
					"instance":      instance,
					"request":       &providerRequest,
				}, nil
			}

			// Forward request to provider (non-streaming)
			response, err := instance.Provider.ChatCompletion(ctx, &providerRequest)
			if err != nil {
				instance.RecordError(err)
				return nil, err
			}

			// Record successful request
			totalTokens := int32(response.Usage.TotalTokens)
			latencyMs := time.Since(startTime).Milliseconds()
			instance.RecordRequest(totalTokens, latencyMs)

			return response, nil
		},
	})

	if err != nil {
		// All failover attempts failed
		h.modelManager.RecordRequestEnd(request.Model, time.Since(startTime), false, err)
		h.logger.Error("Request failed after all failover attempts",
			zap.String("model", request.Model),
			zap.Error(err))
		h.sendError(w, http.StatusServiceUnavailable, "Request failed: "+err.Error())
		return
	}

	// Log failover information if any failovers occurred
	if len(result.Failovers) > 0 {
		h.logger.Info("Request succeeded after failover",
			zap.String("requested_model", request.Model),
			zap.String("final_instance", result.Instance.Config.ID),
			zap.Int("attempts", result.AttemptCount),
			zap.Strings("failovers", result.Failovers))
	}

	// Check if this is a streaming request
	if responseMap, ok := result.Response.(map[string]interface{}); ok {
		if isStreaming, exists := responseMap["__streaming__"]; exists && isStreaming == true {
			// Extract instance and request from response
			instance := responseMap["instance"].(*models.ModelInstance)
			providerRequest := responseMap["request"].(*providers.ChatRequest)
			
			h.logger.Info("Routing to streaming handler after failover",
				zap.String("requested_model", request.Model),
				zap.String("instance_id", instance.Config.ID),
				zap.Int("failover_attempts", result.AttemptCount))
			
			h.handleStreamingChat(w, r, providerRequest, instance, startTime)
			return
		}
	}

	// Non-streaming response
	response := result.Response.(*providers.ChatResponse)
	latency := time.Since(startTime)

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

func (h *ChatHandler) handleStreamingChat(w http.ResponseWriter, r *http.Request, request *providers.ChatRequest, instance *models.ModelInstance, startTime time.Time) {
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

	h.logger.Info("Starting stream processing loop",
		zap.String("model", request.Model))

	totalTokens := int64(0)
	promptTokens := int64(0)
	completionTokens := int64(0)

	// Stream the response
	for streamResponse := range streamChan {
		h.logger.Debug("Received stream chunk", zap.Any("response", streamResponse))

		// Replace model with user's requested model name
		streamResponse.Model = request.Model

		data, err := json.Marshal(streamResponse)
		if err != nil {
			h.logger.Error("Failed to marshal stream response", zap.Error(err))
			continue
		}

		_, writeErr := fmt.Fprintf(w, "data: %s\n\n", string(data))
		if writeErr != nil {
			h.logger.Error("Failed to write stream data", zap.Error(writeErr))
			break
		}
		flusher.Flush()

		// Track token usage from stream chunks if available
		if len(streamResponse.Choices) > 0 && streamResponse.Choices[0].Delta.Content != nil {
			// For simplicity, estimate 1 token per 4 characters
			content := fmt.Sprintf("%v", streamResponse.Choices[0].Delta.Content)
			completionTokens += int64(len(content) / 4)
		}
	}

	// Send final done message
	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	latency := time.Since(startTime)
	latencyMs := latency.Milliseconds()

	// Record successful streaming request
	instance.RecordRequest(int32(totalTokens), latencyMs)
	h.modelManager.RecordRequestEnd(request.Model, latency, true, nil)

	// Emit metrics for streaming if available
	if h.metricsEmitter != nil && middleware.GetMetricsContext(r.Context()) != nil {
		estimatedCost := float64(totalTokens) * 0.001
		middleware.EmitDetailedResponse(r.Context(), h.metricsEmitter,
			totalTokens, promptTokens, completionTokens, estimatedCost, true)
	}

	h.logger.Info("Streaming completed",
		zap.String("model", request.Model),
		zap.Int64("completion_tokens", completionTokens),
		zap.Int64("latency_ms", latencyMs))
}

// Completions creates a text completion (legacy)
// @Summary Create text completion (legacy)
// @Description Creates a completion for the provided prompt and parameters (legacy endpoint)
// @Tags Completions
// @Accept json
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param request body providers.CompletionRequest true "Completion request"
// @Success 200 {object} providers.CompletionResponse
// @Failure 400 {object} providers.ErrorResponse
// @Failure 401 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Router /completions [post]
func (h *ChatHandler) Completions(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Completions endpoint not yet implemented")
}

func (h *ChatHandler) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(providers.ErrorResponse{
		Error: providers.APIError{
			Message: message,
			Type:    "invalid_request_error",
		},
	}); err != nil {
		h.logger.Error("Failed to encode chat error response", zap.Error(err))
	}
}