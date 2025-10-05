package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MetricInterval defines the time intervals for aggregated metrics
type MetricInterval string

const (
	IntervalHourly MetricInterval = "hourly"
	IntervalDaily  MetricInterval = "daily"
)

// ModelMetrics stores aggregated metrics for models over time
type ModelMetrics struct {
	BaseModel
	ModelName string         `gorm:"not null;index:idx_model_metrics_lookup" json:"model_name"`
	Interval  MetricInterval `gorm:"not null;index:idx_model_metrics_lookup" json:"interval"`
	Timestamp time.Time      `gorm:"not null;index:idx_model_metrics_lookup" json:"timestamp"`

	// Health and Performance Metrics
	HealthScore float64 `gorm:"default:0" json:"health_score"`
	AvgLatency  int64   `gorm:"default:0" json:"avg_latency"`
	P95Latency  int64   `gorm:"default:0" json:"p95_latency"`
	P99Latency  int64   `gorm:"default:0" json:"p99_latency"`

	// Request Metrics
	TotalRequests  int64   `gorm:"default:0" json:"total_requests"`
	FailedRequests int64   `gorm:"default:0" json:"failed_requests"`
	SuccessRate    float64 `gorm:"default:0" json:"success_rate"`

	// Token and Cost Metrics
	TotalTokens  int64   `gorm:"default:0" json:"total_tokens"`
	InputTokens  int64   `gorm:"default:0" json:"input_tokens"`
	OutputTokens int64   `gorm:"default:0" json:"output_tokens"`
	TotalCost    float64 `gorm:"default:0" json:"total_cost"`

	// Circuit Breaker State
	CircuitOpen     bool  `gorm:"default:false" json:"circuit_open"`
	CircuitFailures int64 `gorm:"default:0" json:"circuit_failures"`
}

// SystemMetrics stores system-wide aggregated metrics
type SystemMetrics struct {
	BaseModel
	Interval  MetricInterval `gorm:"not null;index:idx_system_metrics_lookup" json:"interval"`
	Timestamp time.Time      `gorm:"not null;index:idx_system_metrics_lookup" json:"timestamp"`

	// System Health
	ShouldShedLoad bool    `gorm:"default:false" json:"should_shed_load"`
	ActiveModels   int     `gorm:"default:0" json:"active_models"`
	TotalModels    int     `gorm:"default:0" json:"total_models"`
	AvgHealthScore float64 `gorm:"default:0" json:"avg_health_score"`

	// Request Metrics
	TotalRequests  int64   `gorm:"default:0" json:"total_requests"`
	FailedRequests int64   `gorm:"default:0" json:"failed_requests"`
	SuccessRate    float64 `gorm:"default:0" json:"success_rate"`

	// Performance
	AvgLatency int64 `gorm:"default:0" json:"avg_latency"`
	P95Latency int64 `gorm:"default:0" json:"p95_latency"`

	// Token and Cost
	TotalTokens int64   `gorm:"default:0" json:"total_tokens"`
	TotalCost   float64 `gorm:"default:0" json:"total_cost"`

	// Cache Stats
	CacheHits    int64   `gorm:"default:0" json:"cache_hits"`
	CacheMisses  int64   `gorm:"default:0" json:"cache_misses"`
	CacheHitRate float64 `gorm:"default:0" json:"cache_hit_rate"`
}

// UserMetrics stores user-level aggregated metrics
type UserMetrics struct {
	BaseModel
	UserID    uuid.UUID      `gorm:"not null;index:idx_user_metrics_lookup" json:"user_id"`
	Interval  MetricInterval `gorm:"not null;index:idx_user_metrics_lookup" json:"interval"`
	Timestamp time.Time      `gorm:"not null;index:idx_user_metrics_lookup" json:"timestamp"`

	// Request Metrics
	TotalRequests int64   `gorm:"default:0" json:"total_requests"`
	TotalTokens   int64   `gorm:"default:0" json:"total_tokens"`
	TotalCost     float64 `gorm:"default:0" json:"total_cost"`

	// Usage by context
	UserRequests int64 `gorm:"default:0" json:"user_requests"` // Direct user key usage
	TeamRequests int64 `gorm:"default:0" json:"team_requests"` // Team key usage

	// Model Distribution (JSON field for flexibility)
	ModelUsage string `gorm:"type:jsonb" json:"model_usage"` // {"gpt-4": 100, "gpt-3.5": 50}
}

// TeamMetrics stores team-level aggregated metrics
type TeamMetrics struct {
	BaseModel
	TeamID    uuid.UUID      `gorm:"not null;index:idx_team_metrics_lookup" json:"team_id"`
	Interval  MetricInterval `gorm:"not null;index:idx_team_metrics_lookup" json:"interval"`
	Timestamp time.Time      `gorm:"not null;index:idx_team_metrics_lookup" json:"timestamp"`

	// Request Metrics
	TotalRequests int64   `gorm:"default:0" json:"total_requests"`
	TotalTokens   int64   `gorm:"default:0" json:"total_tokens"`
	TotalCost     float64 `gorm:"default:0" json:"total_cost"`

	// Member Activity
	ActiveMembers int `gorm:"default:0" json:"active_members"`
	TotalMembers  int `gorm:"default:0" json:"total_members"`

	// Budget Status
	CurrentSpend float64 `gorm:"default:0" json:"current_spend"`
	BudgetUsed   float64 `gorm:"default:0" json:"budget_used"` // Percentage

	// Model Distribution (JSON field for flexibility)
	ModelUsage string `gorm:"type:jsonb" json:"model_usage"` // {"gpt-4": 100, "gpt-3.5": 50}
}

// BeforeCreate sets the ID for metrics
func (m *ModelMetrics) BeforeCreate(tx *gorm.DB) error {
	m.ID = uuid.New()
	return nil
}

func (s *SystemMetrics) BeforeCreate(tx *gorm.DB) error {
	s.ID = uuid.New()
	return nil
}

func (u *UserMetrics) BeforeCreate(tx *gorm.DB) error {
	u.ID = uuid.New()
	return nil
}

func (t *TeamMetrics) BeforeCreate(tx *gorm.DB) error {
	t.ID = uuid.New()
	return nil
}

// Helper functions for querying historical data

// GetModelHealthHistory returns health data for heatmap
func GetModelHealthHistory(db *gorm.DB, days int) ([]ModelMetrics, error) {
	var metrics []ModelMetrics
	since := time.Now().AddDate(0, 0, -days)

	err := db.Where("interval = ? AND timestamp >= ?", IntervalDaily, since).
		Order("model_name ASC, timestamp ASC").
		Find(&metrics).Error

	return metrics, err
}

// GetSystemMetricsHistory returns system metrics for charts
func GetSystemMetricsHistory(db *gorm.DB, interval MetricInterval, since time.Time) ([]SystemMetrics, error) {
	var metrics []SystemMetrics

	err := db.Where("interval = ? AND timestamp >= ?", interval, since).
		Order("timestamp ASC").
		Find(&metrics).Error

	return metrics, err
}

// GetModelLatencyHistory returns latency data for specific models
func GetModelLatencyHistory(db *gorm.DB, modelNames []string, interval MetricInterval, since time.Time) ([]ModelMetrics, error) {
	var metrics []ModelMetrics

	err := db.Where("model_name IN ? AND interval = ? AND timestamp >= ?", modelNames, interval, since).
		Order("model_name ASC, timestamp ASC").
		Find(&metrics).Error

	return metrics, err
}
