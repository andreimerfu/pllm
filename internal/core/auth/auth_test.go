package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/integrations/key"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)



// mockTeamService implements TeamService for testing
type mockTeamService struct{}

func (m *mockTeamService) AddUserToDefaultTeam(ctx context.Context, userID uuid.UUID, role models.TeamRole) (*models.TeamMember, error) {
	return &models.TeamMember{
		UserID: userID,
		TeamID: uuid.New(),
		Role:   role,
	}, nil
}

// mockKeyService implements KeyService for testing
type mockKeyService struct{}

func (m *mockKeyService) CreateDefaultKeyForUser(ctx context.Context, userID uuid.UUID, teamID uuid.UUID) (*models.Key, error) {
	keyGen := key.NewKeyGenerator()
	_, hash, err := keyGen.GenerateAPIKey()
	if err != nil {
		return nil, err
	}

	return &models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Default Key",
		KeyHash:   hash,
		Type:      models.KeyTypeAPI,
		UserID:    &userID,
		TeamID:    &teamID,
		IsActive:  true,
	}, nil
}

func TestAuthService_ValidateKey(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Create master key service
	masterKeyConfig := &MasterKeyConfig{
		DB:          db,
		MasterKey:   "test-master-key",
		JWTSecret:   []byte("test-secret"),
		JWTIssuer:   "test-issuer",
		TokenExpiry: time.Hour,
	}
	masterKeySvc := NewMasterKeyService(masterKeyConfig)

	// Create auth service
	authConfig := &AuthConfig{
		DB:               db,
		JWTSecret:        "test-secret",
		JWTIssuer:        "test-issuer",
		TokenExpiry:      time.Hour,
		MasterKeyService: masterKeySvc,
		TeamService:      &mockTeamService{},
		KeyService:       &mockKeyService{},
	}

	authSvc, err := NewAuthService(authConfig)
	require.NoError(t, err)

	// Create test data
	testUserID := uuid.New()
	testTeamID := uuid.New()

	// Create test user
	testUser := models.User{
		BaseModel: models.BaseModel{ID: testUserID},
		Email:     "test@example.com",
		Username:  "testuser",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&testUser).Error)

	// Create test team
	testTeam := models.Team{
		BaseModel:   models.BaseModel{ID: testTeamID},
		Name:        "Test Team",
		Description: "Test team for testing",
		IsActive:    true,
	}
	require.NoError(t, db.Create(&testTeam).Error)

	// Generate different types of keys
	keyGen := key.NewKeyGenerator()

	apiKeyPlaintext, apiKeyHash, err := keyGen.GenerateAPIKey()
	require.NoError(t, err)

	virtualKeyPlaintext, virtualKeyHash, err := keyGen.GenerateVirtualKey()
	require.NoError(t, err)

	systemKeyPlaintext, systemKeyHash, err := keyGen.GenerateSystemKey()
	require.NoError(t, err)

	// Create test keys in database
	apiKey := models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Test API Key",
		Key:       apiKeyPlaintext,
		KeyHash:   apiKeyHash,
		Type:      models.KeyTypeAPI,
		UserID:    &testUserID,
		TeamID:    &testTeamID,
		IsActive:  true,
	}
	require.NoError(t, db.Create(&apiKey).Error)

	virtualKey := models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Test Virtual Key",
		Key:       virtualKeyPlaintext,
		KeyHash:   virtualKeyHash,
		Type:      models.KeyTypeVirtual,
		UserID:    &testUserID,
		IsActive:  true,
	}
	require.NoError(t, db.Create(&virtualKey).Error)

	systemKey := models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Test System Key",
		Key:       systemKeyPlaintext,
		KeyHash:   systemKeyHash,
		Type:      models.KeyTypeSystem,
		IsActive:  true,
	}
	require.NoError(t, db.Create(&systemKey).Error)

	// Create expired key
	expiredKeyPlaintext, expiredKeyHash, err := keyGen.GenerateAPIKey()
	require.NoError(t, err)
	pastTime := time.Now().Add(-1 * time.Hour)
	expiredKey := models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Expired Key",
		Key:       expiredKeyPlaintext,
		KeyHash:   expiredKeyHash,
		Type:      models.KeyTypeAPI,
		UserID:    &testUserID,
		IsActive:  true,
		ExpiresAt: &pastTime,
	}
	require.NoError(t, db.Create(&expiredKey).Error)

	// Create inactive key
	inactiveKeyPlaintext, inactiveKeyHash, err := keyGen.GenerateAPIKey()
	require.NoError(t, err)
	inactiveKey := models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Inactive Key",
		Key:       inactiveKeyPlaintext,
		KeyHash:   inactiveKeyHash,
		Type:      models.KeyTypeAPI,
		UserID:    &testUserID,
		IsActive:  true, // Create as active first
	}
	require.NoError(t, db.Create(&inactiveKey).Error)
	
	// Then explicitly set to inactive to override the default
	require.NoError(t, db.Model(&inactiveKey).Update("is_active", false).Error)

	ctx := context.Background()

	t.Run("Validate Master Key", func(t *testing.T) {
		validatedKey, err := authSvc.ValidateKey(ctx, "test-master-key")
		require.NoError(t, err)
		assert.Equal(t, models.KeyTypeMaster, validatedKey.Type)
		assert.Equal(t, "Master Key", validatedKey.Name)
		assert.True(t, validatedKey.IsActive)
	})

	t.Run("Validate API Key", func(t *testing.T) {
		validatedKey, err := authSvc.ValidateKey(ctx, apiKeyPlaintext)
		require.NoError(t, err)
		assert.Equal(t, models.KeyTypeAPI, validatedKey.Type)
		assert.Equal(t, "Test API Key", validatedKey.Name)
		assert.Equal(t, &testUserID, validatedKey.UserID)
		assert.Equal(t, &testTeamID, validatedKey.TeamID)
		assert.True(t, validatedKey.IsActive)
	})

	t.Run("Validate Virtual Key", func(t *testing.T) {
		validatedKey, err := authSvc.ValidateKey(ctx, virtualKeyPlaintext)
		require.NoError(t, err)
		assert.Equal(t, models.KeyTypeVirtual, validatedKey.Type)
		assert.Equal(t, "Test Virtual Key", validatedKey.Name)
		assert.Equal(t, &testUserID, validatedKey.UserID)
		assert.True(t, validatedKey.IsActive)
	})

	t.Run("Validate System Key", func(t *testing.T) {
		validatedKey, err := authSvc.ValidateKey(ctx, systemKeyPlaintext)
		require.NoError(t, err)
		assert.Equal(t, models.KeyTypeSystem, validatedKey.Type)
		assert.Equal(t, "Test System Key", validatedKey.Name)
		assert.Nil(t, validatedKey.UserID) // System keys don't have users
		assert.True(t, validatedKey.IsActive)
	})

	t.Run("Validate Invalid Key", func(t *testing.T) {
		_, err := authSvc.ValidateKey(ctx, "invalid-key")
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidAPIKey, err)
	})

	t.Run("Validate Expired Key", func(t *testing.T) {
		_, err := authSvc.ValidateKey(ctx, expiredKeyPlaintext)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidAPIKey, err)
	})

	t.Run("Validate Inactive Key", func(t *testing.T) {
		_, err := authSvc.ValidateKey(ctx, inactiveKeyPlaintext)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidAPIKey, err)
	})

	t.Run("Validate Non-existent Key", func(t *testing.T) {
		nonExistentPlaintext, _, err := keyGen.GenerateAPIKey()
		require.NoError(t, err)
		
		_, err = authSvc.ValidateKey(ctx, nonExistentPlaintext)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidAPIKey, err)
	})

	t.Run("Key Usage Statistics Update", func(t *testing.T) {
		// Get initial usage count
		var initialKey models.Key
		require.NoError(t, db.Where("key_hash = ?", apiKeyHash).First(&initialKey).Error)
		initialUsageCount := initialKey.UsageCount

		// Validate key (should update last used time)
		validatedKey, err := authSvc.ValidateKey(ctx, apiKeyPlaintext)
		require.NoError(t, err)

		// Check that last used time was updated
		assert.NotNil(t, validatedKey.LastUsedAt)

		// Verify in database
		var updatedKey models.Key
		require.NoError(t, db.Where("key_hash = ?", apiKeyHash).First(&updatedKey).Error)
		assert.NotNil(t, updatedKey.LastUsedAt)
		assert.Equal(t, initialUsageCount+1, updatedKey.UsageCount)
	})
}

func TestMasterKeyService_ValidateMasterKey(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	config := &MasterKeyConfig{
		DB:          db,
		MasterKey:   "test-master-key-123",
		JWTSecret:   []byte("test-jwt-secret"),
		JWTIssuer:   "test-issuer",
		TokenExpiry: time.Hour,
	}

	masterKeySvc := NewMasterKeyService(config)
	ctx := context.Background()

	t.Run("Valid Master Key", func(t *testing.T) {
		masterCtx, err := masterKeySvc.ValidateMasterKey(ctx, "test-master-key-123")
		require.NoError(t, err)
		assert.NotNil(t, masterCtx)
		assert.Equal(t, models.KeyTypeMaster, masterCtx.KeyType)
		assert.True(t, masterCtx.IsActive)
		assert.Contains(t, masterCtx.Scopes, "*")
	})

	t.Run("Invalid Master Key", func(t *testing.T) {
		_, err := masterKeySvc.ValidateMasterKey(ctx, "wrong-master-key")
		assert.Error(t, err)
	})

	t.Run("Empty Master Key", func(t *testing.T) {
		_, err := masterKeySvc.ValidateMasterKey(ctx, "")
		assert.Error(t, err)
	})

	t.Run("Master Key Not Configured", func(t *testing.T) {
		emptyConfig := &MasterKeyConfig{
			DB:          db,
			MasterKey:   "",
			JWTSecret:   []byte("test-jwt-secret"),
			JWTIssuer:   "test-issuer",
			TokenExpiry: time.Hour,
		}
		emptySvc := NewMasterKeyService(emptyConfig)
		
		_, err := emptySvc.ValidateMasterKey(ctx, "any-key")
		assert.Error(t, err)
	})
}

func TestKeyValidation_ModelAccess(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Create a key with model restrictions
	keyGen := key.NewKeyGenerator()
	keyPlaintext, keyHash, err := keyGen.GenerateAPIKey()
	require.NoError(t, err)

	testKey := models.Key{
		BaseModel:     models.BaseModel{ID: uuid.New()},
		Name:          "Restricted Key",
		Key:           keyPlaintext,
		KeyHash:       keyHash,
		Type:          models.KeyTypeAPI,
		IsActive:      true,
		AllowedModels: []string{"gpt-4", "gpt-3.5-turbo"},
		BlockedModels: []string{"gpt-4-vision"},
	}
	require.NoError(t, db.Create(&testKey).Error)

	t.Run("Allowed Model Access", func(t *testing.T) {
		assert.True(t, testKey.IsModelAllowed("gpt-4"))
		assert.True(t, testKey.IsModelAllowed("gpt-3.5-turbo"))
	})

	t.Run("Blocked Model Access", func(t *testing.T) {
		assert.False(t, testKey.IsModelAllowed("gpt-4-vision"))
	})

	t.Run("Unspecified Model Access", func(t *testing.T) {
		assert.False(t, testKey.IsModelAllowed("claude-3")) // Not in allowed list
	})

	t.Run("Wildcard Access", func(t *testing.T) {
		wildcardKey := models.Key{
			AllowedModels: []string{"*"},
			BlockedModels: []string{"gpt-4-vision"},
		}
		
		assert.True(t, wildcardKey.IsModelAllowed("any-model"))
		assert.False(t, wildcardKey.IsModelAllowed("gpt-4-vision")) // Still blocked
	})

	t.Run("No Restrictions", func(t *testing.T) {
		unrestrictedKey := models.Key{
			AllowedModels: []string{},
			BlockedModels: []string{},
		}
		
		assert.True(t, unrestrictedKey.IsModelAllowed("any-model"))
	})
}

func TestKeyValidation_BudgetAndLimits(t *testing.T) {
	budget := 100.0
	testKey := models.Key{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		Name:         "Budget Key",
		Type:         models.KeyTypeAPI,
		IsActive:     true,
		MaxBudget:    &budget,
		CurrentSpend: 50.0,
	}

	t.Run("Budget Not Exceeded", func(t *testing.T) {
		assert.False(t, testKey.IsBudgetExceeded())
		assert.True(t, testKey.IsValid())
	})

	t.Run("Budget Exceeded", func(t *testing.T) {
		testKey.CurrentSpend = 150.0
		assert.True(t, testKey.IsBudgetExceeded())
		assert.False(t, testKey.IsValid())
	})

	t.Run("No Budget Limit", func(t *testing.T) {
		noBudgetKey := models.Key{
			BaseModel:    models.BaseModel{ID: uuid.New()},
			Name:         "No Budget Key",
			Type:         models.KeyTypeAPI,
			IsActive:     true,
			MaxBudget:    nil,
			CurrentSpend: 1000.0,
		}
		
		assert.False(t, noBudgetKey.IsBudgetExceeded())
		assert.True(t, noBudgetKey.IsValid())
	})

	t.Run("Record Usage", func(t *testing.T) {
		initialTokens := testKey.TotalTokens
		initialCost := testKey.TotalCost
		initialUsage := testKey.UsageCount
		
		testKey.RecordUsage(1000, 0.02)
		
		assert.Equal(t, initialTokens+1000, testKey.TotalTokens)
		assert.Equal(t, initialCost+0.02, testKey.TotalCost)
		assert.Equal(t, initialUsage+1, testKey.UsageCount)
		assert.NotNil(t, testKey.LastUsedAt)
	})

	t.Run("Rate Limits", func(t *testing.T) {
		tpm := 1000
		rpm := 100
		parallel := 5
		
		rateLimitKey := models.Key{
			TPM:              &tpm,
			RPM:              &rpm,
			MaxParallelCalls: &parallel,
		}
		
		gotTPM, gotRPM, gotParallel := rateLimitKey.GetEffectiveRateLimits(500, 50, 3)
		assert.Equal(t, 1000, gotTPM)
		assert.Equal(t, 100, gotRPM)
		assert.Equal(t, 5, gotParallel)
	})
}