package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/middleware"
	"github.com/amerfu/pllm/internal/services/providers"
	"github.com/amerfu/pllm/internal/testutil"
)



func TestMessagesHandler_AnthropicMessages(t *testing.T) {
	logger := newTestLoggerLLM(t)
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	modelManager := createMockModelManager()
	authSvc, masterKeySvc, testKeys := setupTestAuth(t, db)

	// Create Messages handler
	messagesHandler := NewMessagesHandler(logger, modelManager)

	// Create auth middleware
	authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthConfig{
		Logger:           logger,
		AuthService:      authSvc,
		MasterKeyService: masterKeySvc,
		RequireAuth:      true,
	})

	tests := []struct {
		name           string
		request        providers.MessagesAPIRequest
		authHeader     string
		expectedStatus int
		expectError    bool
		errorMessage   string
	}{
		{
			name: "Valid Messages Request",
			request: providers.MessagesAPIRequest{
				Model:     "test-model",
				MaxTokens: 100,
				Messages: []providers.MessagesAPIMessage{
					{
						Role:    "user",
						Content: "Hello, Claude!",
					},
				},
			},
			authHeader:     "Bearer " + testKeys["api"],
			expectedStatus: http.StatusServiceUnavailable, // No model instance available in tests
			expectError:    true,
			errorMessage:   "No instance available for model: test-model",
		},
		{
			name: "Missing Model",
			request: providers.MessagesAPIRequest{
				MaxTokens: 100,
				Messages: []providers.MessagesAPIMessage{
					{
						Role:    "user",
						Content: "Hello, Claude!",
					},
				},
			},
			authHeader:     "Bearer " + testKeys["api"],
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
			errorMessage:   "model is required",
		},
		{
			name: "Missing MaxTokens",
			request: providers.MessagesAPIRequest{
				Model: "test-model",
				Messages: []providers.MessagesAPIMessage{
					{
						Role:    "user",
						Content: "Hello, Claude!",
					},
				},
			},
			authHeader:     "Bearer " + testKeys["api"],
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
			errorMessage:   "max_tokens is required and must be positive",
		},
		{
			name: "Missing Messages",
			request: providers.MessagesAPIRequest{
				Model:     "test-model",
				MaxTokens: 100,
				Messages:  []providers.MessagesAPIMessage{},
			},
			authHeader:     "Bearer " + testKeys["api"],
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
			errorMessage:   "messages are required",
		},
		{
			name: "Complex Content Format",
			request: providers.MessagesAPIRequest{
				Model:     "test-model",
				MaxTokens: 100,
				Messages: []providers.MessagesAPIMessage{
					{
						Role: "user",
						Content: []providers.MessagesAPIContent{
							{
								Type: "text",
								Text: "Hello, Claude!",
							},
						},
					},
				},
			},
			authHeader:     "Bearer " + testKeys["api"],
			expectedStatus: http.StatusServiceUnavailable, // No model instance available in tests
			expectError:    true,
			errorMessage:   "No instance available for model: test-model",
		},
		{
			name: "With System Message",
			request: providers.MessagesAPIRequest{
				Model:     "test-model",
				MaxTokens: 100,
				System:    "You are a helpful assistant.",
				Messages: []providers.MessagesAPIMessage{
					{
						Role:    "user",
						Content: "Hello, Claude!",
					},
				},
			},
			authHeader:     "Bearer " + testKeys["api"],
			expectedStatus: http.StatusServiceUnavailable, // No model instance available in tests
			expectError:    true,
			errorMessage:   "No instance available for model: test-model",
		},
		{
			name: "With Optional Parameters",
			request: providers.MessagesAPIRequest{
				Model:         "test-model",
				MaxTokens:     100,
				Temperature:   floatPtr(0.7),
				TopP:          floatPtr(0.9),
				TopK:          intPtr(40),
				StopSequences: []string{"Human:", "AI:"},
				Messages: []providers.MessagesAPIMessage{
					{
						Role:    "user",
						Content: "Hello, Claude!",
					},
				},
			},
			authHeader:     "Bearer " + testKeys["api"],
			expectedStatus: http.StatusServiceUnavailable, // No model instance available in tests
			expectError:    true,
			errorMessage:   "No instance available for model: test-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the request
			body, err := json.Marshal(tt.request)
			require.NoError(t, err)

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", tt.authHeader)

			// Create response recorder
			w := httptest.NewRecorder()

			// Create handler with auth middleware
			handler := authMiddleware.Authenticate(http.HandlerFunc(messagesHandler.AnthropicMessages))

			// Execute request
			handler.ServeHTTP(w, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectError {
				// Check error response
				assert.Contains(t, response, "error")
				errorObj := response["error"].(map[string]interface{})
				assert.Contains(t, errorObj["message"], tt.errorMessage)
			}
		})
	}
}

func TestMessagesHandler_ConvertMessagesAPIToOpenAI(t *testing.T) {
	logger := newTestLoggerLLM(t)
	modelManager := createMockModelManager()
	handler := NewMessagesHandler(logger, modelManager)

	tests := []struct {
		name     string
		request  providers.MessagesAPIRequest
		expected providers.ChatRequest
	}{
		{
			name: "Basic conversion",
			request: providers.MessagesAPIRequest{
				Model:     "claude-3-opus",
				MaxTokens: 100,
				Messages: []providers.MessagesAPIMessage{
					{
						Role:    "user",
						Content: "Hello!",
					},
				},
			},
			expected: providers.ChatRequest{
				Model:     "claude-3-opus",
				MaxTokens: intPtr(100),
				Messages: []providers.Message{
					{
						Role:    "user",
						Content: "Hello!",
					},
				},
			},
		},
		{
			name: "With system message",
			request: providers.MessagesAPIRequest{
				Model:     "claude-3-opus",
				MaxTokens: 100,
				System:    "You are a helpful assistant.",
				Messages: []providers.MessagesAPIMessage{
					{
						Role:    "user",
						Content: "Hello!",
					},
				},
			},
			expected: providers.ChatRequest{
				Model:     "claude-3-opus",
				MaxTokens: intPtr(100),
				Messages: []providers.Message{
					{
						Role:    "system",
						Content: "You are a helpful assistant.",
					},
					{
						Role:    "user",
						Content: "Hello!",
					},
				},
			},
		},
		{
			name: "With optional parameters",
			request: providers.MessagesAPIRequest{
				Model:         "claude-3-opus",
				MaxTokens:     200,
				Temperature:   floatPtr(0.7),
				TopP:          floatPtr(0.9),
				StopSequences: []string{"Human:", "AI:"},
				Messages: []providers.MessagesAPIMessage{
					{
						Role:    "user",
						Content: "Hello!",
					},
				},
			},
			expected: providers.ChatRequest{
				Model:       "claude-3-opus",
				MaxTokens:   intPtr(200),
				Temperature: floatPtr(0.7),
				TopP:        floatPtr(0.9),
				Stop:        []string{"Human:", "AI:"},
				Messages: []providers.Message{
					{
						Role:    "user",
						Content: "Hello!",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.convertMessagesAPIToOpenAI(&tt.request)
			require.NoError(t, err)

			assert.Equal(t, tt.expected.Model, result.Model)
			assert.Equal(t, tt.expected.MaxTokens, result.MaxTokens)
			assert.Equal(t, tt.expected.Temperature, result.Temperature)
			assert.Equal(t, tt.expected.TopP, result.TopP)
			assert.Equal(t, tt.expected.Stop, result.Stop)
			assert.Equal(t, len(tt.expected.Messages), len(result.Messages))

			for i, expectedMsg := range tt.expected.Messages {
				assert.Equal(t, expectedMsg.Role, result.Messages[i].Role)
				assert.Equal(t, expectedMsg.Content, result.Messages[i].Content)
			}
		})
	}
}

func TestMessagesHandler_ConvertOpenAIToMessagesAPI(t *testing.T) {
	logger := newTestLoggerLLM(t)
	modelManager := createMockModelManager()
	handler := NewMessagesHandler(logger, modelManager)

	tests := []struct {
		name     string
		response providers.ChatResponse
		request  providers.MessagesAPIRequest
		expected providers.MessagesAPIResponse
	}{
		{
			name: "Basic conversion",
			response: providers.ChatResponse{
				ID:      "test-id",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "gpt-4",
				Choices: []providers.Choice{
					{
						Index: 0,
						Message: providers.Message{
							Role:    "assistant",
							Content: "Hello there!",
						},
						FinishReason: "stop",
					},
				},
				Usage: providers.Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			request: providers.MessagesAPIRequest{
				Model: "claude-3-opus",
			},
			expected: providers.MessagesAPIResponse{
				Type: "message",
				Role: "assistant",
				Content: []providers.MessagesAPIContent{
					{
						Type: "text",
						Text: "Hello there!",
					},
				},
				Model:      "claude-3-opus",
				StopReason: "end_turn",
				Usage: providers.MessagesAPIUsage{
					InputTokens:  10,
					OutputTokens: 5,
				},
			},
		},
		{
			name: "Length finish reason",
			response: providers.ChatResponse{
				Choices: []providers.Choice{
					{
						Message: providers.Message{
							Content: "Truncated response",
						},
						FinishReason: "length",
					},
				},
				Usage: providers.Usage{
					PromptTokens:     20,
					CompletionTokens: 10,
				},
			},
			request: providers.MessagesAPIRequest{
				Model: "claude-3-opus",
			},
			expected: providers.MessagesAPIResponse{
				Type: "message",
				Role: "assistant",
				Content: []providers.MessagesAPIContent{
					{
						Type: "text",
						Text: "Truncated response",
					},
				},
				Model:      "claude-3-opus",
				StopReason: "max_tokens",
				Usage: providers.MessagesAPIUsage{
					InputTokens:  20,
					OutputTokens: 10,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.convertOpenAIToMessagesAPI(&tt.response, &tt.request)
			require.NoError(t, err)

			assert.Equal(t, tt.expected.Type, result.Type)
			assert.Equal(t, tt.expected.Role, result.Role)
			assert.Equal(t, tt.expected.Model, result.Model)
			assert.Equal(t, tt.expected.StopReason, result.StopReason)
			assert.Equal(t, tt.expected.Usage.InputTokens, result.Usage.InputTokens)
			assert.Equal(t, tt.expected.Usage.OutputTokens, result.Usage.OutputTokens)
			assert.Equal(t, len(tt.expected.Content), len(result.Content))

			for i, expectedContent := range tt.expected.Content {
				assert.Equal(t, expectedContent.Type, result.Content[i].Type)
				assert.Equal(t, expectedContent.Text, result.Content[i].Text)
			}

			// Check that ID is generated
			assert.NotEmpty(t, result.ID)
			assert.Contains(t, result.ID, "msg_")
		})
	}
}

func TestMessagesHandler_ConvertOpenAIStreamToMessagesAPI(t *testing.T) {
	logger := newTestLoggerLLM(t)
	modelManager := createMockModelManager()
	handler := NewMessagesHandler(logger, modelManager)

	request := &providers.MessagesAPIRequest{
		Model: "claude-3-opus",
	}

	tests := []struct {
		name     string
		stream   providers.StreamResponse
		expected providers.MessagesAPIStreamResponse
	}{
		{
			name: "Basic stream conversion",
			stream: providers.StreamResponse{
				ID:      "stream-id",
				Object:  "chat.completion.chunk",
				Created: 1234567890,
				Model:   "gpt-4",
				Choices: []providers.StreamChoice{
					{
						Index: 0,
						Delta: providers.Message{
							Content: "Hello",
						},
					},
				},
			},
			expected: providers.MessagesAPIStreamResponse{
				Type:  "content_block_delta",
				Index: 0,
				Delta: map[string]interface{}{
					"type": "text_delta",
					"text": "Hello",
				},
			},
		},
		{
			name: "Empty choices",
			stream: providers.StreamResponse{
				Choices: []providers.StreamChoice{},
			},
			expected: providers.MessagesAPIStreamResponse{
				Type: "content_block_delta",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.convertOpenAIStreamToMessagesAPI(tt.stream, request)
			require.NoError(t, err)

			assert.Equal(t, tt.expected.Type, result.Type)
			assert.Equal(t, tt.expected.Index, result.Index)

			if tt.expected.Delta != nil {
				assert.Equal(t, tt.expected.Delta, result.Delta)
			}
		})
	}
}

func TestMessagesHandler_Authentication(t *testing.T) {
	logger := newTestLoggerLLM(t)
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	modelManager := createMockModelManager()
	authSvc, masterKeySvc, testKeys := setupTestAuth(t, db)

	// Create Messages handler
	messagesHandler := NewMessagesHandler(logger, modelManager)

	// Create auth middleware
	authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthConfig{
		Logger:           logger,
		AuthService:      authSvc,
		MasterKeyService: masterKeySvc,
		RequireAuth:      true,
	})

	// Test messages request
	messagesRequest := providers.MessagesAPIRequest{
		Model:     "test-model",
		MaxTokens: 100,
		Messages: []providers.MessagesAPIMessage{
			{
				Role:    "user",
				Content: "Hello, Claude!",
			},
		},
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
			authHeader: "Bearer test-master-key-123",
			expectAuth: true,
		},
		{
			name:       "Invalid Key",
			keyType:    "invalid",
			authHeader: "Bearer invalid-key",
			expectAuth: false,
		},
		{
			name:       "No Authorization Header",
			keyType:    "none",
			authHeader: "",
			expectAuth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the request
			body, err := json.Marshal(messagesRequest)
			require.NoError(t, err)

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Create response recorder
			w := httptest.NewRecorder()

			// Create handler with auth middleware
			handler := authMiddleware.Authenticate(http.HandlerFunc(messagesHandler.AnthropicMessages))

			// Execute request
			handler.ServeHTTP(w, req)

			if tt.expectAuth {
				// Authentication should succeed, but we expect 503 because no model is available
				assert.Equal(t, http.StatusServiceUnavailable, w.Code, "Got expected 503 - model not available (auth succeeded)")
			} else {
				// Authentication should fail
				assert.Equal(t, http.StatusUnauthorized, w.Code, "Expected 401 for invalid/missing auth")
			}
		})
	}
}

// Helper functions
func floatPtr(f float32) *float32 {
	return &f
}

func intPtr(i int) *int {
	return &i
}