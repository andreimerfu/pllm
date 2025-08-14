package virtualkey

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	
	"github.com/amerfu/pllm/internal/models"
)

// DEPRECATED: This service is deprecated in favor of the unified Key model
// Use models.Key and create keys directly through auth handlers

var (
	ErrKeyNotFound       = errors.New("key not found")
	ErrKeyExpired        = errors.New("key expired") 
	ErrKeyRevoked        = errors.New("key revoked")
	ErrKeyInactive       = errors.New("key inactive")
	ErrBudgetExceeded    = errors.New("budget exceeded")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrModelNotAllowed   = errors.New("model not allowed")
)

type VirtualKeyService struct {
	db *gorm.DB
}

func NewVirtualKeyService(db *gorm.DB) *VirtualKeyService {
	return &VirtualKeyService{db: db}
}

func NewService(db *gorm.DB) *VirtualKeyService {
	return NewVirtualKeyService(db)
}

// ValidateKey validates a key using the new unified Key model
func (s *VirtualKeyService) ValidateKey(ctx context.Context, keyValue string) (*models.Key, error) {
	keyHash := models.HashKey(keyValue)
	
	var key models.Key
	err := s.db.Preload("User").Preload("Team").Where("key_hash = ? AND is_active = ?", keyHash, true).First(&key).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}

	if !key.IsValid() {
		return nil, ErrKeyInactive
	}

	return &key, nil
}

// CheckModelAccess checks if a key has access to a specific model
func (s *VirtualKeyService) CheckModelAccess(ctx context.Context, key *models.Key, model string) error {
	if !key.IsModelAllowed(model) {
		return ErrModelNotAllowed
	}
	return nil
}

// DEPRECATED METHODS - Return errors to indicate they should not be used

func (s *VirtualKeyService) CreateKey(ctx context.Context, req interface{}, createdBy uuid.UUID) (*models.Key, error) {
	return nil, errors.New("deprecated: use auth handler CreateAPIKey endpoint instead")
}

func (s *VirtualKeyService) GetKey(ctx context.Context, keyID uuid.UUID) (*models.Key, error) {
	return nil, errors.New("deprecated: use unified Key model queries instead")
}

func (s *VirtualKeyService) ListKeys(ctx context.Context, userID *uuid.UUID, teamID *uuid.UUID) ([]models.Key, error) {
	return nil, errors.New("deprecated: use auth handler ListAPIKeys endpoint instead")
}

func (s *VirtualKeyService) UpdateKey(ctx context.Context, keyID uuid.UUID, updates map[string]interface{}) error {
	return errors.New("deprecated: use unified Key model updates instead")
}

func (s *VirtualKeyService) RevokeKey(ctx context.Context, keyID uuid.UUID, revokedBy uuid.UUID, reason string) error {
	return errors.New("deprecated: use auth handler DeleteAPIKey endpoint instead")
}