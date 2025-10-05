package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// EventType represents the type of event
type EventType string

const (
	EventTypeUsage  EventType = "usage"
	EventTypeBudget EventType = "budget"
	EventTypeAlert  EventType = "alert"
)

// Event represents a distributed event
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Source    string                 `json:"source"`
}

// EventPublisher handles publishing events to Redis
type EventPublisher struct {
	client *redis.Client
	logger *zap.Logger
}

// NewEventPublisher creates a new event publisher
func NewEventPublisher(client *redis.Client, logger *zap.Logger) *EventPublisher {
	return &EventPublisher{
		client: client,
		logger: logger,
	}
}

// PublishUsageEvent publishes a usage tracking event
func (ep *EventPublisher) PublishUsageEvent(ctx context.Context, userID, keyID, model string, inputTokens, outputTokens int, cost float64, latency time.Duration) error {
	event := Event{
		ID:        generateEventID(),
		Type:      EventTypeUsage,
		Timestamp: time.Now(),
		Source:    "pllm-gateway",
		Data: map[string]interface{}{
			"user_id":       userID,
			"key_id":        keyID,
			"model":         model,
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
			"cost":          cost,
			"latency_ms":    latency.Milliseconds(),
		},
	}

	return ep.publishEvent(ctx, "usage_events", event)
}

// PublishBudgetEvent publishes a budget-related event
func (ep *EventPublisher) PublishBudgetEvent(ctx context.Context, budgetID, entityID, entityType string, amount float64, eventType string) error {
	event := Event{
		ID:        generateEventID(),
		Type:      EventTypeBudget,
		Timestamp: time.Now(),
		Source:    "pllm-gateway",
		Data: map[string]interface{}{
			"budget_id":   budgetID,
			"entity_id":   entityID,
			"entity_type": entityType,
			"amount":      amount,
			"event_type":  eventType, // "check", "update", "alert", "exceeded"
		},
	}

	return ep.publishEvent(ctx, "budget_events", event)
}

// publishEvent publishes an event to a Redis stream
func (ep *EventPublisher) publishEvent(ctx context.Context, stream string, event Event) error {
	eventData, err := json.Marshal(event)
	if err != nil {
		ep.logger.Error("Failed to marshal event", zap.Error(err), zap.String("event_id", event.ID))
		return err
	}

	// Use Redis Streams for reliable event delivery
	args := &redis.XAddArgs{
		Stream: stream,
		MaxLen: 10000, // Keep last 10k events
		Approx: true,  // Approximate trimming for performance
		Values: map[string]interface{}{
			"event_id":   event.ID,
			"event_type": string(event.Type),
			"data":       string(eventData),
		},
	}

	_, err = ep.client.XAdd(ctx, args).Result()
	if err != nil {
		ep.logger.Error("Failed to publish event to Redis",
			zap.Error(err),
			zap.String("stream", stream),
			zap.String("event_id", event.ID))
		return err
	}

	ep.logger.Debug("Event published successfully",
		zap.String("stream", stream),
		zap.String("event_id", event.ID),
		zap.String("type", string(event.Type)))

	return nil
}

// generateEventID generates a unique event ID
func generateEventID() string {
	// Use timestamp + random suffix for uniqueness
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of given length
func randomString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[time.Now().UnixNano()%int64(len(chars))]
	}
	return string(result)
}
