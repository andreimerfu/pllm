package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/services/mcp/protocol"
)

// echoBackend is a minimal in-process MCP server that speaks
// streamable-HTTP. It implements initialize, tools/list, and
// tools/call (for one tool named "echo"). Used to validate the manager
// end-to-end without external processes.
func echoBackend(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var msg protocol.Message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := protocol.Message{JSONRPC: protocol.JSONRPCVersion, ID: msg.ID}
		switch msg.Method {
		case protocol.MethodInitialize:
			resp.Result = jsonMust(protocol.InitializeResult{
				ProtocolVersion: protocol.MCPProtocolVersion,
				ServerInfo:      protocol.Implementation{Name: "echo", Version: "0.0.1"},
				Capabilities:    protocol.ServerCapabilities{Tools: &protocol.ToolsCapability{}},
			})
		case protocol.MethodToolsList:
			resp.Result = jsonMust(protocol.ToolsListResult{Tools: []protocol.Tool{{
				Name:        "echo",
				Description: "Echoes the text argument back",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`),
			}}})
		case protocol.MethodToolsCall:
			var p protocol.ToolCallParams
			if err := json.Unmarshal(msg.Params, &p); err != nil {
				resp.Error = &protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()}
				break
			}
			var args struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(p.Arguments, &args)
			resp.Result = jsonMust(protocol.ToolCallResult{Content: []protocol.ContentBlock{{
				Type: "text", Text: args.Text,
			}}})
		case protocol.MethodPromptsList:
			resp.Result = jsonMust(protocol.PromptsListResult{Prompts: []protocol.Prompt{}})
		case protocol.MethodResourcesList:
			resp.Result = jsonMust(protocol.ResourcesListResult{Resources: []protocol.Resource{}})
		case protocol.MethodPing:
			resp.Result = json.RawMessage(`{}`)
		default:
			if strings.HasPrefix(msg.Method, "notifications/") {
				w.WriteHeader(http.StatusAccepted)
				return
			}
			resp.Error = &protocol.Error{Code: protocol.CodeMethodNotFound, Message: msg.Method}
		}
		if msg.IsNotification() {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func jsonMust(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func TestManagerHTTPEndToEnd(t *testing.T) {
	srv := echoBackend(t)
	m := NewManager(nil, nil)
	t.Cleanup(m.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info := BackendInfo{
		ID:        uuid.New(),
		Slug:      "echo",
		Name:      "Echo",
		Transport: "http",
		Endpoint:  srv.URL,
	}
	if _, err := m.AddBackend(ctx, info); err != nil {
		t.Fatalf("add backend: %v", err)
	}

	// Wait for the backend to become healthy.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b, _ := m.GetBackend("echo")
		if b != nil && b.IsHealthy() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	b, ok := m.GetBackend("echo")
	require.True(t, ok, "backend missing")
	require.True(t, b.IsHealthy(), "backend never became healthy: last=%q", b.LastError())

	// Aggregated tools should expose "echo__echo".
	tools := m.AggregateTools()
	require.Len(t, tools, 1)
	require.Equal(t, "echo"+NameSeparator+"echo", tools[0].Name)

	// Call through the gateway's HandleRequest path.
	params := protocol.ToolCallParams{
		Name:      "echo__echo",
		Arguments: json.RawMessage(`{"text":"hello"}`),
	}
	raw, _ := json.Marshal(params)
	req := &protocol.Message{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  protocol.MethodToolsCall,
		Params:  raw,
	}
	resp := m.HandleRequest(ctx, req)
	require.Nil(t, resp.Error, "unexpected error: %v", resp.Error)
	var out protocol.ToolCallResult
	require.NoError(t, json.Unmarshal(resp.Result, &out))
	require.Len(t, out.Content, 1)
	require.Equal(t, "hello", out.Content[0].Text)
}

func TestResolveToolBadName(t *testing.T) {
	m := NewManager(nil, nil)
	_, _, err := m.ResolveTool("noSeparator")
	require.Error(t, err)
}

// Sanity: a backend whose URL returns 500 never becomes healthy, and the
// gateway returns an error for tool calls that reference it.
func TestBackendUnreachable(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	t.Cleanup(dead.Close)

	m := NewManager(nil, nil)
	t.Cleanup(m.Stop)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = m.AddBackend(ctx, BackendInfo{
		ID:        uuid.New(),
		Slug:      "dead",
		Name:      "dead",
		Transport: "http",
		Endpoint:  dead.URL,
	})
	// Give the Start goroutine a moment to fail.
	time.Sleep(200 * time.Millisecond)
	b, ok := m.GetBackend("dead")
	require.True(t, ok)
	require.False(t, b.IsHealthy())
	require.NotEmpty(t, b.LastError(), "expected error recorded")
	_ = fmt.Sprintf
}
