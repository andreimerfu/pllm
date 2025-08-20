package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/amerfu/pllm/internal/database"
	"github.com/amerfu/pllm/internal/services/cache"
)

type HealthResponse struct {
	Status   string                   `json:"status"`
	Services map[string]ServiceHealth `json:"services"`
}

type ServiceHealth struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func Health(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:   "ok",
		Services: make(map[string]ServiceHealth),
	}

	// Check database
	if database.IsHealthy() {
		response.Services["database"] = ServiceHealth{Status: "healthy"}
	} else {
		response.Services["database"] = ServiceHealth{Status: "unhealthy", Message: "Database connection failed"}
		response.Status = "degraded"
	}

	// Check cache
	if cache.IsHealthy() {
		response.Services["cache"] = ServiceHealth{Status: "healthy"}
	} else {
		response.Services["cache"] = ServiceHealth{Status: "unhealthy", Message: "Cache connection failed"}
		response.Status = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	if response.Status == "ok" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(response)
}

func Ready(w http.ResponseWriter, r *http.Request) {
	if !database.IsHealthy() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "not_ready",
			"error":  "Database not ready",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
	})
}
