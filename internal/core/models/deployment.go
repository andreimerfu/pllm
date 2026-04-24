package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// DeploymentPlatform is the target runtime for a deployment.
type DeploymentPlatform string

const (
	DeploymentPlatformKubernetes DeploymentPlatform = "kubernetes"
	DeploymentPlatformLocal      DeploymentPlatform = "local" // future: docker-compose
)

// DeploymentStatus is the lifecycle state machine.
type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "pending"
	DeploymentStatusDeploying  DeploymentStatus = "deploying"
	DeploymentStatusRunning    DeploymentStatus = "running"
	DeploymentStatusFailed     DeploymentStatus = "failed"
	DeploymentStatusTerminating DeploymentStatus = "terminating"
	DeploymentStatusStopped    DeploymentStatus = "stopped"
)

// Deployment tracks a running instance of a registry server (Phase 4
// covers servers only; agents/skills come later). One Deployment row per
// target (platform + namespace + name) — re-deploying overwrites.
//
// The actual K8s objects (Deployment + Service + NetworkPolicy) are owned
// by the adapter; this row is the pllm-side bookkeeping + state.
type Deployment struct {
	BaseModel

	// What was deployed (registry server row).
	RegistryServerID uuid.UUID `gorm:"type:uuid;index;not null" json:"registry_server_id"`
	// Cached name/version for display without a join.
	ResourceName    string `gorm:"index;not null" json:"resource_name"`
	ResourceVersion string `gorm:"not null" json:"resource_version"`

	// Target.
	Platform  DeploymentPlatform `gorm:"not null" json:"platform"`
	Namespace string             `gorm:"index;not null" json:"namespace"`
	// Name of the underlying workload (k8s Deployment name, docker container name).
	WorkloadName string `gorm:"uniqueIndex:idx_deploy_wl;not null" json:"workload_name"`

	// Lifecycle.
	Status       DeploymentStatus `gorm:"default:'pending';index" json:"status"`
	StatusReason string           `json:"status_reason,omitempty"`

	// Endpoint the gateway should route to — set once the adapter knows
	// the Service URL. Format: "http://svc.ns.svc.cluster.local:8000/mcp".
	Endpoint string `json:"endpoint,omitempty"`

	// Auto-registered gateway backend. Filled after Deploy succeeds so we
	// can delete the backend on Undeploy.
	GatewayBackendID *uuid.UUID `gorm:"type:uuid" json:"gateway_backend_id,omitempty"`

	// Opaque adapter-specific state (e.g. rendered manifest hash) — lets
	// us detect drift and skip no-op reapplies.
	AdapterState datatypes.JSON `json:"adapter_state,omitempty"`

	LastAppliedAt *time.Time `json:"last_applied_at,omitempty"`
}
