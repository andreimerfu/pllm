package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type APIKey struct {
	BaseModel
	Name        string     `gorm:"not null" json:"name"`
	KeyHash     string     `gorm:"uniqueIndex;not null" json:"-"`
	KeyPrefix   string     `gorm:"index;not null" json:"key_prefix"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	Scopes      []string   `gorm:"type:text[]" json:"scopes"`
	RateLimit   int        `gorm:"default:60" json:"rate_limit"`
	
	// Relationships
	UserID  uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	User    User      `gorm:"foreignKey:UserID" json:"-"`
	GroupID *uuid.UUID `gorm:"type:uuid" json:"group_id"`
	Group   *Group    `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	
	// Metadata
	Metadata map[string]interface{} `gorm:"type:jsonb" json:"metadata,omitempty"`
	
	// Usage tracking
	TotalRequests int64 `gorm:"default:0" json:"total_requests"`
	TotalTokens   int64 `gorm:"default:0" json:"total_tokens"`
	TotalCost     float64 `gorm:"default:0" json:"total_cost"`
}

type APIKeyResponse struct {
	APIKey
	Key string `json:"key,omitempty"`
}

func GenerateAPIKey() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	
	key := fmt.Sprintf("pllm_sk_%s", hex.EncodeToString(b))
	
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])
	
	return key, keyHash, nil
}

func (k *APIKey) BeforeCreate(tx *gorm.DB) error {
	if err := k.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}
	
	if k.KeyPrefix == "" && k.KeyHash != "" {
		k.KeyPrefix = k.KeyHash[:8]
	}
	
	return nil
}

func (k *APIKey) IsValid() bool {
	if !k.IsActive {
		return false
	}
	
	if k.ExpiresAt != nil && k.ExpiresAt.Before(time.Now()) {
		return false
	}
	
	return true
}

func (k *APIKey) HasScope(scope string) bool {
	if len(k.Scopes) == 0 {
		return true
	}
	
	for _, s := range k.Scopes {
		if s == scope || s == "*" {
			return true
		}
	}
	
	return false
}

func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}