package use_cases_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/amerfu/pllm/internal/services/use_cases"
)

func TestScopeAllowLLM(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	enforce := true
	uc, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name:         "Support",
		EnforceScope: &enforce,
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindLLM, TargetName: "anthropic/claude-sonnet-4-5"},
		},
	})
	require.NoError(t, err)

	scope := svc.Scope()

	// Declared LLM allowed, in block mode.
	d := scope.AllowLLM(context.Background(), &uc.UseCase.ID, "anthropic/claude-sonnet-4-5")
	require.True(t, d.Allowed)
	require.True(t, d.EnforceMode)

	// Undeclared LLM denied, in block mode.
	d = scope.AllowLLM(context.Background(), &uc.UseCase.ID, "openai/gpt-4o")
	require.False(t, d.Allowed)
	require.True(t, d.EnforceMode)
	require.Contains(t, d.Reason, "not declared")
}

func TestScopeFlagModeAllowsOutOfScope(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	// EnforceScope defaults to false — flag mode.
	uc, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name: "Support",
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindLLM, TargetName: "anthropic/claude-sonnet-4-5"},
		},
	})
	require.NoError(t, err)

	scope := svc.Scope()
	d := scope.AllowLLM(context.Background(), &uc.UseCase.ID, "openai/gpt-4o")
	require.False(t, d.Allowed, "allowed must still report 'not-in-scope'")
	require.False(t, d.EnforceMode, "flag mode should surface as EnforceMode=false")
}

func TestScopeUnscopedKeyBypasses(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	scope := svc.Scope()

	// nil UseCaseID → bypass.
	d := scope.AllowLLM(context.Background(), nil, "anything")
	require.True(t, d.Allowed)
	require.False(t, d.EnforceMode)

	d2 := scope.AllowMCPTool(context.Background(), nil, "anything")
	require.True(t, d2.Allowed)
	require.False(t, d2.EnforceMode)
}

func TestScopeMCPToolMatchesSlugVariants(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	uc, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name: "Filesystem",
		Refs: []models.RegistryRef{
			{
				TargetKind: models.RegistryKindServer,
				TargetName: "io.mcp/server-everything",
				LocalName:  "fs",
			},
		},
	})
	require.NoError(t, err)

	scope := svc.Scope()

	// Accepts the full registry name.
	d := scope.AllowMCPTool(context.Background(), &uc.UseCase.ID, "io.mcp/server-everything")
	require.True(t, d.Allowed)

	// Accepts the last-segment shorthand (matches how gateway slugs are often named).
	d = scope.AllowMCPTool(context.Background(), &uc.UseCase.ID, "server-everything")
	require.True(t, d.Allowed)

	// Accepts the local_name alias (matches auto-registered gateway slugs).
	d = scope.AllowMCPTool(context.Background(), &uc.UseCase.ID, "fs")
	require.True(t, d.Allowed)

	// Rejects an unrelated slug.
	d = scope.AllowMCPTool(context.Background(), &uc.UseCase.ID, "github")
	require.False(t, d.Allowed)
}

func TestScopeInvalidateOnUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	uc, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name: "X",
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindLLM, TargetName: "a"},
		},
	})
	require.NoError(t, err)

	scope := svc.Scope()
	// Prime the cache.
	d := scope.AllowLLM(context.Background(), &uc.UseCase.ID, "a")
	require.True(t, d.Allowed)
	d = scope.AllowLLM(context.Background(), &uc.UseCase.ID, "b")
	require.False(t, d.Allowed)

	// Update the use case so "b" becomes allowed.
	_, err = svc.Update(context.Background(), uc.UseCase.ID, &use_cases.UpsertInput{
		Name: "X",
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindLLM, TargetName: "b"},
		},
	})
	require.NoError(t, err)

	// Next check must reflect the new refs — no stale cache.
	d = scope.AllowLLM(context.Background(), &uc.UseCase.ID, "b")
	require.True(t, d.Allowed)
	d = scope.AllowLLM(context.Background(), &uc.UseCase.ID, "a")
	require.False(t, d.Allowed)
}

// sanity: random UUID lookups don't blow up.
func TestScopeNonExistentUseCase(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	scope := svc.Scope()

	id := uuid.New()
	d := scope.AllowLLM(context.Background(), &id, "a")
	// Unknown UC is treated as permissive; EnforceMode=false ensures we
	// don't start blocking just because the admin UI has a stale link.
	require.True(t, d.Allowed)
	require.False(t, d.EnforceMode)
}
