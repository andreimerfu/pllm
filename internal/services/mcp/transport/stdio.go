package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/services/mcp/protocol"
)

// StdioConfig describes how to spawn a stdio MCP backend.
type StdioConfig struct {
	Command    string
	Args       []string
	Env        []string // "KEY=VALUE"
	WorkingDir string
	Logger     *zap.Logger
}

// stdioTransport runs a child process and speaks JSON-RPC framed by newlines
// (per MCP stdio spec: one JSON message per line, no content-length header).
type stdioTransport struct {
	cfg StdioConfig

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	writeMu sync.Mutex

	nextID   atomic.Int64
	pendMu   sync.Mutex
	pending  map[string]chan *protocol.Message
	incoming chan *protocol.Message

	closeOnce sync.Once
	done      chan struct{}
}

// NewStdio creates a stdio transport. Call Start to spawn.
func NewStdio(cfg StdioConfig) Transport {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	return &stdioTransport{
		cfg:      cfg,
		pending:  make(map[string]chan *protocol.Message),
		incoming: make(chan *protocol.Message, 32),
		done:     make(chan struct{}),
	}
}

func (s *stdioTransport) Start(ctx context.Context) error {
	if s.cfg.Command == "" {
		return fmt.Errorf("stdio: command is required")
	}
	cmd := exec.CommandContext(ctx, s.cfg.Command, s.cfg.Args...)
	cmd.Env = append(cmd.Env, s.cfg.Env...)
	if s.cfg.WorkingDir != "" {
		cmd.Dir = s.cfg.WorkingDir
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdio: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdio: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stdio: stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("stdio: start: %w", err)
	}
	s.cmd = cmd
	s.stdin = stdin
	s.stdout = stdout
	s.stderr = stderr

	go s.readLoop()
	go s.drainStderr()
	return nil
}

func (s *stdioTransport) Send(ctx context.Context, msg *protocol.Message) (*protocol.Message, error) {
	if msg.JSONRPC == "" {
		msg.JSONRPC = protocol.JSONRPCVersion
	}

	// Assign ID for requests if not provided; notifications have no ID.
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
		// Pre-assigned ID (e.g. when forwarding from a client session).
		idKey = string(msg.ID)
		waitCh = make(chan *protocol.Message, 1)
		s.pendMu.Lock()
		s.pending[idKey] = waitCh
		s.pendMu.Unlock()
	}

	if err := s.writeMessage(msg); err != nil {
		if idKey != "" {
			s.pendMu.Lock()
			delete(s.pending, idKey)
			s.pendMu.Unlock()
		}
		return nil, err
	}

	// Notifications don't get responses.
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
		return nil, fmt.Errorf("stdio: transport closed")
	case resp := <-waitCh:
		return resp, nil
	}
}

func (s *stdioTransport) Incoming() <-chan *protocol.Message { return s.incoming }

func (s *stdioTransport) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		if s.stdin != nil {
			_ = s.stdin.Close()
		}
		if s.cmd != nil && s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
			_, _ = s.cmd.Process.Wait()
		}
		close(s.incoming)
	})
	return nil
}

func (s *stdioTransport) writeMessage(msg *protocol.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("stdio: marshal: %w", err)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := s.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("stdio: write: %w", err)
	}
	return nil
}

func (s *stdioTransport) readLoop() {
	reader := bufio.NewReaderSize(s.stdout, 64*1024)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var msg protocol.Message
			if err := json.Unmarshal(line, &msg); err != nil {
				s.cfg.Logger.Warn("stdio: bad json from backend", zap.Error(err))
				continue
			}
			s.dispatch(&msg)
		}
		if err != nil {
			if err != io.EOF {
				s.cfg.Logger.Warn("stdio: read error", zap.Error(err))
			}
			_ = s.Close()
			return
		}
	}
}

func (s *stdioTransport) dispatch(msg *protocol.Message) {
	if msg.IsResponse() {
		s.pendMu.Lock()
		ch, ok := s.pending[string(msg.ID)]
		if ok {
			delete(s.pending, string(msg.ID))
		}
		s.pendMu.Unlock()
		if ok {
			ch <- msg
		}
		return
	}
	// Requests from backend to gateway are rare (sampling, etc.); treat as notifications for now.
	select {
	case s.incoming <- msg:
	default:
		s.cfg.Logger.Warn("stdio: incoming channel full, dropping", zap.String("method", msg.Method))
	}
}

func (s *stdioTransport) drainStderr() {
	reader := bufio.NewScanner(s.stderr)
	for reader.Scan() {
		s.cfg.Logger.Debug("stdio: backend stderr", zap.String("line", reader.Text()))
	}
}

func isServerNotification(method string) bool {
	return len(method) >= len("notifications/") && method[:len("notifications/")] == "notifications/"
}
