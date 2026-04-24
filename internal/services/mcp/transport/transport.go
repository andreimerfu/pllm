// Package transport defines how the gateway speaks JSON-RPC / MCP to a
// backend server. Three flavors: stdio (spawned process), SSE (legacy MCP
// servers), and streamable HTTP (MCP 2024-11-05+).
package transport

import (
	"context"

	"github.com/amerfu/pllm/internal/services/mcp/protocol"
)

// Transport is a bidirectional MCP channel to a single backend server.
// Implementations are expected to be used from one caller at a time for
// requests; notifications from the server arrive on Incoming().
type Transport interface {
	// Start connects/spawns the backend. Must be called before Send.
	Start(ctx context.Context) error

	// Send issues a JSON-RPC message and returns the matching response.
	// For notifications (no ID), Send returns nil, nil after flushing.
	Send(ctx context.Context, msg *protocol.Message) (*protocol.Message, error)

	// Incoming returns a channel of server-initiated notifications.
	// The channel is closed when Close() is invoked.
	Incoming() <-chan *protocol.Message

	// Close tears down the connection / process.
	Close() error
}
