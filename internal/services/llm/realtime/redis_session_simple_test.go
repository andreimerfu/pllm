package realtime

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRedisSessionStore_BasicOperations(t *testing.T) {
	// Create mock Redis server
	mockRedis, err := miniredis.Run()
	require.NoError(t, err)
	defer mockRedis.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: mockRedis.Addr(),
	})

	// Create session store
	logger := zap.NewNop()
	store := NewRedisSessionStore(client, logger)

	ctx := context.Background()

	t.Run("StoreAndGetSession", func(t *testing.T) {
		session := &SessionState{
			ID:           "test-session-1",
			UserID:       "user123",
			TeamID:       "team456",
			Model:        "gpt-4-realtime-preview",
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			Config:       map[string]interface{}{"voice": "alloy"},
			PodID:        "pod-1",
			Status:       "active",
		}

		// Store session
		err := store.StoreSession(ctx, session, time.Hour)
		assert.NoError(t, err)

		// Retrieve session
		retrieved, err := store.GetSession(ctx, "test-session-1")
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, session.ID, retrieved.ID)
		assert.Equal(t, session.UserID, retrieved.UserID)
		assert.Equal(t, session.Model, retrieved.Model)
		assert.Equal(t, session.Status, retrieved.Status)
	})

	t.Run("GetSession_NotExists", func(t *testing.T) {
		retrieved, err := store.GetSession(ctx, "nonexistent-session")
		assert.Error(t, err)
		assert.Nil(t, retrieved)
		assert.Contains(t, err.Error(), "session not found")
	})

	t.Run("DeleteSession", func(t *testing.T) {
		session := &SessionState{
			ID:           "test-session-2",
			Model:        "gpt-4-realtime-preview",
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			Status:       "active",
		}

		// Store session
		err := store.StoreSession(ctx, session, time.Hour)
		assert.NoError(t, err)

		// Verify it exists
		retrieved, err := store.GetSession(ctx, "test-session-2")
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)

		// Delete session
		err = store.DeleteSession(ctx, "test-session-2")
		assert.NoError(t, err)

		// Verify it's gone
		retrieved, err = store.GetSession(ctx, "test-session-2")
		assert.Error(t, err)
		assert.Nil(t, retrieved)
	})

	t.Run("SessionWithTTL", func(t *testing.T) {
		session := &SessionState{
			ID:           "test-session-3",
			Model:        "gpt-4-realtime-preview",
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			Status:       "active",
		}

		// Store session with TTL (we just verify it doesn't error)
		err := store.StoreSession(ctx, session, time.Hour)
		assert.NoError(t, err)

		// Verify it was stored
		retrieved, err := store.GetSession(ctx, "test-session-3")
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, session.ID, retrieved.ID)
	})
}

func TestRedisSessionStore_JSONSerialization(t *testing.T) {
	// Test that complex session states are properly serialized/deserialized
	mockRedis, err := miniredis.Run()
	require.NoError(t, err)
	defer mockRedis.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mockRedis.Addr(),
	})

	store := NewRedisSessionStore(client, zap.NewNop())
	ctx := context.Background()

	complexSession := &SessionState{
		ID:           "complex-session",
		UserID:       "user-with-special-chars-123",
		TeamID:       "team-with-dashes-456",
		Model:        "gpt-4-realtime-preview",
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Config: map[string]interface{}{
			"voice":              "alloy",
			"modalities":         []string{"text", "audio"},
			"temperature":        0.8,
			"max_tokens":         150,
			"nested_config": map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
		},
		PodID:  "pod-with-long-name-12345",
		Status: "active",
	}

	// Store and retrieve
	err = store.StoreSession(ctx, complexSession, time.Hour)
	assert.NoError(t, err)

	retrieved, err := store.GetSession(ctx, "complex-session")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)

	// Verify all fields
	assert.Equal(t, complexSession.ID, retrieved.ID)
	assert.Equal(t, complexSession.UserID, retrieved.UserID)
	assert.Equal(t, complexSession.TeamID, retrieved.TeamID)
	assert.Equal(t, complexSession.Model, retrieved.Model)
	assert.Equal(t, complexSession.PodID, retrieved.PodID)
	assert.Equal(t, complexSession.Status, retrieved.Status)

	// Verify config serialization
	assert.Equal(t, "alloy", retrieved.Config["voice"])
	assert.Equal(t, 0.8, retrieved.Config["temperature"])
	assert.Equal(t, float64(150), retrieved.Config["max_tokens"]) // JSON unmarshals numbers as float64

	// Verify nested config
	nestedConfig := retrieved.Config["nested_config"].(map[string]interface{})
	assert.Equal(t, "value1", nestedConfig["key1"])
	assert.Equal(t, float64(42), nestedConfig["key2"])
}