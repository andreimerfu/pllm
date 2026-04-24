package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/services/mcp/protocol"
)

// SSEConfig describes a legacy-SSE MCP backend: GET opens a long-lived
// event stream; an "endpoint" event delivers the POST URL used for requests.
type SSEConfig struct {
	URL     string
	Headers map[string]string
	Client  *http.Client
	Logger  *zap.Logger
}

type sseTransport struct {
	cfg SSEConfig

	postURL atomic.Value // string — learned from "endpoint" event
	ready   chan struct{}

	nextID   atomic.Int64
	pendMu   sync.Mutex
	pending  map[string]chan *protocol.Message
	incoming chan *protocol.Message

	resp      *http.Response
	closeOnce sync.Once
	done      chan struct{}
}

// NewSSE creates an SSE transport.
func NewSSE(cfg SSEConfig) Transport {
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 0} // no timeout: long-lived stream
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	return &sseTransport{
		cfg:      cfg,
		ready:    make(chan struct{}),
		pending:  make(map[string]chan *protocol.Message),
		incoming: make(chan *protocol.Message, 32),
		done:     make(chan struct{}),
	}
}

func (s *sseTransport) Start(ctx context.Context) error {
	if s.cfg.URL == "" {
		return fmt.Errorf("sse: url is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range s.cfg.Headers {
		req.Header.Set(k, v)
	}
	resp, err := s.cfg.Client.Do(req)
	if err != nil {
		return fmt.Errorf("sse: dial: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		_ = resp.Body.Close()
		return fmt.Errorf("sse: backend %d: %s", resp.StatusCode, string(snippet))
	}
	s.resp = resp
	go s.readLoop()

	// Wait for "endpoint" event (up to 10s) before returning.
	select {
	case <-ctx.Done():
		_ = s.Close()
		return ctx.Err()
	case <-s.ready:
		return nil
	case <-time.After(10 * time.Second):
		_ = s.Close()
		return fmt.Errorf("sse: did not receive endpoint event")
	}
}

func (s *sseTransport) Send(ctx context.Context, msg *protocol.Message) (*protocol.Message, error) {
	if msg.JSONRPC == "" {
		msg.JSONRPC = protocol.JSONRPCVersion
	}
	post, _ := s.postURL.Load().(string)
	if post == "" {
		return nil, fmt.Errorf("sse: endpoint not yet known")
	}

	isRequest := msg.Method != "" && len(msg.ID) == 0 && !isServerNotification(msg.Method)
	var waitCh chan *protocol.Message
	var idKey string
	if isRequest {
		next := s.nextID.Add(1)
		id, _ := json.Marshal(next)
		msg.ID = id
		idKey = strconv.FormatInt(next, 10)
		waitCh = make(chan *protocol.Message, 1)
		s.pendMu.Lock()
		s.pending[idKey] = waitCh
		s.pendMu.Unlock()
	} else if len(msg.ID) > 0 {
		idKey = string(msg.ID)
		waitCh = make(chan *protocol.Message, 1)
		s.pendMu.Lock()
		s.pending[idKey] = waitCh
		s.pendMu.Unlock()
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, post, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range s.cfg.Headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if idKey != "" {
			s.pendMu.Lock()
			delete(s.pending, idKey)
			s.pendMu.Unlock()
		}
		return nil, fmt.Errorf("sse: post: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		if idKey != "" {
			s.pendMu.Lock()
			delete(s.pending, idKey)
			s.pendMu.Unlock()
		}
		return nil, fmt.Errorf("sse: backend post %d", resp.StatusCode)
	}

	if waitCh == nil {
		return nil, nil
	}
	select {
	case <-ctx.Done():
		s.pendMu.Lock()
		delete(s.pending, idKey)
		s.pendMu.Unlock()
		return nil, ctx.Err()
	case <-s.done:
		return nil, fmt.Errorf("sse: transport closed")
	case r := <-waitCh:
		return r, nil
	}
}

func (s *sseTransport) Incoming() <-chan *protocol.Message { return s.incoming }

func (s *sseTransport) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		if s.resp != nil {
			_ = s.resp.Body.Close()
		}
		close(s.incoming)
	})
	return nil
}

func (s *sseTransport) readLoop() {
	reader := bufio.NewReaderSize(s.resp.Body, 64*1024)
	var event string
	var dataLines []string
	flush := func() {
		if len(dataLines) == 0 {
			event = ""
			return
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		switch event {
		case "endpoint":
			s.handleEndpoint(strings.TrimSpace(data))
		case "message", "":
			s.handleMessage([]byte(data))
		}
		event = ""
	}
	for {
		line, err := reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			flush()
		} else if strings.HasPrefix(trimmed, "event:") {
			event = strings.TrimSpace(trimmed[len("event:"):])
		} else if strings.HasPrefix(trimmed, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(trimmed[len("data:"):]))
		}
		if err != nil {
			flush()
			if err != io.EOF {
				s.cfg.Logger.Warn("sse: read error", zap.Error(err))
			}
			_ = s.Close()
			return
		}
	}
}

func (s *sseTransport) handleEndpoint(ep string) {
	// Some servers return relative URLs — resolve against the SSE URL.
	if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
		base := s.cfg.URL
		if idx := strings.Index(base, "://"); idx > 0 {
			if slash := strings.Index(base[idx+3:], "/"); slash > 0 {
				base = base[:idx+3+slash]
			}
		}
		if !strings.HasPrefix(ep, "/") {
			ep = "/" + ep
		}
		ep = base + ep
	}
	s.postURL.Store(ep)
	select {
	case <-s.ready:
	default:
		close(s.ready)
	}
}

func (s *sseTransport) handleMessage(data []byte) {
	var msg protocol.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		s.cfg.Logger.Warn("sse: bad json", zap.Error(err))
		return
	}
	if msg.IsResponse() {
		s.pendMu.Lock()
		ch, ok := s.pending[string(msg.ID)]
		if ok {
			delete(s.pending, string(msg.ID))
		}
		s.pendMu.Unlock()
		if ok {
			ch <- &msg
		}
		return
	}
	select {
	case s.incoming <- &msg:
	default:
		s.cfg.Logger.Warn("sse: incoming full, dropping", zap.String("method", msg.Method))
	}
}
