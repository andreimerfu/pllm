package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/mcp/gateway"
)

// AdminHandler provides CRUD for MCP backend servers.
type AdminHandler struct {
	logger  *zap.Logger
	db      *gorm.DB
	manager *gateway.Manager
}

func NewAdminHandler(logger *zap.Logger, db *gorm.DB, manager *gateway.Manager) *AdminHandler {
	return &AdminHandler{logger: logger, db: db, manager: manager}
}

// upsertRequest is the JSON body shape for POST/PUT of an MCP server.
type upsertRequest struct {
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Description string            `json:"description"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Transport   string            `json:"transport"`
	Endpoint    string            `json:"endpoint,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
}

func (r *upsertRequest) validate() error {
	if r.Name == "" || r.Slug == "" {
		return errors.New("name and slug are required")
	}
	switch r.Transport {
	case models.MCPTransportStdio:
		if r.Command == "" {
			return errors.New("command is required for stdio transport")
		}
	case models.MCPTransportHTTP, models.MCPTransportSSE:
		if r.Endpoint == "" {
			return errors.New("endpoint is required for http/sse transport")
		}
	default:
		return fmt.Errorf("unknown transport %q", r.Transport)
	}
	return nil
}

// List returns all MCP servers.
func (h *AdminHandler) List(w http.ResponseWriter, r *http.Request) {
	var rows []models.MCPServer
	if err := h.db.WithContext(r.Context()).Order("slug asc").Find(&rows).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Enrich with live state if manager knows about them.
	type enriched struct {
		models.MCPServer
		LiveHealth string `json:"live_health,omitempty"`
		ToolCount  int    `json:"tool_count,omitempty"`
	}
	out := make([]enriched, len(rows))
	for i, row := range rows {
		out[i] = enriched{MCPServer: row}
		if b, ok := h.manager.GetBackend(row.Slug); ok {
			if b.IsHealthy() {
				out[i].LiveHealth = models.MCPHealthHealthy
			} else {
				out[i].LiveHealth = models.MCPHealthUnhealthy
			}
			out[i].ToolCount = len(b.Tools())
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// Get returns one MCP server by ID.
func (h *AdminHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var row models.MCPServer
	if err := h.db.WithContext(r.Context()).Preload("Tools").First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSONError(w, http.StatusNotFound, "not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// Create inserts a new MCP server and registers it with the manager.
func (h *AdminHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req upsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := req.validate(); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	row := models.MCPServer{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		Enabled:     true,
		Transport:   req.Transport,
		Endpoint:    req.Endpoint,
		Command:     req.Command,
		Args:        models.StringArray(req.Args),
		WorkingDir:  req.WorkingDir,
	}
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if len(req.Headers) > 0 {
		b, _ := json.Marshal(req.Headers)
		row.Headers = datatypes.JSON(b)
	}
	if len(req.Env) > 0 {
		b, _ := json.Marshal(req.Env)
		row.Env = datatypes.JSON(b)
	}
	if err := h.db.WithContext(r.Context()).Create(&row).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if row.Enabled {
		h.startBackend(r.Context(), &row)
	}
	writeJSON(w, http.StatusCreated, row)
}

// Update replaces an existing MCP server and bounces its backend.
func (h *AdminHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req upsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := req.validate(); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	var row models.MCPServer
	if err := h.db.WithContext(r.Context()).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSONError(w, http.StatusNotFound, "not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	row.Name = req.Name
	row.Slug = req.Slug
	row.Description = req.Description
	row.Transport = req.Transport
	row.Endpoint = req.Endpoint
	row.Command = req.Command
	row.Args = models.StringArray(req.Args)
	row.WorkingDir = req.WorkingDir
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if len(req.Headers) > 0 {
		b, _ := json.Marshal(req.Headers)
		row.Headers = datatypes.JSON(b)
	} else {
		row.Headers = nil
	}
	if len(req.Env) > 0 {
		b, _ := json.Marshal(req.Env)
		row.Env = datatypes.JSON(b)
	} else {
		row.Env = nil
	}
	if err := h.db.WithContext(r.Context()).Save(&row).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Restart the backend with fresh config.
	h.manager.RemoveBackend(row.ID)
	if row.Enabled {
		h.startBackend(r.Context(), &row)
	}
	writeJSON(w, http.StatusOK, row)
}

// Delete removes an MCP server and tears down its backend.
func (h *AdminHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.db.WithContext(r.Context()).Where("mcp_server_id = ?", id).Delete(&models.MCPServerTool{}).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.db.WithContext(r.Context()).Delete(&models.MCPServer{}, "id = ?", id).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.manager.RemoveBackend(id)
	w.WriteHeader(http.StatusNoContent)
}

// Health triggers an immediate probe and returns the current state.
func (h *AdminHandler) Health(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var row models.MCPServer
	if err := h.db.WithContext(r.Context()).First(&row, "id = ?", id).Error; err != nil {
		writeJSONError(w, http.StatusNotFound, "not found")
		return
	}
	b, ok := h.manager.GetBackend(row.Slug)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"health": "unknown", "reason": "backend not loaded"})
		return
	}
	if err := b.HealthCheck(r.Context()); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"health": "unhealthy", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"health":     "healthy",
		"last_seen":  b.LastSeen(),
		"tool_count": len(b.Tools()),
	})
}

func (h *AdminHandler) startBackend(ctx context.Context, row *models.MCPServer) {
	info, err := gateway.RowToInfo(row)
	if err != nil {
		h.logger.Warn("mcp admin: bad row on start", zap.String("slug", row.Slug), zap.Error(err))
		return
	}
	if _, err := h.manager.AddBackend(ctx, info); err != nil {
		h.logger.Warn("mcp admin: start backend", zap.String("slug", row.Slug), zap.Error(err))
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
