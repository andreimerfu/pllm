package models

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// DeploymentEventKind distinguishes the operational signal that produced
// the event. Stored as a string so adding new signals later (crashloop,
// image-pull failure) doesn't require a migration.
type DeploymentEventKind string

const (
	// Auto-bump fired successfully: memory limit raised and re-applied.
	DeploymentEventKindOOMBumped DeploymentEventKind = "oom.bumped"
	// Auto-bump refused: attempts or memory ceiling hit. Workload is Failed.
	DeploymentEventKindOOMCapReached DeploymentEventKind = "oom.cap_reached"
	// Operator cleared the override manually.
	DeploymentEventKindOOMReset DeploymentEventKind = "oom.reset"
	// Operator triggered a one-shot bump without waiting for an OOM.
	DeploymentEventKindOOMManualBump DeploymentEventKind = "oom.manual_bump"
)

// DeploymentTargetKind tells the event row which deployment table the
// DeploymentID points at. Keeps the column schema simple — we didn't want
// two nullable FKs.
type DeploymentTargetKind string

const (
	DeploymentTargetServer DeploymentTargetKind = "server"
	DeploymentTargetAgent  DeploymentTargetKind = "agent"
)

// DeploymentEvent is an operational audit row for a deployment — currently
// OOM auto-scale events, but reserved for any future "what did the
// reconciler do to this workload" entry.
//
// Intentionally thin: Reason is a human-readable sentence, Data is a
// structured JSON blob for anything a UI/SIEM wants to unpack (e.g. old/new
// memory, attempt counter, pod name). No separate table per event type.
type DeploymentEvent struct {
	BaseModel

	// Which deployment this event belongs to. TargetKind disambiguates
	// between Deployment and AgentDeployment rows — we didn't want to
	// burn two nullable FK columns for what's effectively one field.
	DeploymentID uuid.UUID            `gorm:"type:uuid;index;not null" json:"deployment_id"`
	TargetKind   DeploymentTargetKind `gorm:"index;not null" json:"target_kind"`

	// Cached workload name + namespace so the audit UI doesn't need a
	// join with a possibly-deleted deployment row.
	WorkloadName string `gorm:"index" json:"workload_name,omitempty"`
	Namespace    string `gorm:"index" json:"namespace,omitempty"`

	Kind   DeploymentEventKind `gorm:"index;not null" json:"kind"`
	Reason string              `json:"reason,omitempty"`

	// Structured payload. For OOM events: {old_limit, new_limit, attempts,
	// max_attempts, pod_name, exit_code}.
	Data datatypes.JSON `json:"data,omitempty"`
}
