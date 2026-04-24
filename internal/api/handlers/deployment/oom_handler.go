package deployment

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/deployment"
)

// --- server deployment OOM controls ----------------------------------------

// BumpOOM is POST /api/admin/deployments/{id}/oom/bump — one-shot memory
// escalation without waiting for an OOM. Respects the ceiling.
func (h *Handler) BumpOOM(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.deployments.ManualBumpServer(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// ResetOOM is POST /api/admin/deployments/{id}/oom/reset — clears any
// persisted memory override and attempt counter, then re-applies.
func (h *Handler) ResetOOM(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.deployments.ResetServerOOM(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// SetOOMAutoscale is PUT /api/admin/deployments/{id}/oom/autoscale.
// Body: {"enabled": null | true | false} — tri-state override of the
// global config switch.
func (h *Handler) SetOOMAutoscale(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.deployments.SetOOMAutoscaleEnabled(r.Context(), id, body.Enabled); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- agent deployment OOM controls -----------------------------------------

// BumpOOM is POST /api/admin/agents/{id}/oom/bump.
func (h *AgentHandler) BumpOOM(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.service.ManualBumpAgent(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// ResetOOM is POST /api/admin/agents/{id}/oom/reset.
func (h *AgentHandler) ResetOOM(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.service.ResetAgentOOM(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// SetOOMAutoscale is PUT /api/admin/agents/{id}/oom/autoscale.
func (h *AgentHandler) SetOOMAutoscale(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.SetAgentOOMAutoscaleEnabled(r.Context(), id, body.Enabled); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- shared events feed ----------------------------------------------------

// ListEvents is GET /api/admin/deployments/events.
// Query params:
//
//   - deployment_id  — restrict to one deployment
//   - target_kind    — "server" | "agent"
//   - kind           — "oom.bumped" | "oom.cap_reached" | "oom.reset" | "oom.manual_bump"
//   - since          — RFC3339 timestamp
//   - limit          — 1..500 (default 100)
//   - offset         — pagination offset
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "deployment not configured")
		return
	}
	q := r.URL.Query()
	f := deployment.EventsFilter{}
	if raw := q.Get("deployment_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			f.DeploymentID = &id
		}
	}
	if raw := q.Get("target_kind"); raw != "" {
		f.TargetKind = models.DeploymentTargetKind(raw)
	}
	if raw := q.Get("kind"); raw != "" {
		f.Kind = models.DeploymentEventKind(raw)
	}
	if raw := q.Get("since"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			f.Since = &t
		}
	}
	if raw := q.Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			f.Limit = v
		}
	}
	if raw := q.Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			f.Offset = v
		}
	}
	rows, err := h.deployments.ListEvents(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": rows})
}
