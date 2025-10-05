package metrics

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// MetricWorkerConfig configures the metrics worker
type MetricWorkerConfig struct {
	Redis             *redis.Client
	DB                *gorm.DB
	Logger            *zap.Logger
	BatchSize         int           // Number of events to process at once
	BatchTimeout      time.Duration // Max time to wait for batch to fill
	WorkerCount       int           // Number of concurrent workers
	QueueKey          string        // Redis queue key
	AggregateInterval time.Duration // How often to aggregate metrics
}

// MetricWorker processes metric events from Redis queue
type MetricWorker struct {
	config *MetricWorkerConfig
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// In-memory aggregation buffers (thread-safe)
	mu              sync.RWMutex
	modelMetrics    map[string]*MetricBuffer
	systemMetrics   *SystemMetricBuffer
	lastAggregation time.Time
}

// MetricBuffer holds aggregated metrics for a model
type MetricBuffer struct {
	ModelName string
	Timestamp time.Time

	TotalRequests  int64
	FailedRequests int64
	TotalTokens    int64
	InputTokens    int64
	OutputTokens   int64
	TotalCost      float64

	LatencySum     int64
	LatencyCount   int64
	LatencySamples []int64 // For percentile calculation

	HealthScore float64
	CircuitOpen bool

	CacheHits   int64
	CacheMisses int64
}

// SystemMetricBuffer holds system-wide metrics
type SystemMetricBuffer struct {
	Timestamp      time.Time
	TotalRequests  int64
	FailedRequests int64
	TotalTokens    int64
	TotalCost      float64
	CacheHits      int64
	CacheMisses    int64
	ActiveModels   map[string]bool // Track which models are active
}

// NewMetricWorker creates a new background metric worker
func NewMetricWorker(config *MetricWorkerConfig) *MetricWorker {
	ctx, cancel := context.WithCancel(context.Background())

	return &MetricWorker{
		config:          config,
		ctx:             ctx,
		cancel:          cancel,
		modelMetrics:    make(map[string]*MetricBuffer),
		systemMetrics:   &SystemMetricBuffer{ActiveModels: make(map[string]bool)},
		lastAggregation: time.Now(),
	}
}

// Start begins processing metric events
func (w *MetricWorker) Start() error {
	w.config.Logger.Info("Starting metric worker",
		zap.Int("worker_count", w.config.WorkerCount),
		zap.Int("batch_size", w.config.BatchSize),
		zap.Duration("batch_timeout", w.config.BatchTimeout))

	// Start worker goroutines
	for i := 0; i < w.config.WorkerCount; i++ {
		w.wg.Add(1)
		go w.processEvents(i)
	}

	// Start aggregation timer
	w.wg.Add(1)
	go w.runAggregation()

	return nil
}

// Stop stops the metric worker gracefully
func (w *MetricWorker) Stop() {
	w.config.Logger.Info("Stopping metric worker")
	w.cancel()
	w.wg.Wait()

	// Final aggregation before shutdown
	w.aggregateAndFlush()
	w.config.Logger.Info("Metric worker stopped")
}

// processEvents processes events from Redis queue
func (w *MetricWorker) processEvents(workerID int) {
	defer w.wg.Done()

	logger := w.config.Logger.With(zap.Int("worker_id", workerID))
	logger.Info("Metric worker started")

	for {
		select {
		case <-w.ctx.Done():
			logger.Info("Metric worker stopping")
			return
		default:
			w.processBatch(logger)
		}
	}
}

// processBatch processes a batch of events
func (w *MetricWorker) processBatch(logger *zap.Logger) {
	ctx, cancel := context.WithTimeout(w.ctx, w.config.BatchTimeout)
	defer cancel()

	// Block and wait for events (BRPOP = blocking right pop)
	result, err := w.config.Redis.BRPop(ctx, w.config.BatchTimeout, w.config.QueueKey).Result()
	if err != nil {
		if err == redis.Nil {
			// Timeout - no events available
			return
		}
		logger.Error("Failed to pop events from queue", zap.Error(err))
		time.Sleep(1 * time.Second) // Backoff
		return
	}

	if len(result) < 2 {
		return
	}

	// Process the event
	eventData := result[1]
	var event MetricEvent
	if err := json.Unmarshal([]byte(eventData), &event); err != nil {
		logger.Error("Failed to unmarshal metric event", zap.Error(err))
		return
	}

	// Update in-memory buffers
	w.updateMetricBuffers(event, logger)
}

// updateMetricBuffers updates in-memory metric buffers with the event
func (w *MetricWorker) updateMetricBuffers(event MetricEvent, logger *zap.Logger) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now().Truncate(time.Minute)

	switch event.Type {
	case EventTypeRequest:
		// Initialize model buffer if needed
		if _, exists := w.modelMetrics[event.ModelName]; !exists {
			w.modelMetrics[event.ModelName] = &MetricBuffer{
				ModelName:      event.ModelName,
				Timestamp:      now,
				LatencySamples: make([]int64, 0),
			}
		}

		w.systemMetrics.ActiveModels[event.ModelName] = true

	case EventTypeResponse:
		if buffer, exists := w.modelMetrics[event.ModelName]; exists {
			buffer.TotalRequests++
			buffer.TotalTokens += event.Tokens
			buffer.InputTokens += event.PromptTokens
			buffer.OutputTokens += event.OutputTokens
			buffer.TotalCost += event.Cost

			if !event.Success {
				buffer.FailedRequests++
			}

			if event.Latency > 0 {
				buffer.LatencySum += event.Latency
				buffer.LatencyCount++
				buffer.LatencySamples = append(buffer.LatencySamples, event.Latency)
			}

			if event.CacheHit {
				buffer.CacheHits++
			} else {
				buffer.CacheMisses++
			}
		}

		// Update system metrics
		w.systemMetrics.TotalRequests++
		w.systemMetrics.TotalTokens += event.Tokens
		w.systemMetrics.TotalCost += event.Cost

		if !event.Success {
			w.systemMetrics.FailedRequests++
		}

		if event.CacheHit {
			w.systemMetrics.CacheHits++
		} else {
			w.systemMetrics.CacheMisses++
		}

	case EventTypeHealth:
		if buffer, exists := w.modelMetrics[event.ModelName]; exists {
			buffer.HealthScore = event.HealthScore
			buffer.CircuitOpen = event.CircuitOpen
		}
	}
}

// runAggregation runs periodic aggregation
func (w *MetricWorker) runAggregation() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.AggregateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.aggregateAndFlush()
		}
	}
}

// aggregateAndFlush aggregates buffered metrics and writes to database
func (w *MetricWorker) aggregateAndFlush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if time.Since(w.lastAggregation) < w.config.AggregateInterval {
		return
	}

	now := time.Now().Truncate(time.Minute)

	// Process model metrics
	for modelName, buffer := range w.modelMetrics {
		if buffer.TotalRequests == 0 {
			continue // Skip empty buffers
		}

		avgLatency := int64(0)
		if buffer.LatencyCount > 0 {
			avgLatency = buffer.LatencySum / buffer.LatencyCount
		}

		p95Latency, p99Latency := calculatePercentiles(buffer.LatencySamples)

		successRate := float64(100)
		if buffer.TotalRequests > 0 {
			successRate = float64(buffer.TotalRequests-buffer.FailedRequests) / float64(buffer.TotalRequests) * 100
		}

		modelMetrics := models.ModelMetrics{
			ModelName:      modelName,
			Interval:       models.IntervalHourly,
			Timestamp:      now,
			HealthScore:    buffer.HealthScore,
			AvgLatency:     avgLatency,
			P95Latency:     p95Latency,
			P99Latency:     p99Latency,
			TotalRequests:  buffer.TotalRequests,
			FailedRequests: buffer.FailedRequests,
			SuccessRate:    successRate,
			TotalTokens:    buffer.TotalTokens,
			InputTokens:    buffer.InputTokens,
			OutputTokens:   buffer.OutputTokens,
			TotalCost:      buffer.TotalCost,
			CircuitOpen:    buffer.CircuitOpen,
		}

		if err := w.config.DB.Create(&modelMetrics).Error; err != nil {
			w.config.Logger.Error("Failed to save model metrics",
				zap.String("model", modelName),
				zap.Error(err))
		}
	}

	// Process system metrics
	if w.systemMetrics.TotalRequests > 0 {
		successRate := float64(100)
		if w.systemMetrics.TotalRequests > 0 {
			successRate = float64(w.systemMetrics.TotalRequests-w.systemMetrics.FailedRequests) / float64(w.systemMetrics.TotalRequests) * 100
		}

		cacheHitRate := float64(0)
		totalCacheRequests := w.systemMetrics.CacheHits + w.systemMetrics.CacheMisses
		if totalCacheRequests > 0 {
			cacheHitRate = float64(w.systemMetrics.CacheHits) / float64(totalCacheRequests) * 100
		}

		// Calculate average health score from active models
		avgHealthScore := float64(0)
		if len(w.systemMetrics.ActiveModels) > 0 {
			totalHealth := float64(0)
			for modelName := range w.systemMetrics.ActiveModels {
				if buffer, exists := w.modelMetrics[modelName]; exists {
					totalHealth += buffer.HealthScore
				}
			}
			avgHealthScore = totalHealth / float64(len(w.systemMetrics.ActiveModels))
		}

		systemMetrics := models.SystemMetrics{
			Interval:       models.IntervalHourly,
			Timestamp:      now,
			ActiveModels:   len(w.systemMetrics.ActiveModels),
			TotalModels:    len(w.modelMetrics),
			AvgHealthScore: avgHealthScore,
			TotalRequests:  w.systemMetrics.TotalRequests,
			FailedRequests: w.systemMetrics.FailedRequests,
			SuccessRate:    successRate,
			TotalTokens:    w.systemMetrics.TotalTokens,
			TotalCost:      w.systemMetrics.TotalCost,
			CacheHits:      w.systemMetrics.CacheHits,
			CacheMisses:    w.systemMetrics.CacheMisses,
			CacheHitRate:   cacheHitRate,
		}

		if err := w.config.DB.Create(&systemMetrics).Error; err != nil {
			w.config.Logger.Error("Failed to save system metrics", zap.Error(err))
		}
	}

	// Reset buffers
	w.modelMetrics = make(map[string]*MetricBuffer)
	w.systemMetrics = &SystemMetricBuffer{ActiveModels: make(map[string]bool)}
	w.lastAggregation = now

	w.config.Logger.Debug("Metrics aggregated and flushed",
		zap.Time("timestamp", now))
}

// calculatePercentiles calculates P95 and P99 latency from samples
func calculatePercentiles(samples []int64) (p95, p99 int64) {
	if len(samples) == 0 {
		return 0, 0
	}

	// Simple approximation - in production you might want a more efficient algorithm
	// For now, we'll sample from the available data
	if len(samples) >= 20 {
		// Take P95 and P99 from available samples
		p95Index := int(float64(len(samples)) * 0.95)
		p99Index := int(float64(len(samples)) * 0.99)

		if p95Index < len(samples) {
			p95 = samples[p95Index]
		}
		if p99Index < len(samples) {
			p99 = samples[p99Index]
		}
	}

	return p95, p99
}

// GetStats returns worker statistics for monitoring
func (w *MetricWorker) GetStats() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	queueSize, _ := w.config.Redis.LLen(w.ctx, w.config.QueueKey).Result()

	return map[string]interface{}{
		"queue_size":       queueSize,
		"active_models":    len(w.modelMetrics),
		"last_aggregation": w.lastAggregation,
		"worker_count":     w.config.WorkerCount,
		"batch_size":       w.config.BatchSize,
	}
}
