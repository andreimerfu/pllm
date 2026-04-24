package a2a

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

func TestManagerResolveBySlugFindsAgentDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	ucID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: ucID},
		Name:      "uc", Slug: "uc",
	}).Error)

	require.NoError(t, db.Create(&models.AgentDeployment{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		RegistryAgentID: uuid.New(),
		ResourceName:    "pllm.io/sales-research",
		ResourceVersion: "0.1.0",
		UseCaseID:       ucID,
		Platform:        models.DeploymentPlatformKubernetes,
		Namespace:       "pllm-agents",
		WorkloadName:    "sales-research-0-1-0",
		Status:          models.DeploymentStatusRunning,
		Endpoint:        "http://sales-research-0-1-0.pllm-agents.svc.cluster.local:8000",
	}).Error)

	m := NewManager(db)

	got, err := m.Resolve(context.Background(), "sales-research-0-1-0")
	require.NoError(t, err)
	require.Equal(t, "pllm.io/sales-research", got.ResourceName)

	_, err = m.Resolve(context.Background(), "nope")
	require.ErrorIs(t, err, ErrAgentNotFound)
}
