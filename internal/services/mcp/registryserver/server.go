// Package registryserver exposes the pllm registry as an MCP server so
// agents can discover catalog content via MCP tool calls instead of REST.
// This is conceptually distinct from the gateway: the gateway proxies
// external MCP servers; this one is an endpoint pllm itself speaks.
package registryserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/mcp/protocol"
	"github.com/amerfu/pllm/internal/services/registry/service"
)

// Server implements the subset of MCP needed to act as a registry tool
// provider. It's stateless beyond the service dependencies.
type Server struct {
	servers *service.ServerService
	agents  *service.AgentService
	skills  *service.SkillService
	prompts *service.PromptService
}

// New constructs a Server. Any nil dependency disables the corresponding
// tools — handy for tests or partial deployments.
func New(s *service.ServerService, a *service.AgentService,
	sk *service.SkillService, p *service.PromptService) *Server {
	return &Server{servers: s, agents: a, skills: sk, prompts: p}
}

// Info returned on initialize.
var serverInfo = protocol.Implementation{Name: "pllm-registry-mcp", Version: "0.1.0"}

// HandleRequest dispatches one JSON-RPC request. Method surface covers
// initialize, ping, tools/list, tools/call.
func (s *Server) HandleRequest(ctx context.Context, req *protocol.Message) *protocol.Message {
	resp := &protocol.Message{JSONRPC: protocol.JSONRPCVersion, ID: req.ID}
	switch req.Method {
	case protocol.MethodInitialize:
		resp.Result = mustMarshal(protocol.InitializeResult{
			ProtocolVersion: protocol.MCPProtocolVersion,
			ServerInfo:      serverInfo,
			Capabilities: protocol.ServerCapabilities{
				Tools: &protocol.ToolsCapability{},
			},
			Instructions: "Tools query the pllm registry. Results are JSON " +
				"matching the /v1/registry/* REST shapes.",
		})
	case protocol.MethodPing:
		resp.Result = json.RawMessage("{}")
	case protocol.MethodToolsList:
		resp.Result = mustMarshal(protocol.ToolsListResult{Tools: s.toolManifest()})
	case protocol.MethodToolsCall:
		return s.handleCall(ctx, req)
	default:
		resp.Error = &protocol.Error{
			Code:    protocol.CodeMethodNotFound,
			Message: "method not supported: " + req.Method,
		}
	}
	return resp
}

// toolManifest returns the tools this server advertises. Kept small and
// explicit — six tools covering the common discovery surface.
func (s *Server) toolManifest() []protocol.Tool {
	paginated := json.RawMessage(`{
		"type":"object",
		"properties":{
			"search":{"type":"string","description":"Substring match on name or description"},
			"limit":{"type":"integer","description":"Max results (default 30, cap 200)"},
			"latest":{"type":"boolean","description":"Return only is_latest=true rows"}
		}
	}`)
	byName := json.RawMessage(`{
		"type":"object",
		"properties":{
			"name":{"type":"string"},
			"version":{"type":"string","description":"Optional; defaults to latest"}
		},
		"required":["name"]
	}`)
	tools := []protocol.Tool{}
	if s.servers != nil {
		tools = append(tools,
			protocol.Tool{Name: "list_servers", Description: "List MCP servers in the registry.", InputSchema: paginated},
			protocol.Tool{Name: "get_server", Description: "Get a single MCP server entry.", InputSchema: byName},
		)
	}
	if s.agents != nil {
		tools = append(tools,
			protocol.Tool{Name: "list_agents", Description: "List published agents.", InputSchema: paginated},
			protocol.Tool{Name: "get_agent", Description: "Get an agent with its dependency refs.", InputSchema: byName},
		)
	}
	if s.skills != nil {
		tools = append(tools,
			protocol.Tool{Name: "list_skills", Description: "List skill bundles.", InputSchema: paginated},
		)
	}
	if s.prompts != nil {
		tools = append(tools,
			protocol.Tool{Name: "list_prompts", Description: "List prompt templates.", InputSchema: paginated},
		)
	}
	return tools
}

func (s *Server) handleCall(ctx context.Context, req *protocol.Message) *protocol.Message {
	resp := &protocol.Message{JSONRPC: protocol.JSONRPCVersion, ID: req.ID}
	var call protocol.ToolCallParams
	if err := json.Unmarshal(req.Params, &call); err != nil {
		resp.Error = &protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()}
		return resp
	}
	result, err := s.invoke(ctx, call.Name, call.Arguments)
	if err != nil {
		resp.Error = &protocol.Error{Code: protocol.CodeInternalError, Message: err.Error()}
		return resp
	}
	resp.Result = mustMarshal(result)
	return resp
}

// invoke dispatches a single tool call to the underlying service and
// wraps the returned struct into an MCP content block.
func (s *Server) invoke(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResult, error) {
	switch name {
	case "list_servers":
		f, err := parsePaginated(args)
		if err != nil {
			return nil, err
		}
		out, err := s.servers.List(ctx, f)
		if err != nil {
			return nil, err
		}
		return jsonBlock(out)
	case "get_server":
		n, v, err := parseByName(args)
		if err != nil {
			return nil, err
		}
		out, err := s.servers.Get(ctx, n, v)
		if err != nil {
			return nil, err
		}
		return jsonBlock(out)
	case "list_agents":
		f, err := parsePaginated(args)
		if err != nil {
			return nil, err
		}
		out, err := s.agents.List(ctx, f)
		if err != nil {
			return nil, err
		}
		return jsonBlock(out)
	case "get_agent":
		n, v, err := parseByName(args)
		if err != nil {
			return nil, err
		}
		out, err := s.agents.Get(ctx, n, v)
		if err != nil {
			return nil, err
		}
		return jsonBlock(out)
	case "list_skills":
		f, err := parsePaginated(args)
		if err != nil {
			return nil, err
		}
		out, err := s.skills.List(ctx, f)
		if err != nil {
			return nil, err
		}
		return jsonBlock(out)
	case "list_prompts":
		f, err := parsePaginated(args)
		if err != nil {
			return nil, err
		}
		out, err := s.prompts.List(ctx, f)
		if err != nil {
			return nil, err
		}
		return jsonBlock(out)
	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}

// --- arg decoders ---------------------------------------------------------

type paginatedArgs struct {
	Search string `json:"search"`
	Limit  int    `json:"limit"`
	Latest bool   `json:"latest"`
}

func parsePaginated(raw json.RawMessage) (service.ListFilter, error) {
	if len(raw) == 0 {
		return service.ListFilter{}, nil
	}
	var a paginatedArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return service.ListFilter{}, err
	}
	return service.ListFilter{Search: a.Search, Limit: a.Limit, LatestOnly: a.Latest,
		Status: models.RegistryStatusActive}, nil
}

type byNameArgs struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func parseByName(raw json.RawMessage) (string, string, error) {
	var a byNameArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", "", err
	}
	if a.Name == "" {
		return "", "", fmt.Errorf("name is required")
	}
	return a.Name, a.Version, nil
}

// jsonBlock wraps a JSON-serializable payload in a single MCP content block.
// Clients can either parse the text field as JSON or display it raw.
func jsonBlock(v any) (*protocol.ToolCallResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: string(b)}},
	}, nil
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}
