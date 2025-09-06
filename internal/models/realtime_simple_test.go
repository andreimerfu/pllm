package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealtimeEvent_BasicJSON(t *testing.T) {
	event := RealtimeEvent{
		Type:      "session.update",
		EventID:   "event_123",
		Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	// Test marshaling
	data, err := json.Marshal(event)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"type":"session.update"`)
	assert.Contains(t, string(data), `"event_id":"event_123"`)

	// Test unmarshaling
	var unmarshaled RealtimeEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, event.Type, unmarshaled.Type)
	assert.Equal(t, event.EventID, unmarshaled.EventID)
}

func TestRealtimeSessionConfig_BasicJSON(t *testing.T) {
	temp := float32(0.8)
	config := RealtimeSessionConfig{
		Modalities:        []string{"text", "audio"},
		Instructions:      "You are a helpful assistant.",
		Voice:             "alloy",
		InputAudioFormat:  "pcm16",
		OutputAudioFormat: "pcm16",
		Temperature:       &temp,
	}

	// Test marshaling
	data, err := json.Marshal(config)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"modalities":["text","audio"]`)
	assert.Contains(t, string(data), `"voice":"alloy"`)
	assert.Contains(t, string(data), `"temperature":0.8`)

	// Test unmarshaling
	var unmarshaled RealtimeSessionConfig
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, config.Modalities, unmarshaled.Modalities)
	assert.Equal(t, config.Voice, unmarshaled.Voice)
	assert.Equal(t, *config.Temperature, *unmarshaled.Temperature)
}

func TestRealtimeSession_DatabaseModel(t *testing.T) {
	session := RealtimeSession{
		ID:        "sess_123",
		ModelName: "gpt-4-realtime-preview",
		Status:    "active",
	}

	// Test JSON marshaling
	data, err := json.Marshal(session)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"sess_123"`)
	assert.Contains(t, string(data), `"model_name":"gpt-4-realtime-preview"`)
	assert.Contains(t, string(data), `"status":"active"`)

	// Test unmarshaling
	var unmarshaled RealtimeSession
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, session.ID, unmarshaled.ID)
	assert.Equal(t, session.ModelName, unmarshaled.ModelName)
	assert.Equal(t, session.Status, unmarshaled.Status)
}

func TestAudioTranscriptionConfig_JSON(t *testing.T) {
	config := AudioTranscriptionConfig{
		Model: "whisper-1",
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"model":"whisper-1"`)

	var unmarshaled AudioTranscriptionConfig
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, config.Model, unmarshaled.Model)
}

func TestTurnDetectionConfig_JSON(t *testing.T) {
	threshold := float32(0.5)
	paddingMs := 300
	silenceMs := 200

	config := TurnDetectionConfig{
		Type:              "server_vad",
		Threshold:         &threshold,
		PrefixPaddingMs:   &paddingMs,
		SilenceDurationMs: &silenceMs,
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"type":"server_vad"`)
	assert.Contains(t, string(data), `"threshold":0.5`)

	var unmarshaled TurnDetectionConfig
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, config.Type, unmarshaled.Type)
	assert.Equal(t, *config.Threshold, *unmarshaled.Threshold)
	assert.Equal(t, *config.PrefixPaddingMs, *unmarshaled.PrefixPaddingMs)
	assert.Equal(t, *config.SilenceDurationMs, *unmarshaled.SilenceDurationMs)
}

func TestRealtimeTool_JSON(t *testing.T) {
	tool := RealtimeTool{
		Type:        "function",
		Name:        "get_weather",
		Description: "Get current weather for a location",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "City name",
				},
			},
		},
	}

	data, err := json.Marshal(tool)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"type":"function"`)
	assert.Contains(t, string(data), `"name":"get_weather"`)

	var unmarshaled RealtimeTool
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, tool.Type, unmarshaled.Type)
	assert.Equal(t, tool.Name, unmarshaled.Name)
	assert.Equal(t, tool.Description, unmarshaled.Description)
}

func TestSessionUpdateEvent_JSON(t *testing.T) {
	temp := float32(0.8)
	event := SessionUpdateEvent{
		Session: RealtimeSessionConfig{
			Modalities:   []string{"text"},
			Instructions: "Test instructions",
			Temperature:  &temp,
		},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"session"`)
	assert.Contains(t, string(data), `"modalities":["text"]`)

	var unmarshaled SessionUpdateEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, event.Session.Modalities, unmarshaled.Session.Modalities)
	assert.Equal(t, *event.Session.Temperature, *unmarshaled.Session.Temperature)
}

func TestInputAudioBufferAppendEvent_JSON(t *testing.T) {
	event := InputAudioBufferAppendEvent{
		Audio: "base64encodedaudio",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"audio":"base64encodedaudio"`)

	var unmarshaled InputAudioBufferAppendEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, event.Audio, unmarshaled.Audio)
}

func TestErrorEvent_JSON(t *testing.T) {
	event := ErrorEvent{
		Type:    "error",
		Code:    "invalid_request",
		Message: "Invalid request format",
		Param:   "model",
		EventID: "event_123",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"type":"error"`)
	assert.Contains(t, string(data), `"code":"invalid_request"`)
	assert.Contains(t, string(data), `"message":"Invalid request format"`)

	var unmarshaled ErrorEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, event.Type, unmarshaled.Type)
	assert.Equal(t, event.Code, unmarshaled.Code)
	assert.Equal(t, event.Message, unmarshaled.Message)
	assert.Equal(t, event.Param, unmarshaled.Param)
	assert.Equal(t, event.EventID, unmarshaled.EventID)
}