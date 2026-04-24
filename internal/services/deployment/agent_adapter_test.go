package deployment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/core/models"
)

func newTestAgentAdapter() *AgentK8sAdapter {
	return NewAgentK8sAdapter(fake.NewSimpleClientset(), config.DeploymentK8sConfig{
		AgentNamespace: "pllm-agents",
	}, nil)
}

func TestAgentDeployCreatesDeploymentAndService(t *testing.T) {
	adapter := newTestAgentAdapter()
	res, err := adapter.Deploy(context.Background(), &AgentRequest{
		Namespace:       "pllm-agents",
		WorkloadName:    "sales-research-0-1-0",
		DisplayName:     "Sales Research",
		ResourceName:    "pllm.io/sales-research",
		ResourceVersion: "0.1.0",
		Image:           "ghcr.io/acme/sales-research:0.1.0",
		UseCaseID:       "11111111-1111-1111-1111-111111111111",
		SecretName:      "sales-research-0-1-0-key",
		Env: map[string]string{
			"PLLM_BASE_URL":   "http://pllm:8080",
			"PLLM_MCP_URL":    "http://pllm:8080/v1/mcp",
			"PLLM_USE_CASE":   "customer-research-approved",
			"PLLM_AGENT_NAME": "sales-research",
		},
		Replicas: 2,
	})
	require.NoError(t, err)
	require.Contains(t, res.Endpoint, "sales-research-0-1-0.pllm-agents.svc.cluster.local")

	dep, err := adapter.client.AppsV1().Deployments("pllm-agents").
		Get(context.Background(), "sales-research-0-1-0", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, int32(2), *dep.Spec.Replicas)

	container := dep.Spec.Template.Spec.Containers[0]
	require.Equal(t, "ghcr.io/acme/sales-research:0.1.0", container.Image)

	envMap := map[string]string{}
	for _, e := range container.Env {
		envMap[e.Name] = e.Value
	}
	require.Equal(t, "http://pllm:8080", envMap["PLLM_BASE_URL"])
	require.Equal(t, "customer-research-approved", envMap["PLLM_USE_CASE"])

	require.Len(t, container.EnvFrom, 1)
	require.NotNil(t, container.EnvFrom[0].SecretRef)
	require.Equal(t, "sales-research-0-1-0-key", container.EnvFrom[0].SecretRef.Name)

	svc, err := adapter.client.CoreV1().Services("pllm-agents").
		Get(context.Background(), "sales-research-0-1-0", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "agent", svc.Labels["pllm.ai/kind"])

	// Unused var suppressor.
	_ = models.DeploymentStatusPending
}

func TestAgentUndeployIsIdempotent(t *testing.T) {
	adapter := newTestAgentAdapter()
	ctx := context.Background()
	_, err := adapter.Deploy(ctx, &AgentRequest{
		Namespace:    "pllm-agents",
		WorkloadName: "a",
		Image:        "img:1",
		SecretName:   "a-key",
	})
	require.NoError(t, err)

	row := &models.AgentDeployment{Namespace: "pllm-agents", WorkloadName: "a"}
	require.NoError(t, adapter.Undeploy(ctx, row))
	require.NoError(t, adapter.Undeploy(ctx, row))

	_, err = adapter.client.AppsV1().Deployments("pllm-agents").Get(ctx, "a", metav1.GetOptions{})
	require.Error(t, err)
}

func TestAgentStatusReportsRunningWhenReady(t *testing.T) {
	adapter := newTestAgentAdapter()
	ctx := context.Background()
	_, err := adapter.Deploy(ctx, &AgentRequest{
		Namespace:    "pllm-agents",
		WorkloadName: "a",
		Image:        "img:1",
		SecretName:   "a-key",
	})
	require.NoError(t, err)

	dep, err := adapter.client.AppsV1().Deployments("pllm-agents").Get(ctx, "a", metav1.GetOptions{})
	require.NoError(t, err)
	dep.Status.ReadyReplicas = *dep.Spec.Replicas
	_, err = adapter.client.AppsV1().Deployments("pllm-agents").UpdateStatus(ctx, dep, metav1.UpdateOptions{})
	require.NoError(t, err)

	rep, err := adapter.Status(ctx, &models.AgentDeployment{Namespace: "pllm-agents", WorkloadName: "a"})
	require.NoError(t, err)
	require.Equal(t, models.DeploymentStatusRunning, rep.Status)
}
