package use_cases_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/amerfu/pllm/internal/services/use_cases"
)

// TestReconcilerLLMClassificationsFlowUp seeds a UserModel with
// data_classifications=["pii"], references it from a use case, and
// verifies the reconciler lifts that classification onto the use case's
// effective_classifications.
func TestReconcilerLLMClassificationsFlowUp(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Seed a UserModel (LLM) that touches PII.
	um := models.UserModel{
		ModelName:           "openai/gpt-4o",
		DataClassifications: models.StringArrayJSON{"pii"},
		Enabled:             true,
	}
	require.NoError(t, db.Create(&um).Error)

	svc := use_cases.NewService(db, nil)
	uc, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name:                "Customer Support",
		DataClassifications: []string{"customer-financial"},
		Refs: []models.RegistryRef{
			{TargetKind: models.RegistryKindLLM, TargetName: "openai/gpt-4o"},
		},
	})
	require.NoError(t, err)

	// Trigger synchronously to avoid relying on the background goroutine
	// during the test.
	require.NoError(t, svc.TriggerRecompute(context.Background(), uc.UseCase.ID))

	reread, err := svc.Get(context.Background(), uc.UseCase.ID)
	require.NoError(t, err)
	require.ElementsMatch(t,
		[]string{"customer-financial", "pii"},
		[]string(reread.UseCase.EffectiveClassifications),
		"expected use case to inherit pii from the referenced LLM")
	require.NotNil(t, reread.UseCase.LastGraphComputedAt)
}

// TestReconcilerNoOpWhenNoComponents verifies that a use case with its own
// classifications and zero deps still ends up with effective = declared.
func TestReconcilerNoOpWhenNoComponents(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := use_cases.NewService(db, nil)
	uc, err := svc.Create(context.Background(), &use_cases.UpsertInput{
		Name:                "Simple",
		DataClassifications: []string{"confidential"},
	})
	require.NoError(t, err)

	require.NoError(t, svc.TriggerRecompute(context.Background(), uc.UseCase.ID))
	reread, err := svc.Get(context.Background(), uc.UseCase.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"confidential"}, []string(reread.UseCase.EffectiveClassifications))
}
