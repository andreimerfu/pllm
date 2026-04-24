package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/services/mcp/protocol"
)

// HTTPConfig describes a streamable-HTTP MCP backend (MCP 2024-11-05 spec).
// Each POST carries one message; GET (optional) opens an SSE stream for
// server-initiated notifications. We implement POST only; server-push is
// not critical for tool-calls through a gateway.
type HTTPConfig struct {
	URL     string
	Headers map[string]string
	Client  *http.Client
	Logger  *zap.Logger
}

type httpTransport struct {
	cfg    HTTPConfig
	nextID atomic.Int64

	incoming chan *protocol.Message
	done     chan struct{}
	closed   atomic.Bool
	// sessionID is returned by the backend on initialize via Mcp-Session-Id header
	// and echoed on subsequent calls.
	sessionID atomic.Value // string
}

// NewHTTP creates a streamable-HTTP MCP transport. Call Start before Send.
func NewHTTP(cfg HTTPConfig) Transport {
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 60 * time.Second}
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	return &httpTransport{
		cfg:      cfg,
		incoming: make(chan *protocol.Message, 16),
		done:     make(chan struct{}),
	}
}

func (h *httpTransport) Start(_ context.Context) error {
	if h.cfg.URL == "" {
		return fmt.Errorf("http: url is required")
	}
	return nil
}

func (h *httpTransport) Send(ctx context.Context, msg *protocol.Message) (*protocol.Message, error) {
	if msg.JSONRPC == "" {
		msg.JSONRPC = protocol.JSONRPCVersion
	}
	// Assign ID for requests.
	isRequest := msg.Method != "" && len(msg.ID) == 0 && !isServerNotification(msg.Method)
	if isRequest {
		id, _ := json.Marshal(h.nextID.Add(1))
		msg.ID = id
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("http: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range h.cfg.Headers {
		req.Header.Set(k, v)
	}
	if sid, _ := h.sessionID.Load().(string); sid != "" {
		req.Header.Set("Mcp-Session-Id", sid)
	}

	resp, err := h.cfg.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: do: %w", err)
	}
	defer resp.Body.Close()

	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		h.sessionID.Store(sid)
	}

	// 202 Accepted for notifications.
	if resp.StatusCode == http.StatusAccepted {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("http: backend %d: %s", resp.StatusCode, string(snippet))
	}

	// Some servers stream the response via SSE even for POST. Detect and collect.
	ct := resp.Header.Get("Content-Type")
	if hasPrefix(ct, "text/event-stream") {
		return readFirstSSEMessage(resp.Body)
	}

	// Regular JSON response.
	var out protocol.Message
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("http: decode: %w", err)
	}
	if !isRequest {
		return nil, nil
	}
	return &out, nil
}

func (h *httpTransport) Incoming() <-chan *protocol.Message { return h.incoming }

func (h *httpTransport) Close() error {
	if !h.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(h.done)
	close(h.incoming)
	// Best-effort DELETE to tell backend to drop its session.
	if sid, _ := h.sessionID.Load().(string); sid != "" {
		req, err := http.NewRequest(http.MethodDelete, h.cfg.URL, nil)
		if err == nil {
			req.Header.Set("Mcp-Session-Id", sid)
			for k, v := range h.cfg.Headers {
				req.Header.Set(k, v)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			resp, err := h.cfg.Client.Do(req.WithContext(ctx))
			if err == nil {
				_ = resp.Body.Close()
			}
		}
	}
	return nil
}

// readFirstSSEMessage parses a text/event-stream body and returns the first
// "message" event's JSON payload as a protocol.Message.
func readFirstSSEMessage(body io.Reader) (*protocol.Message, error) {
	buf := bufio.NewReaderSize(body, 32*1024)
	var dataLines []string
	for {
		line, err := buf.ReadString('\n')
		if line == "\n" || line == "\r\n" {
			if len(dataLines) > 0 {
				payload := joinLines(dataLines)
				var msg protocol.Message
				if err := json.Unmarshal([]byte(payload), &msg); err != nil {
					return nil, fmt.Errorf("sse decode: %w", err)
				}
				return &msg, nil
			}
		} else if hasPrefix(line, "data:") {
			dataLines = append(dataLines, trim(line[len("data:"):]))
		}
		if err != nil {
			if err == io.EOF && len(dataLines) > 0 {
				// Final event without trailing blank line.
				payload := joinLines(dataLines)
				var msg protocol.Message
				if e := json.Unmarshal([]byte(payload), &msg); e == nil {
					return &msg, nil
				}
			}
			return nil, fmt.Errorf("sse: no message event: %w", err)
		}
	}
}

// --- tiny helpers kept local to avoid extra imports ---

func hasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}

func trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == '\n' || s[end-1] == '\r' || s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func joinLines(lines []string) string {
	total := 0
	for _, l := range lines {
		total += len(l) + 1
	}
	out := make([]byte, 0, total)
	for i, l := range lines {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, l...)
	}
	return string(out)
}

