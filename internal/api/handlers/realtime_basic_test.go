package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/llm/realtime"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRealtimeHandler_BasicFunctionality(t *testing.T) {
	// Create logger
	logger := zap.NewNop()

	// Create session config for manager
	sessionConfig := &realtime.SessionConfig{
		MaxSessions:     100,
		SessionTimeout:  time.Hour,
		CleanupInterval: time.Minute,
	}

	// Create session manager
	sessionManager := realtime.NewSessionManager(logger, nil, sessionConfig)

	// Create realtime handler with nil model manager (for basic tests)
	handler := &RealtimeHandler{
		logger:         logger,
		sessionManager: sessionManager,
		modelManager:   nil, // We'll test basic functionality without model validation
	}

	t.Run("ListSessions_Empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/realtime/sessions", nil)
		rr := httptest.NewRecorder()

		handler.ListSessions(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var response map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "sessions")
		assert.Contains(t, response, "count")
		assert.Equal(t, float64(0), response["count"])
	})

	t.Run("GetSession_NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/realtime/sessions/nonexistent", nil)
		rr := httptest.NewRecorder()

		// Create a router to handle the request with URL parameter
		r := chi.NewRouter()
		r.Get("/v1/realtime/sessions/{id}", handler.GetSession)
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("CreateSession_MissingModel", func(t *testing.T) {
		payload := map[string]interface{}{
			"session": map[string]interface{}{
				"modalities": []string{"text"},
			},
		}

		body, err := json.Marshal(payload)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/realtime/sessions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.CreateSession(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("CreateSession_InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/realtime/sessions", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.CreateSession(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestRealtimeEvent_JSONMarshaling(t *testing.T) {
	t.Run("SessionCreatedEvent", func(t *testing.T) {
		threshold := float32(0.5)
		prefixPadding := int(300)
		silenceDuration := int(200)
		temperature := float32(0.8)
		maxTokens := int(150)

		sessionConfig := &models.RealtimeSessionConfig{
			Modalities:        []string{"text", "audio"},
			Instructions:      "You are a helpful assistant.",
			Voice:            "alloy",
			InputAudioFormat: "pcm16",
			OutputAudioFormat: "pcm16",
			TurnDetection: &models.TurnDetectionConfig{
				Type:               "server_vad",
				Threshold:          &threshold,
				PrefixPaddingMs:    &prefixPadding,
				SilenceDurationMs:  &silenceDuration,
			},
			Tools:                    []models.RealtimeTool{},
			ToolChoice:               "auto",
			Temperature:              &temperature,
			MaxResponseOutputTokens:  &maxTokens,
		}

		sessionCreated := &models.SessionCreatedEvent{
			Session: *sessionConfig,
		}

		event, err := models.NewRealtimeEvent("session.created", sessionCreated)
		require.NoError(t, err)

		// Test marshaling
		data, err := json.Marshal(event)
		require.NoError(t, err)
		assert.Contains(t, string(data), "session.created")

		// Test unmarshaling
		var unmarshaled models.RealtimeEvent
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		assert.Equal(t, "session.created", unmarshaled.Type)
	})

	t.Run("ErrorEvent", func(t *testing.T) {
		errorEvent := &models.ErrorEvent{
			Type:    "error",
			Code:    "model_not_available",
			Message: "Model gpt-4-realtime-preview is not available",
			Param:   "model",
			EventID: "error-123",
		}

		event, err := models.NewRealtimeEvent("error", errorEvent)
		require.NoError(t, err)

		// Test marshaling
		data, err := json.Marshal(event)
		require.NoError(t, err)
		assert.Contains(t, string(data), "error")
		assert.Contains(t, string(data), "model_not_available")

		// Test unmarshaling
		var unmarshaled models.RealtimeEvent
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		assert.Equal(t, "error", unmarshaled.Type)
	})
}

func TestSessionManager_BasicOperations(t *testing.T) {
	logger := zap.NewNop()
	config := &realtime.SessionConfig{
		MaxSessions:     2,
		SessionTimeout:  time.Hour,
		CleanupInterval: time.Minute,
	}

	sm := realtime.NewSessionManager(logger, nil, config)
	defer sm.Close()

	t.Run("GetSession_NotExists", func(t *testing.T) {
		_, exists := sm.GetSession("nonexistent")
		assert.False(t, exists)
	})

	t.Run("ListSessions_Empty", func(t *testing.T) {
		sessions := sm.ListSessions()
		assert.Empty(t, sessions)
	})

	t.Run("GetSessionCount_Zero", func(t *testing.T) {
		count := sm.GetSessionCount()
		assert.Equal(t, 0, count)
	})
}