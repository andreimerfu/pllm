package enrichment_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/amerfu/pllm/internal/services/registry/enrichment"
	"github.com/amerfu/pllm/internal/services/registry/service"
)

// TestRunnerEndToEnd: publish a server with an npm package, enqueue an
// OSV job against a mocked OSV endpoint, run once, assert we have a score.
func TestRunnerEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Fake OSV: one medium vuln.
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type vuln struct {
			ID       string `json:"id"`
			Severity []struct {
				Type  string `json:"type"`
				Score string `json:"score"`
			} `json:"severity"`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"vulns": []vuln{{
					ID: "CVE-1",
					Severity: []struct {
						Type  string `json:"type"`
						Score string `json:"score"`
					}{{Type: "CVSS_V3", Score: "5.0"}},
				}}},
			},
		})
	}))
	defer mock.Close()

	// Publish a server.
	servers := service.NewServerService(db, nil)
	pkgs, _ := json.Marshal([]map[string]any{{
		"registry_type": "npm", "identifier": "@org/thing", "version": "1.0.0",
	}})
	pub, err := servers.Upsert(context.Background(), &models.RegistryServer{
		Name: "io.test/thing", Version: "1.0.0", Packages: datatypes.JSON(pkgs),
	})
	require.NoError(t, err)

	// Runner with OSV pointed at mock.
	runner := enrichment.NewRunner(db, nil, &enrichment.OSVScanner{BaseURL: mock.URL})
	require.NoError(t, runner.Enqueue(context.Background(),
		models.RegistryKindServer, pub.ID, models.EnrichmentTypeOSV))

	ran, err := runner.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, ran)

	scores, err := runner.ScoresFor(context.Background(),
		models.RegistryKindServer, pub.ID)
	require.NoError(t, err)
	require.Len(t, scores, 1)
	// 100 - 6 (medium) = 94
	require.Equal(t, 94.0, scores[0].Score)
	require.Equal(t, models.EnrichmentTypeOSV, scores[0].Type)

	// Re-enqueue: dedup skips pending/running, but a succeeded job is done —
	// another enqueue creates a new pending one. Two Enqueue calls produce
	// exactly one extra pending job, not two.
	require.NoError(t, runner.Enqueue(context.Background(),
		models.RegistryKindServer, pub.ID, models.EnrichmentTypeOSV))
	require.NoError(t, runner.Enqueue(context.Background(),
		models.RegistryKindServer, pub.ID, models.EnrichmentTypeOSV))
	var pending int64
	require.NoError(t, db.Model(&models.EnrichmentJob{}).
		Where("resource_id = ? AND status = ?", pub.ID, models.JobStatusPending).
		Count(&pending).Error)
	require.Equal(t, int64(1), pending,
		"second Enqueue call should be deduped against the first still-pending one")
}
