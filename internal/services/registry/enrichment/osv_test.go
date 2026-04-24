package enrichment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/amerfu/pllm/internal/core/models"
)

func TestPackagesToQueries(t *testing.T) {
	pkgs, _ := json.Marshal([]map[string]any{
		{"registry_type": "npm", "identifier": "@org/a", "version": "1.0.0"},
		{"registry_type": "oci", "identifier": "ghcr.io/org/x", "version": "1"},
		{"registry_type": "pypi", "identifier": "org-b", "version": "2.0"},
	})
	qs, err := packagesToQueries(pkgs)
	require.NoError(t, err)
	require.Len(t, qs, 2, "oci should be skipped (not scannable by OSV)")
	require.Equal(t, "npm", qs[0].Package.Ecosystem)
	require.Equal(t, "@org/a", qs[0].Package.Name)
	require.Equal(t, "PyPI", qs[1].Package.Ecosystem)
}

func TestScoringPenalties(t *testing.T) {
	// One critical + one medium = -30 -6 = 64.
	res := &osvBatchResponse{Results: []struct {
		Vulns []osvVuln `json:"vulns"`
	}{
		{Vulns: []osvVuln{
			{ID: "CVE-1", Severity: []osvSeverity{{Type: "CVSS_V3", Score: "9.8"}}},
		}},
		{Vulns: []osvVuln{
			{ID: "CVE-2", Severity: []osvSeverity{{Type: "CVSS_V3", Score: "5.5"}}},
		}},
	}}
	score, summary, _ := summarize(res)
	require.Equal(t, 64.0, score)
	require.Contains(t, summary, "2 vulns")
}

func TestScoringFloorsAtZero(t *testing.T) {
	var results []struct {
		Vulns []osvVuln `json:"vulns"`
	}
	// Ten criticals = -300, should floor to 0.
	for i := 0; i < 10; i++ {
		results = append(results, struct {
			Vulns []osvVuln `json:"vulns"`
		}{Vulns: []osvVuln{{Severity: []osvSeverity{{Score: "9.5"}}}}})
	}
	score, _, _ := summarize(&osvBatchResponse{Results: results})
	require.Equal(t, 0.0, score)
}

func TestScanAgainstFakeOSV(t *testing.T) {
	// OSV mock returns one high vuln.
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/querybatch", r.URL.Path)
		resp := osvBatchResponse{Results: []struct {
			Vulns []osvVuln `json:"vulns"`
		}{
			{Vulns: []osvVuln{{ID: "GHSA-xxxx", Severity: []osvSeverity{{Score: "7.5"}}}}},
		}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mock.Close()

	pkgs, _ := json.Marshal([]map[string]any{
		{"registry_type": "npm", "identifier": "@org/a", "version": "1.0.0"},
	})
	s := &OSVScanner{BaseURL: mock.URL}
	result, err := s.Scan(context.Background(), &models.RegistryServer{
		Name: "x", Version: "1", Packages: datatypes.JSON(pkgs),
	})
	require.NoError(t, err)
	require.Equal(t, 85.0, result.Score) // -15 for high
	require.Contains(t, result.Summary, "1 vulns")
	m, ok := result.Findings.(map[string]any)
	require.True(t, ok)
	require.Equal(t, 1, m["high"].(int))
}

func TestScanSkipsWhenNoPackages(t *testing.T) {
	s := &OSVScanner{}
	result, err := s.Scan(context.Background(), &models.RegistryServer{
		Name: "x", Version: "1",
	})
	require.NoError(t, err)
	require.Equal(t, 100.0, result.Score)
	require.Contains(t, result.Summary, "no packages")
}
