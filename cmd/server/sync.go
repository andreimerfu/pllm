package main

import (
	"context"
	"time"

	modelService "github.com/amerfu/pllm/internal/services/integrations/model"
	routeService "github.com/amerfu/pllm/internal/services/integrations/route"
	llmModels "github.com/amerfu/pllm/internal/services/llm/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const dbSyncInterval = 30 * time.Second

// startDBSync periodically synchronises user-created models and routes from the
// database into the in-memory model registry. This ensures that models/routes
// created on another replica (or missed during startup) are picked up, and that
// models/routes deleted from the database are removed from the registry.
func startDBSync(ctx context.Context, db *gorm.DB, modelManager *llmModels.ModelManager, logger *zap.Logger) {
	modelSvc := modelService.NewService(db, logger)
	routeSvc := routeService.NewService(db, logger)

	ticker := time.NewTicker(dbSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("DB sync stopped")
			return
		case <-ticker.C:
			syncModels(modelSvc, modelManager, logger)
			syncRoutes(routeSvc, modelManager, logger)
		}
	}
}

// syncModels adds/removes user-sourced model instances so the in-memory registry
// matches the database. System (config-file) instances are never touched.
func syncModels(modelSvc *modelService.Service, manager *llmModels.ModelManager, logger *zap.Logger) {
	dbModels, err := modelSvc.ListUserModels()
	if err != nil {
		logger.Warn("DB sync: failed to list user models", zap.Error(err))
		return
	}

	// Build set of enabled DB model instance IDs
	dbEnabledIDs := make(map[string]bool, len(dbModels))
	for _, um := range dbModels {
		if um.Enabled {
			dbEnabledIDs[um.ID.String()] = true
		}
	}

	// Scan registry for user-sourced instances that should be removed
	allInstances := manager.GetRegistry().GetAllInstances()
	registryUserIDs := make(map[string]bool)
	for _, inst := range allInstances {
		if inst.Config.Source != "user" {
			continue
		}
		registryUserIDs[inst.Config.ID] = true
		if !dbEnabledIDs[inst.Config.ID] {
			logger.Info("DB sync: removing model no longer in DB or disabled",
				zap.String("id", inst.Config.ID),
				zap.String("model", inst.Config.ModelName))
			_ = manager.RemoveInstance(inst.Config.ID)
		}
	}

	// Add DB models not yet in registry
	for _, um := range dbModels {
		if !um.Enabled {
			continue
		}
		if registryUserIDs[um.ID.String()] {
			continue // already registered
		}
		instance := modelSvc.ConvertToModelInstance(um)
		if err := manager.AddInstance(instance); err != nil {
			logger.Warn("DB sync: failed to add model",
				zap.String("id", um.ID.String()),
				zap.String("model", um.ModelName),
				zap.Error(err))
		} else {
			logger.Info("DB sync: added model from DB",
				zap.String("id", um.ID.String()),
				zap.String("model", um.ModelName))
		}
	}
}

// syncRoutes adds/removes/updates user-sourced routes so the in-memory route
// registry matches the database. Config-file routes (source != "user") are
// never touched.
func syncRoutes(routeSvc *routeService.Service, manager *llmModels.ModelManager, logger *zap.Logger) {
	dbRoutes, err := routeSvc.List()
	if err != nil {
		logger.Warn("DB sync: failed to list routes", zap.Error(err))
		return
	}

	// Build set of enabled DB route slugs
	dbEnabledSlugs := make(map[string]bool, len(dbRoutes))
	for _, r := range dbRoutes {
		if r.Enabled {
			dbEnabledSlugs[r.Slug] = true
		}
	}

	// Remove routes that are no longer enabled in DB
	// We only remove routes whose slug is in the DB (to avoid removing config-file routes)
	for _, r := range dbRoutes {
		if !r.Enabled {
			if _, exists := manager.ResolveRoute(r.Slug); exists {
				manager.UnregisterRoute(r.Slug)
				logger.Info("DB sync: removed disabled route", zap.String("slug", r.Slug))
			}
		}
	}

	// Add/update enabled routes from DB
	for _, r := range dbRoutes {
		if !r.Enabled {
			continue
		}

		var routeModels []llmModels.RouteModelEntry
		for _, rm := range r.Models {
			routeModels = append(routeModels, llmModels.RouteModelEntry{
				ModelName: rm.ModelName,
				Weight:    rm.Weight,
				Priority:  rm.Priority,
				Enabled:   rm.Enabled,
			})
		}

		manager.RegisterRoute(&llmModels.RouteEntry{
			Slug:           r.Slug,
			Models:         routeModels,
			FallbackModels: []string(r.FallbackModels),
		}, r.Strategy)
	}
}
