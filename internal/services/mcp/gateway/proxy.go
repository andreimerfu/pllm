package gateway

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/services/mcp/protocol"
)

// ServerInfo is what the gateway advertises on initialize.
var ServerInfo = protocol.Implementation{
	Name:    "pllm-mcp-gateway",
	Version: "0.1.0",
}

// HandleRequest dispatches a single MCP request against the Manager and
// returns the response that should be written back to the client.
// It is transport-agnostic: the HTTP and SSE endpoints both call this.
func (m *Manager) HandleRequest(ctx context.Context, req *protocol.Message) *protocol.Message {
	resp := &protocol.Message{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      req.ID,
	}

	switch req.Method {
	case protocol.MethodInitialize:
		result := protocol.InitializeResult{
			ProtocolVersion: protocol.MCPProtocolVersion,
			ServerInfo:      ServerInfo,
			Capabilities: protocol.ServerCapabilities{
				Tools:     &protocol.ToolsCapability{ListChanged: true},
				Prompts:   &protocol.PromptsCapability{ListChanged: true},
				Resources: &protocol.ResourcesCapability{ListChanged: true},
			},
			Instructions: "Tools, prompts, and resources are aggregated from registered backends. " +
				"Names are prefixed with '<slug>" + NameSeparator + "'.",
		}
		resp.Result = mustMarshal(result)

	case protocol.MethodPing:
		resp.Result = json.RawMessage("{}")

	case protocol.MethodToolsList:
		resp.Result = mustMarshal(protocol.ToolsListResult{Tools: m.AggregateTools()})

	case protocol.MethodPromptsList:
		resp.Result = mustMarshal(protocol.PromptsListResult{Prompts: m.AggregatePrompts()})

	case protocol.MethodResourcesList:
		resp.Result = mustMarshal(protocol.ResourcesListResult{Resources: m.AggregateResources()})

	case protocol.MethodToolsCall:
		return m.handleToolCall(ctx, req)

	case protocol.MethodPromptsGet:
		return m.handlePromptGet(ctx, req)

	case protocol.MethodResourcesRead:
		return m.handleResourcesRead(ctx, req)

	default:
		resp.Error = &protocol.Error{
			Code:    protocol.CodeMethodNotFound,
			Message: "method not supported by gateway: " + req.Method,
		}
	}
	return resp
}

// HandleNotification processes client → gateway notifications (no response).
func (m *Manager) HandleNotification(_ context.Context, note *protocol.Message) {
	// Most client notifications (e.g. notifications/initialized) are advisory;
	// we don't need to forward them to each backend.
	m.logger.Debug("mcp notification", zap.String("method", note.Method))
}

func (m *Manager) handleToolCall(ctx context.Context, req *protocol.Message) *protocol.Message {
	resp := &protocol.Message{JSONRPC: protocol.JSONRPCVersion, ID: req.ID}
	var p protocol.ToolCallParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		resp.Error = &protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()}
		return resp
	}
	backend, localName, err := m.ResolveTool(p.Name)
	if err != nil {
		resp.Error = &protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()}
		return resp
	}
	if !backend.IsHealthy() {
		resp.Error = &protocol.Error{Code: protocol.CodeInternalError, Message: "backend unavailable"}
		return resp
	}
	out, err := backend.Call(ctx, protocol.MethodToolsCall, protocol.ToolCallParams{
		Name:      localName,
		Arguments: p.Arguments,
	})
	if err != nil {
		resp.Error = &protocol.Error{Code: protocol.CodeInternalError, Message: err.Error()}
		return resp
	}
	if out != nil && out.Error != nil {
		resp.Error = out.Error
		return resp
	}
	if out != nil {
		resp.Result = out.Result
	}
	return resp
}

func (m *Manager) handlePromptGet(ctx context.Context, req *protocol.Message) *protocol.Message {
	resp := &protocol.Message{JSONRPC: protocol.JSONRPCVersion, ID: req.ID}
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		resp.Error = &protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()}
		return resp
	}
	backend, localName, err := m.ResolvePrompt(p.Name)
	if err != nil {
		resp.Error = &protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()}
		return resp
	}
	out, err := backend.Call(ctx, protocol.MethodPromptsGet, map[string]any{
		"name":      localName,
		"arguments": p.Arguments,
	})
	if err != nil {
		resp.Error = &protocol.Error{Code: protocol.CodeInternalError, Message: err.Error()}
		return resp
	}
	if out != nil && out.Error != nil {
		resp.Error = out.Error
		return resp
	}
	if out != nil {
		resp.Result = out.Result
	}
	return resp
}

// handleResourcesRead forwards to whichever backend advertised the URI.
// URIs are opaque identifiers chosen by backends — we match by membership
// in the cached resource list.
func (m *Manager) handleResourcesRead(ctx context.Context, req *protocol.Message) *protocol.Message {
	resp := &protocol.Message{JSONRPC: protocol.JSONRPCVersion, ID: req.ID}
	var p struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		resp.Error = &protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()}
		return resp
	}
	for _, b := range m.ListBackends() {
		if !b.IsHealthy() {
			continue
		}
		for _, r := range b.Resources() {
			if r.URI == p.URI {
				out, err := b.Call(ctx, protocol.MethodResourcesRead, map[string]string{"uri": p.URI})
				if err != nil {
					resp.Error = &protocol.Error{Code: protocol.CodeInternalError, Message: err.Error()}
					return resp
				}
				if out != nil && out.Error != nil {
					resp.Error = out.Error
					return resp
				}
				if out != nil {
					resp.Result = out.Result
				}
				return resp
			}
		}
	}
	resp.Error = &protocol.Error{Code: protocol.CodeInvalidParams, Message: "unknown resource uri"}
	return resp
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return b
}
