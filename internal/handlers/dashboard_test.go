package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	"github.com/amerfu/pllm/internal/models"
	"github.com/amerfu/pllm/internal/testutil"
)

func TestDashboardHandler_GetDashboardMetrics(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	handler := NewDashboardHandler(db, logger)

	// Create test users first
	user1 := models.User{
		Email:     "user1@test.com",
		Username:  "user1",
		DexID:     "dex1",
		FirstName: "Test",
		LastName:  "User1",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user1).Error)

	user2 := models.User{
		Email:     "user2@test.com",
		Username:  "user2",
		DexID:     "dex2",
		FirstName: "Test",
		LastName:  "User2",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user2).Error)

	// Create test data
	now := time.Now()
	testUsage := []models.Usage{
		{
			RequestID:    "req-1",
			Timestamp:    now.Add(-30 * time.Minute), // Within last hour
			UserID:       user1.ID,
			ActualUserID: user1.ID,
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
			Timestamp:    now.Add(-15 * time.Minute), // Within last hour
			UserID:       user2.ID,
			ActualUserID: user2.ID,
			Model:        "gpt-3.5",
			TotalTokens:  50,
			TotalCost:    0.005,
			StatusCode:   200,
			Latency:      500,
			CacheHit:     true,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
		{
			RequestID:    "req-3",
			Timestamp:    now.Add(-2 * time.Hour), // Outside last hour
			UserID:       user1.ID,
			ActualUserID: user1.ID,
			Model:        "gpt-4",
			TotalTokens:  200,
			TotalCost:    0.02,
			StatusCode:   500, // Failed request
			Latency:      2000,
			CacheHit:     false,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
	}

	for _, usage := range testUsage {
		require.NoError(t, db.Create(&usage).Error)
	}

	// Create test keys
	testKeys := []models.Key{
		{Key: "key1", KeyHash: "hash1", KeyPrefix: "pllm_1", IsActive: true, RevokedAt: nil},
		{Key: "key2", KeyHash: "hash2", KeyPrefix: "pllm_2", IsActive: true, RevokedAt: nil},
		{Key: "key3", KeyHash: "hash3", KeyPrefix: "pllm_3", IsActive: false, RevokedAt: &now}, // Inactive key
	}

	for _, key := range testKeys {
		require.NoError(t, db.Create(&key).Error)
	}

	// Create test system metrics
	systemMetrics := models.SystemMetrics{
		Interval:     models.IntervalHourly,
		Timestamp:    now,
		ActiveModels: 5,
	}
	require.NoError(t, db.Create(&systemMetrics).Error)

	// Test the handler
	req := httptest.NewRequest(http.MethodGet, "/dashboard/metrics", nil)
	w := httptest.NewRecorder()

	handler.GetDashboardMetrics(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response DashboardMetrics
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	// Verify metrics
	assert.Equal(t, int64(3), response.TotalRequests) // All requests in last 24h
	assert.Equal(t, int64(350), response.TotalTokens) // Sum of all tokens
	assert.Equal(t, 0.035, response.TotalCost)        // Sum of all costs
	assert.Equal(t, 2, response.ActiveKeys)           // Only active keys
	assert.Equal(t, 5, response.ActiveModels)         // From system metrics
	assert.Equal(t, 66.67, response.SuccessRate)      // 2/3 successful requests
	assert.Equal(t, 33.33, response.CacheHitRate)     // 1/3 cache hits

	// Verify recent activity
	assert.Equal(t, int64(2), response.RecentActivity.LastHour.Requests) // Only req-1 and req-2
	assert.Equal(t, int64(150), response.RecentActivity.LastHour.Tokens)
	assert.Equal(t, 0.015, response.RecentActivity.LastHour.Cost)

	// Verify top models
	assert.Len(t, response.TopModels, 2) // gpt-4 and gpt-3.5
}

func TestDashboardHandler_GetModelMetrics(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	handler := NewDashboardHandler(db, logger)

	// Create test users first
	user1 := models.User{
		Email:     "user1@test.com",
		Username:  "user1",
		DexID:     "dex1",
		FirstName: "Test",
		LastName:  "User1",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user1).Error)

	// Create test data for specific model
	now := time.Now()
	testUsage := []models.Usage{
		{
			RequestID:    "req-1",
			Timestamp:    now.Add(-24 * time.Hour),
			UserID:       user1.ID,
			ActualUserID: user1.ID,
			Model:        "gpt-4",
			TotalTokens:  100,
			TotalCost:    0.01,
			StatusCode:   200,
			Latency:      1000,
			CacheHit:     true,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
		{
			RequestID:    "req-2",
			Timestamp:    now.Add(-48 * time.Hour),
			UserID:       user1.ID,
			ActualUserID: user1.ID,
			Model:        "gpt-4",
			TotalTokens:  150,
			TotalCost:    0.015,
			StatusCode:   200,
			Latency:      1500,
			CacheHit:     false,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
		{
			RequestID:    "req-3",
			Timestamp:    now.Add(-24 * time.Hour),
			UserID:       user1.ID,
			ActualUserID: user1.ID,
			Model:        "gpt-3.5", // Different model, should be ignored
			TotalTokens:  50,
			TotalCost:    0.005,
			StatusCode:   200,
			Latency:      500,
			CacheHit:     false,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
	}

	for _, usage := range testUsage {
		require.NoError(t, db.Create(&usage).Error)
	}

	// Create request with model parameter
	req := httptest.NewRequest(http.MethodGet, "/dashboard/models/gpt-4", nil)
	
	// Add chi URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("model", "gpt-4")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	
	w := httptest.NewRecorder()

	handler.GetModelMetrics(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		TotalRequests int64     `json:"total_requests"`
		TotalTokens   int64     `json:"total_tokens"`
		TotalCost     float64   `json:"total_cost"`
		AvgLatency    int64     `json:"avg_latency"`
		SuccessRate   float64   `json:"success_rate"`
		CacheHitRate  float64   `json:"cache_hit_rate"`
		LastUsed      time.Time `json:"last_used"`
	}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	// Verify model-specific metrics
	assert.Equal(t, int64(2), response.TotalRequests) // Only gpt-4 requests
	assert.Equal(t, int64(250), response.TotalTokens) // Sum of gpt-4 tokens
	assert.Equal(t, 0.025, response.TotalCost)        // Sum of gpt-4 costs
	assert.Equal(t, int64(1250), response.AvgLatency) // Average latency
	assert.Equal(t, 100.0, response.SuccessRate)      // All successful
	assert.Equal(t, 50.0, response.CacheHitRate)      // 1/2 cache hits
}

func TestDashboardHandler_GetUsageTrends(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	handler := NewDashboardHandler(db, logger)

	// Create test users first
	user1 := models.User{
		Email:     "user1@test.com",
		Username:  "user1",
		DexID:     "dex1",
		FirstName: "Test",
		LastName:  "User1",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user1).Error)

	// Create test data across multiple days
	now := time.Now()
	testUsage := []models.Usage{
		{
			RequestID:    "req-1",
			Timestamp:    now.Add(-1 * 24 * time.Hour),
			UserID:       user1.ID,
			ActualUserID: user1.ID,
			TotalTokens:  100,
			TotalCost:    0.01,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
		{
			RequestID:    "req-2",
			Timestamp:    now.Add(-1 * 24 * time.Hour),
			UserID:       user1.ID,
			ActualUserID: user1.ID,
			TotalTokens:  50,
			TotalCost:    0.005,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
		{
			RequestID:    "req-3",
			Timestamp:    now.Add(-2 * 24 * time.Hour),
			UserID:       user1.ID,
			ActualUserID: user1.ID,
			TotalTokens:  200,
			TotalCost:    0.02,
			Metadata:     datatypes.JSON([]byte(`{}`)),
		},
	}

	for _, usage := range testUsage {
		require.NoError(t, db.Create(&usage).Error)
	}

	// Test with default days parameter
	req := httptest.NewRequest(http.MethodGet, "/dashboard/usage-trends", nil)
	w := httptest.NewRecorder()

	handler.GetUsageTrends(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []struct {
		Date     string  `json:"date"`
		Requests int64   `json:"requests"`
		Tokens   int64   `json:"tokens"`
		Cost     float64 `json:"cost"`
	}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	// Should have entries for the days with data
	assert.Len(t, response, 2)

	// Verify aggregated data for each day
	found1DayAgo := false
	found2DaysAgo := false

	for _, trend := range response {
		switch trend.Requests {
		case 2:
			found1DayAgo = true
			assert.Equal(t, int64(150), trend.Tokens)
			assert.Equal(t, 0.015, trend.Cost)
		case 1:
			found2DaysAgo = true
			assert.Equal(t, int64(200), trend.Tokens)
			assert.Equal(t, 0.02, trend.Cost)
		}
	}

	assert.True(t, found1DayAgo, "Should find data for 1 day ago")
	assert.True(t, found2DaysAgo, "Should find data for 2 days ago")
}

func TestDashboardHandler_GetUsageTrendsWithDaysParam(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	handler := NewDashboardHandler(db, logger)

	// Test with days parameter
	req := httptest.NewRequest(http.MethodGet, "/dashboard/usage-trends?days=7", nil)
	w := httptest.NewRecorder()

	handler.GetUsageTrends(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []struct {
		Date     string  `json:"date"`
		Requests int64   `json:"requests"`
		Tokens   int64   `json:"tokens"`
		Cost     float64 `json:"cost"`
	}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	// Should return empty array for no data in last 7 days
	assert.Empty(t, response)
}

func TestDashboardHandler_GetModelMetrics_NotFound(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	handler := NewDashboardHandler(db, logger)

	// Test with missing model parameter
	req := httptest.NewRequest(http.MethodGet, "/dashboard/models/", nil)
	w := httptest.NewRecorder()

	handler.GetModelMetrics(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDashboardHandler_GetDashboardMetrics_EmptyData(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	logger := zap.NewNop()
	handler := NewDashboardHandler(db, logger)

	// Test with no data
	req := httptest.NewRequest(http.MethodGet, "/dashboard/metrics", nil)
	w := httptest.NewRecorder()

	handler.GetDashboardMetrics(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response DashboardMetrics
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	// Verify zero values
	assert.Equal(t, int64(0), response.TotalRequests)
	assert.Equal(t, int64(0), response.TotalTokens)
	assert.Equal(t, 0.0, response.TotalCost)
	assert.Equal(t, 0, response.ActiveKeys)
	assert.Equal(t, 0.0, response.SuccessRate)
	assert.Equal(t, 0.0, response.CacheHitRate)
}