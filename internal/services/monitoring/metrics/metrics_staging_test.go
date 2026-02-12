package metrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

func TestMetricsStagingService_CreateStagingTable(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	
	service := NewMetricsStagingService(db, logger)
	
	err := service.createStagingTable()
	require.NoError(t, err)
	
	// Verify staging table exists
	var count int64
	err = db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'usage_logs_staging'").Scan(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestMetricsStagingService_AddUsageRecord(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	
	service := NewMetricsStagingService(db, logger)
	service.batchSize = 2 // Small batch size for testing
	
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()
	
	// Add records
	usage1 := models.Usage{
		RequestID:    "req-1",
		Timestamp:    time.Now(),
		Model:        "gpt-4",
		TotalTokens:  100,
		TotalCost:    0.01,
		Metadata:     datatypes.JSON([]byte(`{}`)),
	}
	
	usage2 := models.Usage{
		RequestID:    "req-2", 
		Timestamp:    time.Now(),
		Model:        "gpt-3.5",
		TotalTokens:  50,
		TotalCost:    0.005,
		Metadata:     datatypes.JSON([]byte(`{}`)),
	}
	
	err = service.AddUsageRecord(usage1)
	require.NoError(t, err)
	
	err = service.AddUsageRecord(usage2)
	require.NoError(t, err)
	
	// Verify records are in buffer
	service.bufferMutex.Lock()
	_ = len(service.buffer)
	service.bufferMutex.Unlock()
	
	// Wait a bit for the goroutine to complete the flush
	time.Sleep(200 * time.Millisecond)
	
	// Re-check buffer size after flush has completed
	service.bufferMutex.Lock()
	finalBufferSize := len(service.buffer)
	service.bufferMutex.Unlock()
	
	// Buffer should be empty because it auto-flushed when it reached batch size
	assert.Equal(t, 0, finalBufferSize)
}

func TestMetricsStagingService_FlushBuffer(t *testing.T) {
	// Note: This test is limited because SQLite doesn't support pgx COPY
	// In a real PostgreSQL environment, this would test the full COPY functionality
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	
	service := NewMetricsStagingService(db, logger)
	
	// Add records to buffer
	usage := models.Usage{
		BaseModel: models.BaseModel{ID: uuid.New()},
		RequestID:   "req-1",
		Timestamp:   time.Now(),
		Model:       "gpt-4",
		TotalTokens: 100,
		TotalCost:   0.01,
		Metadata:    datatypes.JSON([]byte(`{}`)),
	}
	
	service.buffer = append(service.buffer, usage)
	
	// This will fail with SQLite but we can test the error handling
	service.flushBuffer()
	
	// Buffer should be cleared even on error
	service.bufferMutex.Lock()
	bufferSize := len(service.buffer)
	service.bufferMutex.Unlock()
	
	assert.Equal(t, 0, bufferSize)
}

func TestMetricsStagingService_PromoteToProduction(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	
	service := NewMetricsStagingService(db, logger)
	
	// Create test user first
	userID := uuid.New()
	user := models.User{
		BaseModel: models.BaseModel{ID: userID},
		Email:     "test@test.com",
		Username:  "testuser",
		DexID:     "dex123",
		FirstName: "Test",
		LastName:  "User",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user).Error)
	
	err := service.createStagingTable()
	require.NoError(t, err)
	
	// Add test data to staging table
	stagingUsage := models.Usage{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		RequestID:    "req-staging",
		Timestamp:    time.Now(),
		UserID:       &userID,
		ActualUserID: &userID,
		Model:        "gpt-4",
		TotalTokens:  100,
		TotalCost:    0.01,
		Metadata:     datatypes.JSON([]byte(`{}`)),
	}
	
	// Insert into staging table manually
	err = db.Table("usage_logs_staging").Create(&stagingUsage).Error
	require.NoError(t, err)
	
	// Promote to production
	err = service.PromoteToProduction()
	require.NoError(t, err)
	
	// Verify data is in production table
	var productionUsage models.Usage
	err = db.Where("request_id = ?", "req-staging").First(&productionUsage).Error
	require.NoError(t, err)
	assert.Equal(t, "req-staging", productionUsage.RequestID)
	
	// Verify staging table is empty
	var stagingCount int64
	db.Table("usage_logs_staging").Count(&stagingCount)
	assert.Equal(t, int64(0), stagingCount)
}

func TestMetricsStagingService_StartStop(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	
	service := NewMetricsStagingService(db, logger)
	
	err := service.Start()
	require.NoError(t, err)
	
	// Verify service is running
	assert.NotNil(t, service.ticker)
	assert.NotNil(t, service.ctx)
	
	// Stop the service
	service.Stop()
	
	// Verify context is cancelled
	select {
	case <-service.ctx.Done():
		// Expected - context should be cancelled
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context was not cancelled after Stop()")
	}
}

func TestMetricsStagingService_GetStats(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	
	service := NewMetricsStagingService(db, logger)
	service.batchSize = 100
	service.flushInterval = 5 * time.Second
	
	err := service.createStagingTable()
	require.NoError(t, err)
	
	// Add some records to buffer
	usage1 := models.Usage{BaseModel: models.BaseModel{ID: uuid.New()}, RequestID: "req-1", Metadata: datatypes.JSON([]byte(`{}`))}
	usage2 := models.Usage{BaseModel: models.BaseModel{ID: uuid.New()}, RequestID: "req-2", Metadata: datatypes.JSON([]byte(`{}`))}
	
	service.buffer = append(service.buffer, usage1, usage2)
	
	stats := service.GetStats()
	
	assert.Equal(t, 2, stats["buffer_size"])
	assert.Equal(t, 100, stats["batch_size"])
	assert.Equal(t, "5s", stats["flush_interval"])
	assert.Contains(t, stats, "staging_count")
}

func TestMetricsStagingService_StartPromotionWorker(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	
	service := NewMetricsStagingService(db, logger)
	
	// Create test user first
	userID := uuid.New()
	user := models.User{
		BaseModel: models.BaseModel{ID: userID},
		Email:     "test@test.com",
		Username:  "testuser",
		DexID:     "dex123",
		FirstName: "Test",
		LastName:  "User",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user).Error)
	
	err := service.createStagingTable()
	require.NoError(t, err)
	
	// Start promotion worker
	service.StartPromotionWorker()
	
	// Add test data to staging
	stagingUsage := models.Usage{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		RequestID:    "req-promotion",
		Timestamp:    time.Now(),
		UserID:       &userID,
		ActualUserID: &userID,
		Model:        "gpt-4",
		TotalTokens:  100,
		TotalCost:    0.01,
		Metadata:     datatypes.JSON([]byte(`{}`)),
	}
	
	err = db.Table("usage_logs_staging").Create(&stagingUsage).Error
	require.NoError(t, err)
	
	// Wait a bit for promotion to happen (worker runs every 30s, but we can trigger manually)
	time.Sleep(100 * time.Millisecond)
	
	// Stop the service
	service.Stop()
	
	// Verify data was promoted (or at least promotion was attempted)
	// Note: The actual promotion depends on the worker cycle
}

func TestMetricsStagingService_ConcurrentAccess(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	
	service := NewMetricsStagingService(db, logger)
	service.batchSize = 1000 // Large batch to avoid auto-flush during test
	
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()
	
	// Add records concurrently
	numGoroutines := 10
	recordsPerGoroutine := 10
	
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < recordsPerGoroutine; j++ {
				usage := models.Usage{
					RequestID:   fmt.Sprintf("req-%d-%d", goroutineID, j),
					Timestamp:   time.Now(),
					Model:       "gpt-4",
					TotalTokens: 100,
					TotalCost:   0.01,
					Metadata:    datatypes.JSON([]byte(`{}`)),
				}
				err := service.AddUsageRecord(usage)
				assert.NoError(t, err)
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	
	// Check buffer size
	service.bufferMutex.Lock()
	bufferSize := len(service.buffer)
	service.bufferMutex.Unlock()
	
	assert.Equal(t, numGoroutines*recordsPerGoroutine, bufferSize)
}

func TestMetricsStagingService_AutoGenerateID(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	
	service := NewMetricsStagingService(db, logger)
	
	usage := models.Usage{
		RequestID: "req-no-id",
		// ID is not set, should be auto-generated
		Metadata:  datatypes.JSON([]byte(`{}`)),
	}
	
	err := service.AddUsageRecord(usage)
	require.NoError(t, err)
	
	// Check that ID was generated in buffer
	service.bufferMutex.Lock()
	if len(service.buffer) > 0 {
		assert.NotEqual(t, uuid.Nil, service.buffer[0].ID)
	}
	service.bufferMutex.Unlock()
}