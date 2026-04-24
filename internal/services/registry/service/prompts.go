package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

const tableRegistryPrompts = "registry_prompts"

// List returns a page of prompts.
func (s *PromptService) List(ctx context.Context, f ListFilter) (ListResult[models.RegistryPrompt], error) {
	q := ctxWrap(ctx, s.db).Model(&models.RegistryPrompt{})
	q = applyFilter(q, f)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return ListResult[models.RegistryPrompt]{}, fmt.Errorf("count: %w", err)
	}
	var rows []models.RegistryPrompt
	limit := clampLimit(f.Limit)
	if err := q.Order("name asc, published_at desc").
		Limit(limit).Offset(f.Offset).Find(&rows).Error; err != nil {
		return ListResult[models.RegistryPrompt]{}, fmt.Errorf("find: %w", err)
	}
	next := f.Offset + len(rows)
	if int64(next) >= total {
		next = 0
	}
	return ListResult[models.RegistryPrompt]{Items: rows, Total: total, NextOffset: next}, nil
}

// Get returns one (name, version).
func (s *PromptService) Get(ctx context.Context, name, version string) (*models.RegistryPrompt, error) {
	if err := validName(name); err != nil {
		return nil, err
	}
	q := ctxWrap(ctx, s.db).Model(&models.RegistryPrompt{}).Where("name = ?", name)
	if version == "" || version == "latest" {
		q = q.Where("is_latest = ?", true)
	} else {
		q = q.Where("version = ?", version)
	}
	var row models.RegistryPrompt
	if err := q.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// ListVersions returns all versions.
func (s *PromptService) ListVersions(ctx context.Context, name string) ([]models.RegistryPrompt, error) {
	if err := validName(name); err != nil {
		return nil, err
	}
	var rows []models.RegistryPrompt
	if err := ctxWrap(ctx, s.db).Where("name = ?", name).
		Order("published_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Upsert publishes (or overwrites) a prompt version.
func (s *PromptService) Upsert(ctx context.Context, in *models.RegistryPrompt) (*models.RegistryPrompt, error) {
	if err := validName(in.Name); err != nil {
		return nil, err
	}
	if err := validVersion(in.Version); err != nil {
		return nil, err
	}
	ensureBaseFields(&in.Status, &in.PublishedAt)

	var result models.RegistryPrompt
	err := ctxWrap(ctx, s.db).Transaction(func(tx *gorm.DB) error {
		var existing models.RegistryPrompt
		lookup := tx.Where("name = ? AND version = ?", in.Name, in.Version).First(&existing)
		if lookup.Error != nil && !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
		if lookup.Error == nil {
			existing.Title = in.Title
			existing.Description = in.Description
			existing.Status = in.Status
			existing.Template = in.Template
			existing.Arguments = in.Arguments
			existing.Metadata = in.Metadata
			existing.PublishedByUserID = in.PublishedByUserID
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			result = existing
		} else {
			in.ID = uuid.New()
			if err := tx.Create(in).Error; err != nil {
				return err
			}
			result = *in
		}
		if err := setLatestAtomically(tx, tableRegistryPrompts, result.Name, result.ID); err != nil {
			return err
		}
		result.IsLatest = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SoftDelete marks a version deleted and promotes the next active one.
func (s *PromptService) SoftDelete(ctx context.Context, name, version string) error {
	if err := validName(name); err != nil {
		return err
	}
	if err := validVersion(version); err != nil {
		return err
	}
	return ctxWrap(ctx, s.db).Transaction(func(tx *gorm.DB) error {
		var row models.RegistryPrompt
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
		var promote models.RegistryPrompt
		err := tx.Where("name = ? AND status = ?", name, models.RegistryStatusActive).
			Order("published_at desc").First(&promote).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return setLatestAtomically(tx, tableRegistryPrompts, name, promote.ID)
	})
}
