package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

const tableRegistryServers = "registry_servers"

// List returns a page of servers matching the filter.
// If LatestOnly is false, returns every (name, version) row.
func (s *ServerService) List(ctx context.Context, f ListFilter) (ListResult[models.RegistryServer], error) {
	q := ctxWrap(ctx, s.db).Model(&models.RegistryServer{})
	q = applyFilter(q, f)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return ListResult[models.RegistryServer]{}, fmt.Errorf("count: %w", err)
	}

	var rows []models.RegistryServer
	limit := clampLimit(f.Limit)
	if err := q.Order("name asc, published_at desc").
		Limit(limit).Offset(f.Offset).Find(&rows).Error; err != nil {
		return ListResult[models.RegistryServer]{}, fmt.Errorf("find: %w", err)
	}
	next := f.Offset + len(rows)
	if int64(next) >= total {
		next = 0
	}
	return ListResult[models.RegistryServer]{Items: rows, Total: total, NextOffset: next}, nil
}

// Get returns a single (name, version). Version "" or "latest" resolves
// to the is_latest row.
func (s *ServerService) Get(ctx context.Context, name, version string) (*models.RegistryServer, error) {
	if err := validName(name); err != nil {
		return nil, err
	}
	q := ctxWrap(ctx, s.db).Model(&models.RegistryServer{}).Where("name = ?", name)
	if version == "" || version == "latest" {
		q = q.Where("is_latest = ?", true)
	} else {
		q = q.Where("version = ?", version)
	}
	var row models.RegistryServer
	if err := q.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// ListVersions returns every version of a server, newest PublishedAt first.
func (s *ServerService) ListVersions(ctx context.Context, name string) ([]models.RegistryServer, error) {
	if err := validName(name); err != nil {
		return nil, err
	}
	var rows []models.RegistryServer
	if err := ctxWrap(ctx, s.db).Where("name = ?", name).
		Order("published_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Upsert publishes a new version. If (name, version) exists, overwrites it.
// The newest inserted/updated row becomes is_latest; siblings flip off.
//
// The caller is expected to have already validated permissions — the
// service is resource-agnostic.
func (s *ServerService) Upsert(ctx context.Context, in *models.RegistryServer) (*models.RegistryServer, error) {
	if err := validName(in.Name); err != nil {
		return nil, err
	}
	if err := validVersion(in.Version); err != nil {
		return nil, err
	}
	ensureBaseFields(&in.Status, &in.PublishedAt)

	var result models.RegistryServer
	err := ctxWrap(ctx, s.db).Transaction(func(tx *gorm.DB) error {
		// Did this (name, version) exist?
		var existing models.RegistryServer
		lookup := tx.Where("name = ? AND version = ?", in.Name, in.Version).First(&existing)
		if lookup.Error != nil && !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
		if lookup.Error == nil {
			// Overwrite mutable fields on the existing row.
			existing.Title = in.Title
			existing.Description = in.Description
			existing.Status = in.Status
			existing.WebsiteURL = in.WebsiteURL
			existing.Repository = in.Repository
			existing.Packages = in.Packages
			existing.Remotes = in.Remotes
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
		if err := setLatestAtomically(tx, tableRegistryServers, result.Name, result.ID); err != nil {
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

// SoftDelete marks a version as deleted. Others remain searchable.
// If the deleted row was is_latest, the next-newest active row is promoted.
func (s *ServerService) SoftDelete(ctx context.Context, name, version string) error {
	if err := validName(name); err != nil {
		return err
	}
	if err := validVersion(version); err != nil {
		return err
	}
	return ctxWrap(ctx, s.db).Transaction(func(tx *gorm.DB) error {
		var row models.RegistryServer
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
		// Promote the newest still-active sibling.
		var promote models.RegistryServer
		err := tx.Where("name = ? AND status = ?", name, models.RegistryStatusActive).
			Order("published_at desc").First(&promote).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // no active versions left — that's fine
		}
		if err != nil {
			return err
		}
		return setLatestAtomically(tx, tableRegistryServers, name, promote.ID)
	})
}
