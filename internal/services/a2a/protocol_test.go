package a2a

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONRPCRequestRoundTrip(t *testing.T) {
	raw := []byte(`{"jsonrpc":"2.0","method":"tasks/send","params":{"id":"t-1","message":{"role":"user","parts":[{"type":"text","text":"hello"}]}},"id":1}`)
	var req JSONRPCRequest
	require.NoError(t, json.Unmarshal(raw, &req))
	require.Equal(t, "2.0", req.JSONRPC)
	require.Equal(t, "tasks/send", req.Method)

	var p TaskSendParams
	require.NoError(t, json.Unmarshal(req.Params, &p))
	require.Equal(t, "t-1", p.ID)
	require.Equal(t, "user", p.Message.Role)
	require.Equal(t, "hello", p.Message.Parts[0].Text)
}

func TestAgentCardShape(t *testing.T) {
	card := AgentCard{
		Name:         "sales-research",
		URL:          "https://pllm.example.com/v1/a2a/sales-research-0-1-0",
		Version:      "0.1.0",
		Capabilities: AgentCapabilities{Streaming: false, PushNotifications: false},
		Skills:       []AgentCardSkill{{Name: "summarize"}},
	}
	b, err := json.Marshal(card)
	require.NoError(t, err)
	require.Contains(t, string(b), `"streaming":false`)
	require.Contains(t, string(b), `"skills":[{"name":"summarize"}]`)
}
