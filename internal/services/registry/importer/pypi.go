package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PyPISource queries PyPI. Unlike npm, PyPI doesn't have a great public
// search API; the XML-RPC endpoint was deprecated in 2022. We fall back to
// the JSON endpoint for individual package metadata and rely on the caller
// supplying a reasonable seed ("mcp"), matching against a small set of
// well-known packages. For most real-world use, agentregistry recommends
// a curated seed list; we mimic that approach.
//
// Callers can override PackageSeed to provide their own package list.
type PyPISource struct {
	Client      *http.Client
	BaseURL     string   // default "https://pypi.org"
	PackageSeed []string // default: common MCP server packages
}

// NewPyPISource builds a PyPISource with the default seed list.
func NewPyPISource(client *http.Client) *PyPISource {
	if client == nil {
		client = newHTTPClient(15 * time.Second)
	}
	return &PyPISource{
		Client:  client,
		BaseURL: "https://pypi.org",
		PackageSeed: []string{
			// A small curated seed of known MCP server packages.
			// Users can expand this via the admin API if they want.
			"mcp-server-fetch",
			"mcp-server-git",
			"mcp-server-sqlite",
			"mcp-server-time",
			"mcp-server-filesystem",
		},
	}
}

// Name implements Source.
func (s *PyPISource) Name() string { return "pypi" }

// pypiInfoResponse is the shape of /pypi/<name>/json.
type pypiInfoResponse struct {
	Info struct {
		Name            string `json:"name"`
		Version         string `json:"version"`
		Summary         string `json:"summary"`
		HomePage        string `json:"home_page"`
		ProjectURL      string `json:"project_url"`
		ProjectURLs     map[string]string `json:"project_urls"`
		Keywords        string `json:"keywords"`
	} `json:"info"`
}

// Search implements Source. `query` is informational for PyPI — we walk
// the configured seed list and fetch fresh metadata from JSON API.
// This keeps us resilient to PyPI's missing search endpoint.
func (s *PyPISource) Search(ctx context.Context, query string, limit int) ([]Package, error) {
	packages := make([]Package, 0, len(s.PackageSeed))
	for _, name := range s.PackageSeed {
		if len(packages) >= limit {
			break
		}
		p, err := s.fetchOne(ctx, name)
		if err != nil {
			continue // skip quietly; importer aggregates per-source errors only if zero packages returned
		}
		// If query was supplied and non-default, substring-match against name/summary.
		if query != "" && query != "mcp" {
			q := strings.ToLower(query)
			if !strings.Contains(strings.ToLower(p.Name), q) &&
				!strings.Contains(strings.ToLower(p.Description), q) {
				continue
			}
		}
		packages = append(packages, p)
	}
	return packages, nil
}

func (s *PyPISource) fetchOne(ctx context.Context, name string) (Package, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/pypi/%s/json", s.BaseURL, url.PathEscape(name)), nil)
	if err != nil {
		return Package{}, err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return Package{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snip, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return Package{}, fmt.Errorf("pypi: %d %s", resp.StatusCode, string(snip))
	}
	var info pypiInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return Package{}, fmt.Errorf("pypi decode: %w", err)
	}
	web := info.Info.HomePage
	if web == "" {
		web = info.Info.ProjectURL
	}
	repo := info.Info.ProjectURLs["Source"]
	if repo == "" {
		repo = info.Info.ProjectURLs["Repository"]
	}
	return Package{
		Name:         info.Info.Name,
		Identifier:   info.Info.Name,
		Title:        info.Info.Name,
		Description:  info.Info.Summary,
		Version:      info.Info.Version,
		WebsiteURL:   web,
		Repository:   repo,
		RegistryType: "pypi",
	}, nil
}
