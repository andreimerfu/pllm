package discovery

import (
	"encoding/json"
	"net/http"

	"github.com/amerfu/pllm/internal/core/models"
	discoverysvc "github.com/amerfu/pllm/internal/services/discovery"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Handler struct {
	logger *zap.Logger
	svc    *discoverysvc.Service
}

func New(logger *zap.Logger, svc *discoverysvc.Service) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, svc: svc}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	status := models.DiscoveredAgentStatus(r.URL.Query().Get("status"))
	rows, err := h.svc.List(r.Context(), status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rows, "total": len(rows)})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	agent, calls, err := h.svc.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agent": agent,
		"calls": calls,
	})
}

type promoteReq struct {
	Version string `json:"version"`
}

func (h *Handler) Promote(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var body promoteReq
	_ = json.NewDecoder(r.Body).Decode(&body)
	regID, err := h.svc.Promote(r.Context(), id, body.Version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"registry_agent_id": regID,
	})
}

func (h *Handler) Ignore(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.svc.Ignore(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reset zeros the agent's flagged_count and clears the revoker latch
// so subsequent threshold crossings can re-fire auto-revoke. Demo
// helper — real deployments shouldn't need this.
func (h *Handler) Reset(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.svc.Reset(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
