package importer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/amerfu/pllm/internal/services/registry/importer"
	"github.com/amerfu/pllm/internal/services/registry/service"
)

func TestNPMSourceSearch(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/-/v1/search", r.URL.Path)
		require.Equal(t, "keywords:mcp", r.URL.Query().Get("text"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"objects": []map[string]any{
				{
					"package": map[string]any{
						"name":        "@modelcontextprotocol/server-filesystem",
						"version":     "1.2.3",
						"description": "Filesystem MCP server",
						"keywords":    []string{"mcp"},
						"links": map[string]string{
							"homepage":   "https://mcp.example",
							"repository": "https://github.com/org/x",
						},
					},
				},
			},
		})
	}))
	defer mock.Close()

	src := importer.NewNPMSource(nil)
	src.BaseURL = mock.URL
	hits, err := src.Search(context.Background(), "mcp", 50)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, "@modelcontextprotocol/server-filesystem", hits[0].Name)
	require.Equal(t, "npm", hits[0].RegistryType)
	require.Equal(t, "1.2.3", hits[0].Version)
}

func TestPyPISourceFetch(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"info": map[string]any{
				"name":    "mcp-server-sqlite",
				"version": "0.4.2",
				"summary": "SQLite MCP server",
				"project_urls": map[string]string{
					"Source": "https://github.com/org/mcp-sqlite",
				},
			},
		})
	}))
	defer mock.Close()

	src := importer.NewPyPISource(nil)
	src.BaseURL = mock.URL
	src.PackageSeed = []string{"mcp-server-sqlite"}
	hits, err := src.Search(context.Background(), "", 10)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, "pypi", hits[0].RegistryType)
	require.Equal(t, "0.4.2", hits[0].Version)
	require.Equal(t, "https://github.com/org/mcp-sqlite", hits[0].Repository)
}

func TestImporterServiceEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	npmMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"objects": []map[string]any{
				{"package": map[string]any{
					"name": "@org/mcp-a", "version": "1.0.0", "description": "first",
					"keywords": []string{"mcp"},
				}},
				{"package": map[string]any{
					"name": "@org/mcp-b", "version": "2.0.0", "description": "second",
					"keywords": []string{"mcp"},
				}},
			},
		})
	}))
	defer npmMock.Close()

	npmSrc := importer.NewNPMSource(nil)
	npmSrc.BaseURL = npmMock.URL

	servers := service.NewServerService(db, nil)
	imp := importer.NewService(servers, nil, npmSrc)
	reports, err := imp.Import(context.Background(), "mcp", 100)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, 2, reports[0].Found)
	require.Equal(t, 2, reports[0].Imported)

	// Both packages should now be in the registry with the "npm.import/" prefix.
	list, err := servers.List(context.Background(), service.ListFilter{})
	require.NoError(t, err)
	var imported []models.RegistryServer
	for _, r := range list.Items {
		if r.Name == "npm.import/@org/mcp-a" || r.Name == "npm.import/@org/mcp-b" {
			imported = append(imported, r)
		}
	}
	require.Len(t, imported, 2)
}
