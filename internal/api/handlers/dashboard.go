package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

type DashboardHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewDashboardHandler(db *gorm.DB, logger *zap.Logger) *DashboardHandler {
	return &DashboardHandler{
		db:     db,
		logger: logger,
	}
}

type DashboardMetrics struct {
	TotalRequests  int64   `json:"total_requests"`
	TotalTokens    int64   `json:"total_tokens"`
	TotalCost      float64 `json:"total_cost"`
	ActiveKeys     int     `json:"active_keys"`
	ActiveModels   int     `json:"active_models"`
	SuccessRate    float64 `json:"success_rate"`
	CacheHitRate   float64 `json:"cache_hit_rate"`
	AvgLatency     int64   `json:"avg_latency"`
	RecentActivity struct {
		Last24h struct {
			Requests int64   `json:"requests"`
			Tokens   int64   `json:"tokens"`
			Cost     float64 `json:"cost"`
		} `json:"last_24h"`
		LastHour struct {
			Requests int64   `json:"requests"`
			Tokens   int64   `json:"tokens"`
			Cost     float64 `json:"cost"`
		} `json:"last_hour"`
	} `json:"recent_activity"`
	TopModels []ModelUsage `json:"top_models"`
}

type ModelUsage struct {
	Model        string  `json:"model"`
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	Cost         float64 `json:"cost"`
	AvgLatency   int64   `json:"avg_latency"`
	SuccessRate  float64 `json:"success_rate"`
	HealthScore  float64 `json:"health_score"`
	P95Latency   float64 `json:"p95_latency"`
	P99Latency   float64 `json:"p99_latency"`
	CacheHitRate float64 `json:"cache_hit_rate"`
}

func (h *DashboardHandler) GetDashboardMetrics(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	last24h := now.Add(-24 * time.Hour)
	lastHour := now.Add(-1 * time.Hour)

	metrics := DashboardMetrics{}

	// Get overall statistics from usage_logs (all time)
	var overallStats struct {
		TotalRequests int64
		TotalTokens   int64
		TotalCost     float64
		SuccessRate   float64
		CacheHitRate  float64
		AvgLatency    float64
	}

	err := h.db.Raw(`
		SELECT 
			COUNT(*) as total_requests,
			SUM(total_tokens) as total_tokens,
			SUM(total_cost) as total_cost,
			ROUND(AVG(CASE WHEN status_code = 200 THEN 100 ELSE 0 END), 2) as success_rate,
			ROUND(AVG(CASE WHEN cache_hit THEN 100 ELSE 0 END), 2) as cache_hit_rate,
			ROUND(AVG(latency)) as avg_latency
		FROM usage_logs
		WHERE timestamp >= ?
	`, last24h).Scan(&overallStats).Error

	if err != nil {
		h.logger.Error("Failed to get overall stats", zap.Error(err))
		http.Error(w, "Failed to get metrics", http.StatusInternalServerError)
		return
	}

	metrics.TotalRequests = overallStats.TotalRequests
	metrics.TotalTokens = overallStats.TotalTokens
	metrics.TotalCost = overallStats.TotalCost
	metrics.SuccessRate = overallStats.SuccessRate
	metrics.CacheHitRate = overallStats.CacheHitRate
	metrics.AvgLatency = int64(overallStats.AvgLatency)

	// Get active keys count
	var activeKeysCount int64
	h.db.Model(&models.Key{}).Where("is_active = ? AND revoked_at IS NULL", true).Count(&activeKeysCount)
	metrics.ActiveKeys = int(activeKeysCount)

	// Get active models from recent system metrics
	var latestSystemMetrics models.SystemMetrics
	h.db.Where("interval = ?", models.IntervalHourly).
		Order("timestamp DESC").
		First(&latestSystemMetrics)
	metrics.ActiveModels = latestSystemMetrics.ActiveModels

	// Get last 24h activity
	err = h.db.Raw(`
		SELECT 
			COUNT(*) as requests,
			COALESCE(SUM(total_tokens), 0) as tokens,
			COALESCE(SUM(total_cost), 0) as cost
		FROM usage_logs 
		WHERE timestamp >= ?
	`, last24h).Scan(&metrics.RecentActivity.Last24h).Error

	if err != nil {
		h.logger.Error("Failed to get 24h stats", zap.Error(err))
	}

	// Get last hour activity
	err = h.db.Raw(`
		SELECT 
			COUNT(*) as requests,
			COALESCE(SUM(total_tokens), 0) as tokens,
			COALESCE(SUM(total_cost), 0) as cost
		FROM usage_logs 
		WHERE timestamp >= ?
	`, lastHour).Scan(&metrics.RecentActivity.LastHour).Error

	if err != nil {
		h.logger.Error("Failed to get 1h stats", zap.Error(err))
	}

	// Get top models by requests (last 24h) with enhanced metrics
	var topModels []ModelUsage
	err = h.db.Raw(`
		SELECT
			model,
			COUNT(*) as requests,
			SUM(total_tokens) as tokens,
			SUM(total_cost) as cost,
			ROUND(AVG(latency)) as avg_latency,
			ROUND(AVG(CASE WHEN status_code = 200 THEN 100 ELSE 0 END), 2) as success_rate,
			ROUND(AVG(CASE WHEN cache_hit THEN 100 ELSE 0 END), 2) as cache_hit_rate,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency) as p95_latency,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency) as p99_latency
		FROM usage_logs
		WHERE timestamp >= ?
		GROUP BY model
		ORDER BY requests DESC
		LIMIT 10
	`, last24h).Scan(&topModels).Error

	if err != nil {
		h.logger.Error("Failed to get top models", zap.Error(err))
	} else {
		// Calculate health scores based on latency and success rate
		for i := range topModels {
			topModels[i].HealthScore = calculateHealthScore(
				topModels[i].AvgLatency,
				topModels[i].SuccessRate,
				topModels[i].P99Latency,
			)
		}
		metrics.TopModels = topModels
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		h.logger.Error("Failed to encode dashboard metrics", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *DashboardHandler) GetModelMetrics(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model")
	if modelName == "" {
		http.Error(w, "Model name is required", http.StatusBadRequest)
		return
	}

	// Get model metrics for the last 30 days
	since := time.Now().AddDate(0, 0, -30)
	
	var modelStats struct {
		TotalRequests   int64   `json:"total_requests"`
		TotalTokens     int64   `json:"total_tokens"`
		TotalCost       float64 `json:"total_cost"`
		AvgLatency      int64   `json:"avg_latency"`
		SuccessRate     float64 `json:"success_rate"`
		CacheHitRate    float64 `json:"cache_hit_rate"`
		LastUsed        *time.Time `json:"last_used,omitempty"`
	}

	err := h.db.Raw(`
		SELECT 
			COUNT(*) as total_requests,
			SUM(total_tokens) as total_tokens,
			SUM(total_cost) as total_cost,
			ROUND(AVG(latency)) as avg_latency,
			ROUND(AVG(CASE WHEN status_code = 200 THEN 100 ELSE 0 END), 2) as success_rate,
			ROUND(AVG(CASE WHEN cache_hit THEN 100 ELSE 0 END), 2) as cache_hit_rate,
			MAX(timestamp) as last_used
		FROM usage_logs
		WHERE model = ? AND timestamp >= ?
	`, modelName, since).Scan(&modelStats).Error

	if err != nil {
		h.logger.Error("Failed to get model stats", zap.String("model", modelName), zap.Error(err))
		http.Error(w, "Failed to get model metrics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(modelStats); err != nil {
		h.logger.Error("Failed to encode model metrics", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *DashboardHandler) GetUsageTrends(w http.ResponseWriter, r *http.Request) {
	interval := r.URL.Query().Get("interval")
	hoursParam := r.URL.Query().Get("hours")
	daysParam := r.URL.Query().Get("days")

	since := parseTrendTimeRange(hoursParam, daysParam)
	groupExpr := trendGroupExpr(interval)

	var trends []struct {
		Date     string  `json:"date"`
		Requests int64   `json:"requests"`
		Tokens   int64   `json:"tokens"`
		Cost     float64 `json:"cost"`
	}

	query := fmt.Sprintf(`
		SELECT
			%s as date,
			COUNT(*) as requests,
			SUM(total_tokens) as tokens,
			SUM(total_cost) as cost
		FROM usage_logs
		WHERE timestamp >= ?
		GROUP BY %s
		ORDER BY date ASC
	`, groupExpr, groupExpr)

	err := h.db.Raw(query, since).Scan(&trends).Error

	if err != nil {
		h.logger.Error("Failed to get usage trends", zap.Error(err))
		http.Error(w, "Failed to get usage trends", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(trends); err != nil {
		h.logger.Error("Failed to encode usage trends", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *DashboardHandler) GetModelTrends(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model")
	if modelName == "" {
		http.Error(w, "Model name is required", http.StatusBadRequest)
		return
	}

	interval := r.URL.Query().Get("interval")
	hoursParam := r.URL.Query().Get("hours")
	daysParam := r.URL.Query().Get("days")

	since := parseTrendTimeRange(hoursParam, daysParam)
	groupExpr := trendGroupExpr(interval)

	var trends []struct {
		Date        string  `json:"date"`
		Requests    int64   `json:"requests"`
		Tokens      int64   `json:"tokens"`
		Cost        float64 `json:"cost"`
		AvgLatency  int64   `json:"avg_latency"`
		SuccessRate float64 `json:"success_rate"`
	}

	query := fmt.Sprintf(`
		SELECT
			%s as date,
			COUNT(*) as requests,
			SUM(total_tokens) as tokens,
			SUM(total_cost) as cost,
			ROUND(AVG(latency)) as avg_latency,
			ROUND(AVG(CASE WHEN status_code = 200 THEN 100 ELSE 0 END), 2) as success_rate
		FROM usage_logs
		WHERE model = ? AND timestamp >= ?
		GROUP BY %s
		ORDER BY date ASC
	`, groupExpr, groupExpr)

	err := h.db.Raw(query, modelName, since).Scan(&trends).Error

	if err != nil {
		h.logger.Error("Failed to get model trends", zap.String("model", modelName), zap.Error(err))
		http.Error(w, "Failed to get model trends", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(trends); err != nil {
		h.logger.Error("Failed to encode model trends", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// parseTrendTimeRange determines the "since" time from hours/days query params.
// If hours is provided, it takes priority over days.
func parseTrendTimeRange(hoursParam, daysParam string) time.Time {
	if hoursParam != "" {
		h, err := strconv.Atoi(hoursParam)
		if err == nil {
			switch h {
			case 1, 6, 24:
				return time.Now().Add(-time.Duration(h) * time.Hour)
			}
		}
	}

	if daysParam != "" {
		d, err := strconv.Atoi(daysParam)
		if err == nil {
			switch d {
			case 7, 30, 90:
				return time.Now().AddDate(0, 0, -d)
			}
		}
	}

	// Default: 30 days
	return time.Now().AddDate(0, 0, -30)
}

// trendGroupExpr returns the SQL expression used to group trend data.
// Uses TO_CHAR to return a deterministic string format that JavaScript can parse.
func trendGroupExpr(interval string) string {
	if interval == "hourly" {
		return "TO_CHAR(date_trunc('hour', timestamp), 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"')"
	}
	return "TO_CHAR(DATE(timestamp), 'YYYY-MM-DD')"
}

// calculateHealthScore computes a health score (0-100) based on latency and success rate
// Formula: Base score from success rate, penalties for high latency
func calculateHealthScore(avgLatency int64, successRate float64, p99Latency float64) float64 {
	// Start with success rate as base (0-100)
	score := successRate

	// Penalty for average latency
	// Excellent: < 500ms (no penalty)
	// Good: 500ms-1s (small penalty)
	// Degraded: 1s-3s (medium penalty)
	// Poor: > 3s (large penalty)
	if avgLatency > 5000 {
		score -= 30
	} else if avgLatency > 3000 {
		score -= 20
	} else if avgLatency > 1000 {
		score -= 10
	} else if avgLatency > 500 {
		score -= 5
	}

	// Additional penalty for high p99 latency (tail latency)
	if p99Latency > 10000 {
		score -= 15
	} else if p99Latency > 5000 {
		score -= 10
	} else if p99Latency > 2000 {
		score -= 5
	}

	// Ensure score stays within bounds
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}