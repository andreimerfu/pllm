package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/amerfu/pllm/internal/auth"
	"github.com/amerfu/pllm/internal/models"
	"github.com/amerfu/pllm/internal/services/providers"
	"go.uber.org/zap"
)

type BudgetMiddleware struct {
	logger      *zap.Logger
	authService *auth.AuthService
}

type BudgetConfig struct {
	Logger      *zap.Logger
	AuthService *auth.AuthService
}

func NewBudgetMiddleware(cfg *BudgetConfig) *BudgetMiddleware {
	return &BudgetMiddleware{
		logger:      cfg.Logger,
		authService: cfg.AuthService,
	}
}

// ModelPricing holds token pricing for different models
var ModelPricing = map[string]struct {
	InputTokensPerDollar  float64
	OutputTokensPerDollar float64
}{
	"gpt-4":              {InputTokensPerDollar: 33333, OutputTokensPerDollar: 16667},   // $0.03/$0.06 per 1K tokens
	"gpt-4-turbo":        {InputTokensPerDollar: 100000, OutputTokensPerDollar: 33333}, // $0.01/$0.03 per 1K tokens
	"gpt-3.5-turbo":      {InputTokensPerDollar: 500000, OutputTokensPerDollar: 666667}, // $0.002/$0.0015 per 1K tokens
	"gpt-4o":             {InputTokensPerDollar: 200000, OutputTokensPerDollar: 100000}, // $0.005/$0.01 per 1K tokens
	"gpt-4o-mini":        {InputTokensPerDollar: 6666667, OutputTokensPerDollar: 2000000}, // $0.00015/$0.0005 per 1K tokens
	"claude-3-haiku":     {InputTokensPerDollar: 400000, OutputTokensPerDollar: 80000},  // $0.0025/$0.0125 per 1K tokens
	"claude-3-sonnet":    {InputTokensPerDollar: 66667, OutputTokensPerDollar: 13333},   // $0.015/$0.075 per 1K tokens
	"claude-3-opus":      {InputTokensPerDollar: 13333, OutputTokensPerDollar: 2667},    // $0.075/$0.375 per 1K tokens
}

// EstimateTokens roughly estimates input tokens from request content
func EstimateTokens(text string) int {
	// Rough estimation: 1 token ≈ 4 characters for English text
	return len(text) / 4
}

// EstimateCost calculates estimated cost for a request
func (m *BudgetMiddleware) EstimateCost(request *providers.ChatRequest) float64 {
	pricing, exists := ModelPricing[request.Model]
	if !exists {
		// Default to GPT-4 pricing for unknown models (conservative estimate)
		pricing = ModelPricing["gpt-4"]
	}

	// Estimate input tokens from all messages
	inputTokens := 0
	for _, msg := range request.Messages {
		if content, ok := msg.Content.(string); ok {
			inputTokens += EstimateTokens(content)
		}
	}

	// Estimate output tokens based on max_tokens or default
	outputTokens := 150 // Default reasonable response length
	if request.MaxTokens != nil && *request.MaxTokens > 0 {
		outputTokens = *request.MaxTokens
	}

	// Calculate cost
	inputCost := float64(inputTokens) / pricing.InputTokensPerDollar
	outputCost := float64(outputTokens) / pricing.OutputTokensPerDollar

	return inputCost + outputCost
}

// CalculateActualCost calculates the actual cost based on usage
func (m *BudgetMiddleware) CalculateActualCost(model string, inputTokens, outputTokens int) float64 {
	pricing, exists := ModelPricing[model]
	if !exists {
		pricing = ModelPricing["gpt-4"]
	}

	inputCost := float64(inputTokens) / pricing.InputTokensPerDollar
	outputCost := float64(outputTokens) / pricing.OutputTokensPerDollar

	return inputCost + outputCost
}

// BudgetResponseWriter wraps http.ResponseWriter to capture response data
type BudgetResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func NewBudgetResponseWriter(w http.ResponseWriter) *BudgetResponseWriter {
	return &BudgetResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:          &bytes.Buffer{},
	}
}

func (w *BudgetResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *BudgetResponseWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

// Flush implements http.Flusher to support streaming responses
func (w *BudgetResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// EnforceBudget middleware checks and enforces budget limits
func (m *BudgetMiddleware) EnforceBudget(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply to LLM endpoints
		if !strings.Contains(r.URL.Path, "/chat/completions") && 
		   !strings.Contains(r.URL.Path, "/completions") &&
		   !strings.Contains(r.URL.Path, "/embeddings") {
			next.ServeHTTP(w, r)
			return
		}

		// Get user/key context from authentication
		userID, hasUser := GetUserID(r.Context())
		key, hasKey := GetKey(r.Context())
		
		if !hasUser && !hasKey {
			// No authentication context - should not happen with auth middleware
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
		estimatedCost := m.EstimateCost(&chatRequest)

		// Check budget based on auth type
		var budgetOk bool
		var budgetEntity string

		if hasKey && key != nil {
			// API Key authentication - check key budget
			budgetOk, err = m.authService.CheckBudgetCached(r.Context(), "key", key.ID.String(), estimatedCost)
			budgetEntity = fmt.Sprintf("key:%s", key.ID.String())
		} else if hasUser {
			// JWT authentication - check user budget  
			budgetOk, err = m.authService.CheckBudgetCached(r.Context(), "user", userID.String(), estimatedCost)
			budgetEntity = fmt.Sprintf("user:%s", userID.String())
		}

		if err != nil {
			m.logger.Error("Budget check failed", zap.Error(err), zap.String("entity", budgetEntity))
			// On budget check failure, allow request but log (prefer availability over strict enforcement)
			budgetOk = true
		}

		if !budgetOk {
			m.logger.Warn("Request rejected due to budget limit", 
				zap.String("entity", budgetEntity),
				zap.Float64("estimated_cost", estimatedCost),
				zap.String("model", chatRequest.Model))
			
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(providers.ErrorResponse{
				Error: providers.APIError{
					Message: "Budget limit exceeded. Please contact your administrator or upgrade your plan.",
					Type:    "insufficient_quota",
					Code:    "budget_exceeded",
				},
			})
			return
		}

		// Create wrapped response writer to capture actual usage
		wrappedWriter := NewBudgetResponseWriter(w)

		// Record request start time
		startTime := time.Now()

		// Process the request
		next.ServeHTTP(wrappedWriter, r)

		// Track actual usage after request completion
		go m.trackActualUsage(r.Context(), chatRequest, wrappedWriter, estimatedCost, budgetEntity, startTime)
	})
}

// trackActualUsage records the actual token usage and cost
func (m *BudgetMiddleware) trackActualUsage(ctx context.Context, request providers.ChatRequest, 
	writer *BudgetResponseWriter, estimatedCost float64, budgetEntity string, startTime time.Time) {
	
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Panic in trackActualUsage", zap.Any("panic", r))
		}
	}()

	// Only track successful responses
	if writer.statusCode >= 400 {
		return
	}

	var actualCost float64
	latency := time.Since(startTime)

	// For streaming responses, we need to estimate since we can't parse the full response
	if strings.Contains(writer.Header().Get("Content-Type"), "text/event-stream") {
		// For streaming, use estimated cost (token tracking happens in handler)
		actualCost = estimatedCost
		m.logger.Debug("Tracking streaming request usage", 
			zap.String("entity", budgetEntity),
			zap.Float64("estimated_cost", actualCost),
			zap.String("model", request.Model),
			zap.Duration("latency", latency))
	} else {
		// For non-streaming, try to parse actual usage from response
		var response providers.ChatResponse
		if err := json.Unmarshal(writer.body.Bytes(), &response); err == nil && response.Usage.TotalTokens > 0 {
			actualCost = m.CalculateActualCost(request.Model, 
				response.Usage.PromptTokens, 
				response.Usage.CompletionTokens)
			
			m.logger.Debug("Tracking non-streaming request usage",
				zap.String("entity", budgetEntity),
				zap.Float64("actual_cost", actualCost),
				zap.Int("input_tokens", response.Usage.PromptTokens),
				zap.Int("output_tokens", response.Usage.CompletionTokens),
				zap.String("model", request.Model),
				zap.Duration("latency", latency))
		} else {
			// Fallback to estimated cost
			actualCost = estimatedCost
			m.logger.Debug("Using estimated cost for tracking",
				zap.String("entity", budgetEntity),
				zap.Float64("estimated_cost", actualCost),
				zap.Error(err))
		}
	}

	// Record usage in database (via auth service)
	entityType := strings.Split(budgetEntity, ":")[0]
	entityID := strings.Split(budgetEntity, ":")[1]
	
	usage := &models.Usage{
		RequestID:    fmt.Sprintf("req_%d", time.Now().UnixNano()),
		Timestamp:    startTime,
		Model:        request.Model,
		Provider:     "pllm-gateway",
		Method:       "POST",
		Path:         "/chat/completions",
		StatusCode:   writer.statusCode,
		InputTokens:  EstimateTokens(fmt.Sprintf("%v", request.Messages)), // Rough estimate for tracking
		TotalCost:    actualCost,
		Latency:      latency.Milliseconds(),
	}

	// Use context with timeout for database operations
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set entity relationship - both UserID and KeyID are required by the model
	// Unified budget tracking: find user's key for JWT requests, find key's user for API requests
	if entityType == "user" {
		if userUUID, err := uuid.Parse(entityID); err == nil {
			usage.UserID = userUUID
			
			// Find user's active API key to unify budget tracking between JWT and API requests
			if err := m.findUserKey(dbCtx, usage, userUUID); err != nil {
				m.logger.Warn("Could not find active key for user, skipping usage recording", 
					zap.String("user_id", userUUID.String()),
					zap.Error(err))
				return
			}
		}
	} else if entityType == "key" {
		if keyUUID, err := uuid.Parse(entityID); err == nil {
			usage.KeyID = keyUUID
			
			// Find key's associated user for unified budget tracking
			if err := m.findKeyUser(dbCtx, usage, keyUUID); err != nil {
				m.logger.Warn("Could not find user for key, skipping usage recording", 
					zap.String("key_id", keyUUID.String()),
					zap.Error(err))
				return
			}
		}
	}

	if err := m.authService.RecordUsage(dbCtx, usage); err != nil {
		m.logger.Error("Failed to record usage", 
			zap.Error(err),
			zap.String("entity", budgetEntity),
			zap.Float64("cost", actualCost))
	}
}

// findUserKey looks up the user's first active API key for unified budget tracking
func (m *BudgetMiddleware) findUserKey(ctx context.Context, usage *models.Usage, userID uuid.UUID) error {
	// We need to access the database through AuthService
	// For now, let's create a minimal key lookup via a new AuthService method
	key, err := m.authService.GetUserActiveKey(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find active key for user %s: %w", userID.String(), err)
	}
	
	usage.KeyID = key.ID
	return nil
}

// findKeyUser looks up the user associated with an API key
func (m *BudgetMiddleware) findKeyUser(ctx context.Context, usage *models.Usage, keyID uuid.UUID) error {
	// We need to access the database through AuthService
	user, err := m.authService.GetKeyUser(ctx, keyID)
	if err != nil {
		return fmt.Errorf("failed to find user for key %s: %w", keyID.String(), err)
	}
	
	usage.UserID = user.ID
	return nil
}