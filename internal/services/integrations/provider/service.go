package provider

import (
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{db: db, logger: logger}
}

func (s *Service) List() ([]models.ProviderProfile, error) {
	var profiles []models.ProviderProfile
	if err := s.db.Order("created_at DESC").Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("failed to list provider profiles: %w", err)
	}
	return profiles, nil
}

func (s *Service) ListByType(providerType string) ([]models.ProviderProfile, error) {
	var profiles []models.ProviderProfile
	if err := s.db.Where("type = ?", providerType).Order("name ASC").Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("failed to list provider profiles by type: %w", err)
	}
	return profiles, nil
}

func (s *Service) Get(id uuid.UUID) (*models.ProviderProfile, error) {
	var profile models.ProviderProfile
	if err := s.db.First(&profile, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("provider profile not found: %w", err)
	}
	return &profile, nil
}

func (s *Service) Create(profile *models.ProviderProfile) error {
	if err := s.db.Create(profile).Error; err != nil {
		return fmt.Errorf("failed to create provider profile: %w", err)
	}
	return nil
}

func (s *Service) Update(id uuid.UUID, updates map[string]interface{}) (*models.ProviderProfile, error) {
	var profile models.ProviderProfile
	if err := s.db.First(&profile, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("provider profile not found: %w", err)
	}
	if err := s.db.Model(&profile).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update provider profile: %w", err)
	}
	if err := s.db.First(&profile, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("failed to reload provider profile: %w", err)
	}
	return &profile, nil
}

func (s *Service) Delete(id uuid.UUID) error {
	var count int64
	if err := s.db.Model(&models.UserModel{}).Where("provider_profile_id = ?", id).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check model references: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete provider profile: %d models reference it", count)
	}
	result := s.db.Delete(&models.ProviderProfile{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete provider profile: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("provider profile not found")
	}
	return nil
}

func (s *Service) GetModelCount(id uuid.UUID) (int64, error) {
	var count int64
	if err := s.db.Model(&models.UserModel{}).Where("provider_profile_id = ?", id).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
