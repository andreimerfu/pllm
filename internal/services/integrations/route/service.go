package route

import (
	"fmt"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service manages route configurations in the database.
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService creates a new Route service.
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
	}
}

// List returns all routes with their models preloaded.
func (s *Service) List() ([]models.Route, error) {
	var routes []models.Route
	if err := s.db.Preload("Models").Find(&routes).Error; err != nil {
		return nil, fmt.Errorf("failed to list routes: %w", err)
	}
	return routes, nil
}

// GetByID returns a route by its UUID.
func (s *Service) GetByID(id uuid.UUID) (*models.Route, error) {
	var route models.Route
	if err := s.db.Preload("Models").First(&route, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("route not found: %w", err)
	}
	return &route, nil
}

// GetBySlug returns a route by its slug.
func (s *Service) GetBySlug(slug string) (*models.Route, error) {
	var route models.Route
	if err := s.db.Preload("Models").First(&route, "slug = ?", slug).Error; err != nil {
		return nil, fmt.Errorf("route not found: %w", err)
	}
	return &route, nil
}

// Create inserts a new route and its models.
func (s *Service) Create(route *models.Route) error {
	if route.Name == "" {
		return fmt.Errorf("route name is required")
	}
	if route.Slug == "" {
		return fmt.Errorf("route slug is required")
	}
	if route.Strategy == "" {
		route.Strategy = "priority"
	}

	if err := s.db.Create(route).Error; err != nil {
		return fmt.Errorf("failed to create route: %w", err)
	}
	return nil
}

// Update replaces a route's fields and its model list.
func (s *Service) Update(id uuid.UUID, route *models.Route) error {
	var existing models.Route
	if err := s.db.First(&existing, "id = ?", id).Error; err != nil {
		return fmt.Errorf("route not found: %w", err)
	}

	// Delete existing route models and re-create
	if err := s.db.Where("route_id = ?", id).Delete(&models.RouteModel{}).Error; err != nil {
		return fmt.Errorf("failed to delete existing route models: %w", err)
	}

	// Update route fields
	updates := map[string]interface{}{
		"name":            route.Name,
		"slug":            route.Slug,
		"description":     route.Description,
		"strategy":        route.Strategy,
		"fallback_models": route.FallbackModels,
		"enabled":         route.Enabled,
	}
	if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update route: %w", err)
	}

	// Create new route models
	for i := range route.Models {
		route.Models[i].RouteID = id
		if err := s.db.Create(&route.Models[i]).Error; err != nil {
			return fmt.Errorf("failed to create route model: %w", err)
		}
	}

	return nil
}

// Delete removes a route and its models.
func (s *Service) Delete(id uuid.UUID) error {
	// Delete route models first
	if err := s.db.Where("route_id = ?", id).Delete(&models.RouteModel{}).Error; err != nil {
		return fmt.Errorf("failed to delete route models: %w", err)
	}

	result := s.db.Delete(&models.Route{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete route: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("route not found")
	}
	return nil
}
