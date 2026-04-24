package mcp

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/services/mcp/protocol"
	"github.com/amerfu/pllm/internal/services/mcp/registryserver"
)

// RegistryMCPHandler serves the pllm registry over MCP (streamable HTTP).
// Parallel to GatewayHandler but backed by the registryserver package —
// the pllm registry itself is just another MCP server that happens to be
// in-process.
type RegistryMCPHandler struct {
	logger *zap.Logger
	server *registryserver.Server
}

// NewRegistryMCPHandler constructs the handler.
func NewRegistryMCPHandler(logger *zap.Logger, server *registryserver.Server) *RegistryMCPHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RegistryMCPHandler{logger: logger, server: server}
}

// ServeHTTP terminates POST requests carrying JSON-RPC messages.
func (h *RegistryMCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var msg protocol.Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		writeError(w, http.StatusBadRequest, protocol.CodeParseError, err.Error(), nil)
		return
	}
	if msg.IsNotification() {
		// Notifications are acknowledged but the registry server is stateless.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !msg.IsRequest() {
		writeError(w, http.StatusBadRequest, protocol.CodeInvalidRequest, "malformed message", nil)
		return
	}
	resp := h.server.HandleRequest(r.Context(), &msg)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
