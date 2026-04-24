// Package protocol defines JSON-RPC 2.0 and MCP message shapes used by
// the gateway's transport layer and the server it exposes to clients.
//
// We intentionally keep this minimal — only the methods the gateway needs
// to aggregate and proxy: initialize, tools/list, tools/call, prompts/list,
// prompts/get, resources/list, resources/read, plus notifications and pings.
package protocol

import (
	"encoding/json"
	"fmt"
)

const (
	JSONRPCVersion = "2.0"

	// MCP protocol version the gateway advertises.
	// Must match what backend servers negotiate with us.
	MCPProtocolVersion = "2024-11-05"
)

// Standard MCP method names.
const (
	MethodInitialize              = "initialize"
	MethodInitialized             = "notifications/initialized"
	MethodPing                    = "ping"
	MethodToolsList               = "tools/list"
	MethodToolsCall               = "tools/call"
	MethodPromptsList             = "prompts/list"
	MethodPromptsGet              = "prompts/get"
	MethodResourcesList           = "resources/list"
	MethodResourcesRead           = "resources/read"
	MethodResourcesTemplatesList  = "resources/templates/list"
	MethodResourcesSubscribe      = "resources/subscribe"
	MethodResourcesUnsubscribe    = "resources/unsubscribe"
	MethodCompletionComplete      = "completion/complete"
	MethodLoggingSetLevel         = "logging/setLevel"
	MethodNotificationsToolsList  = "notifications/tools/list_changed"
	MethodNotificationsPromptsList = "notifications/prompts/list_changed"
	MethodNotificationsResourcesList = "notifications/resources/list_changed"
)

// Standard JSON-RPC error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Message is a union of request, response, and notification.
// Callers inspect fields to discriminate.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// IsRequest reports whether m is a request (has ID and Method).
func (m *Message) IsRequest() bool {
	return len(m.ID) > 0 && m.Method != ""
}

// IsNotification reports whether m is a notification (has Method, no ID).
func (m *Message) IsNotification() bool {
	return len(m.ID) == 0 && m.Method != ""
}

// IsResponse reports whether m is a response (has ID, no Method).
func (m *Message) IsResponse() bool {
	return len(m.ID) > 0 && m.Method == ""
}

// Error models a JSON-RPC 2.0 error object.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("mcp error %d: %s", e.Code, e.Message)
}

// NewError builds a JSON-RPC error.
func NewError(code int, msg string) *Error {
	return &Error{Code: code, Message: msg}
}

// InitializeParams are the client-sent params on initialize.
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

// InitializeResult is the server reply to initialize.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

// Implementation describes a client or server implementation.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities captures what the client supports.
// Intentionally permissive — we echo everything back to backends on initialize.
type ClientCapabilities struct {
	Experimental map[string]any  `json:"experimental,omitempty"`
	Roots        *RootsCapability `json:"roots,omitempty"`
	Sampling     *struct{}       `json:"sampling,omitempty"`
}

type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerCapabilities describes what a server supports.
type ServerCapabilities struct {
	Experimental map[string]any      `json:"experimental,omitempty"`
	Logging      *struct{}           `json:"logging,omitempty"`
	Prompts      *PromptsCapability  `json:"prompts,omitempty"`
	Resources    *ResourcesCapability `json:"resources,omitempty"`
	Tools        *ToolsCapability    `json:"tools,omitempty"`
}

type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// Tool describes a single tool in a tools/list response.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ToolsListResult is the shape of the tools/list response.
type ToolsListResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// ToolCallParams is the body of tools/call.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolCallResult is the body of a tools/call response.
// Content is opaque JSON — we just forward.
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is one fragment of a tool result.
type ContentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Data     string          `json:"data,omitempty"`
	MimeType string          `json:"mimeType,omitempty"`
	Resource json.RawMessage `json:"resource,omitempty"`
}

// Prompt describes a single prompt in a prompts/list response.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type PromptsListResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

// Resource describes a single resource in resources/list.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type ResourcesListResult struct {
	Resources  []Resource `json:"resources"`
	NextCursor string     `json:"nextCursor,omitempty"`
}
