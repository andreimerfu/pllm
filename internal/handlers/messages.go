package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/amerfu/pllm/internal/middleware"
	"github.com/amerfu/pllm/internal/services"
	"github.com/amerfu/pllm/internal/services/models"
	"github.com/amerfu/pllm/internal/services/providers"
	"go.uber.org/zap"
)

type MessagesHandler struct {
	logger         *zap.Logger
	modelManager   *models.ModelManager
	metricsEmitter *services.MetricEventEmitter
}

func NewMessagesHandler(logger *zap.Logger, modelManager *models.ModelManager) *MessagesHandler {
	return &MessagesHandler{
		logger:       logger,
		modelManager: modelManager,
	}
}

func NewMessagesHandlerWithMetrics(logger *zap.Logger, modelManager *models.ModelManager, metricsEmitter *services.MetricEventEmitter) *MessagesHandler {
	return &MessagesHandler{
		logger:         logger,
		modelManager:   modelManager,
		metricsEmitter: metricsEmitter,
	}
}

// AnthropicMessages creates a message completion in Anthropic format
// @Summary Create message completion (Anthropic format)
// @Description Creates a completion for the messages in Anthropic API format
// @Tags Messages
// @Accept json
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param request body providers.MessagesAPIRequest true "Anthropic messages request"
// @Success 200 {object} providers.MessagesAPIResponse
// @Failure 400 {object} providers.ErrorResponse
// @Failure 401 {object} providers.ErrorResponse
// @Failure 429 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Failure 503 {object} providers.ErrorResponse
// @Router /messages [post]
func (h *MessagesHandler) AnthropicMessages(w http.ResponseWriter, r *http.Request) {
	var request providers.MessagesAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.logger.Error("Failed to decode request body", zap.Error(err))
		h.sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate required fields
	if request.Model == "" {
		h.sendError(w, http.StatusBadRequest, "model is required")
		return
	}
	if request.MaxTokens <= 0 {
		h.sendError(w, http.StatusBadRequest, "max_tokens is required and must be positive")
		return
	}
	if len(request.Messages) == 0 {
		h.sendError(w, http.StatusBadRequest, "messages are required")
		return
	}

	// Debug logging
	h.logger.Info("Received Anthropic messages request",
		zap.String("model", request.Model),
		zap.Int("num_messages", len(request.Messages)),
		zap.Int("max_tokens", request.MaxTokens),
		zap.Bool("stream", request.Stream))

	// Convert Messages API format to OpenAI format for internal processing
	chatRequest, err := h.convertMessagesAPIToOpenAI(&request)
	if err != nil {
		h.logger.Error("Failed to convert Messages API request to OpenAI format", zap.Error(err))
		h.sendError(w, http.StatusBadRequest, "Invalid request format: "+err.Error())
		return
	}

	// Populate metrics context if available
	if metricsCtx, ok := r.Context().Value(middleware.MetricsContextKey).(*middleware.MetricsContext); ok && metricsCtx != nil {
		metricsCtx.ModelName = request.Model
	}

	// Track request start for adaptive routing
	h.modelManager.RecordRequestStart(request.Model)

	// Get best instance for the model
	startTime := time.Now()
	instance, err := h.modelManager.GetBestInstanceAdaptive(r.Context(), request.Model)
	if err != nil {
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
		h.handleStreamingMessages(w, r, &request, chatRequest, instance, startTime)
		return
	}

	// Create a copy of the request with the provider's actual model name
	providerRequest := *chatRequest
	providerRequest.Model = instance.Config.Provider.Model

	// Forward request to provider
	response, err := instance.Provider.ChatCompletion(r.Context(), &providerRequest)
	latency := time.Since(startTime)
	latencyMs := latency.Milliseconds()

	if err != nil {
		instance.RecordError(err)
		h.modelManager.RecordRequestEnd(request.Model, latency, false, err)
		h.logger.Error("Provider request failed", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Provider request failed")
		return
	}

	// Convert OpenAI response to Messages API format
	messagesResponse, err := h.convertOpenAIToMessagesAPI(response, &request)
	if err != nil {
		h.logger.Error("Failed to convert OpenAI response to Messages API format", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to process response")
		return
	}

	// Record successful request
	totalTokens := int32(response.Usage.TotalTokens)
	instance.RecordRequest(totalTokens, latencyMs)
	h.modelManager.RecordRequestEnd(request.Model, latency, true, nil)

	// Emit detailed metrics if metrics emitter is available
	if h.metricsEmitter != nil && middleware.GetMetricsContext(r.Context()) != nil {
		estimatedCost := float64(response.Usage.TotalTokens) * 0.001
		middleware.EmitDetailedResponse(r.Context(), h.metricsEmitter,
			int64(response.Usage.TotalTokens), int64(response.Usage.PromptTokens),
			int64(response.Usage.CompletionTokens), estimatedCost, false)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(messagesResponse); err != nil {
		h.logger.Error("Failed to encode Messages API response", zap.Error(err))
	}
}

func (h *MessagesHandler) handleStreamingMessages(w http.ResponseWriter, r *http.Request, 
	request *providers.MessagesAPIRequest, chatRequest *providers.ChatRequest, 
	instance *models.ModelInstance, startTime time.Time) {
	
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		errMsg := fmt.Sprintf("Streaming not supported - Writer type: %T", w)
		h.logger.Error(errMsg, zap.String("model", request.Model))
		h.sendError(w, http.StatusInternalServerError, errMsg)
		return
	}

	// Create a copy of the request with the provider's actual model name
	providerRequest := *chatRequest
	providerRequest.Model = instance.Config.Provider.Model

	// Get streaming response from provider
	streamChan, err := instance.Provider.ChatCompletionStream(r.Context(), &providerRequest)
	if err != nil {
		instance.RecordError(err)
		h.modelManager.RecordRequestEnd(request.Model, time.Since(startTime), false, err)
		h.logger.Error("Failed to start streaming", zap.String("model", request.Model), zap.Error(err))
		_, _ = fmt.Fprintf(w, "event: error\ndata: {\"error\": {\"message\": \"%s\"}}\n\n", err.Error())
		flusher.Flush()
		return
	}

	h.logger.Info("Starting stream processing loop", zap.String("model", request.Model))

	totalTokens := int64(0)
	promptTokens := int64(0)
	completionTokens := int64(0)

	// Stream the response in Anthropic format
	for streamResponse := range streamChan {
		h.logger.Debug("Received stream chunk", zap.Any("response", streamResponse))

		// Convert OpenAI stream response to Messages API stream format
		messagesStream, err := h.convertOpenAIStreamToMessagesAPI(streamResponse, request)
		if err != nil {
			h.logger.Error("Failed to convert stream response", zap.Error(err))
			continue
		}

		data, err := json.Marshal(messagesStream)
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

		// Track token usage estimation
		if len(streamResponse.Choices) > 0 && streamResponse.Choices[0].Delta.Content != nil {
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

func (h *MessagesHandler) convertMessagesAPIToOpenAI(req *providers.MessagesAPIRequest) (*providers.ChatRequest, error) {
	chatReq := &providers.ChatRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		MaxTokens:   &req.MaxTokens,
		Stop:        req.StopSequences,
	}

	// Convert messages
	chatReq.Messages = make([]providers.Message, len(req.Messages))
	for i, msg := range req.Messages {
		chatReq.Messages[i] = providers.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Handle system message if present
	if req.System != "" {
		systemMsg := providers.Message{
			Role:    "system",
			Content: req.System,
		}
		chatReq.Messages = append([]providers.Message{systemMsg}, chatReq.Messages...)
	}

	return chatReq, nil
}

func (h *MessagesHandler) convertOpenAIToMessagesAPI(resp *providers.ChatResponse, req *providers.MessagesAPIRequest) (*providers.MessagesAPIResponse, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := resp.Choices[0]
	content := []providers.MessagesAPIContent{}

	// Convert message content to Anthropic format
	if choice.Message.Content != nil {
		switch c := choice.Message.Content.(type) {
		case string:
			if c != "" {
				content = append(content, providers.MessagesAPIContent{
					Type: "text",
					Text: c,
				})
			}
		case []interface{}:
			// Handle complex content
			for _, item := range c {
				if contentItem, ok := item.(map[string]interface{}); ok {
					if contentType, exists := contentItem["type"].(string); exists && contentType == "text" {
						if text, textExists := contentItem["text"].(string); textExists {
							content = append(content, providers.MessagesAPIContent{
								Type: "text",
								Text: text,
							})
						}
					}
				}
			}
		}
	}

	// Map finish reason
	stopReason := "end_turn"
	if choice.FinishReason != "" {
		switch choice.FinishReason {
		case "stop":
			stopReason = "end_turn"
		case "length", "max_tokens":
			stopReason = "max_tokens"
		case "tool_calls":
			stopReason = "tool_use"
		default:
			stopReason = choice.FinishReason
		}
	}

	return &providers.MessagesAPIResponse{
		ID:         providers.GenerateMessagesAPIID(),
		Type:       "message",
		Role:       "assistant",
		Content:    content,
		Model:      req.Model,
		StopReason: stopReason,
		Usage: providers.MessagesAPIUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}, nil
}

func (h *MessagesHandler) convertOpenAIStreamToMessagesAPI(stream providers.StreamResponse, req *providers.MessagesAPIRequest) (*providers.MessagesAPIStreamResponse, error) {
	// Convert OpenAI stream format to Messages API stream format
	messagesStream := &providers.MessagesAPIStreamResponse{
		Type: "content_block_delta",
	}

	if len(stream.Choices) > 0 {
		choice := stream.Choices[0]
		messagesStream.Index = choice.Index

		if choice.Delta.Content != nil {
			// Create delta content for Messages API format
			messagesStream.Delta = map[string]interface{}{
				"type": "text_delta",
				"text": choice.Delta.Content,
			}
		}
	}

	return messagesStream, nil
}

func (h *MessagesHandler) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(providers.ErrorResponse{
		Error: providers.APIError{
			Message: message,
			Type:    "invalid_request_error",
		},
	}); err != nil {
		h.logger.Error("Failed to encode messages error response", zap.Error(err))
	}
}