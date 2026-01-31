package model

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/core/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// Service manages user-created model configurations.
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService creates a new UserModel service.
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
	}
}

// ListUserModels returns all enabled user models from the database.
func (s *Service) ListUserModels() ([]models.UserModel, error) {
	var userModels []models.UserModel
	if err := s.db.Find(&userModels).Error; err != nil {
		return nil, fmt.Errorf("failed to list user models: %w", err)
	}
	return userModels, nil
}

// CreateUserModel inserts a new user model into the database.
func (s *Service) CreateUserModel(um *models.UserModel) error {
	if um.ModelName == "" {
		return fmt.Errorf("model_name is required")
	}
	if um.ProviderConfig.Type == "" {
		return fmt.Errorf("provider type is required")
	}
	if um.ProviderConfig.Model == "" {
		return fmt.Errorf("provider model is required")
	}

	// Set defaults
	if um.RPM == 0 {
		um.RPM = 100
	}
	if um.TPM == 0 {
		um.TPM = 100000
	}
	if um.Priority == 0 {
		um.Priority = 50
	}
	if um.Weight == 0 {
		um.Weight = 1.0
	}
	if um.TimeoutSeconds == 0 {
		um.TimeoutSeconds = 60
	}
	um.Enabled = true

	if err := s.db.Create(um).Error; err != nil {
		return fmt.Errorf("failed to create user model: %w", err)
	}
	return nil
}

// GetUserModel fetches a user model by ID.
func (s *Service) GetUserModel(id uuid.UUID) (*models.UserModel, error) {
	var um models.UserModel
	if err := s.db.First(&um, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("user model not found: %w", err)
	}
	return &um, nil
}

// UpdateUserModel updates fields of an existing user model.
func (s *Service) UpdateUserModel(id uuid.UUID, updates map[string]interface{}) (*models.UserModel, error) {
	var um models.UserModel
	if err := s.db.First(&um, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("user model not found: %w", err)
	}

	if err := s.db.Model(&um).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update user model: %w", err)
	}

	// Re-fetch to get updated values
	if err := s.db.First(&um, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("failed to re-fetch user model: %w", err)
	}

	return &um, nil
}

// DeleteUserModel deletes a user model from the database.
func (s *Service) DeleteUserModel(id uuid.UUID) error {
	result := s.db.Delete(&models.UserModel{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete user model: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user model not found")
	}
	return nil
}

// ConvertToModelInstance converts a UserModel into a config.ModelInstance
// suitable for loading into the model registry. Environment variables
// in the format ${VAR_NAME} are expanded at this point.
func (s *Service) ConvertToModelInstance(um models.UserModel) config.ModelInstance {
	provider := config.ProviderParams{
		Type:               um.ProviderConfig.Type,
		Model:              um.ProviderConfig.Model,
		APIKey:             expandEnvVars(um.ProviderConfig.APIKey),
		APISecret:          expandEnvVars(um.ProviderConfig.APISecret),
		BaseURL:            um.ProviderConfig.BaseURL,
		APIVersion:         um.ProviderConfig.APIVersion,
		OrgID:              um.ProviderConfig.OrgID,
		ProjectID:          um.ProviderConfig.ProjectID,
		Region:             um.ProviderConfig.Region,
		Location:           um.ProviderConfig.Location,
		AzureDeployment:    um.ProviderConfig.AzureDeployment,
		AzureEndpoint:      um.ProviderConfig.AzureEndpoint,
		AWSAccessKeyID:     expandEnvVars(um.ProviderConfig.AWSAccessKeyID),
		AWSSecretAccessKey: expandEnvVars(um.ProviderConfig.AWSSecretAccessKey),
		AWSRegionName:      um.ProviderConfig.AWSRegionName,
		VertexProject:      um.ProviderConfig.VertexProject,
		VertexLocation:     um.ProviderConfig.VertexLocation,
	}

	modelInfo := config.ModelInfo{
		Mode:               um.ModelInfoConfig.Mode,
		SupportsFunctions:  um.ModelInfoConfig.SupportsFunctions,
		SupportsVision:     um.ModelInfoConfig.SupportsVision,
		SupportsStreaming:  um.ModelInfoConfig.SupportsStreaming,
		MaxTokens:          um.ModelInfoConfig.MaxTokens,
		MaxInputTokens:     um.ModelInfoConfig.MaxInputTokens,
		MaxOutputTokens:    um.ModelInfoConfig.MaxOutputTokens,
		DefaultMaxTokens:   um.ModelInfoConfig.DefaultMaxTokens,
		SupportedLanguages: um.ModelInfoConfig.SupportedLanguages,
	}

	// Set defaults for model info if not specified
	if modelInfo.Mode == "" {
		modelInfo.Mode = "chat"
		modelInfo.SupportsStreaming = true
		modelInfo.SupportsFunctions = true
		if modelInfo.MaxTokens == 0 {
			modelInfo.MaxTokens = 128000
		}
		if modelInfo.DefaultMaxTokens == 0 {
			modelInfo.DefaultMaxTokens = 2000
		}
	}

	timeout := time.Duration(um.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	tags := []string(um.Tags)

	return config.ModelInstance{
		ID:                 um.ID.String(),
		ModelName:          um.ModelName,
		InstanceName:       um.InstanceName,
		Provider:           provider,
		ModelInfo:          modelInfo,
		RPM:                um.RPM,
		TPM:                um.TPM,
		Priority:           um.Priority,
		Weight:             um.Weight,
		InputCostPerToken:  um.InputCostPerToken,
		OutputCostPerToken: um.OutputCostPerToken,
		Timeout:            timeout,
		Tags:               tags,
		Enabled:            um.Enabled,
		MaxRetries:         3,
		CooldownPeriod:     30 * time.Second,
		Source:             "user",
	}
}

// expandEnvVars expands environment variable references in the format ${VAR_NAME}.
func expandEnvVars(s string) string {
	return envVarRegex.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1] // Remove ${ and }
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match // Return original if env var not set
	})
}
