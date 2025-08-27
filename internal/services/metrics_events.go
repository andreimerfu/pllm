package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// MetricEventType defines the type of metric event
type MetricEventType string

const (
	EventTypeRequest  MetricEventType = "request"
	EventTypeResponse MetricEventType = "response"
	EventTypeError    MetricEventType = "error"
	EventTypeLatency  MetricEventType = "latency"
	EventTypeHealth   MetricEventType = "health"
)

// MetricEvent represents a single metric event
type MetricEvent struct {
	Type      MetricEventType `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	ModelName string          `json:"model_name,omitempty"`
	UserID    string          `json:"user_id,omitempty"`
	TeamID    string          `json:"team_id,omitempty"`
	KeyID     string          `json:"key_id,omitempty"`

	// Request/Response data
	RequestID    string  `json:"request_id,omitempty"`
	Tokens       int64   `json:"tokens,omitempty"`
	PromptTokens int64   `json:"prompt_tokens,omitempty"`
	OutputTokens int64   `json:"output_tokens,omitempty"`
	Cost         float64 `json:"cost,omitempty"`

	// Performance data
	Latency   int64  `json:"latency_ms,omitempty"`
	Success   bool   `json:"success,omitempty"`
	ErrorType string `json:"error_type,omitempty"`

	// Health data
	HealthScore float64 `json:"health_score,omitempty"`
	CircuitOpen bool    `json:"circuit_open,omitempty"`

	// Cache data
	CacheHit bool `json:"cache_hit,omitempty"`
}

// MetricEventEmitter emits metric events to Redis queue
type MetricEventEmitter struct {
	redis    *redis.Client
	logger   *zap.Logger
	queueKey string
	ctx      context.Context
}

// NewMetricEventEmitter creates a new metric event emitter
func NewMetricEventEmitter(redisClient *redis.Client, logger *zap.Logger) *MetricEventEmitter {
	return &MetricEventEmitter{
		redis:    redisClient,
		logger:   logger,
		queueKey: "pllm:metrics:events",
		ctx:      context.Background(),
	}
}

// EmitRequest emits a request metric event (non-blocking)
func (e *MetricEventEmitter) EmitRequest(modelName, userID, teamID, keyID, requestID string) {
	event := MetricEvent{
		Type:      EventTypeRequest,
		Timestamp: time.Now(),
		ModelName: modelName,
		UserID:    userID,
		TeamID:    teamID,
		KeyID:     keyID,
		RequestID: requestID,
	}

	go e.emitEvent(event) // Fire and forget
}

// EmitResponse emits a response metric event (non-blocking)
func (e *MetricEventEmitter) EmitResponse(requestID, modelName string, tokens, promptTokens, outputTokens int64, cost float64, latency int64, success bool, cacheHit bool, errorType string) {
	event := MetricEvent{
		Type:         EventTypeResponse,
		Timestamp:    time.Now(),
		RequestID:    requestID,
		ModelName:    modelName,
		Tokens:       tokens,
		PromptTokens: promptTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
		Latency:      latency,
		Success:      success,
		CacheHit:     cacheHit,
		ErrorType:    errorType,
	}

	go e.emitEvent(event) // Fire and forget
}

// EmitHealth emits a health metric event (non-blocking)
func (e *MetricEventEmitter) EmitHealth(modelName string, healthScore float64, circuitOpen bool) {
	event := MetricEvent{
		Type:        EventTypeHealth,
		Timestamp:   time.Now(),
		ModelName:   modelName,
		HealthScore: healthScore,
		CircuitOpen: circuitOpen,
	}

	go e.emitEvent(event) // Fire and forget
}

// emitEvent sends the event to Redis queue (internal method)
func (e *MetricEventEmitter) emitEvent(event MetricEvent) {
	// Use a short timeout to prevent blocking
	ctx, cancel := context.WithTimeout(e.ctx, 100*time.Millisecond)
	defer cancel()

	eventData, err := json.Marshal(event)
	if err != nil {
		e.logger.Error("Failed to marshal metric event", zap.Error(err))
		return
	}

	// Use Redis LIST as a queue (LPUSH = add to front, RPOP = take from back)
	err = e.redis.LPush(ctx, e.queueKey, eventData).Err()
	if err != nil {
		// Log error but don't block the request
		e.logger.Error("Failed to emit metric event to Redis",
			zap.Error(err),
			zap.String("event_type", string(event.Type)),
			zap.String("model", event.ModelName))
		return
	}

	// Optional: Set TTL on the queue to prevent infinite growth
	e.redis.Expire(ctx, e.queueKey, 7*24*time.Hour) // 7 days TTL
}

// GetQueueSize returns the current queue size (for monitoring)
func (e *MetricEventEmitter) GetQueueSize() (int64, error) {
	ctx, cancel := context.WithTimeout(e.ctx, 1*time.Second)
	defer cancel()

	return e.redis.LLen(ctx, e.queueKey).Result()
}

// FlushQueue clears all events from the queue (for testing/maintenance)
func (e *MetricEventEmitter) FlushQueue() error {
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
	defer cancel()

	return e.redis.Del(ctx, e.queueKey).Err()
}
