package service

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// RefService answers "which things depend on X?" queries.
// Registry dependency edges are stored in the RegistryRef table. Owners
// can now be either Agents (composition) or UseCases (business-activity
// declarations); targets are servers / agents / skills / prompts / LLMs.
// This service resolves either side to concrete rows so the UI can render
// "used by N agents and N use cases" sections.
type RefService struct {
	db *gorm.DB
}

// NewRefService constructs a RefService.
func NewRefService(db *gorm.DB) *RefService {
	return &RefService{db: db}
}

// Dependent models a single artifact that depends on the queried target.
// Structure is intentionally flat for easy JSON rendering.
type Dependent struct {
	OwnerKind    models.RegistryKind `json:"owner_kind"`
	Name         string              `json:"name"`
	Version      string              `json:"version"`
	Title        string              `json:"title,omitempty"`
	Description  string              `json:"description,omitempty"`
	IsLatest     bool                `json:"is_latest"`
	TargetVersion string             `json:"target_version,omitempty"` // pinned version on the edge (empty = latest)
	LocalName    string              `json:"local_name,omitempty"`
}

// ListDependents returns every resource that references the given
// (targetKind, targetName). Only latest versions of registry dependents
// are included so the UI doesn't spam stale historical dependencies;
// use cases are always included since they don't have a version concept.
func (s *RefService) ListDependents(ctx context.Context, targetKind models.RegistryKind, targetName string) ([]Dependent, error) {
	// Name validation is lenient — LLM targets (e.g. "openai/gpt-4o")
	// already match the existing validName pattern.
	if err := validName(targetName); err != nil {
		return nil, err
	}
	if !models.IsValidTargetKind(targetKind) {
		return nil, fmt.Errorf("unsupported target_kind %q", targetKind)
	}

	var refs []models.RegistryRef
	err := ctxWrap(ctx, s.db).
		Where("target_kind = ? AND target_name = ?", targetKind, targetName).
		Find(&refs).Error
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, nil
	}

	// Group owner IDs by kind so we can batch-load the rows.
	byKind := map[models.RegistryKind][]string{}
	// Also remember edge metadata keyed by ownerID so we can attach it.
	edgeByOwner := map[string]models.RegistryRef{}
	for _, r := range refs {
		id := r.OwnerID.String()
		byKind[r.OwnerKind] = append(byKind[r.OwnerKind], id)
		edgeByOwner[id] = r
	}

	out := make([]Dependent, 0, len(refs))
	for kind, ids := range byKind {
		switch kind {
		case models.RegistryKindUseCase:
			// Use cases live in their own table with no version concept.
			ucRows, err := s.loadUseCaseRowsByIDs(ctx, ids)
			if err != nil {
				return nil, err
			}
			for _, uc := range ucRows {
				edge := edgeByOwner[uc.ID.String()]
				out = append(out, Dependent{
					OwnerKind:     models.RegistryKindUseCase,
					Name:          uc.Slug,
					Title:         uc.Name,
					Description:   uc.Description,
					IsLatest:      true, // use cases are always "current"
					TargetVersion: edge.TargetVersion,
					LocalName:     edge.LocalName,
				})
			}
		default:
			// Registry-backed owners (currently only Agent).
			rows, err := s.loadRowsByIDs(ctx, kind, ids)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				if !row.IsLatest {
					continue
				}
				edge := edgeByOwner[row.ID]
				out = append(out, Dependent{
					OwnerKind:     kind,
					Name:          row.Name,
					Version:       row.Version,
					Title:         row.Title,
					Description:   row.Description,
					IsLatest:      row.IsLatest,
					TargetVersion: edge.TargetVersion,
					LocalName:     edge.LocalName,
				})
			}
		}
	}
	return out, nil
}

// loadUseCaseRowsByIDs fetches the minimum UseCase fields needed to render
// a Dependent. Kept separate because the table has no version column.
func (s *RefService) loadUseCaseRowsByIDs(ctx context.Context, ids []string) ([]models.UseCase, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []models.UseCase
	if err := ctxWrap(ctx, s.db).
		Where("id IN ? AND deleted_at IS NULL", ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// genericRow is the subset of registry row fields we need for Dependent.
type genericRow struct {
	ID          string
	Name        string
	Version     string
	Title       string
	Description string
	IsLatest    bool
}

func (s *RefService) loadRowsByIDs(ctx context.Context, kind models.RegistryKind, ids []string) ([]genericRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	table, err := tableFor(kind)
	if err != nil {
		return nil, err
	}
	var rows []genericRow
	if err := ctxWrap(ctx, s.db).
		Table(table).
		Select("id, name, version, title, description, is_latest").
		Where("id IN ? AND deleted_at IS NULL", ids).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func tableFor(kind models.RegistryKind) (string, error) {
	switch kind {
	case models.RegistryKindServer:
		return tableRegistryServers, nil
	case models.RegistryKindAgent:
		return tableRegistryAgents, nil
	case models.RegistryKindSkill:
		return tableRegistrySkills, nil
	case models.RegistryKindPrompt:
		return tableRegistryPrompts, nil
	}
	return "", errors.New("unknown kind")
}
