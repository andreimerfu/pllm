package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AgentDeployment tracks a running instance of a RegistryAgent. Separate
// from Deployment (which is MCP-server-only) so agent-specific columns —
// UseCaseID binding, provisioned key reference, agent env vars — don't
// grow the server deployment row with nullable clutter.
//
// One row per (RegistryAgentID + Namespace). Re-deploying the same agent
// version into the same namespace updates the existing row.
type AgentDeployment struct {
	BaseModel

	// What was deployed (registry agent row).
	RegistryAgentID uuid.UUID `gorm:"type:uuid;index;not null" json:"registry_agent_id"`
	ResourceName    string    `gorm:"index;not null" json:"resource_name"`
	ResourceVersion string    `gorm:"not null" json:"resource_version"`

	// Scope binding — required. An agent always runs under a UseCase.
	UseCaseID uuid.UUID `gorm:"type:uuid;index;not null" json:"use_case_id"`

	// Target.
	Platform     DeploymentPlatform `gorm:"not null" json:"platform"`
	Namespace    string             `gorm:"index;not null" json:"namespace"`
	WorkloadName string             `gorm:"uniqueIndex:idx_agent_deploy_wl;not null" json:"workload_name"`

	// Replica count requested (1 unless overridden).
	Replicas int32 `gorm:"default:1" json:"replicas"`

	// Lifecycle.
	Status       DeploymentStatus `gorm:"default:'pending';index" json:"status"`
	StatusReason string           `json:"status_reason,omitempty"`

	// URL to reach the pod — set after Service is up. Used by Phase 2 A2A.
	Endpoint string `json:"endpoint,omitempty"`

	// Provisioned scoped key — nullable to survive teardown partial failures.
	ProvisionedKeyID *uuid.UUID `gorm:"type:uuid" json:"provisioned_key_id,omitempty"`
	SecretName       string     `json:"secret_name,omitempty"`

	// Opaque adapter-specific state (manifest hash etc.).
	AdapterState datatypes.JSON `json:"adapter_state,omitempty"`

	LastAppliedAt *time.Time `json:"last_applied_at,omitempty"`
}
