package a2a

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	a2asvc "github.com/amerfu/pllm/internal/services/a2a"
)

// GatewayHandler serves /v1/a2a/{slug} (JSON-RPC) and /v1/agents/{slug}/invoke
// (plain JSON alias).
type GatewayHandler struct {
	logger  *zap.Logger
	manager *a2asvc.Manager
	proxy   *a2asvc.Proxy
	scope   ScopeChecker
	events  ScopeEventRecorder
}

func NewGatewayHandler(
	logger *zap.Logger,
	manager *a2asvc.Manager,
	proxy *a2asvc.Proxy,
	scope ScopeChecker,
	events ScopeEventRecorder,
) *GatewayHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &GatewayHandler{
		logger: logger, manager: manager, proxy: proxy,
		scope: scope, events: events,
	}
}

type plainInvokeRequest struct {
	Input  any    `json:"input"`
	TaskID string `json:"task_id,omitempty"`
}

// InvokePlain handles POST /v1/agents/{slug}/invoke.
func (h *GatewayHandler) InvokePlain(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var req plainInvokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.TaskID == "" {
		req.TaskID = uuid.NewString()
	}

	agent, scopeErr := h.resolveAndAuthorize(r, slug)
	if scopeErr != nil {
		writeJSONError(w, scopeErr.status, scopeErr.msg)
		return
	}

	out, err := h.proxy.Invoke(r.Context(), agent.Endpoint, a2asvc.InvokeRequest{
		Input: req.Input, TaskID: req.TaskID,
	})
	if err != nil {
		h.logger.Warn("a2a invoke: pod call failed",
			zap.String("slug", slug), zap.Error(err))
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// JSONRPC handles POST /v1/a2a/{slug}. JSON-RPC 2.0, method "tasks/send".
func (h *GatewayHandler) JSONRPC(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var req a2asvc.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONRPCErr(w, nil, a2asvc.ErrCodeParse, err.Error())
		return
	}
	if req.JSONRPC != "2.0" {
		writeJSONRPCErr(w, req.ID, a2asvc.ErrCodeInvalidRequest, "jsonrpc must be \"2.0\"")
		return
	}
	if req.Method != "tasks/send" {
		writeJSONRPCErr(w, req.ID, a2asvc.ErrCodeMethodNotFound, "unsupported method: "+req.Method)
		return
	}

	var params a2asvc.TaskSendParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeJSONRPCErr(w, req.ID, a2asvc.ErrCodeInvalidParams, err.Error())
		return
	}
	if params.ID == "" {
		params.ID = uuid.NewString()
	}

	agent, scopeErr := h.resolveAndAuthorize(r, slug)
	if scopeErr != nil {
		code := a2asvc.ErrCodeInternal
		switch scopeErr.status {
		case http.StatusNotFound:
			code = a2asvc.ErrCodeAgentNotFound
		case http.StatusForbidden:
			code = a2asvc.ErrCodeScopeDenied
		}
		writeJSONRPCErr(w, req.ID, code, scopeErr.msg)
		return
	}

	flat := flattenMessageText(params.Message)

	out, err := h.proxy.Invoke(r.Context(), agent.Endpoint, a2asvc.InvokeRequest{
		Input: flat, TaskID: params.ID,
	})
	if err != nil {
		writeJSONRPCErr(w, req.ID, a2asvc.ErrCodeAgentUnhealthy, err.Error())
		return
	}

	task := a2asvc.Task{
		ID:     params.ID,
		Status: a2asvc.TaskStatus{State: "completed"},
		Messages: []a2asvc.Message{{
			Role: "agent",
			Parts: []a2asvc.MessagePart{{
				Type: "text",
				Text: fmt.Sprintf("%v", out.Output),
			}},
		}},
	}
	writeJSON(w, http.StatusOK, a2asvc.JSONRPCResponse{
		JSONRPC: "2.0", Result: task, ID: req.ID,
	})
}

// --- internals ------------------------------------------------------------

type httpErr struct {
	status int
	msg    string
}

// resolveAndAuthorize does the agent lookup + scope check.
// Returns (agent, nil) on success, or (nil, httpErr) on any failure
// (agent not found, scope denied, DB error). Records denial events.
func (h *GatewayHandler) resolveAndAuthorize(r *http.Request, slug string) (*models.AgentDeployment, *httpErr) {
	agent, err := h.manager.Resolve(r.Context(), slug)
	if err != nil {
		if errors.Is(err, a2asvc.ErrAgentNotFound) {
			return nil, &httpErr{http.StatusNotFound, "agent not found"}
		}
		return nil, &httpErr{http.StatusInternalServerError, err.Error()}
	}

	var callerUC *uuid.UUID
	var keyID *uuid.UUID
	var keyName string
	if k, ok := middleware.GetKey(r.Context()); ok && k != nil {
		callerUC = k.UseCaseID
		kid := k.ID
		keyID = &kid
		keyName = k.Name
	}

	dec := h.scope.AllowAgent(r.Context(), callerUC, agent.ResourceName)
	if !dec.Allowed && dec.EnforceMode {
		if h.events != nil && callerUC != nil {
			h.events.RecordAgentDenial(
				r.Context(), *callerUC, keyID, keyName,
				agent.ResourceName, dec.Reason, true,
				r.RemoteAddr, chiMiddleware.GetReqID(r.Context()),
			)
		}
		return nil, &httpErr{http.StatusForbidden, "scope: " + dec.Reason}
	}
	return agent, nil
}

// flattenMessageText concatenates text parts; drops data/file parts for
// Phase 2. Phase 2.5 passes structured input through.
func flattenMessageText(m a2asvc.Message) string {
	out := ""
	for _, p := range m.Parts {
		if p.Type == "text" {
			if out != "" {
				out += "\n"
			}
			out += p.Text
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSONRPCErr(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	writeJSON(w, http.StatusOK, a2asvc.JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &a2asvc.JSONRPCError{Code: code, Message: msg},
		ID:      id,
	})
}
