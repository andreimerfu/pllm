package federation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/amerfu/pllm/internal/core/models"
)

// Client is a tiny HTTP wrapper around a peer's /v1/federation/* endpoints.
// One Client per peer per tick — kept stateless so retries can recreate.
type Client struct {
	base    string
	token   string
	client  string // optional: ExpectedClientID echoed in X-PLLM-Federation-Client
	timeout time.Duration
	http    *http.Client
}

// NewClient builds a Client. Empty endpoint is a programmer error and
// returns a Client that always fails — the runner logs but doesn't crash.
func NewClient(endpoint, token, expectedClient string) *Client {
	return &Client{
		base:    strings.TrimRight(endpoint, "/"),
		token:   token,
		client:  expectedClient,
		timeout: 30 * time.Second,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// fetchPage is the generic GET helper. dest must be a pointer the caller
// owns. Returns the raw status code so the runner can decide between
// "auth error" (401/403), "not found" (404 → empty), and "retry later".
func (c *Client) fetchPage(ctx context.Context, path string, dest any) error {
	if c.base == "" {
		return fmt.Errorf("federation client: endpoint not configured")
	}
	u := c.base + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.client != "" {
		req.Header.Set("X-PLLM-Federation-Client", c.client)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s: %d %s: %s", path, resp.StatusCode, resp.Status, truncate(string(body), 200))
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// HealthOK is a cheap reachability ping. The receiver's health endpoint
// returns 200 and a small JSON body; we only care about the status.
func (c *Client) HealthOK(ctx context.Context) error {
	var body struct {
		Status string `json:"status"`
	}
	return c.fetchPage(ctx, "/v1/federation/health", &body)
}

// FederationServerList / ...AgentList / ...SkillList / ...PromptList are
// the export envelopes. Mirror the local registry list shapes so a peer
// can re-use the same JSON decoders.
type FederationServerList struct {
	Servers []models.RegistryServer `json:"servers"`
	Total   int64                   `json:"total"`
	Next    int                     `json:"next"`
}

// FederationAgentList wraps a page of agents.
type FederationAgentList struct {
	Agents []models.RegistryAgent `json:"agents"`
	Total  int64                  `json:"total"`
	Next   int                    `json:"next"`
}

// FederationSkillList wraps a page of skills.
type FederationSkillList struct {
	Skills []models.RegistrySkill `json:"skills"`
	Total  int64                  `json:"total"`
	Next   int                    `json:"next"`
}

// FederationPromptList wraps a page of prompts.
type FederationPromptList struct {
	Prompts []models.RegistryPrompt `json:"prompts"`
	Total   int64                   `json:"total"`
	Next    int                     `json:"next"`
}

// FetchServers fetches active latest-version servers from the peer.
func (c *Client) FetchServers(ctx context.Context) ([]models.RegistryServer, error) {
	out := []models.RegistryServer{}
	offset := 0
	for {
		var page FederationServerList
		if err := c.fetchPage(ctx, fmt.Sprintf("/v1/federation/servers?limit=200&offset=%d", offset), &page); err != nil {
			return nil, err
		}
		out = append(out, page.Servers...)
		if page.Next == 0 || len(page.Servers) == 0 {
			break
		}
		offset = page.Next
	}
	return out, nil
}

// FetchAgents fetches active latest-version agents from the peer.
func (c *Client) FetchAgents(ctx context.Context) ([]models.RegistryAgent, error) {
	out := []models.RegistryAgent{}
	offset := 0
	for {
		var page FederationAgentList
		if err := c.fetchPage(ctx, fmt.Sprintf("/v1/federation/agents?limit=200&offset=%d", offset), &page); err != nil {
			return nil, err
		}
		out = append(out, page.Agents...)
		if page.Next == 0 || len(page.Agents) == 0 {
			break
		}
		offset = page.Next
	}
	return out, nil
}

// FetchSkills fetches active latest-version skills from the peer.
func (c *Client) FetchSkills(ctx context.Context) ([]models.RegistrySkill, error) {
	out := []models.RegistrySkill{}
	offset := 0
	for {
		var page FederationSkillList
		if err := c.fetchPage(ctx, fmt.Sprintf("/v1/federation/skills?limit=200&offset=%d", offset), &page); err != nil {
			return nil, err
		}
		out = append(out, page.Skills...)
		if page.Next == 0 || len(page.Skills) == 0 {
			break
		}
		offset = page.Next
	}
	return out, nil
}

// FetchPrompts fetches active latest-version prompts from the peer.
func (c *Client) FetchPrompts(ctx context.Context) ([]models.RegistryPrompt, error) {
	out := []models.RegistryPrompt{}
	offset := 0
	for {
		var page FederationPromptList
		if err := c.fetchPage(ctx, fmt.Sprintf("/v1/federation/prompts?limit=200&offset=%d", offset), &page); err != nil {
			return nil, err
		}
		out = append(out, page.Prompts...)
		if page.Next == 0 || len(page.Prompts) == 0 {
			break
		}
		offset = page.Next
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
