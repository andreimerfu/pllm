package use_cases_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/amerfu/pllm/internal/services/use_cases"
)

func TestCreateAndGetWithRefs(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	out, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name:        "Mortgage Adverse Action",
		Description: "Drafts adverse action letters for rejected mortgage apps.",
		RiskTier:    models.UseCaseRiskTier1,
		Status:      models.UseCaseStatusUnderReview,
		DataClassifications: []string{"pii", "customer-financial"},
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindServer, TargetName: "io.mcp/customer-db"},
			{TargetKind: models.RegistryKindAgent, TargetName: "acme/triage-agent", TargetVersion: "0.1.0"},
			{TargetKind: models.RegistryKindLLM, TargetName: "anthropic/claude-sonnet-4-5", LocalName: "planner"},
			{TargetKind: models.RegistryKindPrompt, TargetName: "acme/adverse-action-template"},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, out.UseCase.Slug)
	require.Equal(t, "mortgage-adverse-action", out.UseCase.Slug)
	require.Equal(t, models.UseCaseRiskTier1, out.UseCase.RiskTier)
	require.Len(t, out.Servers, 1)
	require.Len(t, out.Agents, 1)
	require.Len(t, out.LLMs, 1)
	require.Len(t, out.Prompts, 1)
	require.Equal(t, "planner", out.LLMs[0].LocalName)

	// Round-trip via slug.
	reread, err := svc.GetBySlug(context.Background(), "mortgage-adverse-action")
	require.NoError(t, err)
	require.Equal(t, out.UseCase.ID, reread.UseCase.ID)
	require.Len(t, reread.Servers, 1)
}

func TestUpdateReplacesRefsWholesale(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	created, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name: "Fraud Scoring",
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindServer, TargetName: "fraud/lookup"},
			{TargetKind: models.RegistryKindLLM, TargetName: "openai/gpt-4o"},
		},
	})
	require.NoError(t, err)

	updated, err := svc.Update(context.Background(), created.UseCase.ID, &use_cases.UpsertInput{
		Name: "Fraud Scoring",
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindLLM, TargetName: "anthropic/claude-sonnet-4-5"},
		},
	})
	require.NoError(t, err)
	require.Empty(t, updated.Servers, "old server ref must be cleared")
	require.Len(t, updated.LLMs, 1)
	require.Equal(t, "anthropic/claude-sonnet-4-5", updated.LLMs[0].TargetName)
}

func TestSearchAndFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	for _, in := range []use_cases.UpsertInput{
		{Name: "Loan Triage", RiskTier: models.UseCaseRiskTier1, Status: models.UseCaseStatusApproved},
		{Name: "Support Chatbot", RiskTier: models.UseCaseRiskTier3, Status: models.UseCaseStatusDraft},
		{Name: "Loan Underwriting", RiskTier: models.UseCaseRiskTier2, Status: models.UseCaseStatusApproved},
	} {
		_, err := svc.Create(context.Background(), &in)
		require.NoError(t, err)
	}

	all, err := svc.List(context.Background(), use_cases.ListFilter{})
	require.NoError(t, err)
	require.Len(t, all, 3)

	hits, err := svc.List(context.Background(), use_cases.ListFilter{Search: "loan"})
	require.NoError(t, err)
	require.Len(t, hits, 2)

	tier1, err := svc.List(context.Background(), use_cases.ListFilter{RiskTier: models.UseCaseRiskTier1})
	require.NoError(t, err)
	require.Len(t, tier1, 1)
	require.Equal(t, "Loan Triage", tier1[0].Name)

	approved, err := svc.List(context.Background(), use_cases.ListFilter{Status: models.UseCaseStatusApproved})
	require.NoError(t, err)
	require.Len(t, approved, 2)
}

// TestComplianceFilters verifies the cross-cutting-query filters that
// power the /compliance page: effective classification + compliance flag
// + third-party LLM.
func TestComplianceFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Seed LLMs: one third-party, one first-party.
	require.NoError(t, db.Create(&models.UserModel{
		ModelName:    "openai/gpt-4o",
		IsFirstParty: false,
		Enabled:      true,
	}).Error)
	require.NoError(t, db.Create(&models.UserModel{
		ModelName:    "local/ollama-llama3",
		IsFirstParty: true,
		Enabled:      true,
	}).Error)

	svc := use_cases.NewService(db, nil)

	// Three use cases with distinct compliance shapes.
	withPII, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name:                "Mortgage Triage",
		DataClassifications: []string{"pii", "customer-financial"},
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindLLM, TargetName: "openai/gpt-4o"},
		},
	})
	require.NoError(t, err)

	_, err = svc.Create(context.Background(), &use_cases.UpsertInput{
		Name:                "Public FAQ",
		DataClassifications: []string{"public"},
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindLLM, TargetName: "local/ollama-llama3"},
		},
	})
	require.NoError(t, err)

	flagged, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name:                "Operations Copilot",
		DataClassifications: []string{"confidential"},
	})
	require.NoError(t, err)
	// Force the flag (would normally be set by the reconciler).
	require.NoError(t, db.Model(&models.UseCase{}).
		Where("id = ?", flagged.UseCase.ID).
		Updates(map[string]any{
			"compliance_flag":        true,
			"compliance_flag_reason": "test",
		}).Error)

	// Effective classification filter (after the background reconciler
	// runs for the PII one, but we also check the declared-class filter).
	hits, err := svc.List(context.Background(), use_cases.ListFilter{
		Classification: "pii",
	})
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, withPII.UseCase.Name, hits[0].Name)

	// Compliance flag filter.
	yes := true
	hits, err = svc.List(context.Background(), use_cases.ListFilter{
		ComplianceFlag: &yes,
	})
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, "Operations Copilot", hits[0].Name)

	// Third-party filter — should surface the one using gpt-4o.
	hits, err = svc.List(context.Background(), use_cases.ListFilter{
		HasThirdParty: &yes,
	})
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, withPII.UseCase.Name, hits[0].Name)

	// Inverse: first-party-only.
	no := false
	hits, err = svc.List(context.Background(), use_cases.ListFilter{
		HasThirdParty: &no,
	})
	require.NoError(t, err)
	// Public FAQ (ollama) + Operations Copilot (no LLMs at all — counts as not-third-party).
	require.Len(t, hits, 2)
}

func TestDeleteCascadesRefs(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	uc, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name: "Temp",
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindServer, TargetName: "x"},
		},
	})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), uc.UseCase.ID))

	var refCount int64
	require.NoError(t, db.Model(&models.RegistryRef{}).
		Where("owner_kind = ? AND owner_id = ?", models.RegistryKindUseCase, uc.UseCase.ID).
		Count(&refCount).Error)
	require.Zero(t, refCount, "refs must be deleted when use case is removed")

	_, err = svc.Get(context.Background(), uc.UseCase.ID)
	require.ErrorIs(t, err, use_cases.ErrNotFound)
}

func TestInvalidTargetKindRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	_, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name: "Bad",
		Refs: []models.RegistryRef{
			{TargetKind: "use_case", TargetName: "x"}, // not a valid target
		},
	})
	require.Error(t, err)
}
