package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAgentDeploymentDefaults(t *testing.T) {
	d := AgentDeployment{
		RegistryAgentID: uuid.New(),
		ResourceName:    "sales-research",
		ResourceVersion: "0.1.0",
		UseCaseID:       uuid.New(),
		Platform:        DeploymentPlatformKubernetes,
		Namespace:       "pllm-agents",
		WorkloadName:    "sales-research-0-1-0",
	}
	require.Equal(t, DeploymentPlatformKubernetes, d.Platform)
	require.Equal(t, "pllm-agents", d.Namespace)
}
