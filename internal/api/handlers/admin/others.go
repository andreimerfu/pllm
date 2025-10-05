package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/monitoring/audit"
)

// AnalyticsHandler handles analytics endpoints
type AnalyticsHandler struct {
	baseHandler
	db           *gorm.DB
	modelManager interface {
		GetModelStats() map[string]interface{}
	}
}

func NewAnalyticsHandler(logger *zap.Logger, db *gorm.DB, modelManager interface {
	GetModelStats() map[string]interface{}
}) *AnalyticsHandler {
	return &AnalyticsHandler{
		baseHandler:  baseHandler{logger: logger},
		db:           db,
		modelManager: modelManager,
	}
}

func (h *AnalyticsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	// Get real statistics from the model manager first
	modelStats := h.modelManager.GetModelStats()

	// Get real database stats
	var activeTeams int64
	var activeKeys int64
	h.db.Model(&models.Team{}).Where("is_active = true").Count(&activeTeams)
	h.db.Model(&models.Key{}).Where("is_active = true").Count(&activeKeys)

	stats := map[string]interface{}{
		"total_requests":   modelStats["total_requests"],
		"total_tokens":     modelStats["total_tokens"],
		"total_cost":       modelStats["total_cost"],
		"active_users":     modelStats["active_users"],
		"active_teams":     activeTeams,
		"active_keys":      activeKeys,
		"load_balancer":    modelStats["load_balancer"], // Pass through model load balancer stats
		"should_shed_load": modelStats["should_shed_load"],
	}
	h.sendJSON(w, http.StatusOK, stats)
}

func (h *AnalyticsHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	// Get real statistics from the model manager
	modelStats := h.modelManager.GetModelStats()

	// Get real database stats for active counts
	var activeTeams int64
	var activeKeys int64
	h.db.Model(&models.Team{}).Where("is_active = true").Count(&activeTeams)
	h.db.Model(&models.Key{}).Where("is_active = true").Count(&activeKeys)

	// Get top users by spending from usage logs
	var topUsers []struct {
		ActualUserID  string  `gorm:"column:actual_user_id"`
		UserEmail     string  `gorm:"column:user_email"`
		TotalRequests int64   `gorm:"column:total_requests"`
		TotalTokens   int64   `gorm:"column:total_tokens"`
		TotalCost     float64 `gorm:"column:total_cost"`
	}

	h.db.Raw(`
		SELECT 
			ul.actual_user_id,
			COALESCE(u.email, 'unknown') as user_email,
			COUNT(*) as total_requests,
			SUM(ul.total_tokens) as total_tokens,
			SUM(ul.total_cost) as total_cost
		FROM usage_logs ul
		LEFT JOIN users u ON ul.actual_user_id = u.id::text
		WHERE ul.created_at >= DATE_TRUNC('month', NOW())
		GROUP BY ul.actual_user_id, u.email
		ORDER BY total_cost DESC
		LIMIT 10
	`).Scan(&topUsers)

	// Get top models from usage logs
	var topModels []struct {
		Model         string  `gorm:"column:model"`
		TotalRequests int64   `gorm:"column:total_requests"`
		TotalTokens   int64   `gorm:"column:total_tokens"`
		TotalCost     float64 `gorm:"column:total_cost"`
	}

	h.db.Raw(`
		SELECT 
			model,
			COUNT(*) as total_requests,
			SUM(total_tokens) as total_tokens,
			SUM(total_cost) as total_cost
		FROM usage_logs
		WHERE created_at >= DATE_TRUNC('month', NOW())
		GROUP BY model
		ORDER BY total_cost DESC
		LIMIT 10
	`).Scan(&topModels)

	// Get recent activity
	var recentActivity []struct {
		Timestamp time.Time `gorm:"column:timestamp"`
		UserEmail string    `gorm:"column:user_email"`
		Model     string    `gorm:"column:model"`
		Tokens    int64     `gorm:"column:tokens"`
	}

	h.db.Raw(`
		SELECT 
			ul.created_at as timestamp,
			COALESCE(u.email, 'unknown') as user_email,
			ul.model,
			ul.total_tokens as tokens
		FROM usage_logs ul
		LEFT JOIN users u ON ul.actual_user_id = u.id::text
		ORDER BY ul.created_at DESC
		LIMIT 10
	`).Scan(&recentActivity)

	// Format data for response
	topUsersArray := make([]map[string]interface{}, 0)
	for _, user := range topUsers {
		topUsersArray = append(topUsersArray, map[string]interface{}{
			"id":    user.ActualUserID,
			"email": user.UserEmail,
			"usage": user.TotalTokens,
			"cost":  user.TotalCost,
		})
	}

	topModelsArray := make([]map[string]interface{}, 0)
	for _, model := range topModels {
		topModelsArray = append(topModelsArray, map[string]interface{}{
			"model":    model.Model,
			"requests": model.TotalRequests,
			"tokens":   model.TotalTokens,
			"cost":     model.TotalCost,
		})
	}

	recentActivityArray := make([]map[string]interface{}, 0)
	for _, activity := range recentActivity {
		recentActivityArray = append(recentActivityArray, map[string]interface{}{
			"timestamp": activity.Timestamp.Format(time.RFC3339),
			"user":      activity.UserEmail,
			"action":    "API call",
			"model":     activity.Model,
			"tokens":    activity.Tokens,
		})
	}

	dashboard := map[string]interface{}{
		"stats": map[string]interface{}{
			"total_requests": modelStats["total_requests"],
			"total_tokens":   modelStats["total_tokens"],
			"total_cost":     modelStats["total_cost"],
			"active_users":   modelStats["active_users"],
		},
		"recent_activity": recentActivityArray,
		"top_users":       topUsersArray,
		"top_models":      topModelsArray,
	}
	h.sendJSON(w, http.StatusOK, dashboard)
}

func (h *AnalyticsHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	// Get daily usage for the current month
	var dailyUsage []struct {
		Date     time.Time `gorm:"column:date"`
		Requests int64     `gorm:"column:requests"`
		Tokens   int64     `gorm:"column:tokens"`
		Cost     float64   `gorm:"column:cost"`
	}

	h.db.Raw(`
		SELECT 
			DATE_TRUNC('day', created_at) as date,
			COUNT(*) as requests,
			SUM(total_tokens) as tokens,
			SUM(total_cost) as cost
		FROM usage_logs
		WHERE created_at >= DATE_TRUNC('month', NOW())
		GROUP BY DATE_TRUNC('day', created_at)
		ORDER BY date
	`).Scan(&dailyUsage)

	// Get total usage for the period
	var totalUsage struct {
		Requests int64   `gorm:"column:requests"`
		Tokens   int64   `gorm:"column:tokens"`
		Cost     float64 `gorm:"column:cost"`
	}

	h.db.Raw(`
		SELECT 
			COUNT(*) as requests,
			SUM(total_tokens) as tokens,
			SUM(total_cost) as cost
		FROM usage_logs
		WHERE created_at >= DATE_TRUNC('month', NOW())
	`).Scan(&totalUsage)

	// Format daily usage for response
	usageArray := make([]map[string]interface{}, 0)
	for _, day := range dailyUsage {
		usageArray = append(usageArray, map[string]interface{}{
			"date":     day.Date.Format("2006-01-02"),
			"requests": day.Requests,
			"tokens":   day.Tokens,
			"cost":     day.Cost,
		})
	}

	usage := map[string]interface{}{
		"period": time.Now().Format("2006-01"),
		"usage":  usageArray,
		"total": map[string]interface{}{
			"requests": totalUsage.Requests,
			"tokens":   totalUsage.Tokens,
			"cost":     totalUsage.Cost,
		},
	}
	h.sendJSON(w, http.StatusOK, usage)
}

func (h *AnalyticsHandler) GetHourlyUsage(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"usage": []interface{}{}})
}

func (h *AnalyticsHandler) GetDailyUsage(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"usage": []interface{}{}})
}

func (h *AnalyticsHandler) GetMonthlyUsage(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"usage": []interface{}{}})
}

func (h *AnalyticsHandler) GetCosts(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"costs": []interface{}{}})
}

func (h *AnalyticsHandler) GetCostBreakdown(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"breakdown": []interface{}{}})
}

func (h *AnalyticsHandler) GetPerformance(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"performance": []interface{}{}})
}

// GetHistoricalModelHealth returns historical model health data for heatmap
func (h *AnalyticsHandler) GetHistoricalModelHealth(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 90 {
			days = parsed
		}
	}

	metrics, err := models.GetModelHealthHistory(h.db, days)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to fetch historical model health data")
		return
	}

	// Group by model and format for frontend
	modelData := make(map[string][]map[string]interface{})
	for _, metric := range metrics {
		modelName := metric.ModelName
		if _, exists := modelData[modelName]; !exists {
			modelData[modelName] = make([]map[string]interface{}, 0)
		}

		modelData[modelName] = append(modelData[modelName], map[string]interface{}{
			"date":         metric.Timestamp.Format("2006-01-02"),
			"health_score": metric.HealthScore,
			"requests":     metric.TotalRequests,
			"failed":       metric.FailedRequests,
			"avg_latency":  metric.AvgLatency,
		})
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"models": modelData,
		"period": fmt.Sprintf("%d days", days),
	})
}

// GetHistoricalSystemMetrics returns historical system metrics for charts
func (h *AnalyticsHandler) GetHistoricalSystemMetrics(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	intervalStr := r.URL.Query().Get("interval")
	if intervalStr == "" {
		intervalStr = "hourly"
	}

	interval := models.IntervalHourly
	if intervalStr == "daily" {
		interval = models.IntervalDaily
	}

	hoursStr := r.URL.Query().Get("hours")
	hours := 24 // Default to 24 hours
	if h := hoursStr; h != "" {
		if parsed, err := strconv.Atoi(h); err == nil && parsed > 0 && parsed <= 720 { // Max 30 days
			hours = parsed
		}
	}

	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	metrics, err := models.GetSystemMetricsHistory(h.db, interval, since)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to fetch historical system metrics")
		return
	}

	// Format for charts
	timestamps := make([]string, 0)
	requests := make([]int64, 0)
	latencies := make([]int64, 0)
	healthScores := make([]float64, 0)
	costs := make([]float64, 0)

	for _, metric := range metrics {
		timestamps = append(timestamps, metric.Timestamp.Format("2006-01-02 15:04"))
		requests = append(requests, metric.TotalRequests)
		latencies = append(latencies, metric.AvgLatency)
		healthScores = append(healthScores, metric.AvgHealthScore)
		costs = append(costs, metric.TotalCost)
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"interval":      intervalStr,
		"period_hours":  hours,
		"timestamps":    timestamps,
		"requests":      requests,
		"latencies":     latencies,
		"health_scores": healthScores,
		"costs":         costs,
	})
}

// GetHistoricalModelLatencies returns historical latency data for specific models
func (h *AnalyticsHandler) GetHistoricalModelLatencies(w http.ResponseWriter, r *http.Request) {
	// Parse models parameter
	modelsParam := r.URL.Query().Get("models")
	if modelsParam == "" {
		h.sendError(w, http.StatusBadRequest, "models parameter is required")
		return
	}

	modelNames := strings.Split(modelsParam, ",")
	for i, name := range modelNames {
		modelNames[i] = strings.TrimSpace(name)
	}

	// Parse other parameters
	intervalStr := r.URL.Query().Get("interval")
	if intervalStr == "" {
		intervalStr = "hourly"
	}

	interval := models.IntervalHourly
	if intervalStr == "daily" {
		interval = models.IntervalDaily
	}

	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if h := hoursStr; h != "" {
		if parsed, err := strconv.Atoi(h); err == nil && parsed > 0 && parsed <= 720 {
			hours = parsed
		}
	}

	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	metrics, err := models.GetModelLatencyHistory(h.db, modelNames, interval, since)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to fetch model latency data")
		return
	}

	// Group by model
	modelLatencies := make(map[string][]map[string]interface{})
	for _, metric := range metrics {
		modelName := metric.ModelName
		if _, exists := modelLatencies[modelName]; !exists {
			modelLatencies[modelName] = make([]map[string]interface{}, 0)
		}

		modelLatencies[modelName] = append(modelLatencies[modelName], map[string]interface{}{
			"timestamp":   metric.Timestamp.Format("2006-01-02 15:04"),
			"avg_latency": metric.AvgLatency,
			"p95_latency": metric.P95Latency,
			"p99_latency": metric.P99Latency,
			"requests":    metric.TotalRequests,
		})
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"models":       modelLatencies,
		"interval":     intervalStr,
		"period_hours": hours,
	})
}

func (h *AnalyticsHandler) GetErrors(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"errors": []interface{}{}})
}

func (h *AnalyticsHandler) GetCacheStats(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"cache": map[string]interface{}{"hits": 0, "misses": 0}})
}

// GetBudgetSummary returns budget analytics from teams and keys
func (h *AnalyticsHandler) GetBudgetSummary(w http.ResponseWriter, r *http.Request) {
	var teams []models.Team
	var keys []models.Key

	// Get all teams with budget data
	if err := h.db.Find(&teams).Error; err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to fetch team budget data")
		return
	}

	// Get all keys with budget data
	if err := h.db.Find(&keys).Error; err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to fetch key budget data")
		return
	}

	// Calculate team budget summary
	teamBudgets := make([]map[string]interface{}, 0)
	totalTeamBudget := 0.0
	totalTeamSpent := 0.0
	teamAlerting := 0
	teamExceeded := 0

	for _, team := range teams {
		if team.MaxBudget > 0 {
			usagePercent := 0.0
			if team.MaxBudget > 0 {
				usagePercent = (team.CurrentSpend / team.MaxBudget) * 100
			}

			teamBudgets = append(teamBudgets, map[string]interface{}{
				"id":              team.ID,
				"name":            team.Name,
				"type":            "team",
				"max_budget":      team.MaxBudget,
				"current_spend":   team.CurrentSpend,
				"remaining":       team.MaxBudget - team.CurrentSpend,
				"usage_percent":   usagePercent,
				"period":          team.BudgetDuration,
				"alert_threshold": team.BudgetAlertAt,
				"is_active":       team.IsActive,
				"should_alert":    team.ShouldAlertBudget(),
				"is_exceeded":     team.IsBudgetExceeded(),
				"reset_at":        team.BudgetResetAt,
			})

			totalTeamBudget += team.MaxBudget
			totalTeamSpent += team.CurrentSpend

			if team.ShouldAlertBudget() {
				teamAlerting++
			}
			if team.IsBudgetExceeded() {
				teamExceeded++
			}
		}
	}

	// Calculate key budget summary
	keyBudgets := make([]map[string]interface{}, 0)
	totalKeyBudget := 0.0
	totalKeySpent := 0.0
	keyAlerting := 0
	keyExceeded := 0

	for _, key := range keys {
		if key.MaxBudget != nil && *key.MaxBudget > 0 {
			usagePercent := 0.0
			if *key.MaxBudget > 0 {
				usagePercent = (key.CurrentSpend / *key.MaxBudget) * 100
			}

			keyBudgets = append(keyBudgets, map[string]interface{}{
				"id":            key.ID,
				"name":          key.Name,
				"type":          "key",
				"max_budget":    *key.MaxBudget,
				"current_spend": key.CurrentSpend,
				"remaining":     *key.MaxBudget - key.CurrentSpend,
				"usage_percent": usagePercent,
				"period":        key.BudgetDuration,
				"is_active":     key.IsActive,
				"reset_at":      key.BudgetResetAt,
				"total_cost":    key.TotalCost,
				"usage_count":   key.UsageCount,
			})

			totalKeyBudget += *key.MaxBudget
			totalKeySpent += key.CurrentSpend

			// Keys don't have alert thresholds, but we can check if they're close to limit
			if usagePercent >= 80 {
				keyAlerting++
			}
			if key.CurrentSpend >= *key.MaxBudget {
				keyExceeded++
			}
		}
	}

	// Budget usage by period
	periodUsage := map[string]map[string]interface{}{
		"daily": {
			"count":  0,
			"budget": 0.0,
			"spent":  0.0,
		},
		"weekly": {
			"count":  0,
			"budget": 0.0,
			"spent":  0.0,
		},
		"monthly": {
			"count":  0,
			"budget": 0.0,
			"spent":  0.0,
		},
		"yearly": {
			"count":  0,
			"budget": 0.0,
			"spent":  0.0,
		},
	}

	for _, team := range teams {
		if team.MaxBudget > 0 {
			period := string(team.BudgetDuration)
			if usage, exists := periodUsage[period]; exists {
				usage["count"] = usage["count"].(int) + 1
				usage["budget"] = usage["budget"].(float64) + team.MaxBudget
				usage["spent"] = usage["spent"].(float64) + team.CurrentSpend
			}
		}
	}

	// Convert period usage to array
	periodArray := make([]map[string]interface{}, 0)
	for period, data := range periodUsage {
		if data["count"].(int) > 0 {
			periodArray = append(periodArray, map[string]interface{}{
				"period": period,
				"count":  data["count"],
				"budget": data["budget"],
				"spent":  data["spent"],
			})
		}
	}

	response := map[string]interface{}{
		"summary": map[string]interface{}{
			"total_budget":    totalTeamBudget + totalKeyBudget,
			"total_spent":     totalTeamSpent + totalKeySpent,
			"total_remaining": (totalTeamBudget + totalKeyBudget) - (totalTeamSpent + totalKeySpent),
			"team_budget":     totalTeamBudget,
			"team_spent":      totalTeamSpent,
			"key_budget":      totalKeyBudget,
			"key_spent":       totalKeySpent,
			"alerting_count":  teamAlerting + keyAlerting,
			"exceeded_count":  teamExceeded + keyExceeded,
			"total_entities":  len(teamBudgets) + len(keyBudgets),
		},
		"team_budgets":    teamBudgets,
		"key_budgets":     keyBudgets,
		"usage_by_period": periodArray,
		"charts": map[string]interface{}{
			"budget_distribution": []map[string]interface{}{
				{"name": "Teams", "value": totalTeamBudget},
				{"name": "Keys", "value": totalKeyBudget},
			},
			"spending_distribution": []map[string]interface{}{
				{"name": "Teams", "value": totalTeamSpent},
				{"name": "Keys", "value": totalKeySpent},
			},
		},
	}

	h.sendJSON(w, http.StatusOK, response)
}

// GetUserBreakdown returns detailed user analytics across teams and keys
func (h *AnalyticsHandler) GetUserBreakdown(w http.ResponseWriter, r *http.Request) {
	var usageStats []struct {
		ActualUserID  string  `gorm:"column:actual_user_id"`
		UserID        string  `gorm:"column:user_id"`
		KeyID         string  `gorm:"column:key_id"`
		TeamID        *string `gorm:"column:team_id"`
		TotalRequests int64   `gorm:"column:total_requests"`
		TotalTokens   int64   `gorm:"column:total_tokens"`
		TotalCost     float64 `gorm:"column:total_cost"`
	}

	// Get user breakdown from usage logs
	if err := h.db.Raw(`
		SELECT 
			actual_user_id,
			user_id,
			key_id,
			team_id,
			COUNT(*) as total_requests,
			SUM(total_tokens) as total_tokens,
			SUM(total_cost) as total_cost
		FROM usage_logs 
		WHERE created_at >= DATE_TRUNC('month', NOW())
		GROUP BY actual_user_id, user_id, key_id, team_id
		ORDER BY total_cost DESC
	`).Scan(&usageStats).Error; err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to get user breakdown data")
		return
	}

	// Get user details
	var users []models.User
	if err := h.db.Select("id, email, username, first_name, last_name").Find(&users).Error; err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to get user details")
		return
	}

	userMap := make(map[string]*models.User)
	for i := range users {
		userMap[users[i].ID.String()] = &users[i]
	}

	// Get team details
	var teams []models.Team
	if err := h.db.Select("id, name").Find(&teams).Error; err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to get team details")
		return
	}

	teamMap := make(map[string]*models.Team)
	for i := range teams {
		teamMap[teams[i].ID.String()] = &teams[i]
	}

	// Get key details
	var keys []models.Key
	if err := h.db.Select("id, name, team_id, user_id").Find(&keys).Error; err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to get key details")
		return
	}

	keyMap := make(map[string]*models.Key)
	for i := range keys {
		keyMap[keys[i].ID.String()] = &keys[i]
	}

	// Aggregate data by actual user
	userBreakdown := make(map[string]*models.UserStats)
	teamBreakdown := make(map[string]*models.TeamStats)

	for _, stat := range usageStats {
		// Get user info
		actualUser := userMap[stat.ActualUserID]
		if actualUser == nil {
			continue
		}

		// Initialize user stats if not exists
		userKey := stat.ActualUserID
		if _, exists := userBreakdown[userKey]; !exists {
			userBreakdown[userKey] = &models.UserStats{
				UserID:    actualUser.ID.String(),
				UserEmail: actualUser.Email,
				UserName:  actualUser.Username,
			}
		}

		userStats := userBreakdown[userKey]
		userStats.Requests += stat.TotalRequests
		userStats.Tokens += stat.TotalTokens
		userStats.Cost += stat.TotalCost

		// Track key usage
		key := keyMap[stat.KeyID]
		if key != nil {
			if stat.TeamID != nil && *stat.TeamID != "" {
				userStats.TeamRequests += stat.TotalRequests
				// Update team breakdown
				teamKey := *stat.TeamID
				if _, exists := teamBreakdown[teamKey]; !exists {
					team := teamMap[*stat.TeamID]
					if team != nil {
						teamBreakdown[teamKey] = &models.TeamStats{
							TeamID:        team.ID.String(),
							TeamName:      team.Name,
							UserBreakdown: make(map[string]*models.UserStats),
						}
					}
				}
				if teamStats, exists := teamBreakdown[teamKey]; exists {
					teamStats.Requests += stat.TotalRequests
					teamStats.Tokens += stat.TotalTokens
					teamStats.Cost += stat.TotalCost
					// Add user to team breakdown
					if _, userExists := teamStats.UserBreakdown[userKey]; !userExists {
						teamStats.UserBreakdown[userKey] = &models.UserStats{
							UserID:    actualUser.ID.String(),
							UserEmail: actualUser.Email,
							UserName:  actualUser.Username,
						}
					}
					teamUserStats := teamStats.UserBreakdown[userKey]
					teamUserStats.Requests += stat.TotalRequests
					teamUserStats.Tokens += stat.TotalTokens
					teamUserStats.Cost += stat.TotalCost
					teamUserStats.TeamRequests += stat.TotalRequests
				}
			} else {
				userStats.UserRequests += stat.TotalRequests
			}
		}
	}

	// Convert maps to slices for JSON response
	userArray := make([]*models.UserStats, 0, len(userBreakdown))
	for _, user := range userBreakdown {
		userArray = append(userArray, user)
	}

	teamArray := make([]*models.TeamStats, 0, len(teamBreakdown))
	for _, team := range teamBreakdown {
		// Count active members
		team.MemberCount = len(team.UserBreakdown)
		team.ActiveMembers = len(team.UserBreakdown) // All members in breakdown are active
		teamArray = append(teamArray, team)
	}

	response := map[string]interface{}{
		"user_breakdown": userArray,
		"team_breakdown": teamArray,
		"summary": map[string]interface{}{
			"total_users":        len(userArray),
			"total_active_teams": len(teamArray),
			"total_requests": func() int64 {
				total := int64(0)
				for _, user := range userArray {
					total += user.Requests
				}
				return total
			}(),
			"total_cost": func() float64 {
				total := 0.0
				for _, user := range userArray {
					total += user.Cost
				}
				return total
			}(),
		},
	}

	h.sendJSON(w, http.StatusOK, response)
}

// GetTeamUserBreakdown returns user analytics for a specific team
func (h *AnalyticsHandler) GetTeamUserBreakdown(w http.ResponseWriter, r *http.Request) {
	teamID := r.URL.Query().Get("team_id")
	if teamID == "" {
		h.sendError(w, http.StatusBadRequest, "team_id parameter is required")
		return
	}

	// Get team info
	var team models.Team
	if err := h.db.First(&team, "id = ?", teamID).Error; err != nil {
		h.sendError(w, http.StatusNotFound, "Team not found")
		return
	}

	// Get usage for this team
	var usageStats []struct {
		ActualUserID  string  `gorm:"column:actual_user_id"`
		KeyID         string  `gorm:"column:key_id"`
		TotalRequests int64   `gorm:"column:total_requests"`
		TotalTokens   int64   `gorm:"column:total_tokens"`
		TotalCost     float64 `gorm:"column:total_cost"`
	}

	if err := h.db.Raw(`
		SELECT 
			actual_user_id,
			key_id,
			COUNT(*) as total_requests,
			SUM(total_tokens) as total_tokens,
			SUM(total_cost) as total_cost
		FROM usage_logs 
		WHERE team_id = ? AND created_at >= DATE_TRUNC('month', NOW())
		GROUP BY actual_user_id, key_id
		ORDER BY total_cost DESC
	`, teamID).Scan(&usageStats).Error; err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to get team usage data")
		return
	}

	// Get user details for this team
	var users []models.User
	if err := h.db.Raw(`
		SELECT DISTINCT u.id, u.email, u.username, u.first_name, u.last_name
		FROM users u
		JOIN team_members tm ON u.id = tm.user_id
		WHERE tm.team_id = ?
	`, teamID).Scan(&users).Error; err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to get team members")
		return
	}

	userMap := make(map[string]*models.User)
	for i := range users {
		userMap[users[i].ID.String()] = &users[i]
	}

	// Aggregate usage by user
	userBreakdown := make(map[string]*models.UserStats)
	for _, stat := range usageStats {
		user := userMap[stat.ActualUserID]
		if user == nil {
			continue
		}

		userKey := stat.ActualUserID
		if _, exists := userBreakdown[userKey]; !exists {
			userBreakdown[userKey] = &models.UserStats{
				UserID:    user.ID.String(),
				UserEmail: user.Email,
				UserName:  user.Username,
			}
		}

		userStats := userBreakdown[userKey]
		userStats.Requests += stat.TotalRequests
		userStats.Tokens += stat.TotalTokens
		userStats.Cost += stat.TotalCost
		userStats.TeamRequests += stat.TotalRequests // All requests are team requests
	}

	userArray := make([]*models.UserStats, 0, len(userBreakdown))
	for _, user := range userBreakdown {
		userArray = append(userArray, user)
	}

	response := map[string]interface{}{
		"team": map[string]interface{}{
			"id":            team.ID,
			"name":          team.Name,
			"description":   team.Description,
			"max_budget":    team.MaxBudget,
			"current_spend": team.CurrentSpend,
		},
		"user_breakdown": userArray,
		"summary": map[string]interface{}{
			"total_members":  len(users),
			"active_members": len(userArray),
			"total_requests": func() int64 {
				total := int64(0)
				for _, user := range userArray {
					total += user.Requests
				}
				return total
			}(),
			"total_cost": func() float64 {
				total := 0.0
				for _, user := range userArray {
					total += user.Cost
				}
				return total
			}(),
		},
	}

	h.sendJSON(w, http.StatusOK, response)
}

// SystemHandler handles system endpoints
type SystemHandler struct {
	baseHandler
	config      *config.Config
	db          *gorm.DB
	auditLogger *audit.Logger
}

func NewSystemHandler(logger *zap.Logger, db *gorm.DB) *SystemHandler {
	return &SystemHandler{
		baseHandler: baseHandler{logger: logger},
		config:      config.Get(),
		db:          db,
		auditLogger: audit.NewLogger(db),
	}
}

func (h *SystemHandler) GetSystemInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"version":  "1.0.0",
		"build":    "2024.01.15",
		"uptime":   "15d 6h 30m",
		"database": "connected",
		"dex":      "connected",
		"status":   "healthy",
	}
	h.sendJSON(w, http.StatusOK, info)
}

func (h *SystemHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"status": "healthy",
		"checks": map[string]string{
			"database": "ok",
			"dex":      "ok",
			"cache":    "ok",
		},
	})
}

func (h *SystemHandler) GetAuthConfig(w http.ResponseWriter, r *http.Request) {
	authConfig := map[string]interface{}{
		"master_key_enabled": h.config.Auth.MasterKey != "",
		"dex_enabled":        h.config.Auth.Dex.Enabled,
		"available_providers": []string{},
	}
	
	// Add available OAuth providers from config
	if h.config.Auth.Dex.Enabled {
		authConfig["available_providers"] = h.config.Auth.Dex.EnabledProviders
		authConfig["dex_public_issuer"] = h.config.Auth.Dex.PublicIssuer
	}
	
	h.sendJSON(w, http.StatusOK, authConfig)
}


func (h *SystemHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	
	// Build router configuration including fallbacks
	routerConfig := map[string]interface{}{
		"routing_strategy":          cfg.Router.RoutingStrategy,
		"circuit_breaker_enabled":   cfg.Router.CircuitBreakerEnabled,
		"circuit_breaker_threshold": cfg.Router.CircuitBreakerThreshold,
		"circuit_breaker_cooldown":  cfg.Router.CircuitBreakerCooldown,
	}
	
	// Add fallbacks if they exist
	if len(cfg.Router.Fallbacks) > 0 {
		routerConfig["fallbacks"] = cfg.Router.Fallbacks
	}
	
	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"config": map[string]interface{}{
			"master_key_configured": cfg.Auth.MasterKey != "",
			"dex_enabled":           cfg.Auth.Dex.Enabled,
			"database_connected":    true,
		},
		"router": routerConfig,
	})
}

func (h *SystemHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Config update not yet implemented")
}

func (h *SystemHandler) GetSystemHealth(w http.ResponseWriter, r *http.Request) {
	h.GetHealth(w, r) // Reuse GetHealth
}

func (h *SystemHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"logs": []interface{}{}})
}

func (h *SystemHandler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for filtering
	query := r.URL.Query()
	
	// Parse pagination parameters with reasonable defaults
	limit := 50 // default limit
	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	// Ensure we never return more than 100 records at once
	if limit > 100 {
		limit = 100
	}
	
	offset := 0
	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	
	// Parse time range filters
	var startTime, endTime time.Time
	if start := query.Get("start_date"); start != "" {
		if parsed, err := time.Parse(time.RFC3339, start); err == nil {
			startTime = parsed
		}
	}
	if end := query.Get("end_date"); end != "" {
		if parsed, err := time.Parse(time.RFC3339, end); err == nil {
			endTime = parsed
		}
	}
	
	// Build filters
	filters := audit.AuditLogFilters{
		Action:    query.Get("action"),
		Resource:  query.Get("resource"),
		Result:    query.Get("result"),
		StartDate: startTime,
		EndDate:   endTime,
		Limit:     limit,
		Offset:    offset,
	}
	
	// Parse user_id filter if provided
	if userIDStr := query.Get("user_id"); userIDStr != "" {
		if userID, err := uuid.Parse(userIDStr); err == nil {
			filters.UserID = &userID
		}
	}
	
	// Parse team_id filter if provided
	if teamIDStr := query.Get("team_id"); teamIDStr != "" {
		if teamID, err := uuid.Parse(teamIDStr); err == nil {
			filters.TeamID = &teamID
		}
	}
	
	// Get audit logs with filters
	logs, total, err := h.auditLogger.GetAuditLogs(r.Context(), filters)
	if err != nil {
		h.logger.Error("Failed to fetch audit logs", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to fetch audit logs")
		return
	}
	
	// Build response
	response := map[string]interface{}{
		"audit_logs": logs,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
		"has_more":   int64(offset+len(logs)) < total,
	}
	
	h.sendJSON(w, http.StatusOK, response)
}

func (h *SystemHandler) ClearCache(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"message": "Cache cleared"})
}

func (h *SystemHandler) SetMaintenance(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"maintenance": false})
}

func (h *SystemHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"settings": map[string]interface{}{}})
}

func (h *SystemHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Settings update not yet implemented")
}

func (h *SystemHandler) GetRateLimits(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"rate_limits": map[string]interface{}{
			"tpm": 100000,
			"rpm": 100,
		},
	})
}

func (h *SystemHandler) UpdateRateLimits(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Rate limits update not yet implemented")
}

func (h *SystemHandler) GetCacheSettings(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"cache": map[string]interface{}{
			"enabled": true,
			"ttl":     3600,
		},
	})
}

func (h *SystemHandler) UpdateCacheSettings(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Cache settings update not yet implemented")
}

func (h *SystemHandler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "Backup created",
		"backup_id": "backup-20240115-123456",
	})
}

func (h *SystemHandler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Backup restore not yet implemented")
}
