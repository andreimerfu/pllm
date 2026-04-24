// Package gateway wires MCP backends (transports + cached metadata) and
// the manager that owns them. Handlers call Manager to list backends,
// route tool calls, and refresh health.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/services/mcp/protocol"
	"github.com/amerfu/pllm/internal/services/mcp/transport"
)

// BackendInfo is the static configuration needed to (re)build a backend.
type BackendInfo struct {
	ID          uuid.UUID
	Slug        string
	Name        string
	Description string
	Transport   string
	// Remote:
	Endpoint string
	Headers  map[string]string
	// Stdio:
	Command    string
	Args       []string
	Env        []string
	WorkingDir string
}

// Backend wraps a live transport and caches the backend's last-seen
// tools / prompts / resources so gateway aggregation is in-memory-fast.
type Backend struct {
	info   BackendInfo
	logger *zap.Logger

	mu        sync.RWMutex
	transport transport.Transport
	ready     bool
	caps      protocol.ServerCapabilities

	toolsMu   sync.RWMutex
	tools     []protocol.Tool
	prompts   []protocol.Prompt
	resources []protocol.Resource

	healthy     atomic.Bool
	lastError   atomic.Value // string
	lastSeenAt  atomic.Value // time.Time
	initialized atomic.Bool
}

// NewBackend constructs a Backend but does not start it.
func NewBackend(info BackendInfo, logger *zap.Logger) *Backend {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Backend{info: info, logger: logger.With(zap.String("backend", info.Slug))}
}

// Info returns the static config.
func (b *Backend) Info() BackendInfo { return b.info }

// IsHealthy reports the last observed health.
func (b *Backend) IsHealthy() bool { return b.healthy.Load() }

// LastError returns the most recent error string (may be empty).
func (b *Backend) LastError() string {
	v, _ := b.lastError.Load().(string)
	return v
}

// LastSeen returns the last time we successfully talked to the backend.
func (b *Backend) LastSeen() time.Time {
	v, _ := b.lastSeenAt.Load().(time.Time)
	return v
}

// Tools returns a copy of the cached tool list.
func (b *Backend) Tools() []protocol.Tool {
	b.toolsMu.RLock()
	defer b.toolsMu.RUnlock()
	out := make([]protocol.Tool, len(b.tools))
	copy(out, b.tools)
	return out
}

// Prompts returns a copy of the cached prompts.
func (b *Backend) Prompts() []protocol.Prompt {
	b.toolsMu.RLock()
	defer b.toolsMu.RUnlock()
	out := make([]protocol.Prompt, len(b.prompts))
	copy(out, b.prompts)
	return out
}

// Resources returns a copy of the cached resources.
func (b *Backend) Resources() []protocol.Resource {
	b.toolsMu.RLock()
	defer b.toolsMu.RUnlock()
	out := make([]protocol.Resource, len(b.resources))
	copy(out, b.resources)
	return out
}

// Start dials/spawns the backend and runs initialize + tools/list.
func (b *Backend) Start(ctx context.Context) error {
	tp, err := b.buildTransport()
	if err != nil {
		b.setErr(err)
		return err
	}
	if err := tp.Start(ctx); err != nil {
		b.setErr(err)
		return err
	}
	b.mu.Lock()
	b.transport = tp
	b.mu.Unlock()

	if err := b.initialize(ctx); err != nil {
		b.setErr(err)
		_ = tp.Close()
		b.mu.Lock()
		b.transport = nil
		b.mu.Unlock()
		return err
	}
	if err := b.RefreshCatalog(ctx); err != nil {
		b.setErr(err)
		return err
	}
	b.healthy.Store(true)
	b.lastSeenAt.Store(time.Now())
	b.ready = true
	return nil
}

// Stop tears down the transport.
func (b *Backend) Stop() {
	b.mu.Lock()
	tp := b.transport
	b.transport = nil
	b.mu.Unlock()
	if tp != nil {
		_ = tp.Close()
	}
	b.healthy.Store(false)
	b.initialized.Store(false)
}

// Call issues a JSON-RPC method on the backend and returns the result payload.
// Notifications (no ID) return nil, nil.
func (b *Backend) Call(ctx context.Context, method string, params any) (*protocol.Message, error) {
	b.mu.RLock()
	tp := b.transport
	b.mu.RUnlock()
	if tp == nil {
		return nil, fmt.Errorf("backend %s: not connected", b.info.Slug)
	}
	var raw json.RawMessage
	if params != nil {
		buf, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		raw = buf
	}
	msg := &protocol.Message{JSONRPC: protocol.JSONRPCVersion, Method: method, Params: raw}
	resp, err := tp.Send(ctx, msg)
	if err != nil {
		b.setErr(err)
		b.healthy.Store(false)
		return nil, err
	}
	if resp != nil && resp.Error != nil {
		// Protocol-level error — still considered alive.
		return resp, nil
	}
	b.lastSeenAt.Store(time.Now())
	b.healthy.Store(true)
	return resp, nil
}

// RefreshCatalog re-fetches tools/prompts/resources from the backend.
// Silently tolerates methods the backend does not implement.
func (b *Backend) RefreshCatalog(ctx context.Context) error {
	if tools, err := b.fetchTools(ctx); err == nil {
		b.toolsMu.Lock()
		b.tools = tools
		b.toolsMu.Unlock()
	} else {
		return err
	}
	if prompts, err := b.fetchPrompts(ctx); err == nil {
		b.toolsMu.Lock()
		b.prompts = prompts
		b.toolsMu.Unlock()
	}
	if resources, err := b.fetchResources(ctx); err == nil {
		b.toolsMu.Lock()
		b.resources = resources
		b.toolsMu.Unlock()
	}
	return nil
}

// HealthCheck pings the backend; updates health status.
// If the transport is missing (e.g. initial Start failed because the
// upstream wasn't ready yet), attempt a full reconnect before giving up.
// This is how newly-deployed backends heal themselves without operator
// intervention once the underlying pod becomes reachable.
func (b *Backend) HealthCheck(ctx context.Context) error {
	b.mu.RLock()
	transportPresent := b.transport != nil
	b.mu.RUnlock()
	if !transportPresent {
		startCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if err := b.Start(startCtx); err != nil {
			return err
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := b.Call(ctx, protocol.MethodPing, nil)
	if err != nil {
		// Some backends don't implement ping; fall back to tools/list.
		if _, err2 := b.Call(ctx, protocol.MethodToolsList, nil); err2 != nil {
			return err2
		}
	}
	return nil
}

// --- internal ---

func (b *Backend) buildTransport() (transport.Transport, error) {
	switch b.info.Transport {
	case "stdio":
		return transport.NewStdio(transport.StdioConfig{
			Command:    b.info.Command,
			Args:       b.info.Args,
			Env:        b.info.Env,
			WorkingDir: b.info.WorkingDir,
			Logger:     b.logger,
		}), nil
	case "http":
		return transport.NewHTTP(transport.HTTPConfig{
			URL:     b.info.Endpoint,
			Headers: b.info.Headers,
			Logger:  b.logger,
		}), nil
	case "sse":
		return transport.NewSSE(transport.SSEConfig{
			URL:     b.info.Endpoint,
			Headers: b.info.Headers,
			Logger:  b.logger,
		}), nil
	default:
		return nil, fmt.Errorf("unknown transport %q", b.info.Transport)
	}
}

func (b *Backend) initialize(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := b.Call(ctx, protocol.MethodInitialize, protocol.InitializeParams{
		ProtocolVersion: protocol.MCPProtocolVersion,
		ClientInfo:      protocol.Implementation{Name: "pllm-mcp-gateway", Version: "0.1.0"},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if resp == nil || resp.Error != nil {
		if resp != nil && resp.Error != nil {
			return fmt.Errorf("initialize: %s", resp.Error.Message)
		}
		return fmt.Errorf("initialize: no response")
	}
	var init protocol.InitializeResult
	if err := json.Unmarshal(resp.Result, &init); err != nil {
		return fmt.Errorf("initialize decode: %w", err)
	}
	b.mu.Lock()
	b.caps = init.Capabilities
	b.mu.Unlock()

	// Best-effort initialized notification.
	_, _ = b.Call(ctx, protocol.MethodInitialized, nil)
	b.initialized.Store(true)
	return nil
}

func (b *Backend) fetchTools(ctx context.Context) ([]protocol.Tool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := b.Call(ctx, protocol.MethodToolsList, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Error != nil {
		return nil, nil
	}
	var out protocol.ToolsListResult
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		return nil, err
	}
	return out.Tools, nil
}

func (b *Backend) fetchPrompts(ctx context.Context) ([]protocol.Prompt, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := b.Call(ctx, protocol.MethodPromptsList, nil)
	if err != nil || resp == nil || resp.Error != nil {
		return nil, err
	}
	var out protocol.PromptsListResult
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		return nil, err
	}
	return out.Prompts, nil
}

func (b *Backend) fetchResources(ctx context.Context) ([]protocol.Resource, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := b.Call(ctx, protocol.MethodResourcesList, nil)
	if err != nil || resp == nil || resp.Error != nil {
		return nil, err
	}
	var out protocol.ResourcesListResult
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		return nil, err
	}
	return out.Resources, nil
}

func (b *Backend) setErr(err error) {
	if err == nil {
		return
	}
	b.lastError.Store(err.Error())
}
