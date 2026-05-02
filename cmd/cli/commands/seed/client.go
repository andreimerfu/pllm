// Package seed implements the `pllm seed` subcommand. All routes go
// through the public admin REST API with the master key — the seed
// never touches the DB directly. That keeps the binary deployable as
// a one-shot K8s Job using just the existing pllm image and master
// key secret.
package seed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Client is a thin REST helper for the demo seed.
type Client struct {
	BaseURL     string
	MasterKey   string
	HTTP        *http.Client
	uuidsBySlug map[string]uuid.UUID // slug -> gateway-assigned UUID
}

// NewClient returns a ready Client. Callers should set BaseURL +
// MasterKey before use; the timeout default is 30s.
func NewClient(baseURL, masterKey string) *Client {
	return &Client{
		BaseURL:     strings.TrimRight(baseURL, "/"),
		MasterKey:   masterKey,
		HTTP:        &http.Client{Timeout: 30 * time.Second},
		uuidsBySlug: map[string]uuid.UUID{},
	}
}

// CacheUUID stores a slug->id mapping for later lookup by dependents.
func (c *Client) CacheUUID(slug string, id uuid.UUID) {
	if slug == "" || id == uuid.Nil {
		return
	}
	c.uuidsBySlug[slug] = id
}

// LookupUUID returns the cached UUID for a slug, or uuid.Nil if not seen.
func (c *Client) LookupUUID(slug string) uuid.UUID {
	return c.uuidsBySlug[slug]
}

// Do issues a JSON request and decodes the response into out (which may
// be nil to discard). Returns the HTTP status and body on non-2xx so
// callers can branch on 404 vs 409 idempotency.
func (c *Client) Do(method, path string, body, out any) (int, []byte, error) {
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, buf)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.MasterKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, respBody, fmt.Errorf("decode %s %s: %w", method, path, err)
		}
	}
	return resp.StatusCode, respBody, nil
}

// MustOK returns nil if status is in 2xx, else an error including the
// response body for diagnostics.
func MustOK(method, path string, status int, body []byte) error {
	if status >= 200 && status < 300 {
		return nil
	}
	return fmt.Errorf("%s %s: HTTP %d: %s", method, path, status, strings.TrimSpace(string(body)))
}

// EscapeName URL-escapes a registry name for use in a path segment
// (e.g. "bank.demo/policy-docs" -> "bank.demo%2Fpolicy-docs").
func EscapeName(name string) string {
	return url.PathEscape(name)
}

// HealthCheck verifies the gateway is reachable + the master key works
// by GETting the use cases listing.
func (c *Client) HealthCheck() error {
	status, body, err := c.Do(http.MethodGet, "/api/admin/use-cases", nil, nil)
	if err != nil {
		return fmt.Errorf("gateway unreachable at %s: %w", c.BaseURL, err)
	}
	if status == http.StatusUnauthorized {
		return fmt.Errorf("master key rejected — check --master-key or $PLLM_MASTER_KEY")
	}
	return MustOK(http.MethodGet, "/api/admin/use-cases", status, body)
}
