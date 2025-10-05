package handlers

import (
	"encoding/json"
	"net/http"
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
	Model       string  `json:"model"`
	Requests    int64   `json:"requests"`
	Tokens      int64   `json:"tokens"`
	Cost        float64 `json:"cost"`
	AvgLatency  int64   `json:"avg_latency"`
	SuccessRate float64 `json:"success_rate"`
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

	// Get top models by requests (last 24h)
	var topModels []ModelUsage
	err = h.db.Raw(`
		SELECT 
			model,
			COUNT(*) as requests,
			SUM(total_tokens) as tokens,
			SUM(total_cost) as cost,
			ROUND(AVG(latency)) as avg_latency,
			ROUND(AVG(CASE WHEN status_code = 200 THEN 100 ELSE 0 END), 2) as success_rate
		FROM usage_logs 
		WHERE timestamp >= ?
		GROUP BY model
		ORDER BY requests DESC
		LIMIT 10
	`, last24h).Scan(&topModels).Error

	if err != nil {
		h.logger.Error("Failed to get top models", zap.Error(err))
	} else {
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
	days := r.URL.Query().Get("days")
	if days == "" {
		days = "30"
	}
	
	var daysInt int
	switch days {
	case "7":
		daysInt = 7
	case "30":
		daysInt = 30
	default:
		daysInt = 30
	}

	since := time.Now().AddDate(0, 0, -daysInt)

	var trends []struct {
		Date     string  `json:"date"`
		Requests int64   `json:"requests"`
		Tokens   int64   `json:"tokens"`
		Cost     float64 `json:"cost"`
	}

	err := h.db.Raw(`
		SELECT 
			DATE(timestamp) as date,
			COUNT(*) as requests,
			SUM(total_tokens) as tokens,
			SUM(total_cost) as cost
		FROM usage_logs
		WHERE timestamp >= ?
		GROUP BY DATE(timestamp)
		ORDER BY date ASC
	`, since).Scan(&trends).Error

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