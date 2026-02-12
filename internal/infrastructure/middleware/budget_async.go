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

	"github.com/amerfu/pllm/internal/core/auth"
	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/services/data/cache"
	"github.com/amerfu/pllm/internal/services/llm/providers"
	redisService "github.com/amerfu/pllm/internal/services/data/redis"
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
	logger         *zap.Logger
	authService    *auth.AuthService
	budgetCache    *redisService.BudgetCache
	eventPub       *redisService.EventPublisher
	usageQueue     *redisService.UsageQueue
	pricingManager *config.ModelPricingManager
	pricingCache   *cache.PricingCache
}

type AsyncBudgetConfig struct {
	Logger         *zap.Logger
	AuthService    *auth.AuthService
	BudgetCache    *redisService.BudgetCache
	EventPub       *redisService.EventPublisher
	UsageQueue     *redisService.UsageQueue
	PricingManager *config.ModelPricingManager
	PricingCache   *cache.PricingCache
}

func NewAsyncBudgetMiddleware(cfg *AsyncBudgetConfig) *AsyncBudgetMiddleware {
	return &AsyncBudgetMiddleware{
		logger:         cfg.Logger,
		authService:    cfg.AuthService,
		budgetCache:    cfg.BudgetCache,
		eventPub:       cfg.EventPub,
		usageQueue:     cfg.UsageQueue,
		pricingManager: cfg.PricingManager,
		pricingCache:   cfg.PricingCache,
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

		// Check if master key is being used (bypasses budget checks)
		if IsMasterKey(r.Context()) {
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

	// Read resolved model info from MetricsContext (set by chat handler after route resolution)
	metricsCtx := GetMetricsContext(ctx)

	actualModel := request.Model
	actualProvider := "pllm-gateway"
	routeSlug := ""
	providerModel := ""

	if metricsCtx != nil {
		if metricsCtx.ResolvedModel != "" {
			actualModel = metricsCtx.ResolvedModel
		}
		if metricsCtx.ProviderType != "" {
			actualProvider = metricsCtx.ProviderType
		}
		routeSlug = metricsCtx.RouteSlug
		providerModel = metricsCtx.ProviderModel
	}

	// Recalculate cost using the provider model ID for accurate pricing
	if providerModel != "" && m.pricingCache != nil {
		costCtx, costCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer costCancel()
		if calc, err := m.pricingCache.CalculateCost(costCtx, providerModel, inputTokens, outputTokens); err == nil {
			actualCost = calc.TotalCost
		}
	}

	// Get the actual user who made the request from context
	actualUserID, hasUser := GetUserID(ctx)

	// Create usage record for queue processing
	usageRecord := &redisService.UsageRecord{
		RequestID:     fmt.Sprintf("req_%d", time.Now().UnixNano()),
		Timestamp:     startTime,
		Model:         actualModel,
		Provider:      actualProvider,
		RouteSlug:     routeSlug,
		ProviderModel: providerModel,
		Method:       "POST",
		Path:         "/chat/completions",
		StatusCode:   writer.statusCode,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		TotalCost:    actualCost,
		Latency:      latency.Milliseconds(),
	}
	
	// Set ActualUserID only if user exists (not for system keys)
	if hasUser {
		usageRecord.ActualUserID = actualUserID.String()
	}

	// Set entity IDs and ownership information
	switch entityType {
	case "key":
		if keyUUID, err := uuid.Parse(entityID); err == nil {
			usageRecord.KeyID = keyUUID.String()
			// Get key ownership details
			if key, hasKey := GetKey(ctx); hasKey {
				usageRecord.KeyType = string(key.Type) // Set key type
				
				// Set key owner (for personal keys)
				if key.UserID != nil {
					usageRecord.KeyOwnerID = key.UserID.String()
					usageRecord.UserID = key.UserID.String() // Key owner
				}
				// Set team info (for team keys)
				if key.TeamID != nil {
					usageRecord.TeamID = key.TeamID.String()
					// For team keys, UserID is the key owner (team), ActualUserID is who made request
					if hasUser {
						usageRecord.UserID = actualUserID.String()
					}
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
	// Try to use cached pricing first for better performance
	if m.pricingCache != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		calculation, err := m.pricingCache.CalculateCost(ctx, request.Model, 
			m.estimateInputTokens(request.Messages), 
			m.estimateOutputTokens(request))
		
		if err == nil {
			return calculation.TotalCost
		}

		m.logger.Warn("Failed to calculate cost using pricing cache, falling back to pricing manager", 
			zap.String("model", request.Model),
			zap.Error(err))
	}

	// Fallback to pricing manager
	if m.pricingManager == nil {
		m.logger.Warn("Neither pricing cache nor pricing manager available, using default cost")
		return 0.01 // Conservative default
	}

	// Use our pricing manager to calculate cost
	calculation, err := m.pricingManager.CalculateCost(request.Model, 
		m.estimateInputTokens(request.Messages), 
		m.estimateOutputTokens(request))
	
	if err != nil {
		m.logger.Warn("Failed to calculate cost using pricing manager", 
			zap.String("model", request.Model),
			zap.Error(err))
		return 0.01 // Conservative default
	}

	return calculation.TotalCost
}

func (m *AsyncBudgetMiddleware) estimateOutputTokens(request *providers.ChatRequest) int {
	outputTokens := 150 // Default estimate
	if request.MaxTokens != nil && *request.MaxTokens > 0 {
		outputTokens = *request.MaxTokens
	}
	return outputTokens
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
