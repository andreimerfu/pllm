// Package deployment exposes the deploy-from-registry admin endpoints.
package deployment

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/services/deployment"
	"github.com/amerfu/pllm/internal/services/registry/service"
)

// Handler wires the deployment service to HTTP.
type Handler struct {
	logger      *zap.Logger
	deployments *deployment.Service
	servers     *service.ServerService
}

// NewHandler constructs a Handler. A nil deployments arg disables the
// endpoints (returns 503).
func NewHandler(logger *zap.Logger, deployments *deployment.Service, servers *service.ServerService) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, deployments: deployments, servers: servers}
}

func (h *Handler) ready() bool { return h.deployments != nil && h.servers != nil }

type deployRequest struct {
	ServerName    string `json:"server_name"`
	ServerVersion string `json:"server_version"`
	Namespace     string `json:"namespace,omitempty"`
}

// Deploy is POST /api/admin/deployments — body: { server_name, server_version?, namespace? }.
// Resolves the registry server row, hands it to the deployment service.
func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "deployment not configured")
		return
	}
	var body deployRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.ServerName == "" {
		writeErr(w, http.StatusBadRequest, "server_name is required")
		return
	}
	srv, err := h.servers.Get(r.Context(), body.ServerName, body.ServerVersion)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "registry server not found")
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	row, err := h.deployments.DeployFromServer(r.Context(), srv, body.Namespace)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

// List is GET /api/admin/deployments.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "deployment not configured")
		return
	}
	rows, err := h.deployments.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deployments": rows})
}

// Get is GET /api/admin/deployments/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.deployments.Get(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "deployment not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// RefreshStatus is POST /api/admin/deployments/{id}/status — forces a live
// poll against the platform and persists the result.
func (h *Handler) RefreshStatus(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.deployments.RefreshStatus(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// Delete is DELETE /api/admin/deployments/{id} — tears down + cleans up.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.deployments.Undeploy(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := chi.URLParam(r, "id")
	if dec, err := url.PathUnescape(raw); err == nil {
		raw = dec
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
