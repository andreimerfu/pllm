package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/amerfu/pllm/internal/auth"
	"github.com/amerfu/pllm/internal/services/providers"
	redisService "github.com/amerfu/pllm/internal/services/redis"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ModelPricing holds token pricing for different models
var ModelPricing = map[string]struct {
	InputTokensPerDollar  float64
	OutputTokensPerDollar float64
}{
	"gpt-4":           {InputTokensPerDollar: 33333, OutputTokensPerDollar: 16667},     // $0.03/$0.06 per 1K tokens
	"gpt-4-turbo":     {InputTokensPerDollar: 100000, OutputTokensPerDollar: 33333},    // $0.01/$0.03 per 1K tokens
	"gpt-3.5-turbo":   {InputTokensPerDollar: 500000, OutputTokensPerDollar: 666667},   // $0.002/$0.0015 per 1K tokens
	"gpt-4o":          {InputTokensPerDollar: 200000, OutputTokensPerDollar: 100000},   // $0.005/$0.01 per 1K tokens
	"gpt-4o-mini":     {InputTokensPerDollar: 6666667, OutputTokensPerDollar: 2000000}, // $0.00015/$0.0005 per 1K tokens
	"claude-3-haiku":  {InputTokensPerDollar: 400000, OutputTokensPerDollar: 80000},    // $0.0025/$0.0125 per 1K tokens
	"claude-3-sonnet": {InputTokensPerDollar: 66667, OutputTokensPerDollar: 13333},     // $0.015/$0.075 per 1K tokens
	"claude-3-opus":   {InputTokensPerDollar: 13333, OutputTokensPerDollar: 2667},      // $0.075/$0.375 per 1K tokens
}

// EstimateTokens roughly estimates input tokens from request content
func EstimateTokens(text string) int {
	// Rough estimation: 1 token ≈ 4 characters for English text
	return len(text) / 4
}

// AsyncBudgetMiddleware provides high-performance budget checking with Redis
type AsyncBudgetMiddleware struct {
	logger      *zap.Logger
	authService *auth.AuthService
	budgetCache *redisService.BudgetCache
	eventPub    *redisService.EventPublisher
	usageQueue  *redisService.UsageQueue
}

type AsyncBudgetConfig struct {
	Logger      *zap.Logger
	AuthService *auth.AuthService
	BudgetCache *redisService.BudgetCache
	EventPub    *redisService.EventPublisher
	UsageQueue  *redisService.UsageQueue
}

func NewAsyncBudgetMiddleware(cfg *AsyncBudgetConfig) *AsyncBudgetMiddleware {
	return &AsyncBudgetMiddleware{
		logger:      cfg.Logger,
		authService: cfg.AuthService,
		budgetCache: cfg.BudgetCache,
		eventPub:    cfg.EventPub,
		usageQueue:  cfg.UsageQueue,
	}
}

// EnforceBudgetAsync provides fast, non-blocking budget enforcement
func (m *AsyncBudgetMiddleware) EnforceBudgetAsync(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply to LLM endpoints
		if !m.isLLMEndpoint(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Get user/key context from authentication
		userID, hasUser := GetUserID(r.Context())
		key, hasKey := GetKey(r.Context())

		if !hasUser && !hasKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Read and parse request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			m.logger.Error("Failed to read request body", zap.Error(err))
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		var chatRequest providers.ChatRequest
		if err := json.Unmarshal(body, &chatRequest); err != nil {
			m.logger.Error("Failed to parse chat request", zap.Error(err))
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Estimate cost for this request
		estimatedCost := m.estimateCost(&chatRequest)

		// Fast budget check using Redis cache
		var entityType, entityID string
		if hasKey && key != nil {
			entityType = "key"
			entityID = key.ID.String()
		} else if hasUser {
			entityType = "user"
			entityID = userID.String()
		}

		// Non-blocking budget check with Redis
		budgetOk, err := m.budgetCache.CheckBudgetAvailable(r.Context(), entityType, entityID, estimatedCost)
		if err != nil {
			// On cache error, allow request but log warning
			m.logger.Warn("Budget cache check failed, allowing request",
				zap.Error(err),
				zap.String("entity", fmt.Sprintf("%s:%s", entityType, entityID)))
			budgetOk = true
		}

		if !budgetOk {
			m.logger.Warn("Request rejected due to cached budget limit",
				zap.String("entity", fmt.Sprintf("%s:%s", entityType, entityID)),
				zap.Float64("estimated_cost", estimatedCost),
				zap.String("model", chatRequest.Model))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			if err := json.NewEncoder(w).Encode(providers.ErrorResponse{
				Error: providers.APIError{
					Message: "Budget limit exceeded. Please contact your administrator or upgrade your plan.",
					Type:    "insufficient_quota",
					Code:    "budget_exceeded",
				},
			}); err != nil {
				log.Printf("Failed to encode budget error response: %v", err)
			}
			return
		}

		// Create streaming-compatible response writer
		wrappedWriter := NewStreamingResponseWriter(w)
		startTime := time.Now()

		// Process the request
		next.ServeHTTP(wrappedWriter, r)

		// Asynchronously track usage - this is completely non-blocking
		go m.trackUsageAsync(r.Context(), chatRequest, wrappedWriter, estimatedCost, entityType, entityID, startTime)
	})
}

// trackUsageAsync records usage asynchronously using Redis queue
func (m *AsyncBudgetMiddleware) trackUsageAsync(ctx context.Context, request providers.ChatRequest,
	writer *StreamingResponseWriter, estimatedCost float64, entityType, entityID string, startTime time.Time) {

	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Panic in trackUsageAsync", zap.Any("panic", r))
		}
	}()

	// Only track successful responses
	if writer.statusCode >= 400 {
		return
	}

	latency := time.Since(startTime)
	var actualCost float64
	var inputTokens, outputTokens int

	// For all responses, use estimates since we use a streaming-compatible writer
	// The background worker can reconcile actual usage from provider responses later
	actualCost = estimatedCost
	inputTokens = m.estimateInputTokens(request.Messages)
	outputTokens = 150 // Default estimate - will be reconciled by worker

	// Get the actual user who made the request from context
	actualUserID, _ := GetUserID(ctx)

	// Create usage record for queue processing
	usageRecord := &redisService.UsageRecord{
		RequestID:    fmt.Sprintf("req_%d", time.Now().UnixNano()),
		Timestamp:    startTime,
		Model:        request.Model,
		Provider:     "pllm-gateway",
		Method:       "POST",
		Path:         "/chat/completions",
		StatusCode:   writer.statusCode,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		TotalCost:    actualCost,
		Latency:      latency.Milliseconds(),
		ActualUserID: actualUserID.String(), // Always track who made the request
	}

	// Set entity IDs and ownership information
	switch entityType {
	case "key":
		if keyUUID, err := uuid.Parse(entityID); err == nil {
			usageRecord.KeyID = keyUUID.String()
			// Get key ownership details
			if key, hasKey := GetKey(ctx); hasKey {
				// Set key owner (for personal keys)
				if key.UserID != nil {
					usageRecord.KeyOwnerID = key.UserID.String()
					usageRecord.UserID = key.UserID.String() // Key owner
				}
				// Set team info (for team keys)
				if key.TeamID != nil {
					usageRecord.TeamID = key.TeamID.String()
					// For team keys, UserID is the key owner (team), ActualUserID is who made request
					usageRecord.UserID = actualUserID.String()
				}
			}
		}
	case "user":
		usageRecord.UserID = entityID
		// For direct user requests, ActualUserID is the same as UserID
		usageRecord.ActualUserID = entityID
	}

	// Enqueue for batch processing - this is fire-and-forget
	if err := m.usageQueue.EnqueueUsage(context.Background(), usageRecord); err != nil {
		m.logger.Error("Failed to enqueue usage record",
			zap.Error(err),
			zap.String("entity", fmt.Sprintf("%s:%s", entityType, entityID)))
		return
	}

	// Asynchronously increment cached budget spent amount
	go m.updateBudgetCacheAsync(entityType, entityID, actualCost)

	// Publish usage event for real-time monitoring (optional)
	go func() {
		if err := m.eventPub.PublishUsageEvent(context.Background(),
			usageRecord.UserID,
			usageRecord.KeyID,
			request.Model,
			inputTokens,
			outputTokens,
			actualCost,
			latency); err != nil {
			log.Printf("Failed to publish usage event: %v", err)
		}
	}()

	m.logger.Debug("Usage tracked asynchronously",
		zap.String("entity", fmt.Sprintf("%s:%s", entityType, entityID)),
		zap.Float64("cost", actualCost),
		zap.Int("tokens", inputTokens+outputTokens),
		zap.Duration("latency", latency))
}

// updateBudgetCacheAsync updates the cached budget spending
func (m *AsyncBudgetMiddleware) updateBudgetCacheAsync(entityType, entityID string, cost float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := m.budgetCache.IncrementSpent(ctx, entityType, entityID, cost); err != nil {
		m.logger.Error("Failed to increment cached budget spent",
			zap.Error(err),
			zap.String("entity_type", entityType),
			zap.String("entity_id", entityID),
			zap.Float64("cost", cost))
	}
}

// Helper methods reused from original budget middleware
func (m *AsyncBudgetMiddleware) isLLMEndpoint(path string) bool {
	return strings.Contains(path, "/chat/completions") ||
		strings.Contains(path, "/completions") ||
		strings.Contains(path, "/embeddings")
}

func (m *AsyncBudgetMiddleware) estimateCost(request *providers.ChatRequest) float64 {
	pricing, exists := ModelPricing[request.Model]
	if !exists {
		pricing = ModelPricing["gpt-4"] // Conservative default
	}

	inputTokens := m.estimateInputTokens(request.Messages)
	outputTokens := 150 // Default estimate
	if request.MaxTokens != nil && *request.MaxTokens > 0 {
		outputTokens = *request.MaxTokens
	}

	inputCost := float64(inputTokens) / pricing.InputTokensPerDollar
	outputCost := float64(outputTokens) / pricing.OutputTokensPerDollar

	return inputCost + outputCost
}

func (m *AsyncBudgetMiddleware) estimateInputTokens(messages []providers.Message) int {
	tokens := 0
	for _, msg := range messages {
		if content, ok := msg.Content.(string); ok {
			tokens += EstimateTokens(content)
		}
	}
	return tokens
}
