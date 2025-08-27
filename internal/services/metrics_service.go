package services

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MetricsServiceConfig configures the metrics service
type MetricsServiceConfig struct {
	// Database
	DB *gorm.DB

	// Redis for event queue
	Redis *redis.Client

	// Logging
	Logger *zap.Logger

	// Worker configuration
	WorkerCount       int           `default:"4"`
	BatchSize         int           `default:"100"`
	BatchTimeout      time.Duration `default:"5s"`
	AggregateInterval time.Duration `default:"1m"`

	// Optional monitoring endpoint
	EnableMonitoring bool `default:"true"`
	MonitoringPort   int  `default:"8081"`
}

// MetricsService manages the complete metrics collection pipeline
type MetricsService struct {
	config  *MetricsServiceConfig
	emitter *MetricEventEmitter
	worker  *MetricWorker
	logger  *zap.Logger

	// Monitoring server
	monitoringServer *http.Server
}

// NewMetricsService creates a new metrics service
func NewMetricsService(config *MetricsServiceConfig) (*MetricsService, error) {
	if config.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if config.DB == nil {
		return nil, fmt.Errorf("database is required")
	}
	if config.Redis == nil {
		return nil, fmt.Errorf("redis client is required")
	}

	// Set defaults
	if config.WorkerCount == 0 {
		config.WorkerCount = 4
	}
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.BatchTimeout == 0 {
		config.BatchTimeout = 5 * time.Second
	}
	if config.AggregateInterval == 0 {
		config.AggregateInterval = 1 * time.Minute
	}
	if config.MonitoringPort == 0 {
		config.MonitoringPort = 8081
	}

	// Create emitter
	emitter := NewMetricEventEmitter(config.Redis, config.Logger)

	// Create worker
	workerConfig := &MetricWorkerConfig{
		Redis:             config.Redis,
		DB:                config.DB,
		Logger:            config.Logger,
		BatchSize:         config.BatchSize,
		BatchTimeout:      config.BatchTimeout,
		WorkerCount:       config.WorkerCount,
		QueueKey:          "pllm:metrics:events",
		AggregateInterval: config.AggregateInterval,
	}

	worker := NewMetricWorker(workerConfig)

	service := &MetricsService{
		config:  config,
		emitter: emitter,
		worker:  worker,
		logger:  config.Logger,
	}

	// Setup monitoring endpoint if enabled
	if config.EnableMonitoring {
		service.setupMonitoring()
	}

	return service, nil
}

// GetEmitter returns the event emitter for the gateway to use
func (s *MetricsService) GetEmitter() *MetricEventEmitter {
	return s.emitter
}

// Start starts the metrics service
func (s *MetricsService) Start(ctx context.Context) error {
	s.logger.Info("Starting metrics service",
		zap.Int("worker_count", s.config.WorkerCount),
		zap.Duration("aggregate_interval", s.config.AggregateInterval))

	// Start worker
	if err := s.worker.Start(); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}

	// Start monitoring server
	if s.config.EnableMonitoring && s.monitoringServer != nil {
		go func() {
			s.logger.Info("Starting metrics monitoring server",
				zap.Int("port", s.config.MonitoringPort))
			if err := s.monitoringServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.logger.Error("Monitoring server error", zap.Error(err))
			}
		}()
	}

	s.logger.Info("Metrics service started successfully")
	return nil
}

// Stop stops the metrics service gracefully
func (s *MetricsService) Stop(ctx context.Context) error {
	s.logger.Info("Stopping metrics service")

	// Stop monitoring server
	if s.monitoringServer != nil {
		if err := s.monitoringServer.Shutdown(ctx); err != nil {
			s.logger.Error("Error shutting down monitoring server", zap.Error(err))
		}
	}

	// Stop worker
	s.worker.Stop()

	s.logger.Info("Metrics service stopped")
	return nil
}

// setupMonitoring sets up the monitoring HTTP server
func (s *MetricsService) setupMonitoring() {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"healthy","service":"metrics"}`)); err != nil {
			log.Printf("Failed to write metrics health response: %v", err)
		}
	})

	// Worker statistics
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := s.worker.GetStats()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := fmt.Sprintf(`{
			"queue_size": %v,
			"active_models": %v,
			"last_aggregation": "%v",
			"worker_count": %v,
			"batch_size": %v,
			"uptime": "%v"
		}`, stats["queue_size"], stats["active_models"],
			stats["last_aggregation"], stats["worker_count"],
			stats["batch_size"], time.Now().Format(time.RFC3339))

		if _, err := w.Write([]byte(response)); err != nil {
			log.Printf("Failed to write metrics stats response: %v", err)
		}
	})

	// Queue management endpoints
	mux.HandleFunc("/queue/size", func(w http.ResponseWriter, r *http.Request) {
		size, err := s.emitter.GetQueueSize()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprintf(w, `{"queue_size": %d}`, size); err != nil {
			log.Printf("Failed to write queue size response: %v", err)
		}
	})

	mux.HandleFunc("/queue/flush", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := s.emitter.FlushQueue(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"message": "queue flushed"}`)); err != nil {
			log.Printf("Failed to write queue flush response: %v", err)
		}
	})

	s.monitoringServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.MonitoringPort),
		Handler: mux,
	}
}

// ValidateConfig validates the service configuration
func (s *MetricsService) ValidateConfig() error {
	if s.config.WorkerCount < 1 || s.config.WorkerCount > 100 {
		return fmt.Errorf("worker_count must be between 1 and 100")
	}

	if s.config.BatchSize < 1 || s.config.BatchSize > 10000 {
		return fmt.Errorf("batch_size must be between 1 and 10000")
	}

	if s.config.BatchTimeout < time.Millisecond || s.config.BatchTimeout > time.Minute {
		return fmt.Errorf("batch_timeout must be between 1ms and 1m")
	}

	if s.config.AggregateInterval < 10*time.Second || s.config.AggregateInterval > time.Hour {
		return fmt.Errorf("aggregate_interval must be between 10s and 1h")
	}

	return nil
}

// Metrics returns current metrics for monitoring
func (s *MetricsService) Metrics() map[string]interface{} {
	stats := s.worker.GetStats()
	queueSize, _ := s.emitter.GetQueueSize()

	return map[string]interface{}{
		"service": "metrics",
		"worker":  stats,
		"queue": map[string]interface{}{
			"size": queueSize,
		},
		"config": map[string]interface{}{
			"worker_count":       s.config.WorkerCount,
			"batch_size":         s.config.BatchSize,
			"batch_timeout":      s.config.BatchTimeout.String(),
			"aggregate_interval": s.config.AggregateInterval.String(),
		},
	}
}
