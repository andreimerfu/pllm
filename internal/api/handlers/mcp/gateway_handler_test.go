package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	"github.com/amerfu/pllm/internal/services/mcp/gateway"
	mcpmetrics "github.com/amerfu/pllm/internal/services/mcp/metrics"
	"github.com/amerfu/pllm/internal/services/mcp/protocol"
)

// fakeBackend responds to initialize, tools/list, tools/call with a
// deterministic echo result.
func fakeBackend(t *testing.T) *httptest.Server {
	t.Helper()
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m protocol.Message
		_ = json.NewDecoder(r.Body).Decode(&m)
		resp := protocol.Message{JSONRPC: protocol.JSONRPCVersion, ID: m.ID}
		switch m.Method {
		case protocol.MethodInitialize:
			resp.Result = json.RawMessage(`{"protocolVersion":"2024-11-05","serverInfo":{"name":"x","version":"0"},"capabilities":{"tools":{}}}`)
		case protocol.MethodToolsList:
			resp.Result = json.RawMessage(`{"tools":[{"name":"read_file","description":"d","inputSchema":{}},{"name":"delete_file","description":"d","inputSchema":{}}]}`)
		case protocol.MethodToolsCall:
			resp.Result = json.RawMessage(`{"content":[{"type":"text","text":"ok"}]}`)
		case protocol.MethodPromptsList:
			resp.Result = json.RawMessage(`{"prompts":[]}`)
		case protocol.MethodResourcesList:
			resp.Result = json.RawMessage(`{"resources":[]}`)
		case protocol.MethodPing:
			resp.Result = json.RawMessage(`{}`)
		}
		if m.IsNotification() {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(h.Close)
	return h
}

// registerBackend wires a new manager with one HTTP backend and waits for
// it to come up.
func registerBackend(t *testing.T, url, slug string) *gateway.Manager {
	t.Helper()
	mgr := gateway.NewManager(zap.NewNop(), nil)
	t.Cleanup(mgr.Stop)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := mgr.AddBackend(ctx, gateway.BackendInfo{
		ID:        uuid.New(),
		Slug:      slug,
		Name:      slug,
		Transport: "http",
		Endpoint:  url,
	})
	require.NoError(t, err)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b, _ := mgr.GetBackend(slug)
		if b != nil && b.IsHealthy() {
			return mgr
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("backend never healthy")
	return nil
}

// postMCP issues a tools/call request through the gateway handler with the
// given authenticated key in context. Returns the decoded message.
func postMCP(t *testing.T, h *GatewayHandler, key *models.Key, toolName string) *protocol.Message {
	t.Helper()
	req := protocol.Message{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  protocol.MethodToolsCall,
		Params:  json.RawMessage(`{"name":"` + toolName + `","arguments":{}}`),
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader(body))
	if key != nil {
		ctx := context.WithValue(r.Context(), middleware.KeyContextKey, key)
		r = r.WithContext(ctx)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	var out protocol.Message
	require.NoError(t, json.NewDecoder(w.Body).Decode(&out))
	return &out
}

func TestGatewayACLEnforcement(t *testing.T) {
	backend := fakeBackend(t)
	mgr := registerBackend(t, backend.URL, "fs")
	h := NewGatewayHandler(zap.NewNop(), mgr)

	// Key with only read_* allowed.
	key := &models.Key{
		AllowedMCPTools: pq.StringArray{"fs/read_*"},
	}

	allowed := postMCP(t, h, key, "fs__read_file")
	require.Nil(t, allowed.Error, "read_file should be allowed: %v", allowed.Error)

	denied := postMCP(t, h, key, "fs__delete_file")
	require.NotNil(t, denied.Error, "delete_file should be denied")
	require.Contains(t, strings.ToLower(denied.Error.Message), "not permitted")
}

func TestGatewayMasterKeyBypass(t *testing.T) {
	backend := fakeBackend(t)
	mgr := registerBackend(t, backend.URL, "fs")
	h := NewGatewayHandler(zap.NewNop(), mgr)

	// Simulate master-key auth by putting AuthTypeMasterKey in context.
	// No Key needs to be set.
	key := &models.Key{AllowedMCPTools: pq.StringArray{"nothing/matches"}}
	req := protocol.Message{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  protocol.MethodToolsCall,
		Params:  json.RawMessage(`{"name":"fs__delete_file","arguments":{}}`),
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader(body))
	ctx := context.WithValue(r.Context(), middleware.KeyContextKey, key)
	ctx = context.WithValue(ctx, middleware.AuthTypeContextKey, middleware.AuthTypeMasterKey)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var out protocol.Message
	_ = json.NewDecoder(w.Body).Decode(&out)
	require.Nil(t, out.Error, "master key should bypass ACL even with restrictive key: %v", out.Error)
}

func TestMetricsRecordedForToolCalls(t *testing.T) {
	backend := fakeBackend(t)
	mgr := registerBackend(t, backend.URL, "fs")
	h := NewGatewayHandler(zap.NewNop(), mgr)

	// Snapshot: we use testutil.ToFloat64 against the actual collectors.
	// Import the metrics package directly to reach them.
	before := readCounter(t, "fs", "read_file", "success")
	_ = postMCP(t, h, nil, "fs__read_file") // no key = allowed
	after := readCounter(t, "fs", "read_file", "success")
	require.Equal(t, before+1, after, "expected success counter to increment")

	// Denied path.
	deniedBefore := readCounter(t, "fs", "delete_file", "denied")
	key := &models.Key{AllowedMCPTools: pq.StringArray{"fs/read_*"}}
	_ = postMCP(t, h, key, "fs__delete_file")
	deniedAfter := readCounter(t, "fs", "delete_file", "denied")
	require.Equal(t, deniedBefore+1, deniedAfter, "expected denied counter to increment")
}

// readCounter grabs the current value of the tool-calls counter.
func readCounter(t *testing.T, server, tool, outcome string) float64 {
	t.Helper()
	return mcpmetrics.CounterValue(server, tool, outcome)
}
