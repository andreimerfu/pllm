package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Proxy is the outbound HTTP client that calls an agent pod's /invoke
// endpoint. Stateless — one Proxy per pllm process is fine.
type Proxy struct {
	client  *http.Client
	timeout time.Duration
}

// NewProxy constructs a Proxy with the given per-call timeout.
// timeout == 0 falls back to 5 minutes.
func NewProxy(timeout time.Duration) *Proxy {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &Proxy{
		client: &http.Client{
			// Overall per-call deadline is enforced via context below;
			// this is a safety net for the transport layer.
			Timeout: timeout + 15*time.Second,
		},
		timeout: timeout,
	}
}

// Invoke POSTs to {endpoint}/invoke with the given request and returns
// the parsed response.
func (p *Proxy) Invoke(ctx context.Context, endpoint string, req InvokeRequest) (*InvokeResponse, error) {
	url := strings.TrimRight(endpoint, "/") + "/invoke"
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("a2a proxy: marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("a2a proxy: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("a2a proxy: call: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("a2a proxy: pod returned %d: %s", resp.StatusCode, string(respBody))
	}

	var out InvokeResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("a2a proxy: decode response: %w", err)
	}
	return &out, nil
}
