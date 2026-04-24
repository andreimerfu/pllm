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

// NPMSource queries the public npm search endpoint.
type NPMSource struct {
	Client  *http.Client
	BaseURL string // default "https://registry.npmjs.org"
}

// NewNPMSource builds an NPMSource with sane defaults.
func NewNPMSource(client *http.Client) *NPMSource {
	if client == nil {
		client = newHTTPClient(15 * time.Second)
	}
	return &NPMSource{Client: client, BaseURL: "https://registry.npmjs.org"}
}

// Name implements Source.
func (s *NPMSource) Name() string { return "npm" }

// npm search response shape (abbreviated).
type npmSearchResponse struct {
	Objects []struct {
		Package struct {
			Name        string   `json:"name"`
			Version     string   `json:"version"`
			Description string   `json:"description"`
			Keywords    []string `json:"keywords"`
			Links       struct {
				Homepage   string `json:"homepage"`
				Repository string `json:"repository"`
			} `json:"links"`
		} `json:"package"`
	} `json:"objects"`
}

// Search implements Source. We prefer the keyword qualifier so we get
// packages that actually tag themselves as MCP servers rather than anything
// mentioning "mcp" in prose.
func (s *NPMSource) Search(ctx context.Context, query string, limit int) ([]Package, error) {
	if query == "" {
		query = "mcp"
	}
	q := url.Values{}
	q.Set("text", fmt.Sprintf("keywords:%s", query))
	q.Set("size", fmt.Sprint(limit))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/-/v1/search?%s", s.BaseURL, q.Encode()), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("npm: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snip, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("npm: http %d: %s", resp.StatusCode, string(snip))
	}
	var out npmSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("npm: decode: %w", err)
	}
	packages := make([]Package, 0, len(out.Objects))
	for _, o := range out.Objects {
		p := o.Package
		packages = append(packages, Package{
			Name:         p.Name,
			Identifier:   p.Name,
			Title:        p.Name,
			Description:  p.Description,
			Version:      p.Version,
			WebsiteURL:   p.Links.Homepage,
			Repository:   p.Links.Repository,
			RegistryType: "npm",
		})
	}
	return packages, nil
}

// KeywordsContain is a small helper for tests or callers wanting to
// post-filter hits by keyword presence.
func KeywordsContain(keywords []string, want string) bool {
	want = strings.ToLower(want)
	for _, k := range keywords {
		if strings.ToLower(k) == want {
			return true
		}
	}
	return false
}
