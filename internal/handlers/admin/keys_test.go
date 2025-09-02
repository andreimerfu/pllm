package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/middleware"
	"github.com/amerfu/pllm/internal/models"
	"github.com/amerfu/pllm/internal/services/budget"
	"github.com/amerfu/pllm/internal/services/key"
	"github.com/amerfu/pllm/internal/testutil"
)

// newTestLogger creates a test logger
func newTestLogger(t *testing.T) *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	logger, _ := config.Build()
	return logger
}


// mockBudgetService implements the budget.Service interface for testing
type mockBudgetService struct{}

func (m *mockBudgetService) CheckBudget(ctx context.Context, keyID uuid.UUID, estimatedCost float64) (*budget.BudgetCheck, error) {
	return &budget.BudgetCheck{
		Allowed:         true,
		RemainingBudget: 100.0,
		UsedBudget:      0.0,
		TotalBudget:     100.0,
	}, nil
}

func (m *mockBudgetService) CheckBudgetCached(ctx context.Context, entityType, entityID string, requestCost float64) (bool, error) {
	return true, nil
}

func (m *mockBudgetService) RecordUsage(ctx context.Context, keyID uuid.UUID, cost float64, model string, inputTokens, outputTokens int) error {
	return nil
}

func (m *mockBudgetService) UpdateSpending(ctx context.Context, entityType, entityID string, cost float64) error {
	return nil
}

// createAuthContext creates a test authentication context
func createAuthContext(userID uuid.UUID) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.UserContextKey, userID)
	ctx = context.WithValue(ctx, middleware.AuthTypeContextKey, middleware.AuthTypeJWT)
	return ctx
}

func TestKeyHandler_CreateKey(t *testing.T) {
	logger := newTestLogger(t)
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	budgetSvc := &mockBudgetService{}
	handler := NewKeyHandler(logger, db, budgetSvc)

	// Create a test user
	testUserID := uuid.New()
	testUser := models.User{
		BaseModel: models.BaseModel{ID: testUserID},
		Email:     "test@example.com",
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&testUser).Error)

	tests := []struct {
		name           string
		requestBody    CreateKeyRequest
		expectedStatus int
		expectedType   models.KeyType
		checkResponse  func(t *testing.T, response KeyResponse)
	}{
		{
			name: "Create API Key",
			requestBody: CreateKeyRequest{
				Name:    "Test API Key",
				KeyType: "api",
			},
			expectedStatus: http.StatusCreated,
			expectedType:   models.KeyTypeAPI,
			checkResponse: func(t *testing.T, response KeyResponse) {
				assert.Equal(t, "Test API Key", response.Name)
				assert.Equal(t, models.KeyTypeAPI, response.Type)
				assert.NotEmpty(t, response.PlaintextKey)
				assert.True(t, response.IsActive)
				assert.Contains(t, response.PlaintextKey, "sk-api")
			},
		},
		{
			name: "Create Virtual Key",
			requestBody: CreateKeyRequest{
				Name:    "Test Virtual Key",
				KeyType: "virtual",
			},
			expectedStatus: http.StatusCreated,
			expectedType:   models.KeyTypeVirtual,
			checkResponse: func(t *testing.T, response KeyResponse) {
				assert.Equal(t, "Test Virtual Key", response.Name)
				assert.Equal(t, models.KeyTypeVirtual, response.Type)
				assert.NotEmpty(t, response.PlaintextKey)
				assert.True(t, response.IsActive)
				assert.Contains(t, response.PlaintextKey, "sk-vrt")
			},
		},
		{
			name: "Create System Key",
			requestBody: CreateKeyRequest{
				Name:    "Test System Key",
				KeyType: "system",
			},
			expectedStatus: http.StatusCreated,
			expectedType:   models.KeyTypeSystem,
			checkResponse: func(t *testing.T, response KeyResponse) {
				assert.Equal(t, "Test System Key", response.Name)
				assert.Equal(t, models.KeyTypeSystem, response.Type)
				assert.NotEmpty(t, response.PlaintextKey)
				assert.True(t, response.IsActive)
				assert.Contains(t, response.PlaintextKey, "sk-sys")
			},
		},
		{
			name: "Create Key with User ID",
			requestBody: CreateKeyRequest{
				Name:    "Test User Key",
				KeyType: "api",
				UserID:  &testUserID,
			},
			expectedStatus: http.StatusCreated,
			expectedType:   models.KeyTypeAPI,
			checkResponse: func(t *testing.T, response KeyResponse) {
				assert.Equal(t, "Test User Key", response.Name)
				assert.Equal(t, &testUserID, response.UserID)
			},
		},
		{
			name: "Create Key with Expiration",
			requestBody: CreateKeyRequest{
				Name:      "Test Expiring Key",
				KeyType:   "api",
				ExpiresAt: &time.Time{},
			},
			expectedStatus: http.StatusCreated,
			expectedType:   models.KeyTypeAPI,
			checkResponse: func(t *testing.T, response KeyResponse) {
				assert.Equal(t, "Test Expiring Key", response.Name)
				assert.NotNil(t, response.ExpiresAt)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			body, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/admin/keys", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			// Add auth context
			ctx := createAuthContext(testUserID)
			req = req.WithContext(ctx)

			// Create response recorder
			w := httptest.NewRecorder()

			// Call handler
			handler.CreateKey(w, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusCreated {
				// Parse response
				var response KeyResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				// Run custom checks
				if tt.checkResponse != nil {
					tt.checkResponse(t, response)
				}

				// Verify key is stored in database
				var dbKey models.Key
				err = db.Where("id = ?", response.ID).First(&dbKey).Error
				require.NoError(t, err)
				assert.Equal(t, tt.expectedType, dbKey.Type)
				assert.Equal(t, tt.requestBody.Name, dbKey.Name)
				assert.True(t, dbKey.IsActive)
				assert.NotEmpty(t, dbKey.KeyHash)

				// Verify key prefix is set correctly
				assert.NotEmpty(t, dbKey.KeyPrefix)
			}
		})
	}
}

func TestKeyHandler_CreateKey_ValidationErrors(t *testing.T) {
	logger := newTestLogger(t)
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	budgetSvc := &mockBudgetService{}
	handler := NewKeyHandler(logger, db, budgetSvc)

	testUserID := uuid.New()

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "Invalid JSON",
			requestBody: `{"invalid": json}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name: "Missing Name",
			requestBody: CreateKeyRequest{
				KeyType: "api",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name: "Invalid Key Type",
			requestBody: CreateKeyRequest{
				Name:    "Test Key",
				KeyType: "invalid",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid key type",
		},
		{
			name: "Empty Key Type",
			requestBody: CreateKeyRequest{
				Name:    "Test Key",
				KeyType: "",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid key type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			var err error

			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/admin/keys", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			// Add auth context
			ctx := createAuthContext(testUserID)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.CreateKey(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				var errorResponse map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
				require.NoError(t, err)
				assert.Contains(t, errorResponse["error"], tt.expectedError)
			}
		})
	}
}

func TestKeyGenerator_AllKeyTypes(t *testing.T) {
	keyGen := key.NewKeyGenerator()

	t.Run("Generate API Key", func(t *testing.T) {
		plaintext, hash, err := keyGen.GenerateAPIKey()
		require.NoError(t, err)
		assert.Contains(t, plaintext, "sk-api")
		assert.NotEmpty(t, hash)
		assert.True(t, keyGen.IsAPIKey(plaintext))
		assert.False(t, keyGen.IsVirtualKey(plaintext))
		assert.False(t, keyGen.IsMasterKey(plaintext))
		assert.False(t, keyGen.IsSystemKey(plaintext))
	})

	t.Run("Generate Virtual Key", func(t *testing.T) {
		plaintext, hash, err := keyGen.GenerateVirtualKey()
		require.NoError(t, err)
		assert.Contains(t, plaintext, "sk-vrt")
		assert.NotEmpty(t, hash)
		assert.True(t, keyGen.IsVirtualKey(plaintext))
		assert.False(t, keyGen.IsAPIKey(plaintext))
		assert.False(t, keyGen.IsMasterKey(plaintext))
		assert.False(t, keyGen.IsSystemKey(plaintext))
	})

	t.Run("Generate Master Key", func(t *testing.T) {
		plaintext, hash, err := keyGen.GenerateMasterKey()
		require.NoError(t, err)
		assert.Contains(t, plaintext, "sk-mst")
		assert.NotEmpty(t, hash)
		assert.True(t, keyGen.IsMasterKey(plaintext))
		assert.False(t, keyGen.IsAPIKey(plaintext))
		assert.False(t, keyGen.IsVirtualKey(plaintext))
		assert.False(t, keyGen.IsSystemKey(plaintext))
	})

	t.Run("Generate System Key", func(t *testing.T) {
		plaintext, hash, err := keyGen.GenerateSystemKey()
		require.NoError(t, err)
		assert.Contains(t, plaintext, "sk-sys")
		assert.NotEmpty(t, hash)
		assert.True(t, keyGen.IsSystemKey(plaintext))
		assert.False(t, keyGen.IsAPIKey(plaintext))
		assert.False(t, keyGen.IsVirtualKey(plaintext))
		assert.False(t, keyGen.IsMasterKey(plaintext))
	})
}

func TestKeyModel_Validation(t *testing.T) {
	t.Run("Key Expiration", func(t *testing.T) {
		future := time.Now().Add(1 * time.Hour)
		past := time.Now().Add(-1 * time.Hour)

		key := models.Key{
			Name:      "Test Key",
			Type:      models.KeyTypeAPI,
			IsActive:  true,
			ExpiresAt: &future,
		}
		assert.False(t, key.IsExpired())
		assert.True(t, key.CanUse())

		key.ExpiresAt = &past
		assert.True(t, key.IsExpired())
		assert.False(t, key.CanUse())
	})

	t.Run("Key Revocation", func(t *testing.T) {
		key := models.Key{
			Name:     "Test Key",
			Type:     models.KeyTypeAPI,
			IsActive: true,
		}
		assert.False(t, key.IsRevoked())
		assert.True(t, key.CanUse())

		userID := uuid.New()
		key.Revoke(userID, "Test revocation")
		assert.True(t, key.IsRevoked())
		assert.False(t, key.CanUse())
		assert.False(t, key.IsActive)
		assert.Equal(t, "Test revocation", key.RevocationReason)
	})

	t.Run("Key Type Detection", func(t *testing.T) {
		tests := []struct {
			keyPrefix    string
			expectedType models.KeyType
		}{
			{"sk-api-", models.KeyTypeAPI},
			{"sk-vrt-", models.KeyTypeVirtual},
			{"sk-mst-", models.KeyTypeMaster},
			{"sk-sys-", models.KeyTypeSystem},
		}

		for _, tt := range tests {
			key := models.Key{
				Key: tt.keyPrefix + "test123",
			}
			assert.Equal(t, tt.expectedType, key.GetType())
		}
	})

	t.Run("Model Access Control", func(t *testing.T) {
		key := models.Key{
			Name:          "Test Key",
			Type:          models.KeyTypeAPI,
			AllowedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			BlockedModels: []string{"gpt-4-vision"},
		}

		assert.True(t, key.IsModelAllowed("gpt-4"))
		assert.True(t, key.IsModelAllowed("gpt-3.5-turbo"))
		assert.False(t, key.IsModelAllowed("gpt-4-vision"))
		assert.False(t, key.IsModelAllowed("claude-3"))

		// Test wildcard access
		key.AllowedModels = []string{"*"}
		assert.True(t, key.IsModelAllowed("any-model"))
		assert.False(t, key.IsModelAllowed("gpt-4-vision")) // Still blocked
	})
}