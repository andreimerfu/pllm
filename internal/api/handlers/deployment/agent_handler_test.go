package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	deploysvc "github.com/amerfu/pllm/internal/services/deployment"
)

func TestAgentHandlerDeploy(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	useCaseID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: useCaseID},
		Name:      "uc", Slug: "uc",
	}).Error)
	pkgs, _ := json.Marshal([]map[string]string{{
		"registry_type": "oci",
		"identifier":    "ghcr.io/acme/a",
		"version":       "0.1.0",
	}})
	agentID := uuid.New()
	require.NoError(t, db.Create(&models.RegistryAgent{
		BaseModel: models.BaseModel{ID: agentID},
		Name:      "pllm.io/a", Version: "0.1.0",
		Packages: datatypes.JSON(pkgs),
	}).Error)

	k8s := fake.NewSimpleClientset()
	svc := deploysvc.NewService(db, nil, nil,
		deploysvc.NewAgentK8sAdapter(k8s, config.DeploymentK8sConfig{AgentNamespace: "pllm-agents"}, nil),
		deploysvc.NewAgentSecretHelper(db, k8s, nil),
		nil, "pllm-mcp", "pllm-agents",
		map[string]string{"PLLM_BASE_URL": "http://pllm:8080"})

	h := NewAgentHandler(nil, svc)
	body, _ := json.Marshal(map[string]any{
		"agent_id":    agentID,
		"use_case_id": useCaseID,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/agents", bytes.NewReader(body)).
		WithContext(context.Background())
	rec := httptest.NewRecorder()
	h.Deploy(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	deps, err := k8s.AppsV1().Deployments("pllm-agents").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, deps.Items, 1)
}
