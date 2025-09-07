package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/services/guardrails"
)

// GuardrailsHandler handles guardrails admin endpoints
type GuardrailsHandler struct {
	logger   *zap.Logger
	config   *config.Config
	executor *guardrails.Executor
}

// NewGuardrailsHandler creates a new guardrails admin handler
func NewGuardrailsHandler(logger *zap.Logger, config *config.Config, executor *guardrails.Executor) *GuardrailsHandler {
	return &GuardrailsHandler{
		logger:   logger.Named("admin_guardrails"),
		config:   config,
		executor: executor,
	}
}

// GuardrailInfo represents guardrail information for the API
type GuardrailInfo struct {
	Name        string                 `json:"name"`
	Provider    string                 `json:"provider"`
	Mode        []string               `json:"mode"`
	Enabled     bool                   `json:"enabled"`
	DefaultOn   bool                   `json:"default_on"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Stats       *GuardrailStatsInfo    `json:"stats,omitempty"`
	Healthy     bool                   `json:"healthy"`
}

// GuardrailStatsInfo represents guardrail statistics
type GuardrailStatsInfo struct {
	TotalExecutions int64         `json:"total_executions"`
	TotalPassed     int64         `json:"total_passed"`
	TotalBlocked    int64         `json:"total_blocked"`
	TotalErrors     int64         `json:"total_errors"`
	AverageLatency  int64         `json:"average_latency"` // Average latency in milliseconds
	LastExecuted    time.Time     `json:"last_executed"`
	BlockRate       float64       `json:"block_rate"`
	ErrorRate       float64       `json:"error_rate"`
}

// ListGuardrails returns all configured guardrails
// @Summary List all guardrails
// @Description Get all configured guardrails with their status and statistics
// @Tags Admin, Guardrails
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/admin/guardrails [get]
func (h *GuardrailsHandler) ListGuardrails(w http.ResponseWriter, r *http.Request) {
	var guardrailInfos []GuardrailInfo

	// Get guardrails from config
	for _, railConfig := range h.config.Guardrails.Guardrails {
		info := GuardrailInfo{
			Name:      railConfig.Name,
			Provider:  railConfig.Provider,
			Mode:      railConfig.Mode,
			Enabled:   railConfig.Enabled,
			DefaultOn: railConfig.DefaultOn,
			Config:    railConfig.Config,
			Healthy:   false, // Default to false
		}

		// Get statistics if executor is available
		if h.executor != nil {
			stats := h.executor.GetStats()
			if railStats, exists := stats[railConfig.Name]; exists {
				info.Stats = &GuardrailStatsInfo{
					TotalExecutions: railStats.TotalExecutions,
					TotalPassed:     railStats.TotalPassed,
					TotalBlocked:    railStats.TotalBlocked,
					TotalErrors:     railStats.TotalErrors,
					AverageLatency:  int64(railStats.AverageLatency / time.Millisecond),
					LastExecuted:    railStats.LastExecuted,
				}

				// Calculate rates
				if railStats.TotalExecutions > 0 {
					info.Stats.BlockRate = float64(railStats.TotalBlocked) / float64(railStats.TotalExecutions)
					info.Stats.ErrorRate = float64(railStats.TotalErrors) / float64(railStats.TotalExecutions)
				}
			}

			// Check health
			healthResults := h.executor.HealthCheck(r.Context())
			if healthErr, exists := healthResults[railConfig.Name]; exists {
				info.Healthy = healthErr == nil
			}
		}

		guardrailInfos = append(guardrailInfos, info)
	}

	response := map[string]interface{}{
		"guardrails": guardrailInfos,
		"enabled":    h.config.Guardrails.Enabled,
		"count":      len(guardrailInfos),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode guardrails response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetGuardrail returns information about a specific guardrail
// @Summary Get guardrail details
// @Description Get detailed information about a specific guardrail
// @Tags Admin, Guardrails
// @Accept json
// @Produce json
// @Param name path string true "Guardrail name"
// @Success 200 {object} GuardrailInfo
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/admin/guardrails/{name} [get]
func (h *GuardrailsHandler) GetGuardrail(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		http.Error(w, "Guardrail name is required", http.StatusBadRequest)
		return
	}

	// Find guardrail config
	var railConfig *config.GuardrailConfig
	for _, gc := range h.config.Guardrails.Guardrails {
		if gc.Name == name {
			railConfig = &gc
			break
		}
	}

	if railConfig == nil {
		http.Error(w, "Guardrail not found", http.StatusNotFound)
		return
	}

	info := GuardrailInfo{
		Name:      railConfig.Name,
		Provider:  railConfig.Provider,
		Mode:      railConfig.Mode,
		Enabled:   railConfig.Enabled,
		DefaultOn: railConfig.DefaultOn,
		Config:    railConfig.Config,
		Healthy:   false,
	}

	// Get statistics if executor is available
	if h.executor != nil {
		stats := h.executor.GetStats()
		if railStats, exists := stats[name]; exists {
			info.Stats = &GuardrailStatsInfo{
				TotalExecutions: railStats.TotalExecutions,
				TotalPassed:     railStats.TotalPassed,
				TotalBlocked:    railStats.TotalBlocked,
				TotalErrors:     railStats.TotalErrors,
				AverageLatency:  int64(railStats.AverageLatency / time.Millisecond),
				LastExecuted:    railStats.LastExecuted,
			}

			// Calculate rates
			if railStats.TotalExecutions > 0 {
				info.Stats.BlockRate = float64(railStats.TotalBlocked) / float64(railStats.TotalExecutions)
				info.Stats.ErrorRate = float64(railStats.TotalErrors) / float64(railStats.TotalExecutions)
			}
		}

		// Check health
		healthResults := h.executor.HealthCheck(r.Context())
		if healthErr, exists := healthResults[name]; exists {
			info.Healthy = healthErr == nil
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		h.logger.Error("Failed to encode guardrail response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetGuardrailStats returns statistics for all guardrails
// @Summary Get guardrail statistics
// @Description Get execution statistics for all guardrails
// @Tags Admin, Guardrails
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/admin/guardrails/stats [get]
func (h *GuardrailsHandler) GetGuardrailStats(w http.ResponseWriter, r *http.Request) {
	if h.executor == nil {
		response := map[string]interface{}{
			"error":   "Guardrails executor not available",
			"enabled": h.config.Guardrails.Enabled,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}

	stats := h.executor.GetStats()
	statsInfo := make(map[string]GuardrailStatsInfo)

	for name, railStats := range stats {
		info := GuardrailStatsInfo{
			TotalExecutions: railStats.TotalExecutions,
			TotalPassed:     railStats.TotalPassed,
			TotalBlocked:    railStats.TotalBlocked,
			TotalErrors:     railStats.TotalErrors,
			AverageLatency:  int64(railStats.AverageLatency / time.Millisecond),
			LastExecuted:    railStats.LastExecuted,
		}

		// Calculate rates
		if railStats.TotalExecutions > 0 {
			info.BlockRate = float64(railStats.TotalBlocked) / float64(railStats.TotalExecutions)
			info.ErrorRate = float64(railStats.TotalErrors) / float64(railStats.TotalExecutions)
		}

		statsInfo[name] = info
	}

	response := map[string]interface{}{
		"stats":   statsInfo,
		"enabled": h.config.Guardrails.Enabled,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode guardrail stats response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// CheckGuardrailHealth performs health checks on all guardrails
// @Summary Check guardrail health
// @Description Perform health checks on all configured guardrails
// @Tags Admin, Guardrails
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/admin/guardrails/health [get]
func (h *GuardrailsHandler) CheckGuardrailHealth(w http.ResponseWriter, r *http.Request) {
	if h.executor == nil {
		response := map[string]interface{}{
			"error":   "Guardrails executor not available",
			"enabled": h.config.Guardrails.Enabled,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	healthResults := h.executor.HealthCheck(ctx)
	healthInfo := make(map[string]interface{})
	allHealthy := true

	for name, err := range healthResults {
		if err != nil {
			healthInfo[name] = map[string]interface{}{
				"healthy": false,
				"error":   err.Error(),
			}
			allHealthy = false
		} else {
			healthInfo[name] = map[string]interface{}{
				"healthy": true,
			}
		}
	}

	response := map[string]interface{}{
		"health":     healthInfo,
		"all_healthy": allHealthy,
		"enabled":    h.config.Guardrails.Enabled,
		"checked_at": time.Now(),
	}

	statusCode := http.StatusOK
	if !allHealthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode health check response", zap.Error(err))
	}
}

// TestGuardrail allows testing a guardrail with sample input
// @Summary Test guardrail
// @Description Test a guardrail with sample input text
// @Tags Admin, Guardrails
// @Accept json
// @Produce json
// @Param name path string true "Guardrail name"
// @Param request body map[string]interface{} true "Test request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/admin/guardrails/{name}/test [post]
func (h *GuardrailsHandler) TestGuardrail(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		http.Error(w, "Guardrail name is required", http.StatusBadRequest)
		return
	}

	if h.executor == nil {
		http.Error(w, "Guardrails executor not available", http.StatusServiceUnavailable)
		return
	}

	// Parse test request
	var testRequest struct {
		Text     string                 `json:"text"`
		Messages []map[string]interface{} `json:"messages,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&testRequest); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// For now, return a placeholder response
	// TODO: Implement actual test functionality with the guardrail
	response := map[string]interface{}{
		"message":     "Guardrail testing endpoint created",
		"guardrail":   name,
		"test_input":  testRequest.Text,
		"status":      "not_implemented",
		"note":        "Full testing implementation requires additional guardrail interface methods",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode test response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}