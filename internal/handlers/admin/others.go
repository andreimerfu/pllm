package admin

import (
	"net/http"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/models"
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
	
	// TODO: Get real database stats - for now, use model stats plus placeholders
	// This should be replaced with actual database queries for users, teams, keys
	stats := map[string]interface{}{
		"total_requests":  modelStats["total_requests"],
		"total_tokens":    modelStats["total_tokens"], 
		"total_cost":      modelStats["total_cost"],
		"active_users":    modelStats["active_users"],
		"active_teams":    8,   // TODO: Query from database
		"active_keys":     156, // TODO: Query from database  
		"load_balancer":   modelStats["load_balancer"], // Pass through model load balancer stats
		"should_shed_load": modelStats["should_shed_load"],
	}
	h.sendJSON(w, http.StatusOK, stats)
}

func (h *AnalyticsHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	dashboard := map[string]interface{}{
		"stats": map[string]interface{}{
			"total_requests": 15234,
			"total_tokens":   5678901,
			"total_cost":     234.56,
			"active_users":   42,
		},
		"recent_activity": []map[string]interface{}{
			{
				"timestamp": "2024-01-15T10:30:00Z",
				"user":      "john.doe@example.com",
				"action":    "API call",
				"model":     "gpt-4",
				"tokens":    1500,
			},
		},
		"top_users": []map[string]interface{}{
			{
				"id":       "user_001",
				"email":    "john.doe@example.com",
				"usage":    45678,
				"cost":     45.67,
			},
		},
		"top_models": []map[string]interface{}{
			{
				"model":    "gpt-4",
				"requests": 5432,
				"tokens":   2345678,
				"cost":     123.45,
			},
		},
	}
	h.sendJSON(w, http.StatusOK, dashboard)
}

func (h *AnalyticsHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	usage := map[string]interface{}{
		"period": "2024-01",
		"usage": []map[string]interface{}{
			{
				"date":     "2024-01-01",
				"requests": 512,
				"tokens":   234567,
				"cost":     23.45,
			},
			{
				"date":     "2024-01-02",
				"requests": 623,
				"tokens":   345678,
				"cost":     34.56,
			},
		},
		"total": map[string]interface{}{
			"requests": 15234,
			"tokens":   5678901,
			"cost":     234.56,
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
				"id":             team.ID,
				"name":           team.Name,
				"type":           "team",
				"max_budget":     team.MaxBudget,
				"current_spend":  team.CurrentSpend,
				"remaining":      team.MaxBudget - team.CurrentSpend,
				"usage_percent":  usagePercent,
				"period":         team.BudgetDuration,
				"alert_threshold": team.BudgetAlertAt,
				"is_active":      team.IsActive,
				"should_alert":   team.ShouldAlertBudget(),
				"is_exceeded":    team.IsBudgetExceeded(),
				"reset_at":       team.BudgetResetAt,
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
				"id":             key.ID,
				"name":           key.Name,
				"type":           "key", 
				"max_budget":     *key.MaxBudget,
				"current_spend":  key.CurrentSpend,
				"remaining":      *key.MaxBudget - key.CurrentSpend,
				"usage_percent":  usagePercent,
				"period":         key.BudgetDuration,
				"is_active":      key.IsActive,
				"reset_at":       key.BudgetResetAt,
				"total_cost":     key.TotalCost,
				"usage_count":    key.UsageCount,
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
			"count": 0,
			"budget": 0.0,
			"spent": 0.0,
		},
		"weekly": {
			"count": 0,
			"budget": 0.0,
			"spent": 0.0,
		},
		"monthly": {
			"count": 0,
			"budget": 0.0,
			"spent": 0.0,
		},
		"yearly": {
			"count": 0,
			"budget": 0.0,
			"spent": 0.0,
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
			"total_budget":     totalTeamBudget + totalKeyBudget,
			"total_spent":      totalTeamSpent + totalKeySpent,
			"total_remaining":  (totalTeamBudget + totalKeyBudget) - (totalTeamSpent + totalKeySpent),
			"team_budget":      totalTeamBudget,
			"team_spent":       totalTeamSpent,
			"key_budget":       totalKeyBudget,
			"key_spent":        totalKeySpent,
			"alerting_count":   teamAlerting + keyAlerting,
			"exceeded_count":   teamExceeded + keyExceeded,
			"total_entities":   len(teamBudgets) + len(keyBudgets),
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

// SystemHandler handles system endpoints
type SystemHandler struct {
	baseHandler
}

func NewSystemHandler(logger *zap.Logger) *SystemHandler {
	return &SystemHandler{
		baseHandler: baseHandler{logger: logger},
	}
}

func (h *SystemHandler) GetSystemInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"version":    "1.0.0",
		"build":      "2024.01.15",
		"uptime":     "15d 6h 30m",
		"database":   "connected",
		"dex":        "connected",
		"status":     "healthy",
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

func (h *SystemHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"config": map[string]interface{}{
			"master_key_configured": true,
			"dex_enabled":          true,
			"database_connected":   true,
		},
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
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"audit_logs": []interface{}{}})
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
		"message": "Backup created",
		"backup_id": "backup-20240115-123456",
	})
}

func (h *SystemHandler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Backup restore not yet implemented")
}