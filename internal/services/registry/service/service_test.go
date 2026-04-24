package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/amerfu/pllm/internal/services/registry/service"
)

func TestServerUpsertAndLatestTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := service.NewServerService(db, nil)
	ctx := context.Background()

	// Insert v1 — becomes latest.
	out1, err := svc.Upsert(ctx, &models.RegistryServer{
		Name: "io.example/fs", Version: "1.0.0", Description: "first",
	})
	require.NoError(t, err)
	require.True(t, out1.IsLatest, "first version should be latest")

	// Insert v2 — flips v1 off.
	out2, err := svc.Upsert(ctx, &models.RegistryServer{
		Name: "io.example/fs", Version: "2.0.0", Description: "second",
	})
	require.NoError(t, err)
	require.True(t, out2.IsLatest, "second version should be latest")

	// Reload v1 — is_latest must now be false.
	v1, err := svc.Get(ctx, "io.example/fs", "1.0.0")
	require.NoError(t, err)
	require.False(t, v1.IsLatest, "v1 should no longer be latest")

	// "latest" resolves to v2.
	latest, err := svc.Get(ctx, "io.example/fs", "latest")
	require.NoError(t, err)
	require.Equal(t, "2.0.0", latest.Version)
	require.Equal(t, "second", latest.Description)

	// Overwriting v2 with new description should keep it as latest.
	out2b, err := svc.Upsert(ctx, &models.RegistryServer{
		Name: "io.example/fs", Version: "2.0.0", Description: "second (patched)",
	})
	require.NoError(t, err)
	require.Equal(t, out2.ID, out2b.ID, "overwrite should keep the same row ID")
	require.Equal(t, "second (patched)", out2b.Description)
	require.True(t, out2b.IsLatest)

	// List versions returns both.
	versions, err := svc.ListVersions(ctx, "io.example/fs")
	require.NoError(t, err)
	require.Len(t, versions, 2)
	// Newest first.
	require.Equal(t, "2.0.0", versions[0].Version)

	// Soft-delete the latest version — v1 should be promoted.
	require.NoError(t, svc.SoftDelete(ctx, "io.example/fs", "2.0.0"))
	latest, err = svc.Get(ctx, "io.example/fs", "latest")
	require.NoError(t, err)
	require.Equal(t, "1.0.0", latest.Version,
		"after deleting v2, v1 should be promoted to latest")

	// Soft-deleted row is hidden from the default list.
	list, err := svc.List(ctx, service.ListFilter{})
	require.NoError(t, err)
	for _, r := range list.Items {
		require.NotEqual(t, "2.0.0", r.Version, "soft-deleted row should not appear")
	}
}

func TestServerSearchFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := service.NewServerService(db, nil)
	ctx := context.Background()

	for _, row := range []models.RegistryServer{
		{Name: "io.example/fs", Version: "1.0.0", Description: "filesystem tools"},
		{Name: "io.example/github", Version: "1.0.0", Description: "github integration"},
		{Name: "io.other/slack", Version: "1.0.0", Description: "slack bot"},
	} {
		row := row
		_, err := svc.Upsert(ctx, &row)
		require.NoError(t, err)
	}

	got, err := svc.List(ctx, service.ListFilter{Search: "github"})
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.Equal(t, "io.example/github", got.Items[0].Name)

	// Search by description text.
	got, err = svc.List(ctx, service.ListFilter{Search: "bot"})
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.Equal(t, "io.other/slack", got.Items[0].Name)
}

func TestAgentUpsertWithRefs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	agents := service.NewAgentService(db, nil)
	ctx := context.Background()

	packages, _ := json.Marshal([]any{
		map[string]any{"registry_type": "npm", "identifier": "@org/a", "version": "1.0.0"},
	})

	out, err := agents.Upsert(ctx, &service.UpsertInput{
		Agent: models.RegistryAgent{
			Name: "acme/triage-bot", Version: "0.1.0",
			Description: "categorizes incoming tickets",
			Framework:   "langgraph", Language: "python",
			Packages: packages,
		},
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindServer, TargetName: "io.example/fs", TargetVersion: "2.0.0", LocalName: "fs"},
			{TargetKind: models.RegistryKindSkill, TargetName: "acme/ticket-triage", LocalName: "triage"},
			{TargetKind: models.RegistryKindPrompt, TargetName: "acme/classifier-system"},
		},
	})
	require.NoError(t, err)
	require.True(t, out.Agent.IsLatest)
	require.Len(t, out.Servers, 1)
	require.Len(t, out.Skills, 1)
	require.Len(t, out.Prompts, 1)
	require.Equal(t, "fs", out.Servers[0].LocalName)

	// Overwriting with fewer refs should drop the old ones.
	out2, err := agents.Upsert(ctx, &service.UpsertInput{
		Agent: models.RegistryAgent{
			Name: "acme/triage-bot", Version: "0.1.0",
			Description: "now simpler",
		},
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindPrompt, TargetName: "acme/classifier-system"},
		},
	})
	require.NoError(t, err)
	require.Empty(t, out2.Servers)
	require.Empty(t, out2.Skills)
	require.Len(t, out2.Prompts, 1)

	// Get should round-trip the refs for the latest version.
	reread, err := agents.Get(ctx, "acme/triage-bot", "")
	require.NoError(t, err)
	require.Equal(t, "0.1.0", reread.Agent.Version)
	require.Len(t, reread.Prompts, 1)
}

func TestGetNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := service.NewServerService(db, nil)
	_, err := svc.Get(context.Background(), "never/exists", "1.0.0")
	require.ErrorIs(t, err, service.ErrNotFound)
}

func TestNameValidation(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	svc := service.NewServerService(db, nil)

	_, err := svc.Upsert(context.Background(), &models.RegistryServer{
		Name: "bad name!", Version: "1.0.0",
	})
	require.Error(t, err)
}
