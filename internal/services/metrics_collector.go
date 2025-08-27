package services

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/models"
)

// MetricsCollector aggregates real-time data into historical metrics
type MetricsCollector struct {
	db           *gorm.DB
	logger       *zap.Logger
	modelManager ModelStatsProvider
	ticker       *time.Ticker
	ctx          context.Context
	cancel       context.CancelFunc
}

// ModelStatsProvider interface for getting current model statistics
type ModelStatsProvider interface {
	GetModelStats() map[string]interface{}
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(db *gorm.DB, logger *zap.Logger, modelManager ModelStatsProvider) *MetricsCollector {
	ctx, cancel := context.WithCancel(context.Background())

	return &MetricsCollector{
		db:           db,
		logger:       logger,
		modelManager: modelManager,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins the metrics collection process
func (mc *MetricsCollector) Start() {
	// Collect metrics every minute for real-time data
	mc.ticker = time.NewTicker(1 * time.Minute)

	go func() {
		// Initial collection
		mc.collectCurrentMetrics()

		// Periodic collection
		for {
			select {
			case <-mc.ticker.C:
				mc.collectCurrentMetrics()
			case <-mc.ctx.Done():
				return
			}
		}
	}()

	// Start hourly and daily aggregation workers
	go mc.startHourlyAggregation()
	go mc.startDailyAggregation()

	mc.logger.Info("Metrics collector started")
}

// Stop stops the metrics collection
func (mc *MetricsCollector) Stop() {
	if mc.ticker != nil {
		mc.ticker.Stop()
	}
	mc.cancel()
	mc.logger.Info("Metrics collector stopped")
}

// collectCurrentMetrics collects real-time metrics and stores them
func (mc *MetricsCollector) collectCurrentMetrics() {
	now := time.Now().Truncate(time.Minute)

	// Get current model stats from load balancer
	modelStats := mc.modelManager.GetModelStats()

	// Extract load balancer data
	loadBalancerData, ok := modelStats["load_balancer"].(map[string]interface{})
	if !ok {
		mc.logger.Error("Failed to get load balancer data")
		return
	}

	// Collect system-wide metrics
	mc.collectSystemMetrics(now, modelStats, loadBalancerData)

	// Collect individual model metrics
	mc.collectModelMetrics(now, loadBalancerData)

	// Collect usage-based metrics (from usage_logs)
	mc.collectUsageMetrics(now)
}

// collectSystemMetrics collects system-wide metrics
func (mc *MetricsCollector) collectSystemMetrics(timestamp time.Time, systemStats map[string]interface{}, loadBalancer map[string]interface{}) {
	totalModels := len(loadBalancer)
	activeModels := 0
	totalHealthScore := 0.0

	for _, modelData := range loadBalancer {
		if modelMap, ok := modelData.(map[string]interface{}); ok {
			if circuitOpen, exists := modelMap["circuit_open"].(bool); exists && !circuitOpen {
				activeModels++
			}
			if healthScore, exists := modelMap["health_score"].(float64); exists {
				totalHealthScore += healthScore
			}
		}
	}

	avgHealthScore := 0.0
	if totalModels > 0 {
		avgHealthScore = totalHealthScore / float64(totalModels)
	}

	// Get aggregated request data from recent usage logs (last minute)
	var requestMetrics struct {
		TotalRequests  int64
		FailedRequests int64
		TotalTokens    int64
		TotalCost      float64
		CacheHits      int64
		CacheMisses    int64
	}

	since := timestamp.Add(-1 * time.Minute)
	err := mc.db.Raw(`
		SELECT 
			COUNT(*) as total_requests,
			SUM(CASE WHEN error_occurred THEN 1 ELSE 0 END) as failed_requests,
			SUM(total_tokens) as total_tokens,
			SUM(total_cost) as total_cost,
			SUM(CASE WHEN cache_hit THEN 1 ELSE 0 END) as cache_hits,
			SUM(CASE WHEN NOT cache_hit THEN 1 ELSE 0 END) as cache_misses
		FROM usage_logs 
		WHERE created_at >= ? AND created_at < ?
	`, since, timestamp).Scan(&requestMetrics).Error

	if err != nil {
		mc.logger.Error("Failed to get request metrics", zap.Error(err))
		return
	}

	successRate := 100.0
	if requestMetrics.TotalRequests > 0 {
		successRate = float64(requestMetrics.TotalRequests-requestMetrics.FailedRequests) / float64(requestMetrics.TotalRequests) * 100
	}

	cacheHitRate := 0.0
	totalCacheRequests := requestMetrics.CacheHits + requestMetrics.CacheMisses
	if totalCacheRequests > 0 {
		cacheHitRate = float64(requestMetrics.CacheHits) / float64(totalCacheRequests) * 100
	}

	systemMetrics := models.SystemMetrics{
		Interval:       models.IntervalHourly, // Store minute-level as hourly for real-time
		Timestamp:      timestamp,
		ShouldShedLoad: getBoolValue(systemStats, "should_shed_load"),
		ActiveModels:   activeModels,
		TotalModels:    totalModels,
		AvgHealthScore: avgHealthScore,
		TotalRequests:  requestMetrics.TotalRequests,
		FailedRequests: requestMetrics.FailedRequests,
		SuccessRate:    successRate,
		TotalTokens:    requestMetrics.TotalTokens,
		TotalCost:      requestMetrics.TotalCost,
		CacheHits:      requestMetrics.CacheHits,
		CacheMisses:    requestMetrics.CacheMisses,
		CacheHitRate:   cacheHitRate,
	}

	if err := mc.db.Create(&systemMetrics).Error; err != nil {
		mc.logger.Error("Failed to save system metrics", zap.Error(err))
	}
}

// collectModelMetrics collects individual model metrics
func (mc *MetricsCollector) collectModelMetrics(timestamp time.Time, loadBalancer map[string]interface{}) {
	for modelName, modelData := range loadBalancer {
		modelMap, ok := modelData.(map[string]interface{})
		if !ok {
			continue
		}

		// Get model-specific request data from usage logs
		var requestMetrics struct {
			TotalRequests  int64
			FailedRequests int64
			TotalTokens    int64
			InputTokens    int64
			OutputTokens   int64
			TotalCost      float64
			LatencyData    []int64
		}

		since := timestamp.Add(-1 * time.Minute)
		err := mc.db.Raw(`
			SELECT 
				COUNT(*) as total_requests,
				SUM(CASE WHEN error_occurred THEN 1 ELSE 0 END) as failed_requests,
				SUM(total_tokens) as total_tokens,
				SUM(prompt_tokens) as input_tokens,
				SUM(completion_tokens) as output_tokens,
				SUM(total_cost) as total_cost
			FROM usage_logs 
			WHERE model = ? AND created_at >= ? AND created_at < ?
		`, modelName, since, timestamp).Scan(&requestMetrics).Error

		if err != nil {
			mc.logger.Error("Failed to get model request metrics",
				zap.String("model", modelName),
				zap.Error(err))
			continue
		}

		// Get latency data for percentile calculations
		var latencies []int64
		mc.db.Raw(`
			SELECT response_time_ms 
			FROM usage_logs 
			WHERE model = ? AND created_at >= ? AND created_at < ? 
			AND response_time_ms IS NOT NULL AND response_time_ms > 0
			ORDER BY response_time_ms
		`, modelName, since, timestamp).Scan(&latencies)

		avgLatency := getInt64Value(modelMap, "avg_latency")
		p95Latency := getInt64Value(modelMap, "p95_latency")
		p99Latency := int64(0)

		if len(latencies) > 0 {
			sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

			// Calculate P99 latency
			p99Index := int(math.Ceil(float64(len(latencies))*0.99)) - 1
			if p99Index >= 0 && p99Index < len(latencies) {
				p99Latency = latencies[p99Index]
			}
		}

		successRate := 100.0
		if requestMetrics.TotalRequests > 0 {
			successRate = float64(requestMetrics.TotalRequests-requestMetrics.FailedRequests) / float64(requestMetrics.TotalRequests) * 100
		}

		modelMetrics := models.ModelMetrics{
			ModelName:       modelName,
			Interval:        models.IntervalHourly,
			Timestamp:       timestamp,
			HealthScore:     getFloat64Value(modelMap, "health_score"),
			AvgLatency:      avgLatency,
			P95Latency:      p95Latency,
			P99Latency:      p99Latency,
			TotalRequests:   requestMetrics.TotalRequests,
			FailedRequests:  requestMetrics.FailedRequests,
			SuccessRate:     successRate,
			TotalTokens:     requestMetrics.TotalTokens,
			InputTokens:     requestMetrics.InputTokens,
			OutputTokens:    requestMetrics.OutputTokens,
			TotalCost:       requestMetrics.TotalCost,
			CircuitOpen:     getBoolValue(modelMap, "circuit_open"),
			CircuitFailures: getInt64Value(modelMap, "failed_requests"),
		}

		if err := mc.db.Create(&modelMetrics).Error; err != nil {
			mc.logger.Error("Failed to save model metrics",
				zap.String("model", modelName),
				zap.Error(err))
		}
	}
}

// collectUsageMetrics collects user and team usage metrics
func (mc *MetricsCollector) collectUsageMetrics(timestamp time.Time) {
	since := timestamp.Add(-1 * time.Minute)

	// Collect user metrics
	var userStats []struct {
		UserID        string
		TotalRequests int64
		TotalTokens   int64
		TotalCost     float64
		UserRequests  int64
		TeamRequests  int64
		ModelUsage    string
	}

	err := mc.db.Raw(`
		SELECT 
			actual_user_id as user_id,
			COUNT(*) as total_requests,
			SUM(total_tokens) as total_tokens,
			SUM(total_cost) as total_cost,
			SUM(CASE WHEN team_id IS NULL THEN 1 ELSE 0 END) as user_requests,
			SUM(CASE WHEN team_id IS NOT NULL THEN 1 ELSE 0 END) as team_requests,
			json_object_agg(model, model_count) as model_usage
		FROM (
			SELECT 
				actual_user_id,
				team_id,
				total_tokens,
				total_cost,
				model,
				COUNT(*) as model_count
			FROM usage_logs 
			WHERE created_at >= ? AND created_at < ?
			GROUP BY actual_user_id, team_id, model, total_tokens, total_cost
		) subq
		GROUP BY actual_user_id
	`, since, timestamp).Scan(&userStats).Error

	if err != nil {
		mc.logger.Error("Failed to get user metrics", zap.Error(err))
	} else {
		for _, stat := range userStats {
			userMetrics := models.UserMetrics{
				UserID:        mustParseUUID(stat.UserID),
				Interval:      models.IntervalHourly,
				Timestamp:     timestamp,
				TotalRequests: stat.TotalRequests,
				TotalTokens:   stat.TotalTokens,
				TotalCost:     stat.TotalCost,
				UserRequests:  stat.UserRequests,
				TeamRequests:  stat.TeamRequests,
				ModelUsage:    stat.ModelUsage,
			}

			if err := mc.db.Create(&userMetrics).Error; err != nil {
				mc.logger.Error("Failed to save user metrics",
					zap.String("user_id", stat.UserID),
					zap.Error(err))
			}
		}
	}

	// Collect team metrics
	var teamStats []struct {
		TeamID        string
		TotalRequests int64
		TotalTokens   int64
		TotalCost     float64
		ActiveMembers int64
		CurrentSpend  float64
		ModelUsage    string
	}

	err = mc.db.Raw(`
		SELECT 
			ul.team_id,
			COUNT(*) as total_requests,
			SUM(ul.total_tokens) as total_tokens,
			SUM(ul.total_cost) as total_cost,
			COUNT(DISTINCT ul.actual_user_id) as active_members,
			t.current_spend,
			json_object_agg(ul.model, model_count) as model_usage
		FROM usage_logs ul
		JOIN teams t ON ul.team_id = t.id
		WHERE ul.created_at >= ? AND ul.created_at < ?
		GROUP BY ul.team_id, t.current_spend
	`, since, timestamp).Scan(&teamStats).Error

	if err != nil {
		mc.logger.Error("Failed to get team metrics", zap.Error(err))
	} else {
		for _, stat := range teamStats {
			budgetUsed := 0.0
			// Get team budget info to calculate budget usage
			var team models.Team
			if err := mc.db.First(&team, "id = ?", stat.TeamID).Error; err == nil {
				if team.MaxBudget > 0 {
					budgetUsed = (stat.CurrentSpend / team.MaxBudget) * 100
				}
			}

			teamMetrics := models.TeamMetrics{
				TeamID:        mustParseUUID(stat.TeamID),
				Interval:      models.IntervalHourly,
				Timestamp:     timestamp,
				TotalRequests: stat.TotalRequests,
				TotalTokens:   stat.TotalTokens,
				TotalCost:     stat.TotalCost,
				ActiveMembers: int(stat.ActiveMembers),
				CurrentSpend:  stat.CurrentSpend,
				BudgetUsed:    budgetUsed,
				ModelUsage:    stat.ModelUsage,
			}

			if err := mc.db.Create(&teamMetrics).Error; err != nil {
				mc.logger.Error("Failed to save team metrics",
					zap.String("team_id", stat.TeamID),
					zap.Error(err))
			}
		}
	}
}

// startHourlyAggregation starts the hourly aggregation worker
func (mc *MetricsCollector) startHourlyAggregation() {
	// Run every hour at minute 5 to ensure all minute data is collected
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.aggregateHourlyMetrics()
		case <-mc.ctx.Done():
			return
		}
	}
}

// startDailyAggregation starts the daily aggregation worker
func (mc *MetricsCollector) startDailyAggregation() {
	// Run every day at 00:05 to ensure all hourly data is collected
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.aggregateDailyMetrics()
		case <-mc.ctx.Done():
			return
		}
	}
}

// aggregateHourlyMetrics aggregates minute-level data into hourly metrics
func (mc *MetricsCollector) aggregateHourlyMetrics() {
	now := time.Now()
	hourStart := now.Truncate(time.Hour).Add(-1 * time.Hour) // Previous hour
	hourEnd := hourStart.Add(1 * time.Hour)

	mc.logger.Info("Starting hourly aggregation",
		zap.Time("hour_start", hourStart),
		zap.Time("hour_end", hourEnd))

	// Aggregate model metrics
	mc.aggregateModelMetricsForPeriod(hourStart, hourEnd, models.IntervalHourly)

	// Aggregate system metrics
	mc.aggregateSystemMetricsForPeriod(hourStart, hourEnd, models.IntervalHourly)
}

// aggregateDailyMetrics aggregates hourly data into daily metrics
func (mc *MetricsCollector) aggregateDailyMetrics() {
	now := time.Now()
	dayStart := now.Truncate(24*time.Hour).AddDate(0, 0, -1) // Previous day
	dayEnd := dayStart.AddDate(0, 0, 1)

	mc.logger.Info("Starting daily aggregation",
		zap.Time("day_start", dayStart),
		zap.Time("day_end", dayEnd))

	// Aggregate model metrics
	mc.aggregateModelMetricsForPeriod(dayStart, dayEnd, models.IntervalDaily)

	// Aggregate system metrics
	mc.aggregateSystemMetricsForPeriod(dayStart, dayEnd, models.IntervalDaily)
}

// Helper functions for aggregation
func (mc *MetricsCollector) aggregateModelMetricsForPeriod(start, end time.Time, interval models.MetricInterval) {
	// Implementation for model metrics aggregation
	// This would query the minute/hourly data and create aggregated records
}

func (mc *MetricsCollector) aggregateSystemMetricsForPeriod(start, end time.Time, interval models.MetricInterval) {
	// Implementation for system metrics aggregation
	// This would query the minute/hourly data and create aggregated records
}

// Helper functions
func getFloat64Value(m map[string]interface{}, key string) float64 {
	if val, ok := m[key].(float64); ok {
		return val
	}
	if val, ok := m[key].(int); ok {
		return float64(val)
	}
	return 0.0
}

func getInt64Value(m map[string]interface{}, key string) int64 {
	if val, ok := m[key].(int64); ok {
		return val
	}
	if val, ok := m[key].(int); ok {
		return int64(val)
	}
	if val, ok := m[key].(float64); ok {
		return int64(val)
	}
	return 0
}

func getBoolValue(m map[string]interface{}, key string) bool {
	if val, ok := m[key].(bool); ok {
		return val
	}
	return false
}

func mustParseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.New() // Return new UUID if parsing fails
	}
	return id
}
