package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/models"
)

type MetricsStagingService struct {
	db           *gorm.DB
	logger       *zap.Logger
	buffer       []models.Usage
	bufferMutex  sync.Mutex
	batchSize    int
	flushInterval time.Duration
	ticker       *time.Ticker
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

func NewMetricsStagingService(db *gorm.DB, logger *zap.Logger) *MetricsStagingService {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &MetricsStagingService{
		db:            db,
		logger:        logger,
		buffer:        make([]models.Usage, 0, 1000),
		batchSize:     500,
		flushInterval: 5 * time.Second,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (s *MetricsStagingService) Start() error {
	// Create staging table if not exists
	if err := s.createStagingTable(); err != nil {
		return fmt.Errorf("failed to create staging table: %w", err)
	}

	s.ticker = time.NewTicker(s.flushInterval)
	
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-s.ticker.C:
				s.flushBuffer()
			case <-s.ctx.Done():
				s.flushBuffer() // Final flush
				return
			}
		}
	}()

	s.logger.Info("Metrics staging service started",
		zap.Int("batch_size", s.batchSize),
		zap.Duration("flush_interval", s.flushInterval))
	return nil
}

func (s *MetricsStagingService) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.cancel()
	s.wg.Wait()
	s.logger.Info("Metrics staging service stopped")
}

func (s *MetricsStagingService) AddUsageRecord(usage models.Usage) error {
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()

	// Set ID if not set
	if usage.ID == uuid.Nil {
		usage.ID = uuid.New()
	}

	s.buffer = append(s.buffer, usage)

	// Check if we need to flush immediately
	if len(s.buffer) >= s.batchSize {
		go s.flushBuffer()
	}

	return nil
}

func (s *MetricsStagingService) flushBuffer() {
	s.bufferMutex.Lock()
	if len(s.buffer) == 0 {
		s.bufferMutex.Unlock()
		return
	}
	
	batch := make([]models.Usage, len(s.buffer))
	copy(batch, s.buffer)
	s.buffer = s.buffer[:0] // Clear buffer
	s.bufferMutex.Unlock()

	if err := s.bulkInsertToStaging(batch); err != nil {
		s.logger.Error("Failed to flush metrics to staging", 
			zap.Int("batch_size", len(batch)),
			zap.Error(err))
		return
	}

	s.logger.Debug("Flushed metrics batch to staging",
		zap.Int("records", len(batch)))
}

func (s *MetricsStagingService) bulkInsertToStaging(records []models.Usage) error {
	if len(records) == 0 {
		return nil
	}

	// For now, use regular GORM batch insert
	// In production with PostgreSQL, this could be optimized with pgx.CopyFrom
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Set timestamps for records that don't have them
	now := time.Now()
	for i := range records {
		if records[i].CreatedAt.IsZero() {
			records[i].CreatedAt = now
		}
		if records[i].UpdatedAt.IsZero() {
			records[i].UpdatedAt = now
		}
	}

	// Insert into staging table using CreateInBatches for better performance
	if err := tx.Table("usage_logs_staging").CreateInBatches(records, 100).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to batch insert to staging: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit staging batch: %w", err)
	}

	return nil
}

func (s *MetricsStagingService) createStagingTable() error {
	// Create UNLOGGED staging table for performance (no WAL)
	createStaging := `
	CREATE TABLE IF NOT EXISTS usage_logs_staging (
		LIKE usage_logs INCLUDING ALL
	) WITH (
		oids=false
	);`

	if err := s.db.Exec(createStaging).Error; err != nil {
		return fmt.Errorf("failed to create staging table: %w", err)
	}

	s.logger.Info("Created usage_logs_staging table")
	return nil
}

func (s *MetricsStagingService) PromoteToProduction() error {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Move data from staging to production
	if err := tx.Exec(`
		INSERT INTO usage_logs 
		SELECT * FROM usage_logs_staging
		ON CONFLICT (request_id) DO NOTHING
	`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to promote staging data: %w", err)
	}

	// Clear staging table
	if err := tx.Exec(`TRUNCATE usage_logs_staging`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to clear staging: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit promotion: %w", err)
	}

	s.logger.Debug("Promoted staging data to production")
	return nil
}

func (s *MetricsStagingService) StartPromotionWorker() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		
		// Promote staging data every 30 seconds
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := s.PromoteToProduction(); err != nil {
					s.logger.Error("Failed to promote staging data", zap.Error(err))
				}
			case <-s.ctx.Done():
				// Final promotion
				if err := s.PromoteToProduction(); err != nil {
					s.logger.Error("Failed final staging promotion", zap.Error(err))
				}
				return
			}
		}
	}()

	s.logger.Info("Started staging promotion worker")
}

// GetStats returns statistics about the staging service
func (s *MetricsStagingService) GetStats() map[string]interface{} {
	s.bufferMutex.Lock()
	bufferSize := len(s.buffer)
	s.bufferMutex.Unlock()

	var stagingCount int64
	s.db.Table("usage_logs_staging").Count(&stagingCount)

	return map[string]interface{}{
		"buffer_size":    bufferSize,
		"staging_count":  stagingCount,
		"batch_size":     s.batchSize,
		"flush_interval": s.flushInterval.String(),
	}
}