package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

const tableRegistryAgents = "registry_agents"

// AgentWithRefs bundles an agent row with its dependency edges.
// Used by Get / Upsert so callers see the full picture in one round-trip.
type AgentWithRefs struct {
	Agent   models.RegistryAgent  `json:"agent"`
	Servers []models.RegistryRef  `json:"servers,omitempty"`
	Skills  []models.RegistryRef  `json:"skills,omitempty"`
	Prompts []models.RegistryRef  `json:"prompts,omitempty"`
}

// List returns a page of agents.
func (s *AgentService) List(ctx context.Context, f ListFilter) (ListResult[models.RegistryAgent], error) {
	q := ctxWrap(ctx, s.db).Model(&models.RegistryAgent{})
	q = applyFilter(q, f)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return ListResult[models.RegistryAgent]{}, fmt.Errorf("count: %w", err)
	}
	var rows []models.RegistryAgent
	limit := clampLimit(f.Limit)
	if err := q.Order("name asc, published_at desc").
		Limit(limit).Offset(f.Offset).Find(&rows).Error; err != nil {
		return ListResult[models.RegistryAgent]{}, fmt.Errorf("find: %w", err)
	}
	next := f.Offset + len(rows)
	if int64(next) >= total {
		next = 0
	}
	return ListResult[models.RegistryAgent]{Items: rows, Total: total, NextOffset: next}, nil
}

// Get returns one agent (latest by default) along with its refs.
func (s *AgentService) Get(ctx context.Context, name, version string) (*AgentWithRefs, error) {
	if err := validName(name); err != nil {
		return nil, err
	}
	q := ctxWrap(ctx, s.db).Model(&models.RegistryAgent{}).Where("name = ?", name)
	if version == "" || version == "latest" {
		q = q.Where("is_latest = ?", true)
	} else {
		q = q.Where("version = ?", version)
	}
	var agent models.RegistryAgent
	if err := q.First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	refs, err := s.loadRefs(ctx, agent.ID)
	if err != nil {
		return nil, err
	}
	return &AgentWithRefs{
		Agent:   agent,
		Servers: filterRefsByKind(refs, models.RegistryKindServer),
		Skills:  filterRefsByKind(refs, models.RegistryKindSkill),
		Prompts: filterRefsByKind(refs, models.RegistryKindPrompt),
	}, nil
}

// ListVersions returns every version of an agent.
func (s *AgentService) ListVersions(ctx context.Context, name string) ([]models.RegistryAgent, error) {
	if err := validName(name); err != nil {
		return nil, err
	}
	var rows []models.RegistryAgent
	if err := ctxWrap(ctx, s.db).Where("name = ?", name).
		Order("published_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// UpsertInput is the combined "publish an agent + its deps" body.
// Empty Refs means "no dependencies".
type UpsertInput struct {
	Agent models.RegistryAgent
	Refs  []models.RegistryRef // OwnerKind/OwnerID will be overwritten
}

// Upsert publishes (or overwrites) an agent version, replacing its refs wholesale.
func (s *AgentService) Upsert(ctx context.Context, in *UpsertInput) (*AgentWithRefs, error) {
	if err := validName(in.Agent.Name); err != nil {
		return nil, err
	}
	if err := validVersion(in.Agent.Version); err != nil {
		return nil, err
	}
	for i, r := range in.Refs {
		switch r.TargetKind {
		case models.RegistryKindServer, models.RegistryKindSkill, models.RegistryKindPrompt:
		default:
			return nil, fmt.Errorf("refs[%d]: invalid target kind %q", i, r.TargetKind)
		}
		if err := validName(r.TargetName); err != nil {
			return nil, fmt.Errorf("refs[%d]: %w", i, err)
		}
	}
	ensureBaseFields(&in.Agent.Status, &in.Agent.PublishedAt)

	var result AgentWithRefs
	err := ctxWrap(ctx, s.db).Transaction(func(tx *gorm.DB) error {
		var existing models.RegistryAgent
		lookup := tx.Where("name = ? AND version = ?", in.Agent.Name, in.Agent.Version).First(&existing)
		if lookup.Error != nil && !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
		if lookup.Error == nil {
			// Overwrite mutable fields.
			existing.Title = in.Agent.Title
			existing.Description = in.Agent.Description
			existing.Status = in.Agent.Status
			existing.Language = in.Agent.Language
			existing.Framework = in.Agent.Framework
			existing.ModelProvider = in.Agent.ModelProvider
			existing.ModelName = in.Agent.ModelName
			existing.WebsiteURL = in.Agent.WebsiteURL
			existing.Repository = in.Agent.Repository
			existing.Packages = in.Agent.Packages
			existing.Remotes = in.Agent.Remotes
			existing.Metadata = in.Agent.Metadata
			existing.PublishedByUserID = in.Agent.PublishedByUserID
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			result.Agent = existing
		} else {
			in.Agent.ID = uuid.New()
			if err := tx.Create(&in.Agent).Error; err != nil {
				return err
			}
			result.Agent = in.Agent
		}

		if err := setLatestAtomically(tx, tableRegistryAgents, result.Agent.Name, result.Agent.ID); err != nil {
			return err
		}
		result.Agent.IsLatest = true

		// Replace refs wholesale.
		if err := tx.Where("owner_kind = ? AND owner_id = ?",
			models.RegistryKindAgent, result.Agent.ID).
			Delete(&models.RegistryRef{}).Error; err != nil {
			return err
		}
		if len(in.Refs) > 0 {
			refs := make([]models.RegistryRef, 0, len(in.Refs))
			for _, r := range in.Refs {
				refs = append(refs, models.RegistryRef{
					BaseModel:     models.BaseModel{ID: uuid.New()},
					OwnerKind:     models.RegistryKindAgent,
					OwnerID:       result.Agent.ID,
					TargetKind:    r.TargetKind,
					TargetName:    r.TargetName,
					TargetVersion: r.TargetVersion,
					LocalName:     r.LocalName,
				})
			}
			if err := tx.Create(&refs).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	refs, _ := s.loadRefs(ctx, result.Agent.ID)
	result.Servers = filterRefsByKind(refs, models.RegistryKindServer)
	result.Skills = filterRefsByKind(refs, models.RegistryKindSkill)
	result.Prompts = filterRefsByKind(refs, models.RegistryKindPrompt)
	return &result, nil
}

// SoftDelete marks a version deleted and promotes the next-newest active one.
func (s *AgentService) SoftDelete(ctx context.Context, name, version string) error {
	if err := validName(name); err != nil {
		return err
	}
	if err := validVersion(version); err != nil {
		return err
	}
	return ctxWrap(ctx, s.db).Transaction(func(tx *gorm.DB) error {
		var row models.RegistryAgent
		if err := tx.Where("name = ? AND version = ?", name, version).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		row.Status = models.RegistryStatusDeleted
		row.IsLatest = false
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		var promote models.RegistryAgent
		err := tx.Where("name = ? AND status = ?", name, models.RegistryStatusActive).
			Order("published_at desc").First(&promote).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return setLatestAtomically(tx, tableRegistryAgents, name, promote.ID)
	})
}

func (s *AgentService) loadRefs(ctx context.Context, agentID uuid.UUID) ([]models.RegistryRef, error) {
	var refs []models.RegistryRef
	err := ctxWrap(ctx, s.db).
		Where("owner_kind = ? AND owner_id = ?", models.RegistryKindAgent, agentID).
		Order("target_kind asc, target_name asc").
		Find(&refs).Error
	return refs, err
}

func filterRefsByKind(refs []models.RegistryRef, kind models.RegistryKind) []models.RegistryRef {
	out := make([]models.RegistryRef, 0, len(refs))
	for _, r := range refs {
		if r.TargetKind == kind {
			out = append(out, r)
		}
	}
	return out
}
