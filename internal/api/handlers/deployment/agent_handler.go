package deployment

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/services/deployment"
)

// AgentHandler exposes /api/admin/agents.
type AgentHandler struct {
	logger  *zap.Logger
	service *deployment.Service
}

func NewAgentHandler(logger *zap.Logger, svc *deployment.Service) *AgentHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AgentHandler{logger: logger, service: svc}
}

func (h *AgentHandler) ready() bool { return h.service != nil }

type agentDeployRequest struct {
	AgentID       uuid.UUID         `json:"agent_id"`
	UseCaseID     uuid.UUID         `json:"use_case_id"`
	Namespace     string            `json:"namespace,omitempty"`
	Replicas      int32             `json:"replicas,omitempty"`
	ImageOverride string            `json:"image_override,omitempty"`
	ExtraEnv      map[string]string `json:"extra_env,omitempty"`
}

// Deploy is POST /api/admin/agents.
func (h *AgentHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	var body agentDeployRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.AgentID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	if body.UseCaseID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "use_case_id is required")
		return
	}
	row, err := h.service.DeployAgent(r.Context(), deployment.DeployAgentRequest{
		AgentID:       body.AgentID,
		UseCaseID:     body.UseCaseID,
		Namespace:     body.Namespace,
		Replicas:      body.Replicas,
		ImageOverride: body.ImageOverride,
		ExtraEnv:      body.ExtraEnv,
	})
	if err != nil {
		if errors.Is(err, deployment.ErrAgentNotConfigured) {
			writeErr(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

// List is GET /api/admin/agents.
func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	rows, err := h.service.ListAgents(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": rows})
}

// Get is GET /api/admin/agents/{id}.
func (h *AgentHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.service.GetAgent(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "agent deployment not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// Delete is DELETE /api/admin/agents/{id}.
func (h *AgentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.service.UndeployAgent(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RefreshStatus is POST /api/admin/agents/{id}/status.
func (h *AgentHandler) RefreshStatus(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.service.RefreshAgentStatus(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, row)
}
