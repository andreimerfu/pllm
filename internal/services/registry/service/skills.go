package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

const tableRegistrySkills = "registry_skills"

// List returns a page of skills.
func (s *SkillService) List(ctx context.Context, f ListFilter) (ListResult[models.RegistrySkill], error) {
	q := ctxWrap(ctx, s.db).Model(&models.RegistrySkill{})
	q = applyFilter(q, f)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return ListResult[models.RegistrySkill]{}, fmt.Errorf("count: %w", err)
	}
	var rows []models.RegistrySkill
	limit := clampLimit(f.Limit)
	if err := q.Order("name asc, published_at desc").
		Limit(limit).Offset(f.Offset).Find(&rows).Error; err != nil {
		return ListResult[models.RegistrySkill]{}, fmt.Errorf("find: %w", err)
	}
	next := f.Offset + len(rows)
	if int64(next) >= total {
		next = 0
	}
	return ListResult[models.RegistrySkill]{Items: rows, Total: total, NextOffset: next}, nil
}

// Get returns one (name, version).
func (s *SkillService) Get(ctx context.Context, name, version string) (*models.RegistrySkill, error) {
	if err := validName(name); err != nil {
		return nil, err
	}
	q := ctxWrap(ctx, s.db).Model(&models.RegistrySkill{}).Where("name = ?", name)
	if version == "" || version == "latest" {
		q = q.Where("is_latest = ?", true)
	} else {
		q = q.Where("version = ?", version)
	}
	var row models.RegistrySkill
	if err := q.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// ListVersions returns all versions of a skill.
func (s *SkillService) ListVersions(ctx context.Context, name string) ([]models.RegistrySkill, error) {
	if err := validName(name); err != nil {
		return nil, err
	}
	var rows []models.RegistrySkill
	if err := ctxWrap(ctx, s.db).Where("name = ?", name).
		Order("published_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Upsert publishes (or overwrites) a version.
func (s *SkillService) Upsert(ctx context.Context, in *models.RegistrySkill) (*models.RegistrySkill, error) {
	if err := validName(in.Name); err != nil {
		return nil, err
	}
	if err := validVersion(in.Version); err != nil {
		return nil, err
	}
	ensureBaseFields(&in.Status, &in.PublishedAt)

	var result models.RegistrySkill
	err := ctxWrap(ctx, s.db).Transaction(func(tx *gorm.DB) error {
		var existing models.RegistrySkill
		lookup := tx.Where("name = ? AND version = ?", in.Name, in.Version).First(&existing)
		if lookup.Error != nil && !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
		if lookup.Error == nil {
			existing.Title = in.Title
			existing.Description = in.Description
			existing.Status = in.Status
			existing.Image = in.Image
			existing.Manifest = in.Manifest
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
		if err := setLatestAtomically(tx, tableRegistrySkills, result.Name, result.ID); err != nil {
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

// SoftDelete marks a version deleted and promotes the next-newest active.
func (s *SkillService) SoftDelete(ctx context.Context, name, version string) error {
	if err := validName(name); err != nil {
		return err
	}
	if err := validVersion(version); err != nil {
		return err
	}
	return ctxWrap(ctx, s.db).Transaction(func(tx *gorm.DB) error {
		var row models.RegistrySkill
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
		var promote models.RegistrySkill
		err := tx.Where("name = ? AND status = ?", name, models.RegistryStatusActive).
			Order("published_at desc").First(&promote).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return setLatestAtomically(tx, tableRegistrySkills, name, promote.ID)
	})
}
