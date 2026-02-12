package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// UsageRecord represents a single usage record to be processed
type UsageRecord struct {
	ID           string     `json:"id"`
	RequestID    string     `json:"request_id"`
	Timestamp    time.Time  `json:"timestamp"`
	UserID       string     `json:"user_id,omitempty"`        // Who made the request
	ActualUserID string     `json:"actual_user_id,omitempty"` // Who actually used the key (for team keys)
	KeyID        string     `json:"key_id,omitempty"`
	KeyOwnerID   string     `json:"key_owner_id,omitempty"` // Who owns the key
	KeyType      string     `json:"key_type,omitempty"`     // Type of key (personal, team, system, etc.)
	TeamID       string     `json:"team_id,omitempty"`
	Model         string     `json:"model"`
	Provider      string     `json:"provider"`
	RouteSlug     string     `json:"route_slug,omitempty"`
	ProviderModel string     `json:"provider_model,omitempty"`
	Method       string     `json:"method"`
	Path         string     `json:"path"`
	StatusCode   int        `json:"status_code"`
	InputTokens  int        `json:"input_tokens"`
	OutputTokens int        `json:"output_tokens"`
	TotalTokens  int        `json:"total_tokens"`
	TotalCost    float64    `json:"total_cost"`
	Latency      int64      `json:"latency_ms"`
	Retries      int        `json:"retries"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
}

// UsageQueue manages the Redis queue for usage records
type UsageQueue struct {
	client     *redis.Client
	logger     *zap.Logger
	queueName  string
	batchSize  int
	maxRetries int
}

// UsageQueueConfig configuration for the usage queue
type UsageQueueConfig struct {
	Client     *redis.Client
	Logger     *zap.Logger
	QueueName  string
	BatchSize  int
	MaxRetries int
}

// NewUsageQueue creates a new usage queue
func NewUsageQueue(config *UsageQueueConfig) *UsageQueue {
	if config.QueueName == "" {
		config.QueueName = "usage_processing_queue"
	}
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	return &UsageQueue{
		client:     config.Client,
		logger:     config.Logger,
		queueName:  config.QueueName,
		batchSize:  config.BatchSize,
		maxRetries: config.MaxRetries,
	}
}

// EnqueueUsage adds a usage record to the processing queue
func (uq *UsageQueue) EnqueueUsage(ctx context.Context, record *UsageRecord) error {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal usage record: %w", err)
	}

	// Add to Redis list (LPUSH for FIFO processing with RPOP)
	err = uq.client.LPush(ctx, uq.queueName, data).Err()
	if err != nil {
		uq.logger.Error("Failed to enqueue usage record",
			zap.Error(err),
			zap.String("record_id", record.ID))
		return fmt.Errorf("failed to enqueue usage record: %w", err)
	}

	uq.logger.Debug("Usage record enqueued",
		zap.String("record_id", record.ID),
		zap.String("user_id", record.UserID),
		zap.String("key_id", record.KeyID),
		zap.Float64("cost", record.TotalCost))

	return nil
}

// DequeueUsageBatch retrieves a batch of usage records for processing
func (uq *UsageQueue) DequeueUsageBatch(ctx context.Context) ([]*UsageRecord, error) {
	// Use RPOP to get records in FIFO order
	pipe := uq.client.Pipeline()

	// Pop up to batchSize records
	var cmds []*redis.StringCmd
	for i := 0; i < uq.batchSize; i++ {
		cmd := pipe.RPop(ctx, uq.queueName)
		cmds = append(cmds, cmd)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to dequeue usage records: %w", err)
	}

	var records []*UsageRecord
	for _, cmd := range cmds {
		result, err := cmd.Result()
		if err == redis.Nil {
			break // No more records
		}
		if err != nil {
			uq.logger.Error("Error getting queued record", zap.Error(err))
			continue
		}

		var record UsageRecord
		if err := json.Unmarshal([]byte(result), &record); err != nil {
			uq.logger.Error("Failed to unmarshal usage record",
				zap.Error(err),
				zap.String("data", result))
			continue
		}

		records = append(records, &record)
	}

	if len(records) > 0 {
		uq.logger.Debug("Dequeued usage records batch",
			zap.Int("count", len(records)))
	}

	return records, nil
}

// EnqueueUsageFailed moves a failed record to a retry queue
func (uq *UsageQueue) EnqueueUsageFailed(ctx context.Context, record *UsageRecord, errorMsg string) error {
	record.Retries++

	if record.Retries >= uq.maxRetries {
		// Move to dead letter queue after max retries
		return uq.moveToDeadLetterQueue(ctx, record, errorMsg)
	}

	// Add delay before retry (exponential backoff)
	retryDelay := time.Duration(record.Retries*record.Retries) * 10 * time.Second
	retryAt := time.Now().Add(retryDelay)

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal failed record: %w", err)
	}

	// Use sorted set for delayed retry
	retryQueueName := fmt.Sprintf("%s:retry", uq.queueName)
	err = uq.client.ZAdd(ctx, retryQueueName, redis.Z{
		Score:  float64(retryAt.Unix()),
		Member: data,
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to enqueue retry record: %w", err)
	}

	uq.logger.Warn("Usage record queued for retry",
		zap.String("record_id", record.ID),
		zap.Int("retry_count", record.Retries),
		zap.Duration("delay", retryDelay),
		zap.String("error", errorMsg))

	return nil
}

// ProcessRetryQueue processes records that are ready for retry
func (uq *UsageQueue) ProcessRetryQueue(ctx context.Context) error {
	retryQueueName := fmt.Sprintf("%s:retry", uq.queueName)
	now := float64(time.Now().Unix())

	// Get records that are ready for retry (score <= current timestamp)
	records, err := uq.client.ZRangeByScore(ctx, retryQueueName, &redis.ZRangeBy{
		Min:   "0",
		Max:   fmt.Sprintf("%.0f", now),
		Count: int64(uq.batchSize),
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to get retry records: %w", err)
	}

	if len(records) == 0 {
		return nil // No records to retry
	}

	// Move records back to main queue and remove from retry queue
	pipe := uq.client.Pipeline()
	for _, recordData := range records {
		pipe.LPush(ctx, uq.queueName, recordData)
		pipe.ZRem(ctx, retryQueueName, recordData)
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to move retry records: %w", err)
	}

	uq.logger.Info("Moved records from retry queue back to main queue",
		zap.Int("count", len(records)))

	return nil
}

// moveToDeadLetterQueue moves failed records to dead letter queue
func (uq *UsageQueue) moveToDeadLetterQueue(ctx context.Context, record *UsageRecord, errorMsg string) error {
	deadLetterQueue := fmt.Sprintf("%s:dead_letter", uq.queueName)

	deadLetterRecord := map[string]interface{}{
		"record":      record,
		"error":       errorMsg,
		"failed_at":   time.Now(),
		"final_retry": record.Retries,
	}

	data, err := json.Marshal(deadLetterRecord)
	if err != nil {
		return fmt.Errorf("failed to marshal dead letter record: %w", err)
	}

	err = uq.client.LPush(ctx, deadLetterQueue, data).Err()
	if err != nil {
		return fmt.Errorf("failed to enqueue dead letter record: %w", err)
	}

	uq.logger.Error("Usage record moved to dead letter queue",
		zap.String("record_id", record.ID),
		zap.Int("retries", record.Retries),
		zap.String("error", errorMsg))

	return nil
}

// GetQueueStats returns statistics about the usage queue
func (uq *UsageQueue) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	pipe := uq.client.Pipeline()

	mainQueueCmd := pipe.LLen(ctx, uq.queueName)
	retryQueueCmd := pipe.ZCard(ctx, fmt.Sprintf("%s:retry", uq.queueName))
	deadLetterCmd := pipe.LLen(ctx, fmt.Sprintf("%s:dead_letter", uq.queueName))

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue stats: %w", err)
	}

	mainCount, _ := mainQueueCmd.Result()
	retryCount, _ := retryQueueCmd.Result()
	deadLetterCount, _ := deadLetterCmd.Result()

	return &QueueStats{
		MainQueue:       mainCount,
		RetryQueue:      retryCount,
		DeadLetterQueue: deadLetterCount,
		TotalPending:    mainCount + retryCount,
	}, nil
}

// QueueStats represents queue statistics
type QueueStats struct {
	MainQueue       int64 `json:"main_queue"`
	RetryQueue      int64 `json:"retry_queue"`
	DeadLetterQueue int64 `json:"dead_letter_queue"`
	TotalPending    int64 `json:"total_pending"`
}

// ClearQueue clears all records from the queue (use with caution)
func (uq *UsageQueue) ClearQueue(ctx context.Context) error {
	pipe := uq.client.Pipeline()
	pipe.Del(ctx, uq.queueName)
	pipe.Del(ctx, fmt.Sprintf("%s:retry", uq.queueName))
	pipe.Del(ctx, fmt.Sprintf("%s:dead_letter", uq.queueName))

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear queue: %w", err)
	}

	uq.logger.Warn("Usage queue cleared")
	return nil
}

// HealthCheck checks if the queue system is healthy
func (uq *UsageQueue) HealthCheck(ctx context.Context) error {
	// Test basic Redis operations
	testKey := fmt.Sprintf("%s:healthcheck", uq.queueName)
	err := uq.client.Set(ctx, testKey, "ok", time.Second).Err()
	if err != nil {
		return fmt.Errorf("redis write failed: %w", err)
	}

	val, err := uq.client.Get(ctx, testKey).Result()
	if err != nil {
		return fmt.Errorf("redis read failed: %w", err)
	}

	if val != "ok" {
		return fmt.Errorf("redis data integrity check failed")
	}

	// Clean up test key
	uq.client.Del(ctx, testKey)

	return nil
}
