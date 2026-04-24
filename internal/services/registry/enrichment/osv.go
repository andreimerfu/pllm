// Package enrichment scores registry resources using external scanners.
// Only OSV (vulnerability lookup) is implemented in Phase 2; Scorecard,
// container scan, and dependency health plug in later behind the same
// Scanner interface.
package enrichment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/amerfu/pllm/internal/core/models"
)

// Scanner produces an EnrichmentScore for a given RegistryServer.
// We bind to servers specifically because packages-to-query is the input.
type Scanner interface {
	Type() models.EnrichmentType
	Scan(ctx context.Context, server *models.RegistryServer) (*ScanResult, error)
}

// ScanResult is the output of a Scanner run. Score in [0, 100].
type ScanResult struct {
	Score    float64
	Summary  string
	Findings any
}

// OSVScanner queries the public OSV API (https://api.osv.dev/v1/querybatch)
// for each package listed in a RegistryServer's Packages field and
// aggregates the severity into a penalty-based score.
type OSVScanner struct {
	Client  *http.Client
	BaseURL string // override for tests; default "https://api.osv.dev"
}

// Type implements Scanner.
func (s *OSVScanner) Type() models.EnrichmentType { return models.EnrichmentTypeOSV }

// Scan implements Scanner.
func (s *OSVScanner) Scan(ctx context.Context, server *models.RegistryServer) (*ScanResult, error) {
	if server == nil {
		return nil, errors.New("osv: server is nil")
	}
	queries, err := packagesToQueries(server.Packages)
	if err != nil {
		return nil, err
	}
	if len(queries) == 0 {
		// Nothing to scan = perfect score (no packages = no known risk).
		return &ScanResult{Score: 100, Summary: "no packages to scan"}, nil
	}
	res, err := s.postBatch(ctx, queries)
	if err != nil {
		return nil, err
	}
	score, summary, findings := summarize(res)
	return &ScanResult{Score: score, Summary: summary, Findings: findings}, nil
}

// --- wire types ----------------------------------------------------------

type osvPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

type osvQuery struct {
	Package osvPackage `json:"package"`
	Version string     `json:"version"`
}

type osvBatchRequest struct {
	Queries []osvQuery `json:"queries"`
}

type osvSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"` // CVSS vector
}

type osvVuln struct {
	ID       string        `json:"id"`
	Severity []osvSeverity `json:"severity,omitempty"`
}

type osvBatchResponse struct {
	Results []struct {
		Vulns []osvVuln `json:"vulns"`
	} `json:"results"`
}

// packagesToQueries converts the server's Packages jsonb into OSV queries.
// We accept the shape published by both agentregistry and pllm:
//
//	[{"registry_type":"npm","identifier":"@foo/bar","version":"1.2.3"}]
func packagesToQueries(raw []byte) ([]osvQuery, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var entries []struct {
		RegistryType string `json:"registry_type"`
		Identifier   string `json:"identifier"`
		Version      string `json:"version"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("osv: parse packages: %w", err)
	}
	out := make([]osvQuery, 0, len(entries))
	for _, e := range entries {
		eco := ecosystemFor(e.RegistryType)
		if eco == "" || e.Identifier == "" || e.Version == "" {
			continue
		}
		out = append(out, osvQuery{
			Package: osvPackage{Name: e.Identifier, Ecosystem: eco},
			Version: e.Version,
		})
	}
	return out, nil
}

// ecosystemFor translates a package registry type to the OSV ecosystem name.
func ecosystemFor(registryType string) string {
	switch strings.ToLower(registryType) {
	case "npm":
		return "npm"
	case "pypi":
		return "PyPI"
	case "oci", "docker":
		// OSV does not scan OCI directly; return "" to skip.
		return ""
	case "go":
		return "Go"
	case "cargo":
		return "crates.io"
	case "maven":
		return "Maven"
	case "nuget":
		return "NuGet"
	case "rubygems":
		return "RubyGems"
	}
	return ""
}

func (s *OSVScanner) postBatch(ctx context.Context, queries []osvQuery) (*osvBatchResponse, error) {
	base := s.BaseURL
	if base == "" {
		base = "https://api.osv.dev"
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	body, err := json.Marshal(osvBatchRequest{Queries: queries})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/querybatch", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("osv: post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snip, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("osv: http %d: %s", resp.StatusCode, string(snip))
	}
	var out osvBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("osv: decode: %w", err)
	}
	return &out, nil
}

// summarize converts an OSV response into (score, summary, findings).
// Scoring rule: start at 100, subtract per vuln by severity:
//   critical (>=9) -> 30, high (>=7) -> 15, medium (>=4) -> 6, low -> 2.
// Floor at 0. Unknown severities treated as low.
func summarize(res *osvBatchResponse) (float64, string, any) {
	var critical, high, medium, low int
	var vulnIDs []string
	for _, r := range res.Results {
		for _, v := range r.Vulns {
			vulnIDs = append(vulnIDs, v.ID)
			sev := worstCVSS(v.Severity)
			switch {
			case sev >= 9:
				critical++
			case sev >= 7:
				high++
			case sev >= 4:
				medium++
			default:
				low++
			}
		}
	}
	score := 100.0 - float64(critical)*30 - float64(high)*15 - float64(medium)*6 - float64(low)*2
	if score < 0 {
		score = 0
	}
	total := critical + high + medium + low
	var summary string
	if total == 0 {
		summary = "no known vulnerabilities"
	} else {
		summary = fmt.Sprintf("%d vulns (C:%d H:%d M:%d L:%d)",
			total, critical, high, medium, low)
	}
	return score, summary, map[string]any{
		"critical":     critical,
		"high":         high,
		"medium":       medium,
		"low":          low,
		"vuln_ids":     vulnIDs,
	}
}

// worstCVSS picks the highest numeric base score from a list of CVSS
// severity strings. OSV mixes CVSS_V2/V3 vectors; we look for the "AV:"
// preamble and the trailing score in the vector string, or a decimal prefix.
func worstCVSS(sevs []osvSeverity) float64 {
	best := 0.0
	for _, s := range sevs {
		score := parseCVSSScore(s.Score)
		if score > best {
			best = score
		}
	}
	return best
}

// parseCVSSScore handles both bare numeric strings ("7.5") and CVSS vectors
// that may include a base-score suffix. Returns 0 on failure.
func parseCVSSScore(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Some scanners encode the score at the end after a slash, e.g.
	// "CVSS:3.1/AV:N/...; base=7.5". We just try strconv.ParseFloat on
	// progressively-shortened tails and slashes.
	for _, part := range strings.Split(s, "/") {
		part = strings.TrimSpace(part)
		if f, ok := parseFloat(part); ok {
			return f
		}
	}
	if f, ok := parseFloat(s); ok {
		return f
	}
	return 0
}

func parseFloat(s string) (float64, bool) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	if err != nil {
		return 0, false
	}
	return f, true
}
