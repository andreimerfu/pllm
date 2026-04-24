package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// EnrichmentType identifies the scanner that produced a finding set.
type EnrichmentType string

const (
	EnrichmentTypeOSV          EnrichmentType = "osv"
	EnrichmentTypeScorecard    EnrichmentType = "scorecard"
	EnrichmentTypeContainer    EnrichmentType = "container"
	EnrichmentTypeDependencies EnrichmentType = "dependencies"
)

// EnrichmentJobStatus is the worker-queue lifecycle.
type EnrichmentJobStatus string

const (
	JobStatusPending   EnrichmentJobStatus = "pending"
	JobStatusRunning   EnrichmentJobStatus = "running"
	JobStatusSucceeded EnrichmentJobStatus = "succeeded"
	JobStatusFailed    EnrichmentJobStatus = "failed"
)

// EnrichmentJob is a queued scan for one (resource, type).
// Multiple types run in parallel; results land in EnrichmentScore.
type EnrichmentJob struct {
	BaseModel

	ResourceKind RegistryKind        `gorm:"index;not null" json:"resource_kind"`
	ResourceID   uuid.UUID           `gorm:"type:uuid;index;not null" json:"resource_id"`
	Type         EnrichmentType      `gorm:"index;not null" json:"type"`
	Status       EnrichmentJobStatus `gorm:"default:'pending';index" json:"status"`

	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
	Attempts    int        `json:"attempts"`
}

// EnrichmentScore is the latest scan result per (resource, type).
// One row per combination; overwritten on re-scan.
type EnrichmentScore struct {
	BaseModel

	ResourceKind RegistryKind   `gorm:"uniqueIndex:idx_enrich_uniq;not null" json:"resource_kind"`
	ResourceID   uuid.UUID      `gorm:"type:uuid;uniqueIndex:idx_enrich_uniq;not null" json:"resource_id"`
	Type         EnrichmentType `gorm:"uniqueIndex:idx_enrich_uniq;not null" json:"type"`

	// Numeric score in [0, 100]. Higher = better.
	// Semantics differ per type: for OSV it's 100 minus a severity-weighted
	// vulnerability penalty; for Scorecard it's the upstream score.
	Score   float64        `json:"score"`
	Summary string         `json:"summary,omitempty"` // one-line human readable
	Findings datatypes.JSON `json:"findings,omitempty"` // scanner-specific structured output
	ScannedAt time.Time    `json:"scanned_at"`
}
