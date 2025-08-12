package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
)

// StreamingResponseWriter preserves the Flusher interface for SSE streaming
// It wraps http.ResponseWriter and ensures Flush() is always available
type StreamingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
	wrapped    int64 // Track bytes written for metrics
}

// NewStreamingResponseWriter creates a new StreamingResponseWriter
func NewStreamingResponseWriter(w http.ResponseWriter) *StreamingResponseWriter {
	return &StreamingResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		written:        false,
		wrapped:        0,
	}
}

// WriteHeader captures the status code
func (w *StreamingResponseWriter) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
		w.ResponseWriter.WriteHeader(code)
	}
}

// Write writes data to the response
func (w *StreamingResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.wrapped += int64(n)
	return n, err
}

// Flush implements the http.Flusher interface
// This is the critical method for SSE streaming
func (w *StreamingResponseWriter) Flush() {
	// Try to flush the underlying writer
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements the http.Hijacker interface
func (w *StreamingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("hijack not supported")
}

// Push implements the http.Pusher interface for HTTP/2
func (w *StreamingResponseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// StatusCode returns the captured status code
func (w *StreamingResponseWriter) StatusCode() int {
	return w.statusCode
}

// Written returns whether headers have been written
func (w *StreamingResponseWriter) Written() bool {
	return w.written
}

// BytesWritten returns the number of bytes written
func (w *StreamingResponseWriter) BytesWritten() int64 {
	return w.wrapped
}

// Unwrap returns the underlying ResponseWriter for compatibility
func (w *StreamingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}