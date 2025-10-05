package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/auth"
	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/integrations/key"
	modelsService "github.com/amerfu/pllm/internal/services/llm/models"
	"github.com/amerfu/pllm/internal/services/llm/providers"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

// newTestLoggerLLM creates a test logger for LLM tests
func newTestLoggerLLM(t *testing.T) *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	logger, _ := config.Build()
	return logger
}


func createMockModelManager() *modelsService.ModelManager {
	// Create a minimal mock that won't panic
	logger, _ := zap.NewDevelopment()
	router := config.RouterSettings{}
	manager := modelsService.NewModelManager(logger, router, nil)
	return manager
}

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

// setupTestAuth creates auth service with test keys
func setupTestAuth(t *testing.T, db *gorm.DB) (*auth.AuthService, *auth.MasterKeyService, map[string]string) {

	// Create master key service
	masterKeyConfig := &auth.MasterKeyConfig{
		DB:          db,
		MasterKey:   "test-master-key-123",
		JWTSecret:   []byte("test-secret"),
		JWTIssuer:   "test-issuer",
		TokenExpiry: time.Hour,
	}
	masterKeySvc := auth.NewMasterKeyService(masterKeyConfig)

	// Create auth service
	authConfig := &auth.AuthConfig{
		DB:               db,
		JWTSecret:        "test-secret",
		JWTIssuer:        "test-issuer",
		TokenExpiry:      time.Hour,
		MasterKeyService: masterKeySvc,
		TeamService:      &mockTeamService{},
		KeyService:       &mockKeyService{},
	}

	authSvc, err := auth.NewAuthService(authConfig)
	require.NoError(t, err)

	// Create test user and team
	testUserID := uuid.New()
	testTeamID := uuid.New()

	testUser := models.User{
		BaseModel: models.BaseModel{ID: testUserID},
		Email:     "test@example.com",
		Username:  "testuser",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&testUser).Error)

	testTeam := models.Team{
		BaseModel: models.BaseModel{ID: testTeamID},
		Name:      "Test Team",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&testTeam).Error)

	// Generate test keys
	keyGen := key.NewKeyGenerator()
	keys := make(map[string]string)

	// API Key
	apiKeyPlaintext, apiKeyHash, err := keyGen.GenerateAPIKey()
	require.NoError(t, err)
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
	keys["api"] = apiKeyPlaintext

	// Virtual Key
	virtualKeyPlaintext, virtualKeyHash, err := keyGen.GenerateVirtualKey()
	require.NoError(t, err)
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
	keys["virtual"] = virtualKeyPlaintext

	// System Key
	systemKeyPlaintext, systemKeyHash, err := keyGen.GenerateSystemKey()
	require.NoError(t, err)
	systemKey := models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Test System Key",
		Key:       systemKeyPlaintext,
		KeyHash:   systemKeyHash,
		Type:      models.KeyTypeSystem,
		IsActive:  true,
	}
	require.NoError(t, db.Create(&systemKey).Error)
	keys["system"] = systemKeyPlaintext

	// Master Key
	keys["master"] = "test-master-key-123"

	return authSvc, masterKeySvc, keys
}

func TestChatCompletions_AuthenticationTypes(t *testing.T) {
	logger := newTestLoggerLLM(t)
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	modelManager := createMockModelManager()
	authSvc, masterKeySvc, testKeys := setupTestAuth(t, db)

	// Create Chat handler
	chatHandler := NewChatHandler(logger, modelManager)

	// Create auth middleware
	authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthConfig{
		Logger:           logger,
		AuthService:      authSvc,
		MasterKeyService: masterKeySvc,
		RequireAuth:      true,
	})

	// Test chat completion request
	chatRequest := providers.ChatRequest{
		Model: "test-model",
		Messages: []providers.Message{
			{
				Role:    "user",
				Content: "Hello, world!",
			},
		},
		Stream: false,
	}

	tests := []struct {
		name       string
		keyType    string
		authHeader string
		expectAuth bool
	}{
		{
			name:       "API Key Authentication",
			keyType:    "api",
			authHeader: "Bearer " + testKeys["api"],
			expectAuth: true,
		},
		{
			name:       "Virtual Key Authentication",
			keyType:    "virtual",
			authHeader: "Bearer " + testKeys["virtual"],
			expectAuth: true,
		},
		{
			name:       "System Key Authentication",
			keyType:    "system",
			authHeader: "Bearer " + testKeys["system"],
			expectAuth: true,
		},
		{
			name:       "Master Key Authentication",
			keyType:    "master",
			authHeader: "Bearer " + testKeys["master"],
			expectAuth: true,
		},
		{
			name:       "API Key with X-API-Key Header",
			keyType:    "api",
			authHeader: "",
			expectAuth: true,
		},
		{
			name:       "No Authentication",
			keyType:    "",
			authHeader: "",
			expectAuth: false,
		},
		{
			name:       "Invalid Key",
			keyType:    "invalid",
			authHeader: "Bearer invalid-key-123",
			expectAuth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request body
			body, err := json.Marshal(chatRequest)
			require.NoError(t, err)

			// Create HTTP request
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Set authentication header
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			} else if tt.keyType == "api" && tt.authHeader == "" {
				// Test X-API-Key header
				req.Header.Set("X-API-Key", testKeys["api"])
			}

			// Create response recorder
			w := httptest.NewRecorder()

			// Apply auth middleware
			handler := authMiddleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// If we reach here, authentication succeeded
				// Check auth context
				authType := r.Context().Value(middleware.AuthTypeContextKey)
				
				if tt.expectAuth {
					assert.NotEqual(t, middleware.AuthTypeNone, authType)
					
					// Check specific auth type based on key type
					switch tt.keyType {
					case "master":
						assert.Equal(t, middleware.AuthTypeMasterKey, authType)
						assert.NotNil(t, r.Context().Value(middleware.MasterKeyContextKey))
					case "api", "virtual", "system":
						assert.Equal(t, middleware.AuthTypeAPIKey, authType)
						key := r.Context().Value(middleware.KeyContextKey)
						assert.NotNil(t, key)
						
						if keyModel, ok := key.(*models.Key); ok {
							switch tt.keyType {
							case "api":
								assert.Equal(t, models.KeyTypeAPI, keyModel.Type)
							case "virtual":
								assert.Equal(t, models.KeyTypeVirtual, keyModel.Type)
							case "system":
								assert.Equal(t, models.KeyTypeSystem, keyModel.Type)
							}
						}
					}
				}

				// Call the actual LLM handler
				chatHandler.ChatCompletions(w, r)
			}))

			// Execute request
			handler.ServeHTTP(w, req)

			if tt.expectAuth {
				// Should not be 401 Unauthorized
				assert.NotEqual(t, http.StatusUnauthorized, w.Code, "Expected authentication to succeed")
				
				// For our mock, we expect 503 (no real model instance available)
				// but the important part is that auth succeeded
				if w.Code == http.StatusServiceUnavailable {
					// This is expected - our mock doesn't have real model instances
					t.Logf("Got expected 503 - model not available (auth succeeded)")
				} else {
					t.Logf("Got status %d, response: %s", w.Code, w.Body.String())
				}
			} else {
				// Should be 401 Unauthorized
				assert.Equal(t, http.StatusUnauthorized, w.Code, "Expected authentication to fail")
			}
		})
	}
}

func TestChatCompletions_KeyRestrictions(t *testing.T) {
	logger := newTestLoggerLLM(t)
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	modelManager := createMockModelManager()
	
	// Create auth service
	authSvc, masterKeySvc, _ := setupTestAuth(t, db)

	// Create test user first  
	testUserID := uuid.New()
	testUser := models.User{
		BaseModel: models.BaseModel{ID: testUserID},
		Email:     "restricted@example.com",
		Username:  "restricteduser",
		DexID:     "restricted-user-dex-id", // Unique dex_id to avoid constraint violation
		IsActive:  true,
	}
	require.NoError(t, db.Create(&testUser).Error)

	// Create restricted key
	keyGen := key.NewKeyGenerator()
	restrictedKeyPlaintext, restrictedKeyHash, err := keyGen.GenerateAPIKey()
	require.NoError(t, err)

	restrictedKey := models.Key{
		BaseModel:     models.BaseModel{ID: uuid.New()},
		Name:          "Restricted Key",
		Key:           restrictedKeyPlaintext,
		KeyHash:       restrictedKeyHash,
		Type:          models.KeyTypeAPI,
		UserID:        &testUserID,
		IsActive:      true,
		AllowedModels: pq.StringArray{"gpt-4"},           // Only allow gpt-4
		BlockedModels: pq.StringArray{"test-model"},      // Block test-model
	}
	require.NoError(t, db.Create(&restrictedKey).Error)

	// Create Chat handler
	chatHandler := NewChatHandler(logger, modelManager)

	// Create auth middleware
	authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthConfig{
		Logger:           logger,
		AuthService:      authSvc,
		MasterKeyService: masterKeySvc,
		RequireAuth:      true,
	})

	tests := []struct {
		name          string
		requestModel  string
		expectSuccess bool
		description   string
	}{
		{
			name:          "Allowed Model",
			requestModel:  "gpt-4",
			expectSuccess: true,
			description:   "Should succeed - gpt-4 is in allowed list",
		},
		{
			name:          "Blocked Model",
			requestModel:  "test-model",
			expectSuccess: false,
			description:   "Should fail - test-model is in blocked list",
		},
		{
			name:          "Unspecified Model",
			requestModel:  "claude-3",
			expectSuccess: false,
			description:   "Should fail - claude-3 is not in allowed list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create chat request with specific model
			chatRequest := providers.ChatRequest{
				Model: tt.requestModel,
				Messages: []providers.Message{
					{
						Role:    "user",
						Content: "Hello, world!",
					},
				},
				Stream: false,
			}

			body, err := json.Marshal(chatRequest)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+restrictedKeyPlaintext)

			w := httptest.NewRecorder()

			// Apply auth middleware with model restriction check
			handler := authMiddleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Get the key from context
				key := r.Context().Value(middleware.KeyContextKey)
				if keyModel, ok := key.(*models.Key); ok {
					// Check if model is allowed
					if !keyModel.IsModelAllowed(tt.requestModel) {
						w.WriteHeader(http.StatusForbidden)
						_ = json.NewEncoder(w).Encode(map[string]string{
							"error": "Model access denied",
						})
						return
					}
				}

				// Call the actual LLM handler
				chatHandler.ChatCompletions(w, r)
			}))

			handler.ServeHTTP(w, req)

			if tt.expectSuccess {
				assert.NotEqual(t, http.StatusForbidden, w.Code, tt.description)
				assert.NotEqual(t, http.StatusUnauthorized, w.Code, tt.description)
			} else {
				assert.Equal(t, http.StatusForbidden, w.Code, tt.description)
			}

			t.Logf("%s: Status %d - %s", tt.name, w.Code, tt.description)
		})
	}
}

func TestChatCompletions_HeaderFormats(t *testing.T) {
	logger := newTestLoggerLLM(t)
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	authSvc, masterKeySvc, testKeys := setupTestAuth(t, db)

	// Create auth middleware
	authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthConfig{
		Logger:           logger,
		AuthService:      authSvc,
		MasterKeyService: masterKeySvc,
		RequireAuth:      true,
	})

	tests := []struct {
		name         string
		setHeaders   func(req *http.Request)
		expectAuth   bool
		expectedType middleware.AuthType
	}{
		{
			name: "Bearer Token Format",
			setHeaders: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+testKeys["api"])
			},
			expectAuth:   true,
			expectedType: middleware.AuthTypeAPIKey,
		},
		{
			name: "X-API-Key Header",
			setHeaders: func(req *http.Request) {
				req.Header.Set("X-API-Key", testKeys["api"])
			},
			expectAuth:   true,
			expectedType: middleware.AuthTypeAPIKey,
		},
		{
			name: "Master Key Bearer",
			setHeaders: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+testKeys["master"])
			},
			expectAuth:   true,
			expectedType: middleware.AuthTypeMasterKey,
		},
		{
			name: "System Key Bearer",
			setHeaders: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+testKeys["system"])
			},
			expectAuth:   true,
			expectedType: middleware.AuthTypeAPIKey,
		},
		{
			name: "No Authorization Header",
			setHeaders: func(req *http.Request) {
				// No headers set
			},
			expectAuth:   false,
			expectedType: middleware.AuthTypeNone,
		},
		{
			name: "Invalid Bearer Format",
			setHeaders: func(req *http.Request) {
				req.Header.Set("Authorization", "InvalidFormat "+testKeys["api"])
			},
			expectAuth:   false,
			expectedType: middleware.AuthTypeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			tt.setHeaders(req)

			w := httptest.NewRecorder()

			handler := authMiddleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authType := r.Context().Value(middleware.AuthTypeContextKey)
				
				if tt.expectAuth {
					assert.Equal(t, tt.expectedType, authType)
					w.WriteHeader(http.StatusOK)
				} else {
					// Should not reach here if auth is required
					t.Error("Handler should not be called when auth is expected to fail")
				}
			}))

			handler.ServeHTTP(w, req)

			if tt.expectAuth {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusUnauthorized, w.Code)
			}
		})
	}
}

func TestChatCompletions_ConcurrentKeyUsage(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	authSvc, _, testKeys := setupTestAuth(t, db)

	// Test concurrent usage of the same key
	numRequests := 10
	results := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			ctx := context.Background()
			
			// Validate key concurrently
			_, err := authSvc.ValidateKey(ctx, testKeys["api"])
			results <- (err == nil)
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		if <-results {
			successCount++
		}
	}

	// All requests should succeed
	assert.Equal(t, numRequests, successCount, "All concurrent key validations should succeed")

	// Verify usage count was incremented for each request
	var finalKey models.Key
	keyHash := models.HashKey(testKeys["api"])
	require.NoError(t, db.Where("key_hash = ?", keyHash).First(&finalKey).Error)
	
	// Usage count should be at least the number of concurrent requests
	// (it might be higher due to setup calls)
	assert.GreaterOrEqual(t, int(finalKey.UsageCount), numRequests)
}