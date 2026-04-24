// Package mcp exposes MCP-over-HTTP handlers (gateway + admin CRUD).
package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	"github.com/amerfu/pllm/internal/services/mcp/gateway"
	mcpmetrics "github.com/amerfu/pllm/internal/services/mcp/metrics"
	"github.com/amerfu/pllm/internal/services/mcp/protocol"
)

// GatewayHandler terminates MCP-over-streamable-HTTP from clients (Claude
// Desktop, Cursor, VS Code) and dispatches to the manager.
type GatewayHandler struct {
	logger  *zap.Logger
	manager *gateway.Manager
}

func NewGatewayHandler(logger *zap.Logger, manager *gateway.Manager) *GatewayHandler {
	return &GatewayHandler{logger: logger, manager: manager}
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
