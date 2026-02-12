package admin

import (
	"encoding/json"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/amerfu/pllm/internal/core/models"
	routeService "github.com/amerfu/pllm/internal/services/integrations/route"
	llmModels "github.com/amerfu/pllm/internal/services/llm/models"
	"github.com/amerfu/pllm/internal/services/llm/models/routing"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// RouteHandler handles CRUD operations for routes.
type RouteHandler struct {
	baseHandler
	db           *gorm.DB
	service      *routeService.Service
	modelManager *llmModels.ModelManager
}

// NewRouteHandler creates a new RouteHandler.
func NewRouteHandler(logger *zap.Logger, db *gorm.DB, modelManager *llmModels.ModelManager) *RouteHandler {
	return &RouteHandler{
		baseHandler:  baseHandler{logger: logger},
		db:           db,
		service:      routeService.NewService(db, logger),
		modelManager: modelManager,
	}
}

// routeResponse is the API response format for a route.
type routeResponse struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Slug           string               `json:"slug"`
	Description    string               `json:"description,omitempty"`
	Strategy       string               `json:"strategy"`
	Models         []routeModelResponse `json:"models"`
	FallbackModels []string             `json:"fallback_models,omitempty"`
	Enabled        bool                 `json:"enabled"`
	Source         string               `json:"source"`
	CreatedAt      string               `json:"created_at,omitempty"`
	UpdatedAt      string               `json:"updated_at,omitempty"`
}

type routeModelResponse struct {
	ID        string `json:"id,omitempty"`
	ModelName string `json:"model_name"`
	Weight    int    `json:"weight"`
	Priority  int    `json:"priority"`
	Enabled   bool   `json:"enabled"`
}

func toRouteResponse(r models.Route) routeResponse {
	resp := routeResponse{
		ID:             r.ID.String(),
		Name:           r.Name,
		Slug:           r.Slug,
		Description:    r.Description,
		Strategy:       r.Strategy,
		FallbackModels: []string(r.FallbackModels),
		Enabled:        r.Enabled,
		Source:         r.Source,
		CreatedAt:      r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      r.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if resp.FallbackModels == nil {
		resp.FallbackModels = []string{}
	}
	resp.Models = make([]routeModelResponse, 0, len(r.Models))
	for _, rm := range r.Models {
		resp.Models = append(resp.Models, routeModelResponse{
			ID:        rm.ID.String(),
			ModelName: rm.ModelName,
			Weight:    rm.Weight,
			Priority:  rm.Priority,
			Enabled:   rm.Enabled,
		})
	}
	return resp
}

// ListRoutes returns all routes (system from config + user from DB).
func (h *RouteHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	var allRoutes []routeResponse

	// Collect DB route slugs first so we can skip them from the model manager
	dbSlugs := make(map[string]bool)
	var dbRoutes []models.Route
	if h.db != nil {
		var err error
		dbRoutes, err = h.service.List()
		if err != nil {
			h.logger.Error("Failed to list routes from database", zap.Error(err))
		} else {
			for _, dr := range dbRoutes {
				dbSlugs[dr.Slug] = true
			}
		}
	}

	// System routes from model manager (skip any that exist in DB to avoid duplicates)
	for slug, entry := range h.modelManager.GetRoutes() {
		if dbSlugs[slug] {
			continue
		}

		strategyName := "priority"
		if entry.Strategy != nil {
			strategyName = entry.Strategy.Name()
		}

		rr := routeResponse{
			ID:             slug,
			Name:           slug,
			Slug:           slug,
			Strategy:       strategyName,
			FallbackModels: entry.FallbackModels,
			Enabled:        true,
			Source:         "system",
		}
		if rr.FallbackModels == nil {
			rr.FallbackModels = []string{}
		}
		rr.Models = make([]routeModelResponse, 0, len(entry.Models))
		for _, rm := range entry.Models {
			rr.Models = append(rr.Models, routeModelResponse{
				ModelName: rm.ModelName,
				Weight:    rm.Weight,
				Priority:  rm.Priority,
				Enabled:   rm.Enabled,
			})
		}
		allRoutes = append(allRoutes, rr)
	}

	// User routes from database
	for _, dr := range dbRoutes {
		allRoutes = append(allRoutes, toRouteResponse(dr))
	}

	if allRoutes == nil {
		allRoutes = []routeResponse{}
	}

	h.sendResponse(w, http.StatusOK, map[string]interface{}{
		"routes": allRoutes,
		"total":  len(allRoutes),
	})
}

// CreateRouteRequest is the request body for creating a route.
type CreateRouteRequest struct {
	Name           string               `json:"name"`
	Slug           string               `json:"slug"`
	Description    string               `json:"description,omitempty"`
	Strategy       string               `json:"strategy"`
	Models         []routeModelResponse `json:"models"`
	FallbackModels []string             `json:"fallback_models,omitempty"`
	Enabled        *bool                `json:"enabled,omitempty"`
}

// CreateRoute creates a new user route.
func (h *RouteHandler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	var req CreateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if req.Name == "" {
		h.sendError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Slug == "" {
		h.sendError(w, http.StatusBadRequest, "slug is required")
		return
	}
	if !slugRegex.MatchString(req.Slug) {
		h.sendError(w, http.StatusBadRequest, "slug must be URL-safe (lowercase alphanumeric, hyphens, underscores)")
		return
	}

	// Validate strategy
	strategy := req.Strategy
	if strategy == "" {
		strategy = "priority"
	}
	if err := routing.ValidateStrategy(strategy); err != nil {
		h.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check slug doesn't conflict with existing model names
	availableModels := h.modelManager.GetAvailableModels()
	for _, m := range availableModels {
		if m == req.Slug {
			// Check if it's actually a route (allow overwriting)
			if _, isRoute := h.modelManager.ResolveRoute(m); !isRoute {
				h.sendError(w, http.StatusConflict, "slug conflicts with existing model name: "+req.Slug)
				return
			}
		}
	}

	// Validate model names exist in registry
	registeredModels := h.modelManager.GetRegistry().GetAvailableModels()
	registeredSet := make(map[string]bool, len(registeredModels))
	for _, m := range registeredModels {
		registeredSet[m] = true
	}
	for _, rm := range req.Models {
		if !registeredSet[rm.ModelName] {
			h.sendError(w, http.StatusBadRequest, "model not found in registry: "+rm.ModelName)
			return
		}
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// Build DB model
	route := &models.Route{
		Name:           req.Name,
		Slug:           req.Slug,
		Description:    req.Description,
		Strategy:       strategy,
		FallbackModels: models.StringArrayJSON(req.FallbackModels),
		Enabled:        enabled,
		Source:         "user",
	}
	for _, rm := range req.Models {
		rmEnabled := true
		if rm.Enabled {
			rmEnabled = rm.Enabled
		}
		weight := rm.Weight
		if weight <= 0 {
			weight = 50
		}
		priority := rm.Priority
		if priority <= 0 {
			priority = 50
		}
		route.Models = append(route.Models, models.RouteModel{
			ModelName: rm.ModelName,
			Weight:    weight,
			Priority:  priority,
			Enabled:   rmEnabled,
		})
	}

	if err := h.service.Create(route); err != nil {
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			h.sendError(w, http.StatusConflict, "A route with this slug already exists")
			return
		}
		h.logger.Error("Failed to create route", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to create route: "+err.Error())
		return
	}

	// Register in model manager
	h.registerRouteInManager(*route)

	h.sendResponse(w, http.StatusCreated, toRouteResponse(*route))
}

// GetRoute returns a single route by ID.
func (h *RouteHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	routeID := chi.URLParam(r, "routeID")
	if routeID == "" {
		h.sendError(w, http.StatusBadRequest, "route ID is required")
		return
	}

	// Try UUID (user route)
	if id, err := uuid.Parse(routeID); err == nil {
		route, err := h.service.GetByID(id)
		if err != nil {
			h.sendError(w, http.StatusNotFound, "Route not found")
			return
		}
		h.sendResponse(w, http.StatusOK, toRouteResponse(*route))
		return
	}

	// System route by slug
	if entry, exists := h.modelManager.ResolveRoute(routeID); exists {
		strategyName := "priority"
		if entry.Strategy != nil {
			strategyName = entry.Strategy.Name()
		}
		rr := routeResponse{
			ID:             routeID,
			Name:           routeID,
			Slug:           routeID,
			Strategy:       strategyName,
			FallbackModels: entry.FallbackModels,
			Enabled:        true,
			Source:         "system",
		}
		if rr.FallbackModels == nil {
			rr.FallbackModels = []string{}
		}
		rr.Models = make([]routeModelResponse, 0, len(entry.Models))
		for _, rm := range entry.Models {
			rr.Models = append(rr.Models, routeModelResponse{
				ModelName: rm.ModelName,
				Weight:    rm.Weight,
				Priority:  rm.Priority,
				Enabled:   rm.Enabled,
			})
		}
		h.sendResponse(w, http.StatusOK, rr)
		return
	}

	h.sendError(w, http.StatusNotFound, "Route not found")
}

// UpdateRoute updates a user route.
func (h *RouteHandler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	routeID := chi.URLParam(r, "routeID")
	if routeID == "" {
		h.sendError(w, http.StatusBadRequest, "route ID is required")
		return
	}

	id, err := uuid.Parse(routeID)
	if err != nil {
		h.sendError(w, http.StatusForbidden, "System routes are managed via config.yaml")
		return
	}

	existing, err := h.service.GetByID(id)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "Route not found")
		return
	}
	if existing.Source == "system" {
		h.sendError(w, http.StatusForbidden, "System routes are managed via config.yaml")
		return
	}

	var req CreateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if req.Strategy != "" {
		if err := routing.ValidateStrategy(req.Strategy); err != nil {
			h.sendError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	name := req.Name
	if name == "" {
		name = existing.Name
	}
	slug := req.Slug
	if slug == "" {
		slug = existing.Slug
	}
	strategy := req.Strategy
	if strategy == "" {
		strategy = existing.Strategy
	}

	// Unregister old route
	h.modelManager.UnregisterRoute(existing.Slug)

	route := &models.Route{
		Name:           name,
		Slug:           slug,
		Description:    req.Description,
		Strategy:       strategy,
		FallbackModels: models.StringArrayJSON(req.FallbackModels),
		Enabled:        enabled,
	}
	for _, rm := range req.Models {
		weight := rm.Weight
		if weight <= 0 {
			weight = 50
		}
		priority := rm.Priority
		if priority <= 0 {
			priority = 50
		}
		rmEnabled := rm.Enabled
		route.Models = append(route.Models, models.RouteModel{
			ModelName: rm.ModelName,
			Weight:    weight,
			Priority:  priority,
			Enabled:   rmEnabled,
		})
	}

	if err := h.service.Update(id, route); err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to update route: "+err.Error())
		return
	}

	// Re-fetch to get full data
	updated, err := h.service.GetByID(id)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to re-fetch route")
		return
	}

	// Re-register in manager
	h.registerRouteInManager(*updated)

	h.sendResponse(w, http.StatusOK, toRouteResponse(*updated))
}

// DeleteRoute deletes a user route.
func (h *RouteHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	routeID := chi.URLParam(r, "routeID")
	if routeID == "" {
		h.sendError(w, http.StatusBadRequest, "route ID is required")
		return
	}

	id, err := uuid.Parse(routeID)
	if err != nil {
		h.sendError(w, http.StatusForbidden, "System routes are managed via config.yaml")
		return
	}

	// Get the route first to know the slug
	existing, err := h.service.GetByID(id)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "Route not found")
		return
	}
	if existing.Source == "system" {
		h.sendError(w, http.StatusForbidden, "System routes are managed via config.yaml")
		return
	}

	if err := h.service.Delete(id); err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to delete route: "+err.Error())
		return
	}

	// Unregister from manager
	h.modelManager.UnregisterRoute(existing.Slug)

	h.sendResponse(w, http.StatusOK, map[string]string{
		"message": "Route deleted successfully",
	})
}

// GetRouteStats returns traffic distribution stats for a route.
func (h *RouteHandler) GetRouteStats(w http.ResponseWriter, r *http.Request) {
	routeID := chi.URLParam(r, "routeID")
	if routeID == "" {
		h.sendError(w, http.StatusBadRequest, "route ID is required")
		return
	}

	// Resolve route slug
	var slug string
	if id, err := uuid.Parse(routeID); err == nil {
		// User route — look up slug from DB
		route, err := h.service.GetByID(id)
		if err != nil {
			h.sendError(w, http.StatusNotFound, "Route not found")
			return
		}
		slug = route.Slug
	} else {
		// System route — the ID is the slug
		if _, exists := h.modelManager.ResolveRoute(routeID); !exists {
			h.sendError(w, http.StatusNotFound, "Route not found")
			return
		}
		slug = routeID
	}

	// Parse time range (default 24 hours)
	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if parsed, err := strconv.Atoi(h); err == nil && parsed > 0 {
			hours = parsed
		}
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	if h.db == nil {
		h.sendResponse(w, http.StatusOK, map[string]interface{}{
			"total_requests": 0,
			"total_tokens":   0,
			"total_cost":     0,
			"models":         []interface{}{},
		})
		return
	}

	// Query usage_logs grouped by model/provider
	type modelStat struct {
		Model      string  `json:"model"`
		Provider   string  `json:"provider"`
		Requests   int64   `json:"requests"`
		Tokens     int64   `json:"tokens"`
		Cost       float64 `json:"cost"`
		AvgLatency float64 `json:"avg_latency"`
	}
	var stats []modelStat

	err := h.db.Table("usage_logs").
		Select("model, provider, COUNT(*) as requests, COALESCE(SUM(total_tokens), 0) as tokens, COALESCE(SUM(total_cost), 0) as cost, COALESCE(AVG(latency), 0) as avg_latency").
		Where("route_slug = ? AND timestamp >= ?", slug, since).
		Group("model, provider").
		Order("requests DESC").
		Find(&stats).Error

	if err != nil {
		h.logger.Error("Failed to query route stats", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to query route stats")
		return
	}

	// Calculate totals and percentages
	var totalRequests int64
	var totalTokens int64
	var totalCost float64
	for _, s := range stats {
		totalRequests += s.Requests
		totalTokens += s.Tokens
		totalCost += s.Cost
	}

	type modelStatResponse struct {
		Model      string  `json:"model"`
		Provider   string  `json:"provider"`
		Requests   int64   `json:"requests"`
		Tokens     int64   `json:"tokens"`
		Cost       float64 `json:"cost"`
		AvgLatency float64 `json:"avg_latency"`
		Percentage float64 `json:"percentage"`
	}

	modelStats := make([]modelStatResponse, 0, len(stats))
	for _, s := range stats {
		pct := 0.0
		if totalRequests > 0 {
			pct = math.Round(float64(s.Requests)/float64(totalRequests)*10000) / 100
		}
		modelStats = append(modelStats, modelStatResponse{
			Model:      s.Model,
			Provider:   s.Provider,
			Requests:   s.Requests,
			Tokens:     s.Tokens,
			Cost:       s.Cost,
			AvgLatency: math.Round(s.AvgLatency),
			Percentage: pct,
		})
	}

	h.sendResponse(w, http.StatusOK, map[string]interface{}{
		"total_requests": totalRequests,
		"total_tokens":   totalTokens,
		"total_cost":     totalCost,
		"models":         modelStats,
	})
}

// registerRouteInManager converts a DB route to a RouteEntry and registers it.
func (h *RouteHandler) registerRouteInManager(r models.Route) {
	if !r.Enabled {
		return
	}

	var routeModels []llmModels.RouteModelEntry
	for _, rm := range r.Models {
		routeModels = append(routeModels, llmModels.RouteModelEntry{
			ModelName: rm.ModelName,
			Weight:    rm.Weight,
			Priority:  rm.Priority,
			Enabled:   rm.Enabled,
		})
	}

	h.modelManager.RegisterRoute(&llmModels.RouteEntry{
		Slug:           r.Slug,
		Models:         routeModels,
		FallbackModels: []string(r.FallbackModels),
	}, r.Strategy)
}
