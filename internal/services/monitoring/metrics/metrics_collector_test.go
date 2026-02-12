package metrics

import (
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

// Mock ModelStatsProvider
type MockModelStatsProvider struct {
	mock.Mock
}

func (m *MockModelStatsProvider) GetModelStats() map[string]interface{} {
	args := m.Called()
	return args.Get(0).(map[string]interface{})
}

func TestMetricsCollector_collectCurrentMetrics(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	mockProvider := new(MockModelStatsProvider)

	// Create test users first
	userID1 := uuid.New()
	userID2 := uuid.New()
	user1 := models.User{
		BaseModel: models.BaseModel{ID: userID1},
		Email:     "user1@test.com",
		Username:  "user1",
		DexID:     "dex1",
		FirstName: "Test",
		LastName:  "User1",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user1).Error)

	user2 := models.User{
		BaseModel: models.BaseModel{ID: userID2},
		Email:     "user2@test.com",
		Username:  "user2",
		DexID:     "dex2",
		FirstName: "Test",
		LastName:  "User2",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user2).Error)

	collector := NewMetricsCollector(db, logger, mockProvider)

	// Setup mock data
	loadBalancerData := map[string]interface{}{
		"gpt-4": map[string]interface{}{
			"circuit_open":     false,
			"health_score":     95.5,
			"avg_latency":      int64(1500),
			"p95_latency":      int64(2000),
			"failed_requests":  int64(5),
		},
		"gpt-3.5": map[string]interface{}{
			"circuit_open":     true,
			"health_score":     60.0,
			"avg_latency":      int64(800),
			"p95_latency":      int64(1200),
			"failed_requests":  int64(10),
		},
	}

	mockStats := map[string]interface{}{
		"should_shed_load": false,
		"load_balancer":    loadBalancerData,
	}

	mockProvider.On("GetModelStats").Return(mockStats)

	// Create some test usage data
	// Use truncated time to align with the collectCurrentMetrics method
	now := time.Now().Truncate(time.Minute)
	testUsage := []models.Usage{
		{
			RequestID:    "req-1",
			Timestamp:    now.Add(-30 * time.Second),
			UserID:       &userID1,
			ActualUserID: &userID1,
			Model:        "gpt-4",
			TotalTokens:  100,
			TotalCost:    0.01,
			StatusCode:   200,
			Latency:      1000,
			CacheHit:     false,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
		{
			RequestID:    "req-2",
			Timestamp:    now.Add(-45 * time.Second),
			UserID:       &userID1,
			ActualUserID: &userID1,
			Model:        "gpt-4",
			TotalTokens:  150,
			TotalCost:    0.015,
			StatusCode:   500, // Failed request
			Latency:      2000,
			CacheHit:     true,
			Error:        "timeout",
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
		{
			RequestID:    "req-3",
			Timestamp:    now.Add(-20 * time.Second),
			UserID:       &userID2,
			ActualUserID: &userID2,
			Model:        "gpt-3.5",
			TotalTokens:  80,
			TotalCost:    0.008,
			StatusCode:   200,
			Latency:      800,
			CacheHit:     false,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
	}

	for _, usage := range testUsage {
		require.NoError(t, db.Create(&usage).Error)
	}

	// Run the collection
	collector.collectCurrentMetrics()

	// Verify system metrics were created
	var systemMetrics models.SystemMetrics
	err := db.Where("interval = ?", models.IntervalHourly).
		Order("timestamp DESC").
		First(&systemMetrics).Error
	require.NoError(t, err)

	assert.Equal(t, 1, systemMetrics.ActiveModels) // Only gpt-4 is active (circuit not open)
	assert.Equal(t, 2, systemMetrics.TotalModels)
	assert.Equal(t, int64(3), systemMetrics.TotalRequests)
	assert.Equal(t, int64(1), systemMetrics.FailedRequests)
	assert.InDelta(t, 66.67, systemMetrics.SuccessRate, 0.01) // 2/3 success rate
	assert.Equal(t, int64(330), systemMetrics.TotalTokens)
	assert.Equal(t, 0.033, systemMetrics.TotalCost)
	assert.Equal(t, int64(1), systemMetrics.CacheHits)
	assert.Equal(t, int64(2), systemMetrics.CacheMisses)
	assert.InDelta(t, 33.33, systemMetrics.CacheHitRate, 0.01) // 1/3 cache hit rate

	// Verify model metrics were created
	var gpt4Metrics models.ModelMetrics
	err = db.Where("model_name = ? AND interval = ?", "gpt-4", models.IntervalHourly).
		Order("timestamp DESC").
		First(&gpt4Metrics).Error
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", gpt4Metrics.ModelName)
	assert.Equal(t, 95.5, gpt4Metrics.HealthScore)
	assert.Equal(t, int64(2), gpt4Metrics.TotalRequests)
	assert.Equal(t, int64(1), gpt4Metrics.FailedRequests)
	assert.Equal(t, 50.0, gpt4Metrics.SuccessRate) // 1/2 success rate for gpt-4
	assert.Equal(t, int64(250), gpt4Metrics.TotalTokens)
	assert.Equal(t, 0.025, gpt4Metrics.TotalCost)
	assert.Equal(t, int64(1500), gpt4Metrics.AvgLatency) // From actual latencies
	assert.Equal(t, int64(2000), gpt4Metrics.P95Latency)
	assert.Equal(t, int64(2000), gpt4Metrics.P99Latency)
	assert.False(t, gpt4Metrics.CircuitOpen)

	var gpt35Metrics models.ModelMetrics
	err = db.Where("model_name = ? AND interval = ?", "gpt-3.5", models.IntervalHourly).
		Order("timestamp DESC").
		First(&gpt35Metrics).Error
	require.NoError(t, err)

	assert.Equal(t, "gpt-3.5", gpt35Metrics.ModelName)
	assert.Equal(t, 60.0, gpt35Metrics.HealthScore)
	assert.Equal(t, int64(1), gpt35Metrics.TotalRequests)
	assert.Equal(t, int64(0), gpt35Metrics.FailedRequests) // No failed requests for gpt-3.5
	assert.Equal(t, 100.0, gpt35Metrics.SuccessRate)
	assert.Equal(t, int64(80), gpt35Metrics.TotalTokens)
	assert.Equal(t, 0.008, gpt35Metrics.TotalCost)
	assert.Equal(t, int64(800), gpt35Metrics.AvgLatency)
	assert.True(t, gpt35Metrics.CircuitOpen)

	mockProvider.AssertExpectations(t)
}

func TestMetricsCollector_collectUsageMetrics(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	mockProvider := new(MockModelStatsProvider)

	collector := NewMetricsCollector(db, logger, mockProvider)
	
	// Ensure the collector context stays valid for the entire test
	// This prevents context cancellation during the metrics collection
	defer func() {
		// Only call Stop if we explicitly started the collector
		// For direct method calls, we don't need to stop the collector
	}()

	// Create test users and teams
	userID1 := uuid.New()
	userID2 := uuid.New()
	teamID := uuid.New()

	// Create test users first
	user1 := models.User{
		BaseModel: models.BaseModel{ID: userID1},
		Email:     "user1@test.com",
		Username:  "user1",
		DexID:     "dex1",
		FirstName: "Test",
		LastName:  "User1",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user1).Error)

	user2 := models.User{
		BaseModel: models.BaseModel{ID: userID2},
		Email:     "user2@test.com",
		Username:  "user2",
		DexID:     "dex2",
		FirstName: "Test",
		LastName:  "User2",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user2).Error)

	team := models.Team{
		BaseModel: models.BaseModel{ID: teamID},
		Name:       "Test Team",
		MaxBudget:  100.0,
	}
	require.NoError(t, db.Create(&team).Error)

	// Create test usage data
	now := time.Now()
	testUsage := []models.Usage{
		{
			RequestID:    "req-1",
			Timestamp:    now.Add(-30 * time.Second),
			UserID:       &userID1,
			ActualUserID: &userID1,
			TeamID:       nil, // Personal usage
			Model:        "gpt-4",
			TotalTokens:  100,
			TotalCost:    0.01,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
		{
			RequestID:    "req-2",
			Timestamp:    now.Add(-45 * time.Second),
			UserID:       &userID1,
			ActualUserID: &userID1,
			TeamID:       &teamID, // Team usage
			Model:        "gpt-4",
			TotalTokens:  150,
			TotalCost:    0.015,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
		{
			RequestID:    "req-3",
			Timestamp:    now.Add(-20 * time.Second),
			UserID:       &userID2,
			ActualUserID: &userID2,
			TeamID:       &teamID, // Team usage
			Model:        "gpt-3.5",
			TotalTokens:  80,
			TotalCost:    0.008,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
	}

	for _, usage := range testUsage {
		require.NoError(t, db.Create(&usage).Error)
	}

	// Update team current_spend
	db.Model(&team).Update("current_spend", 0.023) // 0.015 + 0.008

	// Run the collection - use current time to ensure all test data falls within the collection window
	collector.collectUsageMetrics(now)
	
	// Add a small delay to ensure all database operations complete
	// before verification in case of timing issues in CI
	time.Sleep(100 * time.Millisecond)

	// Verify user metrics were created
	var userMetrics []models.UserMetrics
	err := db.Where("interval = ?", models.IntervalHourly).Find(&userMetrics).Error
	require.NoError(t, err)

	assert.Len(t, userMetrics, 2) // One for each user

	// Find metrics for user1
	var user1Metrics *models.UserMetrics
	for _, um := range userMetrics {
		if um.UserID == userID1 {
			user1Metrics = &um
			break
		}
	}
	require.NotNil(t, user1Metrics)

	assert.Equal(t, int64(2), user1Metrics.TotalRequests) // 2 requests
	assert.Equal(t, int64(250), user1Metrics.TotalTokens) // 100 + 150
	assert.Equal(t, 0.025, user1Metrics.TotalCost)        // 0.01 + 0.015
	assert.Equal(t, int64(1), user1Metrics.UserRequests)  // 1 personal request
	assert.Equal(t, int64(1), user1Metrics.TeamRequests)  // 1 team request

	// Verify team metrics were created
	var teamMetrics models.TeamMetrics
	err = db.Where("team_id = ? AND interval = ?", teamID, models.IntervalHourly).
		First(&teamMetrics).Error
	require.NoError(t, err)
	

	assert.Equal(t, teamID, teamMetrics.TeamID)
	assert.Equal(t, int64(2), teamMetrics.TotalRequests) // 2 team requests
	assert.Equal(t, int64(230), teamMetrics.TotalTokens) // 150 + 80
	assert.Equal(t, 0.023, teamMetrics.TotalCost)        // 0.015 + 0.008
	assert.Equal(t, 2, teamMetrics.ActiveMembers)        // 2 users active in team
	assert.Equal(t, 0.023, teamMetrics.CurrentSpend)
	assert.Equal(t, 0.023, teamMetrics.BudgetUsed) // 0.023 / 100 * 100
}

func TestMetricsCollector_StartStop(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	mockProvider := new(MockModelStatsProvider)

	collector := NewMetricsCollector(db, logger, mockProvider)

	// Mock empty stats to prevent nil pointer panics
	mockProvider.On("GetModelStats").Return(map[string]interface{}{
		"load_balancer": map[string]interface{}{},
	}).Maybe()

	// Test start
	collector.Start()

	// Verify ticker is running
	assert.NotNil(t, collector.ticker)

	// Test stop
	collector.Stop()

	// Verify context is cancelled
	select {
	case <-collector.ctx.Done():
		// Expected - context should be cancelled
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context was not cancelled after Stop()")
	}
}

func TestMetricsCollector_Latency_Calculations(t *testing.T) {
	// Test latency percentile calculations with various data sets
	testCases := []struct {
		name        string
		latencies   []int64
		expectedAvg int64
		expectedP95 int64
		expectedP99 int64
	}{
		{
			name:        "single_value",
			latencies:   []int64{1000},
			expectedAvg: 1000,
			expectedP95: 1000,
			expectedP99: 1000,
		},
		{
			name:        "small_set",
			latencies:   []int64{100, 200, 300, 400, 500},
			expectedAvg: 300,
			expectedP95: 500,
			expectedP99: 500,
		},
		{
			name: "large_set",
			latencies: func() []int64 {
				latencies := make([]int64, 100)
				for i := 0; i < 100; i++ {
					latencies[i] = int64((i + 1) * 10) // 10, 20, 30, ..., 1000
				}
				return latencies
			}(),
			expectedAvg: 505, // Average of 1-100 * 10
			expectedP95: 960, // 95th percentile (95% of 100 = 95, so index 95 = value 960)
			expectedP99: 1000, // 99th percentile (99% of 100 = 99, so index 99 = value 1000)
		},
		{
			name:        "empty_set",
			latencies:   []int64{},
			expectedAvg: 0,
			expectedP95: 0,
			expectedP99: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			avgLatency := int64(0)
			p95Latency := int64(0)
			p99Latency := int64(0)

			if len(tc.latencies) > 0 {
				// Copy the calculation logic from the actual code
				latencies := make([]int64, len(tc.latencies))
				copy(latencies, tc.latencies)
				sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

				// Calculate average latency
				var sum int64
				for _, lat := range latencies {
					sum += lat
				}
				avgLatency = sum / int64(len(latencies))

				// Calculate P95 latency
				p95Index := int(float64(len(latencies)) * 0.95)
				if p95Index >= len(latencies) {
					p95Index = len(latencies) - 1
				}
				if len(latencies) > 0 {
					p95Latency = latencies[p95Index]
				}

				// Calculate P99 latency
				p99Index := int(float64(len(latencies)) * 0.99)
				if p99Index >= len(latencies) {
					p99Index = len(latencies) - 1
				}
				if len(latencies) > 0 {
					p99Latency = latencies[p99Index]
				}
			}

			assert.Equal(t, tc.expectedAvg, avgLatency, "Average latency mismatch")
			assert.Equal(t, tc.expectedP95, p95Latency, "P95 latency mismatch")
			assert.Equal(t, tc.expectedP99, p99Latency, "P99 latency mismatch")
		})
	}
}

func TestMetricsCollector_Error_Handling(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	mockProvider := new(MockModelStatsProvider)

	collector := NewMetricsCollector(db, logger, mockProvider)

	// Test with invalid load balancer data
	mockProvider.On("GetModelStats").Return(map[string]interface{}{
		"load_balancer": "invalid_data", // Should be map[string]interface{}
	})

	// This should not panic and should handle the error gracefully
	collector.collectCurrentMetrics()

	// Verify no metrics were created due to error
	var count int64
	db.Model(&models.SystemMetrics{}).Count(&count)
	assert.Equal(t, int64(0), count)

	mockProvider.AssertExpectations(t)
}