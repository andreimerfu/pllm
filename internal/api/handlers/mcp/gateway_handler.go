// Package mcp exposes MCP-over-HTTP handlers (gateway + admin CRUD).
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	"github.com/amerfu/pllm/internal/services/mcp/gateway"
	mcpmetrics "github.com/amerfu/pllm/internal/services/mcp/metrics"
	"github.com/amerfu/pllm/internal/services/mcp/protocol"
)

// ScopeChecker is the minimum surface the gateway needs from the
// use-cases package. We depend on an interface to avoid pulling in the
// full use_cases package here (and to keep the gateway testable with a
// fake scope resolver).
type ScopeChecker interface {
	AllowMCPTool(ctx context.Context, useCaseID *uuid.UUID, serverSlug string) ScopeDecision
}

// ScopeDecision mirrors use_cases.Decision at the interface boundary so
// the handler doesn't import the concrete package.
type ScopeDecision struct {
	Allowed     bool
	EnforceMode bool
	Reason      string
}

// ScopeEventRecorder accepts persisted audit rows for out-of-scope MCP
// tool calls. The concrete implementation lives in the use_cases
// package; we take the interface so the handler stays isolated.
type ScopeEventRecorder interface {
	RecordMCPDenial(ctx context.Context, useCaseID uuid.UUID, keyID *uuid.UUID, keyName, target, reason string, enforced bool, remoteAddr, requestID string)
}

// GatewayHandler terminates MCP-over-streamable-HTTP from clients (Claude
// Desktop, Cursor, VS Code) and dispatches to the manager.
type GatewayHandler struct {
	logger   *zap.Logger
	manager  *gateway.Manager
	scope    ScopeChecker       // optional; nil disables use-case scope enforcement
	recorder ScopeEventRecorder // optional; nil disables audit persistence
}

func NewGatewayHandler(logger *zap.Logger, manager *gateway.Manager) *GatewayHandler {
	return &GatewayHandler{logger: logger, manager: manager}
}

// WithScope returns a GatewayHandler with use-case scope enforcement
// enabled. Wiring is a builder rather than a constructor arg so the
// dependency stays optional for setups without a DB.
func (h *GatewayHandler) WithScope(scope ScopeChecker) *GatewayHandler {
	h.scope = scope
	return h
}

// WithRecorder attaches the audit-log recorder. When both scope and
// recorder are set, every out-of-scope call (denied or flagged) gets
// persisted to scope_events.
func (h *GatewayHandler) WithRecorder(rec ScopeEventRecorder) *GatewayHandler {
	h.recorder = rec
	return h
}

// ServeHTTP handles POST /v1/mcp — each request carries a single JSON-RPC
// message (MCP 2024-11-05 streamable-HTTP transport). Responses are returned
// inline; notifications return 202.
func (h *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg protocol.Message
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&msg); err != nil {
		writeError(w, http.StatusBadRequest, protocol.CodeParseError, err.Error(), nil)
		return
	}

	if msg.IsNotification() {
		h.manager.HandleNotification(r.Context(), &msg)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !msg.IsRequest() {
		writeError(w, http.StatusBadRequest, protocol.CodeInvalidRequest, "malformed message", nil)
		return
	}

	// ACL check + metrics are only meaningful for tools/call. Every other
	// method is either informational (tools/list) or already scoped to the
	// backends the user can see.
	resp := h.dispatchWithACL(r, &msg)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Warn("mcp: write response", zap.Error(err))
	}
}

// dispatchWithACL is the auth+metrics wrapper around manager.HandleRequest.
// For tools/call: verify the authenticated key may invoke this tool and
// record a Prometheus counter + histogram observation. Master key bypasses
// the ACL but still emits metrics.
func (h *GatewayHandler) dispatchWithACL(r *http.Request, msg *protocol.Message) *protocol.Message {
	if msg.Method != protocol.MethodToolsCall {
		return h.manager.HandleRequest(r.Context(), msg)
	}

	var params protocol.ToolCallParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return &protocol.Message{
			JSONRPC: protocol.JSONRPCVersion,
			ID:      msg.ID,
			Error:   &protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()},
		}
	}

	server, tool, splitErr := splitToolName(params.Name)
	if splitErr != nil {
		return &protocol.Message{
			JSONRPC: protocol.JSONRPCVersion,
			ID:      msg.ID,
			Error:   &protocol.Error{Code: protocol.CodeInvalidParams, Message: splitErr.Error()},
		}
	}

	if !h.toolAllowed(r, params.Name) {
		mcpmetrics.RecordToolCall(server, tool, mcpmetrics.OutcomeDenied, 0)
		h.logger.Info("mcp tool call denied by ACL",
			zap.String("tool", params.Name))
		return &protocol.Message{
			JSONRPC: protocol.JSONRPCVersion,
			ID:      msg.ID,
			Error: &protocol.Error{
				Code:    protocol.CodeInvalidRequest,
				Message: "this key is not permitted to call tool: " + params.Name,
			},
		}
	}

	// Use-case scope check. Audit-first: out-of-scope calls are logged
	// always; denial only happens when the bound use case has
	// EnforceScope=true. Masterkey and unscoped keys bypass.
	if h.scope != nil && !middleware.IsMasterKey(r.Context()) {
		if key, ok := middleware.GetKey(r.Context()); ok && key != nil && key.UseCaseID != nil {
			decision := h.scope.AllowMCPTool(r.Context(), key.UseCaseID, server)
			if !decision.Allowed {
				h.logger.Warn("mcp tool out-of-scope",
					zap.String("tool", params.Name),
					zap.String("use_case", key.UseCaseID.String()),
					zap.Bool("enforced", decision.EnforceMode),
					zap.String("reason", decision.Reason))
				if h.recorder != nil {
					h.recorder.RecordMCPDenial(
						r.Context(),
						*key.UseCaseID,
						&key.ID, key.Name,
						params.Name, decision.Reason,
						decision.EnforceMode,
						r.RemoteAddr, r.Header.Get("X-Request-Id"),
					)
				}
				if decision.EnforceMode {
					mcpmetrics.RecordToolCall(server, tool, mcpmetrics.OutcomeDenied, 0)
					return &protocol.Message{
						JSONRPC: protocol.JSONRPCVersion,
						ID:      msg.ID,
						Error: &protocol.Error{
							Code:    protocol.CodeInvalidRequest,
							Message: "out of scope for bound use case: " + decision.Reason,
						},
					}
				}
				// Flag mode — let it through but attribute the call.
			}
		}
	}

	start := time.Now()
	resp := h.manager.HandleRequest(r.Context(), msg)
	elapsed := time.Since(start)

	outcome := mcpmetrics.OutcomeSuccess
	if resp.Error != nil {
		outcome = mcpmetrics.OutcomeError
	}
	mcpmetrics.RecordToolCall(server, tool, outcome, elapsed)
	return resp
}

// toolAllowed returns true if the caller may invoke the given qualified
// tool name. Master-key callers always allowed; unauthenticated calls
// (e.g. when MCP is mounted on a public router in future) also allowed.
func (h *GatewayHandler) toolAllowed(r *http.Request, qualified string) bool {
	if middleware.IsMasterKey(r.Context()) {
		return true
	}
	key, ok := middleware.GetKey(r.Context())
	if !ok || key == nil {
		return true
	}
	return key.IsMCPToolAllowed(qualified)
}

// splitToolName peels the "<slug>__<tool>" qualifier into its parts for
// metric labels. Returns error if the separator is missing.
func splitToolName(q string) (slug, tool string, err error) {
	slug, tool, ok := strings.Cut(q, gateway.NameSeparator)
	if !ok {
		return "", "", fmt.Errorf("tool name missing %q prefix", gateway.NameSeparator)
	}
	return slug, tool, nil
}

func writeError(w http.ResponseWriter, status, code int, msg string, id json.RawMessage) {
	resp := protocol.Message{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      id,
		Error:   &protocol.Error{Code: code, Message: msg},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
