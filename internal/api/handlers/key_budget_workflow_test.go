package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/core/models"
	budgetService "github.com/amerfu/pllm/internal/services/data/budget"
	redisService "github.com/amerfu/pllm/internal/services/data/redis"
	keyService "github.com/amerfu/pllm/internal/services/integrations/key"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

// TestAPIKeyBudgetWorkflow_EndToEnd simulates real-world budget scenarios
func TestAPIKeyBudgetWorkflow_EndToEnd(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	redisClient, redisCleanup := testutil.NewTestRedis(t)
	defer redisCleanup()

	logger := zap.NewNop()
	ctx := context.Background()

	// Setup services
	budgetCache := redisService.NewBudgetCache(redisClient, logger, 5*time.Minute)
	eventPub := redisService.NewEventPublisher(redisClient, logger)

	// Don't use async usage queue in tests - use synchronous recording instead
	service := budgetService.NewUnifiedService(&budgetService.UnifiedServiceConfig{
		DB:          db,
		Logger:      logger,
		BudgetCache: budgetCache,
		UsageQueue:  nil, // nil forces synchronous recording
		EventPub:    eventPub,
	})

	keyGen := keyService.NewKeyGenerator()

	t.Run("Scenario_CreateKey_UseUntilExhausted_Increase_ContinueUsing", func(t *testing.T) {
		// Step 1: Create user
		user := models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     "workflow-" + uuid.New().String() + "@example.com",
			Username:  "workflowuser-" + uuid.New().String(),
			IsActive:  true,
			DexID:     uuid.New().String(),
		}
		require.NoError(t, db.Create(&user).Error)

		// Step 2: Create API key with budget limit
		plaintext, hash, err := keyGen.GenerateAPIKey()
		require.NoError(t, err)

		budget := 50.0
		key := models.Key{
			BaseModel:    models.BaseModel{ID: uuid.New()},
			Name:         "Workflow Test Key",
			Key:          plaintext,
			KeyHash:      hash,
			Type:         models.KeyTypeAPI,
			UserID:       &user.ID,
			IsActive:     true,
			MaxBudget:    &budget,
			CurrentSpend: 0.0,
		}
		require.NoError(t, db.Create(&key).Error)

		// Step 3: Make multiple requests within budget
		for i := 0; i < 4; i++ {
			result, err := service.CheckBudget(ctx, key.ID, 10.0)
			require.NoError(t, err)
			assert.True(t, result.Allowed, "Request %d should be allowed", i+1)

			// Record the usage
			err = service.UpdateSpending(ctx, "key", key.ID.String(), 10.0)
			require.NoError(t, err)
		}


		// Verify spending
		var updatedKey models.Key
		require.NoError(t, db.First(&updatedKey, key.ID).Error)
		assert.Equal(t, 40.0, updatedKey.CurrentSpend)

		// Step 4: Try request that would exceed budget
		result, err := service.CheckBudget(ctx, key.ID, 15.0)
		require.NoError(t, err)
		assert.False(t, result.Allowed, "Request should be denied (40 + 15 > 50)")
		assert.Equal(t, 10.0, result.RemainingBudget)

		// Step 5: Increase budget
		newBudget := 100.0
		require.NoError(t, db.Model(&key).Update("max_budget", newBudget).Error)

		// Step 6: Previous request should now be allowed
		result, err = service.CheckBudget(ctx, key.ID, 15.0)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Request should now be allowed after budget increase")
		assert.Equal(t, 60.0, result.RemainingBudget)

		// Record the usage
		err = service.UpdateSpending(ctx, "key", key.ID.String(), 15.0)
		require.NoError(t, err)

		// Step 7: Continue using with new budget
		result, err = service.CheckBudget(ctx, key.ID, 30.0)
		require.NoError(t, err)
		assert.True(t, result.Allowed)

		err = service.UpdateSpending(ctx, "key", key.ID.String(), 30.0)
		require.NoError(t, err)


		// Verify final spending
		require.NoError(t, db.First(&updatedKey, key.ID).Error)
		assert.Equal(t, 85.0, updatedKey.CurrentSpend)
		assert.Equal(t, 100.0, *updatedKey.MaxBudget)
	})

	t.Run("Scenario_RapidRequests_BudgetExhaustion", func(t *testing.T) {
		// Create user
		user := models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     "rapid-" + uuid.New().String() + "@example.com",
			Username:  "rapiduser-" + uuid.New().String(),
			IsActive:  true,
			DexID:     uuid.New().String(),
		}
		require.NoError(t, db.Create(&user).Error)

		// Create key with small budget
		plaintext, hash, err := keyGen.GenerateAPIKey()
		require.NoError(t, err)

		budget := 100.0
		key := models.Key{
			BaseModel:    models.BaseModel{ID: uuid.New()},
			Name:         "Rapid Test Key",
			Key:          plaintext,
			KeyHash:      hash,
			Type:         models.KeyTypeAPI,
			UserID:       &user.ID,
			IsActive:     true,
			MaxBudget:    &budget,
			CurrentSpend: 0.0,
		}
		require.NoError(t, db.Create(&key).Error)

		// Simulate rapid requests
		const numRequests = 15
		const costPerRequest = 8.0

		successCount := 0
		deniedCount := 0

		for i := 0; i < numRequests; i++ {
			result, err := service.CheckBudget(ctx, key.ID, costPerRequest)
			require.NoError(t, err)

			if result.Allowed {
				successCount++
				err = service.UpdateSpending(ctx, "key", key.ID.String(), costPerRequest)
				require.NoError(t, err)
			} else {
				deniedCount++
			}
		}


		// Should allow ~12 requests (96) and deny ~3 requests
		assert.Equal(t, 12, successCount, "Should allow 12 requests")
		assert.Equal(t, 3, deniedCount, "Should deny 3 requests")

		// Verify final state
		var finalKey models.Key
		require.NoError(t, db.First(&finalKey, key.ID).Error)
		assert.Equal(t, 96.0, finalKey.CurrentSpend)
	})

	t.Run("Scenario_TeamBudget_MultipleKeys_SharedLimit", func(t *testing.T) {
		// Create team with shared budget
		team := models.Team{
			BaseModel:    models.BaseModel{ID: uuid.New()},
			Name:         "Workflow Team " + uuid.New().String(),
			MaxBudget:    200.0,
			CurrentSpend: 0.0,
			IsActive:     true,
		}
		require.NoError(t, db.Create(&team).Error)

		// Create two users in the team
		user1 := models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     "team1-" + uuid.New().String() + "@example.com",
			Username:  "teamuser1-" + uuid.New().String(),
			IsActive:  true,
			DexID:     uuid.New().String(),
		}
		require.NoError(t, db.Create(&user1).Error)

		user2 := models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     "team2-" + uuid.New().String() + "@example.com",
			Username:  "teamuser2-" + uuid.New().String(),
			IsActive:  true,
			DexID:     uuid.New().String(),
		}
		require.NoError(t, db.Create(&user2).Error)

		// Create keys for both users
		plaintext1, hash1, _ := keyGen.GenerateAPIKey()
		key1 := models.Key{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Name:      "Team Key 1",
			Key:       plaintext1,
			KeyHash:   hash1,
			Type:      models.KeyTypeAPI,
			UserID:    &user1.ID,
			TeamID:    &team.ID,
			IsActive:  true,
		}
		require.NoError(t, db.Create(&key1).Error)

		plaintext2, hash2, _ := keyGen.GenerateAPIKey()
		key2 := models.Key{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Name:      "Team Key 2",
			Key:       plaintext2,
			KeyHash:   hash2,
			Type:      models.KeyTypeAPI,
			UserID:    &user2.ID,
			TeamID:    &team.ID,
			IsActive:  true,
		}
		require.NoError(t, db.Create(&key2).Error)

		// User 1 uses budget
		for i := 0; i < 8; i++ {
			result, err := service.CheckBudget(ctx, key1.ID, 15.0)
			require.NoError(t, err)
			assert.True(t, result.Allowed, "User 1 request %d should be allowed", i+1)
			
			// Update team spending
			err = service.UpdateSpending(ctx, "team", team.ID.String(), 15.0)
			require.NoError(t, err)
		}

		// Verify team spending
		var updatedTeam models.Team
		require.NoError(t, db.First(&updatedTeam, team.ID).Error)
		assert.Equal(t, 120.0, updatedTeam.CurrentSpend)

		// User 2 tries to use budget
		result, err := service.CheckBudget(ctx, key2.ID, 30.0)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "User 2 should be allowed (120 + 30 < 200)")

		err = service.UpdateSpending(ctx, "team", team.ID.String(), 30.0)
		require.NoError(t, err)

		// User 2 tries large request
		result, err = service.CheckBudget(ctx, key2.ID, 100.0)
		require.NoError(t, err)
		assert.False(t, result.Allowed, "User 2 should be denied (150 + 100 > 200)")
		assert.Equal(t, 50.0, result.RemainingBudget)

		// User 2 can still use within remaining budget
		result, err = service.CheckBudget(ctx, key2.ID, 40.0)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
	})

	t.Run("Scenario_BudgetReset_ContinueUsage", func(t *testing.T) {
		// Create user
		user := models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     "reset-" + uuid.New().String() + "@example.com",
			Username:  "resetuser-" + uuid.New().String(),
			IsActive:  true,
			DexID:     uuid.New().String(),
		}
		require.NoError(t, db.Create(&user).Error)

		// Create key with budget
		plaintext, hash, err := keyGen.GenerateAPIKey()
		require.NoError(t, err)

		budget := 100.0
		period := models.BudgetPeriodDaily
		key := models.Key{
			BaseModel:      models.BaseModel{ID: uuid.New()},
			Name:           "Reset Test Key",
			Key:            plaintext,
			KeyHash:        hash,
			Type:           models.KeyTypeAPI,
			UserID:         &user.ID,
			IsActive:       true,
			MaxBudget:      &budget,
			BudgetDuration: &period,
			CurrentSpend:   95.0, // Near limit
		}
		require.NoError(t, db.Create(&key).Error)

		// Request should be denied
		result, err := service.CheckBudget(ctx, key.ID, 10.0)
		require.NoError(t, err)
		assert.False(t, result.Allowed)

		// Simulate budget reset (manual reset for test)
		require.NoError(t, db.Model(&key).Update("current_spend", 0.0).Error)

		// Request should now be allowed
		result, err = service.CheckBudget(ctx, key.ID, 10.0)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, 100.0, result.RemainingBudget)
	})

	t.Run("Scenario_CachedBudgetCheck_Performance", func(t *testing.T) {
		// Setup budget in cache for performance testing
		entityType := "key"
		entityID := uuid.New().String()
		
		err := budgetCache.UpdateBudgetCache(ctx, entityType, entityID, 100.0, 0.0, 100.0, false)
		require.NoError(t, err)

		// Perform many cached checks (should be fast)
		start := time.Now()
		const numChecks = 100

		for i := 0; i < numChecks; i++ {
			allowed, err := service.CheckBudgetCached(ctx, entityType, entityID, 5.0)
			require.NoError(t, err)
			assert.True(t, allowed)
		}

		duration := time.Since(start)
		
		// All checks should complete in under 1 second (cached checks are fast)
		assert.Less(t, duration.Seconds(), 1.0, "100 cached checks should complete in under 1 second")
		
		t.Logf("Completed %d cached budget checks in %v (avg: %v per check)", 
			numChecks, duration, duration/numChecks)
	})

	t.Run("Scenario_BudgetExhaustion_WithRetry", func(t *testing.T) {
		// Create user
		user := models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     "retry-" + uuid.New().String() + "@example.com",
			Username:  "retryuser-" + uuid.New().String(),
			IsActive:  true,
			DexID:     uuid.New().String(),
		}
		require.NoError(t, db.Create(&user).Error)

		// Create key with minimal budget
		plaintext, hash, err := keyGen.GenerateAPIKey()
		require.NoError(t, err)

		budget := 30.0
		key := models.Key{
			BaseModel:    models.BaseModel{ID: uuid.New()},
			Name:         "Retry Test Key",
			Key:          plaintext,
			KeyHash:      hash,
			Type:         models.KeyTypeAPI,
			UserID:       &user.ID,
			IsActive:     true,
			MaxBudget:    &budget,
			CurrentSpend: 0.0,
		}
		require.NoError(t, db.Create(&key).Error)

		// First request succeeds
		result, err := service.CheckBudget(ctx, key.ID, 25.0)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		
		err = service.UpdateSpending(ctx, "key", key.ID.String(), 25.0)
		require.NoError(t, err)


		// Second request denied
		result, err = service.CheckBudget(ctx, key.ID, 10.0)
		require.NoError(t, err)
		assert.False(t, result.Allowed)

		// User gets notification and increases budget
		newBudget := 60.0
		require.NoError(t, db.Model(&key).Update("max_budget", newBudget).Error)

		// Retry request now succeeds
		result, err = service.CheckBudget(ctx, key.ID, 10.0)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, 35.0, result.RemainingBudget)
		
		err = service.UpdateSpending(ctx, "key", key.ID.String(), 10.0)
		require.NoError(t, err)

		// Verify final state
		var finalKey models.Key
		require.NoError(t, db.First(&finalKey, key.ID).Error)
		assert.Equal(t, 35.0, finalKey.CurrentSpend)
		assert.Equal(t, 60.0, *finalKey.MaxBudget)
	})
}

// TestAPIKeyBudgetWorkflow_HTTP tests budget enforcement through HTTP handlers
func TestAPIKeyBudgetWorkflow_HTTP(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	redisClient, redisCleanup := testutil.NewTestRedis(t)
	defer redisCleanup()

	logger := zap.NewNop()
	keyGen := keyService.NewKeyGenerator()

	// Setup services
	budgetCache := redisService.NewBudgetCache(redisClient, logger, 5*time.Minute)
	eventPub := redisService.NewEventPublisher(redisClient, logger)

	// Don't use async usage queue in tests - use synchronous recording instead
	service := budgetService.NewUnifiedService(&budgetService.UnifiedServiceConfig{
		DB:          db,
		Logger:      logger,
		BudgetCache: budgetCache,
		UsageQueue:  nil, // nil forces synchronous recording
		EventPub:    eventPub,
	})

	t.Run("HTTP_Request_Budget_Enforcement", func(t *testing.T) {
		// Create user
		user := models.User{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Email:     "http-" + uuid.New().String() + "@example.com",
			Username:  "httpuser-" + uuid.New().String(),
			IsActive:  true,
			DexID:     uuid.New().String(),
		}
		require.NoError(t, db.Create(&user).Error)

		// Create key with budget
		plaintext, hash, err := keyGen.GenerateAPIKey()
		require.NoError(t, err)

		budget := 100.0
		key := models.Key{
			BaseModel:    models.BaseModel{ID: uuid.New()},
			Name:         "HTTP Test Key",
			Key:          plaintext,
			KeyHash:      hash,
			Type:         models.KeyTypeAPI,
			UserID:       &user.ID,
			IsActive:     true,
			MaxBudget:    &budget,
			CurrentSpend: 45.0,
		}
		require.NoError(t, db.Create(&key).Error)

		// Create test router
		r := chi.NewRouter()
		
		// Mock endpoint that checks budget
		r.Post("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
			// Extract key from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// Check budget
			result, err := service.CheckBudget(r.Context(), key.ID, 10.0)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !result.Allowed {
				w.WriteHeader(http.StatusPaymentRequired)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]interface{}{
						"message": result.Message,
						"type":    "budget_exceeded",
						"code":    "budget_limit_reached",
					},
				})
				return
			}

			// Success response
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "success",
			})
		})

		// Test request within budget
		reqBody := bytes.NewBufferString(`{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}`)
		req := httptest.NewRequest("POST", "/v1/chat/completions", reqBody)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", plaintext))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Update spending directly to bring close to budget limit (45 + 50 = 95, leaves 5 remaining)
		err = service.UpdateSpending(context.Background(), "key", key.ID.String(), 50.0)
		require.NoError(t, err)

		// Test request that exceeds budget (95 + 10 = 105 > 100)
		reqBody = bytes.NewBufferString(`{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}`)
		req = httptest.NewRequest("POST", "/v1/chat/completions", reqBody)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", plaintext))
		req.Header.Set("Content-Type", "application/json")

		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusPaymentRequired, w.Code)

		var errorResp map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&errorResp)
		require.NoError(t, err)
		
		errorObj := errorResp["error"].(map[string]interface{})
		assert.Contains(t, errorObj["message"], "would exceed")
	})
}
