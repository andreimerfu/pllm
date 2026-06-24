// Package catalog implements the read-only Catalog API facade — the
// producer side of the Policy Control-Plane Integration Contract (§2). It
// re-exposes pllm's registry resources, use cases, and their bindings as
// vendor-neutral DTOs so an external control plane can ingest them and
// attach policy keyed on the canonical identity (the resource name).
package catalog

import (
	"time"

	"github.com/amerfu/pllm/internal/core/models"
)

// Resource is the vendor-neutral catalog DTO for a governable resource
// (contract §2.1). The wire shape is independent of internal models.
type Resource struct {
	Kind                string    `json:"kind"`
	Name                string    `json:"name"` // canonical identity (the join key)
	Version             string    `json:"version"`
	DisplayName         string    `json:"display_name"`
	Description         string    `json:"description"`
	Source              string    `json:"source"` // "local" or an upstream peer id
	DataClassifications []string  `json:"data_classifications"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ResourceRow is the neutral intermediate a ResourceLister returns — the
// minimal fields the DTO needs, decoupled from the four registry model types.
type ResourceRow struct {
	Kind         string
	Name         string
	Version      string
	Title        string
	Description  string
	SourcePeerID string
	UpdatedAt    time.Time
}

func toResourceDTO(r ResourceRow) Resource {
	source := r.SourcePeerID
	if source == "" {
		source = "local"
	}
	return Resource{
		Kind:                r.Kind,
		Name:                r.Name,
		Version:             r.Version,
		DisplayName:         r.Title,
		Description:         r.Description,
		Source:              source,
		DataClassifications: []string{}, // registry resources carry none in pllm; classifications live on use cases
		UpdatedAt:           r.UpdatedAt,
	}
}

// UseCase is the vendor-neutral catalog DTO for a use case (contract §2.2).
type UseCase struct {
	ID                       string    `json:"id"`
	Slug                     string    `json:"slug"`
	Name                     string    `json:"name"`
	RiskTier                 string    `json:"risk_tier"`
	Status                   string    `json:"status"`
	DataClassifications      []string  `json:"data_classifications"`
	EffectiveClassifications []string  `json:"effective_classifications"`
	ComplianceFlag           bool      `json:"compliance_flag"`
	UpdatedAt                time.Time `json:"updated_at"`
}

func toUseCaseDTO(uc models.UseCase) UseCase {
	return UseCase{
		ID:                       uc.ID.String(),
		Slug:                     uc.Slug,
		Name:                     uc.Name,
		RiskTier:                 string(uc.RiskTier),
		Status:                   string(uc.Status),
		DataClassifications:      nonNil(uc.DataClassifications),
		EffectiveClassifications: nonNil(uc.EffectiveClassifications),
		ComplianceFlag:           uc.ComplianceFlag,
		UpdatedAt:                uc.UpdatedAt,
	}
}

// Binding is the vendor-neutral (use_case → resource) edge DTO (contract §2.3).
type Binding struct {
	UseCase      string `json:"use_case"` // the owning use case id
	ResourceKind string `json:"resource_kind"`
	ResourceName string `json:"resource_name"` // the join key
}

func toBindingDTO(ref models.RegistryRef) Binding {
	return Binding{
		UseCase:      ref.OwnerID.String(),
		ResourceKind: string(ref.TargetKind),
		ResourceName: ref.TargetName,
	}
}

// nonNil normalizes a possibly-nil string slice to a non-nil one so JSON is
// always an array, never null.
func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
