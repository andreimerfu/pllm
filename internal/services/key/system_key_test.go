package key

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/models"
	"github.com/amerfu/pllm/internal/testutil"
)


// newTestLogger creates a test logger
func newTestLogger() *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	logger, _ := config.Build()
	return logger
}

func TestSystemKey_Creation(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	_ = newTestLogger()
	_ = NewService(db, newTestLogger())
	keyGen := NewKeyGenerator()

	_ = context.Background()

	t.Run("Create System Key Without User", func(t *testing.T) {
		// System keys should not require a user
		systemKeyPlaintext, systemKeyHash, err := keyGen.GenerateSystemKey()
		require.NoError(t, err)

		systemKey := models.Key{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Key:       systemKeyPlaintext, // Store plaintext for unique constraint
			Name:      "Backend Service Key",
			KeyHash:   systemKeyHash,
			Type:      models.KeyTypeSystem,
			UserID:    nil, // No user ownership
			TeamID:    nil, // No team ownership
			IsActive:  true,
			Scopes:    pq.StringArray{"chat:completions", "embeddings", "models:list"},
		}

		err = db.Create(&systemKey).Error
		require.NoError(t, err)

		// No need to update scopes - they're set in the struct

		// Verify key was created correctly
		var retrievedKey models.Key
		err = db.Where("id = ?", systemKey.ID).First(&retrievedKey).Error
		require.NoError(t, err)

		assert.Equal(t, models.KeyTypeSystem, retrievedKey.Type)
		assert.Nil(t, retrievedKey.UserID)
		assert.Nil(t, retrievedKey.TeamID)
		assert.True(t, retrievedKey.IsActive)
		assert.Contains(t, retrievedKey.Scopes, "chat:completions")
		
		// Verify key format
		assert.Contains(t, systemKeyPlaintext, "sk-sys")
		assert.True(t, keyGen.IsSystemKey(systemKeyPlaintext))
	})

	t.Run("System Key Scope Validation", func(t *testing.T) {
		systemKeyPlaintext, systemKeyHash, err := keyGen.GenerateSystemKey()
		require.NoError(t, err)

		systemKey := models.Key{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Key:       systemKeyPlaintext, // Store plaintext for unique constraint
			Name:      "Limited System Key",
			KeyHash:   systemKeyHash,
			Type:      models.KeyTypeSystem,
			IsActive:  true,
			Scopes:    pq.StringArray{"embeddings", "models:list"},
		}

		err = db.Create(&systemKey).Error
		require.NoError(t, err)

		// No need to update scopes - they're set in the struct

		// Test scope checking
		assert.True(t, systemKey.HasScope("embeddings"))
		assert.True(t, systemKey.HasScope("models:list"))
		assert.False(t, systemKey.HasScope("chat:completions"))
		
		// Test wildcard scope
		systemKey.Scopes = []string{"*"}
		assert.True(t, systemKey.HasScope("any:scope"))
	})

	t.Run("System Key Rate Limiting", func(t *testing.T) {
		// System keys should support custom rate limits for backend services
		tpm := 10000 // Higher limit for backend services
		rpm := 1000
		parallel := 10

		systemKey := models.Key{
			BaseModel:        models.BaseModel{ID: uuid.New()},
			Name:             "High-Performance System Key",
			Type:             models.KeyTypeSystem,
			IsActive:         true,
			TPM:              &tpm,
			RPM:              &rpm,
			MaxParallelCalls: &parallel,
		}

		// Get effective rate limits
		effectiveTPM, effectiveRPM, effectiveParallel := systemKey.GetEffectiveRateLimits(1000, 100, 3)

		assert.Equal(t, 10000, effectiveTPM)
		assert.Equal(t, 1000, effectiveRPM)
		assert.Equal(t, 10, effectiveParallel)
	})

	t.Run("System Key Model Restrictions", func(t *testing.T) {
		// System keys should support model restrictions
		systemKey := models.Key{
			BaseModel:     models.BaseModel{ID: uuid.New()},
			Name:          "Restricted System Key",
			Type:          models.KeyTypeSystem,
			IsActive:      true,
			AllowedModels: []string{"gpt-3.5-turbo", "gpt-4"},
			BlockedModels: []string{"gpt-4-vision"},
		}

		// Test model access
		assert.True(t, systemKey.IsModelAllowed("gpt-3.5-turbo"))
		assert.True(t, systemKey.IsModelAllowed("gpt-4"))
		assert.False(t, systemKey.IsModelAllowed("gpt-4-vision"))
		assert.False(t, systemKey.IsModelAllowed("claude-3"))
	})

	t.Run("System Key Budget Management", func(t *testing.T) {
		// System keys should support budget limits
		budget := 1000.0
		systemKey := models.Key{
			BaseModel:    models.BaseModel{ID: uuid.New()},
			Name:         "Budget-Limited System Key",
			Type:         models.KeyTypeSystem,
			IsActive:     true,
			MaxBudget:    &budget,
			CurrentSpend: 0.0,
		}

		// Initially within budget
		assert.False(t, systemKey.IsBudgetExceeded())
		assert.True(t, systemKey.IsValid())

		// Record usage
		systemKey.RecordUsage(1000, 0.50)
		assert.Equal(t, int64(1000), systemKey.TotalTokens)
		assert.Equal(t, 0.50, systemKey.CurrentSpend)
		assert.False(t, systemKey.IsBudgetExceeded())

		// Exceed budget
		systemKey.RecordUsage(50000, 1000.0)
		assert.True(t, systemKey.IsBudgetExceeded())
		assert.False(t, systemKey.IsValid())
	})
}

func TestSystemKey_Authentication(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	keyGen := NewKeyGenerator()

	// Create system key
	systemKeyPlaintext, systemKeyHash, err := keyGen.GenerateSystemKey()
	require.NoError(t, err)

	systemKey := models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Auth Test System Key",
		KeyHash:   systemKeyHash,
		Type:      models.KeyTypeSystem,
		IsActive:  true,
	}
	require.NoError(t, db.Create(&systemKey).Error)

	t.Run("System Key Authentication Success", func(t *testing.T) {
		// Verify system key can be found by hash
		var retrievedKey models.Key
		err := db.Where("key_hash = ? AND is_active = ?", systemKeyHash, true).First(&retrievedKey).Error
		require.NoError(t, err)

		assert.Equal(t, models.KeyTypeSystem, retrievedKey.Type)
		assert.Equal(t, "Auth Test System Key", retrievedKey.Name)
		assert.True(t, retrievedKey.CanUse())
	})

	t.Run("System Key Format Validation", func(t *testing.T) {
		// Test key format validation
		assert.True(t, keyGen.IsSystemKey(systemKeyPlaintext))
		assert.False(t, keyGen.IsAPIKey(systemKeyPlaintext))
		assert.False(t, keyGen.IsVirtualKey(systemKeyPlaintext))
		assert.False(t, keyGen.IsMasterKey(systemKeyPlaintext))

		// Test prefix extraction
		prefix := keyGen.ExtractKeyPrefix(systemKeyPlaintext)
		assert.Equal(t, "sk-sys", prefix)

		// Test format validation
		assert.NoError(t, keyGen.ValidateKeyFormat(systemKeyPlaintext))
	})

	t.Run("System Key Lifecycle", func(t *testing.T) {
		// Test key expiration
		futureTime := time.Now().Add(1 * time.Hour)
		pastTime := time.Now().Add(-1 * time.Hour)

		systemKey.ExpiresAt = &futureTime
		assert.False(t, systemKey.IsExpired())
		assert.True(t, systemKey.CanUse())

		systemKey.ExpiresAt = &pastTime
		assert.True(t, systemKey.IsExpired())
		assert.False(t, systemKey.CanUse())

		// Test key revocation
		systemKey.ExpiresAt = nil // Reset expiration
		adminUserID := uuid.New()
		systemKey.Revoke(adminUserID, "Security incident")
		
		assert.True(t, systemKey.IsRevoked())
		assert.False(t, systemKey.IsActive)
		assert.False(t, systemKey.CanUse())
		assert.Equal(t, "Security incident", systemKey.RevocationReason)
		assert.Equal(t, &adminUserID, systemKey.RevokedBy)
	})
}

func TestSystemKey_UseCases(t *testing.T) {
	keyGen := NewKeyGenerator()

	t.Run("Microservice Authentication", func(t *testing.T) {
		// Simulate creating keys for different microservices
		services := []struct {
			name     string
			scopes   []string
			models   []string
			tpm      int
		}{
			{
				name:   "chat-service",
				scopes: []string{"chat:completions", "chat:streaming"},
				models: []string{"gpt-3.5-turbo", "gpt-4"},
				tpm:    5000,
			},
			{
				name:   "embedding-service",
				scopes: []string{"embeddings", "models:list"},
				models: []string{"text-embedding-ada-002", "text-embedding-3-small"},
				tpm:    10000,
			},
			{
				name:   "analytics-service",
				scopes: []string{"usage:read", "models:list"},
				models: []string{"*"}, // Can access all models for analytics
				tpm:    1000,
			},
		}

		for _, service := range services {
			// Generate system key for each service
			keyPlaintext, keyHash, err := keyGen.GenerateSystemKey()
			require.NoError(t, err)

			systemKey := models.Key{
				BaseModel:     models.BaseModel{ID: uuid.New()},
				Name:          service.name + "-system-key",
				KeyHash:       keyHash,
				Type:          models.KeyTypeSystem,
				IsActive:      true,
				Scopes:        service.scopes,
				AllowedModels: service.models,
				TPM:           &service.tpm,
			}

			// Verify key properties
			assert.Contains(t, keyPlaintext, "sk-sys")
			assert.Equal(t, models.KeyTypeSystem, systemKey.Type)
			assert.True(t, systemKey.IsActive)

			// Test scope validation
			for _, scope := range service.scopes {
				assert.True(t, systemKey.HasScope(scope), 
					"Service %s should have scope %s", service.name, scope)
			}

			// Test model access
			for _, model := range service.models {
				if model == "*" {
					assert.True(t, systemKey.IsModelAllowed("any-model"))
				} else {
					assert.True(t, systemKey.IsModelAllowed(model),
						"Service %s should have access to model %s", service.name, model)
				}
			}

			// Test rate limits
			effectiveTPM, _, _ := systemKey.GetEffectiveRateLimits(1000, 100, 3)
			assert.Equal(t, service.tpm, effectiveTPM,
				"Service %s should have TPM limit of %d", service.name, service.tpm)

			t.Logf("Created system key for %s: %s", service.name, keyPlaintext[:20]+"...")
		}
	})

	t.Run("Backend Service Integration", func(t *testing.T) {
		// Test a complete backend service integration scenario
		_, keyHash, err := keyGen.GenerateSystemKey()
		require.NoError(t, err)

		// Create a system key with realistic backend service requirements
		budget := 500.0
		tpm := 8000
		rpm := 800
		parallel := 15

		backendKey := models.Key{
			BaseModel:        models.BaseModel{ID: uuid.New()},
			Name:             "production-backend-service",
			KeyHash:          keyHash,
			Type:             models.KeyTypeSystem,
			IsActive:         true,
			MaxBudget:        &budget,
			TPM:              &tpm,
			RPM:              &rpm,
			MaxParallelCalls: &parallel,
			Scopes:           []string{"chat:completions", "embeddings", "models:list", "usage:write"},
			AllowedModels:    []string{"gpt-3.5-turbo", "gpt-4", "text-embedding-ada-002"},
			Tags:             []string{"production", "backend", "critical"},
		}

		// Simulate service operations
		assert.True(t, backendKey.IsValid())
		assert.True(t, backendKey.HasScope("chat:completions"))
		assert.True(t, backendKey.IsModelAllowed("gpt-4"))
		assert.False(t, backendKey.IsBudgetExceeded())

		// Simulate usage over time
		usageEvents := []struct {
			tokens int
			cost   float64
		}{
			{1000, 0.02},
			{1500, 0.03},
			{2000, 0.04},
			{800, 0.016},
		}

		for _, usage := range usageEvents {
			backendKey.RecordUsage(usage.tokens, usage.cost)
		}

		// Verify accumulated usage
		expectedTokens := int64(1000 + 1500 + 2000 + 800)
		expectedCost := 0.02 + 0.03 + 0.04 + 0.016
		expectedUsageCount := int64(len(usageEvents))

		assert.Equal(t, expectedTokens, backendKey.TotalTokens)
		assert.InDelta(t, expectedCost, backendKey.TotalCost, 0.001)
		assert.Equal(t, expectedUsageCount, backendKey.UsageCount)
		assert.NotNil(t, backendKey.LastUsedAt)

		// Should still be within budget
		assert.False(t, backendKey.IsBudgetExceeded())
		assert.True(t, backendKey.IsValid())

		t.Logf("Backend service key processed %d tokens for $%.3f", 
			backendKey.TotalTokens, backendKey.TotalCost)
	})

	t.Run("System Key Security Features", func(t *testing.T) {
		// Test security-focused features for system keys
		keyPlaintext, keyHash, err := keyGen.GenerateSystemKey()
		require.NoError(t, err)

		secureKey := models.Key{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Name:      "secure-system-key",
			KeyHash:   keyHash,
			Type:      models.KeyTypeSystem,
			IsActive:  true,
			// Restricted access
			AllowedModels: []string{"gpt-3.5-turbo"}, // Only one model
			BlockedModels: []string{"gpt-4-vision"},  // Explicitly block expensive models
			Scopes:        []string{"chat:completions"}, // Limited scope
			// No metadata in test to avoid JSON conversion issues
		}

		// Test security restrictions
		assert.True(t, secureKey.IsModelAllowed("gpt-3.5-turbo"))
		assert.False(t, secureKey.IsModelAllowed("gpt-4"))
		assert.False(t, secureKey.IsModelAllowed("gpt-4-vision"))
		assert.True(t, secureKey.HasScope("chat:completions"))
		assert.False(t, secureKey.HasScope("embeddings"))

		// Metadata removed for test simplicity

		// Test key validation
		assert.NoError(t, keyGen.ValidateKeyFormat(keyPlaintext))
		assert.True(t, keyGen.IsSystemKey(keyPlaintext))

		t.Logf("Secure system key created with restricted access: %s", 
			secureKey.Name)
	})
}