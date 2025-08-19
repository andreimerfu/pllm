package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	
	"github.com/amerfu/pllm/internal/services/budget"
)

type UsageTracker struct {
	logger        *zap.Logger
	db            *gorm.DB
	budgetService *budget.BudgetService
}

type UsageConfig struct {
	Logger        *zap.Logger
	DB            *gorm.DB
	BudgetService *budget.BudgetService
}

type UsageMetrics struct {
	Tokens       int     `json:"tokens"`
	Cost         float64 `json:"cost"`
	Model        string  `json:"model"`
	Provider     string  `json:"provider"`
	Duration     int64   `json:"duration_ms"`
	StatusCode   int     `json:"status_code"`
	Error        string  `json:"error,omitempty"`
}

func NewUsageTracker(config *UsageConfig) *UsageTracker {
	return &UsageTracker{
		logger:        config.Logger,
		db:            config.DB,
		budgetService: config.BudgetService,
	}
}

func (u *UsageTracker) TrackUsage() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip non-API endpoints
			if !strings.HasPrefix(r.URL.Path, "/v1/") {
				next.ServeHTTP(w, r)
				return
			}

			// Record start time
			startTime := time.Now()

			// Wrap response writer to capture response
			wrapped := &responseCapture{ResponseWriter: w, statusCode: 200}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(startTime)

			// Extract usage metrics from response if applicable
			go u.recordUsage(r.Context(), r, wrapped, duration)
		})
	}
}

func (u *UsageTracker) recordUsage(ctx context.Context, r *http.Request, w *responseCapture, duration time.Duration) {
	// Only track usage for successful API calls
	if w.statusCode >= 400 {
		return
	}

	// Get authentication context
	authType := GetAuthType(ctx)
	if authType != AuthTypeAPIKey {
		return
	}

	key, ok := GetKey(ctx)
	if !ok {
		return
	}

	// Extract metrics from response
	metrics := u.extractMetrics(r, w)
	if metrics == nil {
		return
	}

	// Record usage in key directly
	key.RecordUsage(int(metrics.Tokens), metrics.Cost)
	if err := u.db.Save(key).Error; err != nil {
		u.logger.Error("Failed to record key usage", 
			zap.String("key_id", key.ID.String()),
			zap.Error(err))
	}

	// Record usage in budget service
	if key.UserID != nil {
		if err := u.budgetService.RecordUsage(ctx, key.UserID, nil, metrics.Cost); err != nil {
			u.logger.Error("Failed to record user budget usage",
				zap.String("user_id", key.UserID.String()),
				zap.Error(err))
		}
	}

	if key.TeamID != nil {
		if err := u.budgetService.RecordUsage(ctx, nil, key.TeamID, metrics.Cost); err != nil {
			u.logger.Error("Failed to record team budget usage",
				zap.String("team_id", key.TeamID.String()),
				zap.Error(err))
		}
	}

	// Log usage metrics
	u.logger.Info("API usage recorded",
		zap.String("key_id", key.ID.String()),
		zap.Int("tokens", metrics.Tokens),
		zap.Float64("cost", metrics.Cost),
		zap.String("model", metrics.Model),
		zap.Int64("duration_ms", duration.Milliseconds()))
}

func (u *UsageTracker) extractMetrics(r *http.Request, w *responseCapture) *UsageMetrics {
	// Parse response to extract token usage and cost
	if w.body == nil {
		return nil
	}

	// Try to parse as OpenAI-compatible response
	var response map[string]interface{}
	if err := json.Unmarshal(w.body.Bytes(), &response); err != nil {
		return nil
	}

	// Extract usage data
	metrics := &UsageMetrics{
		StatusCode: w.statusCode,
	}

	// Extract tokens from usage field
	if usage, ok := response["usage"].(map[string]interface{}); ok {
		if totalTokens, ok := usage["total_tokens"].(float64); ok {
			metrics.Tokens = int(totalTokens)
		}
	}

	// Extract model from response or request
	if model, ok := response["model"].(string); ok {
		metrics.Model = model
	} else {
		// Try to get from request body
		metrics.Model = u.extractModelFromRequest(r)
	}

	// Calculate cost (simplified - should use actual pricing)
	metrics.Cost = u.calculateCost(metrics.Model, metrics.Tokens)

	return metrics
}

func (u *UsageTracker) extractModelFromRequest(r *http.Request) string {
	// Clone the request body to avoid consuming it
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var reqBody map[string]interface{}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return ""
	}

	if model, ok := reqBody["model"].(string); ok {
		return model
	}
	return ""
}

func (u *UsageTracker) calculateCost(model string, tokens int) float64 {
	// Simplified cost calculation - should use actual pricing data
	var costPerToken float64
	
	switch {
	case strings.Contains(model, "gpt-4"):
		costPerToken = 0.00003 // $0.03 per 1K tokens
	case strings.Contains(model, "gpt-3.5"):
		costPerToken = 0.000002 // $0.002 per 1K tokens
	case strings.Contains(model, "claude"):
		costPerToken = 0.000008 // $0.008 per 1K tokens
	default:
		costPerToken = 0.000001 // Default low cost
	}
	
	return float64(tokens) * costPerToken
}

type responseCapture struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// Ensure responseCapture implements http.Flusher for streaming support
func (rc *responseCapture) Flush() {
	if flusher, ok := rc.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	if rc.body == nil {
		rc.body = &bytes.Buffer{}
	}
	rc.body.Write(b)
	return rc.ResponseWriter.Write(b)
}

func (rc *responseCapture) WriteHeader(statusCode int) {
	rc.statusCode = statusCode
	rc.ResponseWriter.WriteHeader(statusCode)
}

// Legacy function for backward compatibility
func UsageTracking() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Legacy implementation - just pass through
			next.ServeHTTP(w, r)
		})
	}
}