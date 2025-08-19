package key

import (
	"context"
	"fmt"
	"time"

	"github.com/amerfu/pllm/internal/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service handles key management operations
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService creates a new key service
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
	}
}

// CreateKey creates a new API key
func (s *Service) CreateKey(ctx context.Context, req CreateKeyRequest) (*models.Key, error) {
	// Generate key value and hash
	keyValue, keyHash, err := models.GenerateKey(req.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Create key model
	key := &models.Key{
		Name:      req.Name,
		Key:       keyValue,
		KeyHash:   keyHash,
		Type:      req.Type,
		UserID:    req.UserID,
		TeamID:    req.TeamID,
		ExpiresAt: req.ExpiresAt,
		TPM:       req.TPM,
		RPM:       req.RPM,
		MaxBudget: req.MaxBudget,
		Scopes:    req.Scopes,
		IsActive:  true,
	}

	// Set expiry if duration provided
	if req.Duration != nil {
		expiresAt := time.Now().Add(time.Duration(*req.Duration) * time.Second)
		key.ExpiresAt = &expiresAt
	}

	// Save to database
	if err := s.db.WithContext(ctx).Create(key).Error; err != nil {
		return nil, fmt.Errorf("failed to create key: %w", err)
	}

	s.logger.Info("API key created",
		zap.String("key_id", key.ID.String()),
		zap.String("name", key.Name),
		zap.String("type", string(key.Type)))

	return key, nil
}

// GetKey retrieves a key by ID
func (s *Service) GetKey(ctx context.Context, keyID string) (*models.Key, error) {
	var key models.Key
	id, err := uuid.Parse(keyID)
	if err != nil {
		return nil, fmt.Errorf("invalid key ID: %w", err)
	}

	if err := s.db.WithContext(ctx).
		Preload("User").
		Preload("Team").
		Where("id = ?", id).
		First(&key).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	return &key, nil
}

// GetKeyByHash retrieves a key by its hash (for authentication)
func (s *Service) GetKeyByHash(ctx context.Context, keyHash string) (*models.Key, error) {
	var key models.Key
	if err := s.db.WithContext(ctx).
		Preload("User").
		Preload("Team").
		Where("key_hash = ? AND is_active = ?", keyHash, true).
		First(&key).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to get key by hash: %w", err)
	}

	// Check if key is expired
	if key.IsExpired() {
		return nil, ErrKeyExpired
	}

	return &key, nil
}

// ListKeys retrieves keys for a user or team
func (s *Service) ListKeys(ctx context.Context, req ListKeysRequest) ([]*models.Key, error) {
	query := s.db.WithContext(ctx).
		Preload("User").
		Preload("Team")

	// Filter by user or team
	if req.UserID != nil {
		query = query.Where("user_id = ?", *req.UserID)
	}
	if req.TeamID != nil {
		query = query.Where("team_id = ?", *req.TeamID)
	}

	// Add pagination
	if req.Limit > 0 {
		query = query.Limit(req.Limit)
	}
	if req.Offset > 0 {
		query = query.Offset(req.Offset)
	}

	var keys []*models.Key
	if err := query.Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	return keys, nil
}

// UpdateKey updates an existing key
func (s *Service) UpdateKey(ctx context.Context, keyID string, req UpdateKeyRequest) (*models.Key, error) {
	key, err := s.GetKey(ctx, keyID)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.Name != "" {
		key.Name = req.Name
	}
	if req.Scopes != nil {
		key.Scopes = req.Scopes
	}
	if req.ExpiresAt != nil {
		key.ExpiresAt = req.ExpiresAt
	}
	if req.TPM != nil {
		key.TPM = req.TPM
	}
	if req.RPM != nil {
		key.RPM = req.RPM
	}
	if req.MaxBudget != nil {
		key.MaxBudget = req.MaxBudget
	}
	if req.IsActive != nil {
		key.IsActive = *req.IsActive
	}

	// Save changes
	if err := s.db.WithContext(ctx).Save(key).Error; err != nil {
		return nil, fmt.Errorf("failed to update key: %w", err)
	}

	s.logger.Info("API key updated",
		zap.String("key_id", key.ID.String()),
		zap.String("name", key.Name))

	return key, nil
}

// DeleteKey deletes a key
func (s *Service) DeleteKey(ctx context.Context, keyID string) error {
	id, err := uuid.Parse(keyID)
	if err != nil {
		return fmt.Errorf("invalid key ID: %w", err)
	}

	result := s.db.WithContext(ctx).Delete(&models.Key{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete key: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrKeyNotFound
	}

	s.logger.Info("API key deleted", zap.String("key_id", keyID))
	return nil
}

// ValidateKey validates an API key and returns the associated key model
func (s *Service) ValidateKey(ctx context.Context, keyValue string) (*models.Key, error) {
	// Hash the key to look it up
	keyHash := models.HashKey(keyValue)
	key, err := s.GetKeyByHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}

	// Check if key can be used
	if !key.CanUse() {
		return nil, fmt.Errorf("key cannot be used")
	}

	// Check budget if applicable
	if key.IsBudgetExceeded() {
		return nil, fmt.Errorf("budget exceeded")
	}

	return key, nil
}

// RecordUsage records usage for a key
func (s *Service) RecordUsage(ctx context.Context, keyID string, tokens int, cost float64) error {
	id, err := uuid.Parse(keyID)
	if err != nil {
		return fmt.Errorf("invalid key ID: %w", err)
	}

	// Update key's usage statistics
	if err := s.db.WithContext(ctx).Model(&models.Key{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"usage_count":   gorm.Expr("usage_count + 1"),
			"total_tokens":  gorm.Expr("total_tokens + ?", tokens),
			"total_cost":    gorm.Expr("total_cost + ?", cost),
			"current_spend": gorm.Expr("current_spend + ?", cost),
			"last_used_at":  time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("failed to update key usage: %w", err)
	}

	return nil
}

// RevokeKey revokes a key
func (s *Service) RevokeKey(ctx context.Context, keyID string, userID uuid.UUID, reason string) error {
	key, err := s.GetKey(ctx, keyID)
	if err != nil {
		return err
	}

	key.Revoke(userID, reason)

	if err := s.db.WithContext(ctx).Save(key).Error; err != nil {
		return fmt.Errorf("failed to revoke key: %w", err)
	}

	s.logger.Info("API key revoked",
		zap.String("key_id", keyID),
		zap.String("reason", reason))

	return nil
}

// CreateDefaultKeyForUser creates a default API key for a new user
func (s *Service) CreateDefaultKeyForUser(ctx context.Context, userID uuid.UUID, teamID uuid.UUID) (*models.Key, error) {
	req := CreateKeyRequest{
		Name:   "Default API Key",
		Type:   models.KeyTypeAPI,
		UserID: &userID,
		TeamID: &teamID,
		Scopes: []string{"*"}, // Full access for default key
	}
	
	return s.CreateKey(ctx, req)
}

// Request and response types
type CreateKeyRequest struct {
	Name      string              `json:"name" binding:"required"`
	Type      models.KeyType      `json:"type"`
	UserID    *uuid.UUID          `json:"user_id"`
	TeamID    *uuid.UUID          `json:"team_id"`
	Duration  *int                `json:"duration"` // in seconds
	MaxBudget *float64            `json:"max_budget"`
	TPM       *int                `json:"tpm"`
	RPM       *int                `json:"rpm"`
	Scopes    []string            `json:"scopes"`
	ExpiresAt *time.Time          `json:"expires_at"`
}

type UpdateKeyRequest struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`
	TPM       *int       `json:"tpm"`
	RPM       *int       `json:"rpm"`
	MaxBudget *float64   `json:"max_budget"`
	IsActive  *bool      `json:"is_active"`
}

type ListKeysRequest struct {
	UserID *uuid.UUID `json:"user_id"`
	TeamID *uuid.UUID `json:"team_id"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

// Error definitions
var (
	ErrKeyNotFound = fmt.Errorf("key not found")
	ErrKeyExpired  = fmt.Errorf("key expired")
)