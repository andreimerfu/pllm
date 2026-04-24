package registryserver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/amerfu/pllm/internal/services/mcp/protocol"
	"github.com/amerfu/pllm/internal/services/mcp/registryserver"
	"github.com/amerfu/pllm/internal/services/registry/service"
)

func TestRegistryMCPRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	servers := service.NewServerService(db, nil)
	agents := service.NewAgentService(db, nil)
	skills := service.NewSkillService(db, nil)
	prompts := service.NewPromptService(db, nil)

	// Seed a few rows so the tools return something.
	_, err := servers.Upsert(context.Background(), &models.RegistryServer{
		Name: "io.example/fs", Version: "1.0.0", Description: "filesystem",
	})
	require.NoError(t, err)

	mcpSrv := registryserver.New(servers, agents, skills, prompts)

	// initialize → tools/list → tools/call(list_servers) → tools/call(get_server)
	ctx := context.Background()

	initResp := mcpSrv.HandleRequest(ctx, &protocol.Message{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  protocol.MethodInitialize,
		Params:  json.RawMessage(`{}`),
	})
	require.Nil(t, initResp.Error)
	var init protocol.InitializeResult
	require.NoError(t, json.Unmarshal(initResp.Result, &init))
	require.NotNil(t, init.Capabilities.Tools)

	listResp := mcpSrv.HandleRequest(ctx, &protocol.Message{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`2`),
		Method:  protocol.MethodToolsList,
	})
	require.Nil(t, listResp.Error)
	var tools protocol.ToolsListResult
	require.NoError(t, json.Unmarshal(listResp.Result, &tools))
	// Should contain list_servers, get_server, list_agents, ...
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	require.True(t, names["list_servers"])
	require.True(t, names["get_server"])
	require.True(t, names["list_agents"])

	// list_servers call.
	listServersResp := mcpSrv.HandleRequest(ctx, &protocol.Message{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`3`),
		Method:  protocol.MethodToolsCall,
		Params:  json.RawMessage(`{"name":"list_servers","arguments":{"search":"filesystem"}}`),
	})
	require.Nil(t, listServersResp.Error)
	var tcr protocol.ToolCallResult
	require.NoError(t, json.Unmarshal(listServersResp.Result, &tcr))
	require.Len(t, tcr.Content, 1)
	var payload struct {
		Items []models.RegistryServer `json:"Items"`
	}
	require.NoError(t, json.Unmarshal([]byte(tcr.Content[0].Text), &payload))
	require.Len(t, payload.Items, 1)
	require.Equal(t, "io.example/fs", payload.Items[0].Name)

	// get_server call.
	getResp := mcpSrv.HandleRequest(ctx, &protocol.Message{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`4`),
		Method:  protocol.MethodToolsCall,
		Params:  json.RawMessage(`{"name":"get_server","arguments":{"name":"io.example/fs"}}`),
	})
	require.Nil(t, getResp.Error)
	var tcr2 protocol.ToolCallResult
	require.NoError(t, json.Unmarshal(getResp.Result, &tcr2))
	require.Len(t, tcr2.Content, 1)
	var fetched models.RegistryServer
	require.NoError(t, json.Unmarshal([]byte(tcr2.Content[0].Text), &fetched))
	require.Equal(t, "io.example/fs", fetched.Name)
	require.Equal(t, "1.0.0", fetched.Version)
}

func TestUnknownToolReturnsError(t *testing.T) {
	mcpSrv := registryserver.New(nil, nil, nil, nil)
	resp := mcpSrv.HandleRequest(context.Background(), &protocol.Message{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  protocol.MethodToolsCall,
		Params:  json.RawMessage(`{"name":"wat"}`),
	})
	require.NotNil(t, resp.Error)
	require.Contains(t, resp.Error.Message, "unknown tool")
}

func TestToolsListFiltersByConfiguredServices(t *testing.T) {
	// Only agents configured -> only agent tools should appear.
	mcpSrv := registryserver.New(nil, service.NewAgentService(nil, nil), nil, nil)
	resp := mcpSrv.HandleRequest(context.Background(), &protocol.Message{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  protocol.MethodToolsList,
	})
	var tools protocol.ToolsListResult
	require.NoError(t, json.Unmarshal(resp.Result, &tools))
	for _, tool := range tools.Tools {
		require.Contains(t, []string{"list_agents", "get_agent"}, tool.Name)
	}
}
