// Package a2a implements enough of the Agent-to-Agent JSON-RPC protocol
// for Phase 2: tasks/send (non-streaming) + an agent card.
//
// Spec reference: github.com/google/A2A (tracking v0.x — pre-1.0).
// We deliberately don't chase every optional field; only what the
// bank demo and the plain-JSON alias need.
package a2a

import "encoding/json"

// --- JSON-RPC 2.0 envelope ------------------------------------------------

// JSONRPCRequest is the top-level request envelope.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"` // always "2.0"
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"` // string, number, or null
}

// JSONRPCResponse is the top-level response envelope.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

// JSONRPCError codes we use.
const (
	ErrCodeParse           = -32700
	ErrCodeInvalidRequest  = -32600
	ErrCodeMethodNotFound  = -32601
	ErrCodeInvalidParams   = -32602
	ErrCodeInternal        = -32603
	// App-level codes (outside the reserved JSON-RPC range).
	ErrCodeScopeDenied     = -32001
	ErrCodeAgentNotFound   = -32002
	ErrCodeAgentUnhealthy  = -32003
)

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// --- A2A task / message types --------------------------------------------

// TaskSendParams is the payload of "tasks/send".
type TaskSendParams struct {
	ID      string  `json:"id"`
	Message Message `json:"message"`
}

// Message is an A2A user/agent message. Parts can be mixed (text, data,
// file) but for Phase 2 we only pass-through; we don't interpret.
type Message struct {
	Role  string        `json:"role"` // "user" | "agent"
	Parts []MessagePart `json:"parts"`
}

type MessagePart struct {
	Type string `json:"type"` // "text" | "data" | "file"
	Text string `json:"text,omitempty"`
	Data any    `json:"data,omitempty"`
}

// Task is the canonical response shape.
type Task struct {
	ID       string     `json:"id"`
	Status   TaskStatus `json:"status"`
	Messages []Message  `json:"messages,omitempty"`
}

type TaskStatus struct {
	State   string `json:"state"` // "submitted" | "working" | "completed" | "failed"
	Message string `json:"message,omitempty"`
}

// --- Agent card (well-known) ---------------------------------------------

// AgentCard is served at /v1/a2a/{slug}/.well-known/agent.json.
type AgentCard struct {
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	URL          string            `json:"url"` // where to POST A2A calls
	Version      string            `json:"version"`
	Capabilities AgentCapabilities `json:"capabilities"`
	Skills       []AgentCardSkill  `json:"skills,omitempty"`
}

// AgentCapabilities describes what this agent supports. Phase 2 is
// request-response only; streaming + push lands in Phase 2.5.
type AgentCapabilities struct {
	Streaming         bool `json:"streaming"`
	PushNotifications bool `json:"pushNotifications"`
}

// AgentCardSkill is the trimmed-down view of a registry skill we surface
// as "what this agent knows how to do". Only name + description are
// mandatory per the A2A spec; we fill both from the RegistryRef target.
type AgentCardSkill struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// --- Pod contract (internal) ---------------------------------------------
//
// The agent pod exposes POST {endpoint}/invoke with the following shape:
//
//   request:  {"input": <any>, "task_id": "<string>"}
//   response: {"output": <any>, "error": "<optional string>"}
//
// These types are used by the proxy to build request bodies and parse
// responses. Not serialized on the external surface.

type InvokeRequest struct {
	Input  any    `json:"input"`
	TaskID string `json:"task_id"`
}

type InvokeResponse struct {
	Output any    `json:"output"`
	Error  string `json:"error,omitempty"`
}
