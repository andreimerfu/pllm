package deployment

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

func TestDeployAgentEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	ctx := context.Background()

	useCaseID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: useCaseID},
		Name:      "customer-research-approved",
		Slug:      "customer-research-approved",
	}).Error)

	pkgs, err := json.Marshal([]map[string]string{{
		"registry_type": "oci",
		"identifier":    "ghcr.io/acme/sales-research",
		"version":       "0.1.0",
	}})
	require.NoError(t, err)
	agentID := uuid.New()
	require.NoError(t, db.Create(&models.RegistryAgent{
		BaseModel:   models.BaseModel{ID: agentID},
		Name:        "pllm.io/sales-research",
		Version:     "0.1.0",
		Title:       "Sales Research",
		Description: "demo agent",
		IsLatest:    true,
		Packages:    datatypes.JSON(pkgs),
	}).Error)

	k8s := fake.NewSimpleClientset()
	agentAdapter := NewAgentK8sAdapter(k8s, config.DeploymentK8sConfig{AgentNamespace: "pllm-agents"}, nil)
	secrets := NewAgentSecretHelper(db, k8s, nil)

	svc := NewService(db, nil, nil, agentAdapter, secrets, nil, "pllm-mcp", "pllm-agents",
		map[string]string{"PLLM_BASE_URL": "http://pllm:8080", "PLLM_MCP_URL": "http://pllm:8080/v1/mcp"})

	row, err := svc.DeployAgent(ctx, DeployAgentRequest{
		AgentID:   agentID,
		UseCaseID: useCaseID,
		Replicas:  1,
	})
	require.NoError(t, err)
	require.Equal(t, models.DeploymentStatusRunning, row.Status)
	require.NotNil(t, row.ProvisionedKeyID)
	require.NotEmpty(t, row.SecretName)

	var key models.Key
	require.NoError(t, db.First(&key, "id = ?", *row.ProvisionedKeyID).Error)
	require.NotNil(t, key.UseCaseID)
	require.Equal(t, useCaseID, *key.UseCaseID)

	_, err = k8s.AppsV1().Deployments("pllm-agents").Get(ctx, row.WorkloadName, metav1.GetOptions{})
	require.NoError(t, err)

	_, err = k8s.CoreV1().Secrets("pllm-agents").Get(ctx, row.SecretName, metav1.GetOptions{})
	require.NoError(t, err)
}

func TestUndeployAgentCleansUpKeyAndSecret(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	ctx := context.Background()

	useCaseID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: useCaseID},
		Name:      "uc", Slug: "uc",
	}).Error)
	pkgs, err := json.Marshal([]map[string]string{{
		"registry_type": "oci",
		"identifier":    "ghcr.io/acme/a",
		"version":       "0.1.0",
	}})
	require.NoError(t, err)
	agentID := uuid.New()
	require.NoError(t, db.Create(&models.RegistryAgent{
		BaseModel: models.BaseModel{ID: agentID},
		Name:      "pllm.io/a", Version: "0.1.0",
		Packages: datatypes.JSON(pkgs),
	}).Error)

	k8s := fake.NewSimpleClientset()
	svc := NewService(db, nil, nil,
		NewAgentK8sAdapter(k8s, config.DeploymentK8sConfig{AgentNamespace: "pllm-agents"}, nil),
		NewAgentSecretHelper(db, k8s, nil),
		nil, "pllm-mcp", "pllm-agents",
		map[string]string{"PLLM_BASE_URL": "http://pllm:8080"})

	row, err := svc.DeployAgent(ctx, DeployAgentRequest{AgentID: agentID, UseCaseID: useCaseID})
	require.NoError(t, err)
	keyID := *row.ProvisionedKeyID
	secretName := row.SecretName

	require.NoError(t, svc.UndeployAgent(ctx, row.ID))

	// Soft-deleted: live query returns 0.
	var liveCount int64
	require.NoError(t, db.Model(&models.Key{}).Where("id = ?", keyID).Count(&liveCount).Error)
	require.Equal(t, int64(0), liveCount)

	_, err = k8s.CoreV1().Secrets("pllm-agents").Get(ctx, secretName, metav1.GetOptions{})
	require.Error(t, err)
}
