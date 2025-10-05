package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOpenRouterProvider(t *testing.T) {
	tests := []struct {
		name        string
		config      ProviderConfig
		wantErr     bool
		expectedURL string
	}{
		{
			name: "valid config",
			config: ProviderConfig{
				APIKey:   "sk-or-v1-test-key",
				Priority: 10,
			},
			wantErr:     false,
			expectedURL: "https://openrouter.ai/api/v1",
		},
		{
			name: "custom base URL",
			config: ProviderConfig{
				APIKey:  "sk-or-v1-test-key",
				BaseURL: "https://custom.openrouter.ai/api/v1",
			},
			wantErr:     false,
			expectedURL: "https://custom.openrouter.ai/api/v1",
		},
		{
			name: "missing API key",
			config: ProviderConfig{
				Priority: 10,
			},
			wantErr: true,
		},
		{
			name: "with extra config",
			config: ProviderConfig{
				APIKey: "sk-or-v1-test-key",
				Extra: map[string]interface{}{
					"http_referer": "https://myapp.com",
					"x_title":      "My App",
					"app_name":     "MyApp/1.0",
				},
			},
			wantErr:     false,
			expectedURL: "https://openrouter.ai/api/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenRouterProvider("test", tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewOpenRouterProvider() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("NewOpenRouterProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if provider.baseURL != tt.expectedURL {
				t.Errorf("NewOpenRouterProvider() baseURL = %v, want %v", provider.baseURL, tt.expectedURL)
			}

			if provider.GetType() != "openrouter" {
				t.Errorf("GetType() = %v, want openrouter", provider.GetType())
			}

			if provider.GetName() != "test" {
				t.Errorf("GetName() = %v, want test", provider.GetName())
			}
		})
	}
}

func TestOpenRouterProvider_ChatCompletion(t *testing.T) {
	// Mock server that returns a valid OpenAI-compatible response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check headers
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header = %v, want Bearer test-key", r.Header.Get("Authorization"))
		}
		if r.Header.Get("HTTP-Referer") == "" {
			t.Errorf("HTTP-Referer header should be set")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "openai/gpt-4",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello! How can I help you today?"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 15,
				"total_tokens": 25
			}
		}`))
	}))
	defer server.Close()

	provider, err := NewOpenRouterProvider("test", ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenRouterProvider() error = %v", err)
	}

	request := &ChatRequest{
		Model: "openai/gpt-4",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := provider.ChatCompletion(ctx, request)
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	if response.ID != "chatcmpl-test" {
		t.Errorf("ChatCompletion() ID = %v, want chatcmpl-test", response.ID)
	}

	if len(response.Choices) != 1 {
		t.Errorf("ChatCompletion() choices count = %v, want 1", len(response.Choices))
	}

	if response.Choices[0].Message.Content != "Hello! How can I help you today?" {
		t.Errorf("ChatCompletion() message content = %v, want 'Hello! How can I help you today?'", response.Choices[0].Message.Content)
	}
}

func TestOpenRouterProvider_HealthCheck(t *testing.T) {
	// Mock server for health check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("Health check path = %v, want /models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer server.Close()

	provider, err := NewOpenRouterProvider("test", ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenRouterProvider() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = provider.HealthCheck(ctx)
	if err != nil {
		t.Errorf("HealthCheck() error = %v, want nil", err)
	}

	if !provider.IsHealthy() {
		t.Errorf("IsHealthy() = false, want true after successful health check")
	}
}

func TestOpenRouterProvider_FetchModels(t *testing.T) {
	// Mock server that returns models
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "openai/gpt-4-turbo",
					"name": "GPT-4 Turbo",
					"description": "GPT-4 Turbo model",
					"pricing": {},
					"context_length": 128000
				},
				{
					"id": "anthropic/claude-3-opus",
					"name": "Claude 3 Opus",
					"description": "Anthropic Claude 3 Opus",
					"pricing": {},
					"context_length": 200000
				}
			]
		}`))
	}))
	defer server.Close()

	provider, err := NewOpenRouterProvider("test", ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenRouterProvider() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	models, err := provider.FetchModels(ctx)
	if err != nil {
		t.Fatalf("FetchModels() error = %v", err)
	}

	expectedModels := []string{"openai/gpt-4-turbo", "anthropic/claude-3-opus"}
	if len(models) != len(expectedModels) {
		t.Errorf("FetchModels() count = %v, want %v", len(models), len(expectedModels))
	}

	for i, model := range models {
		if model != expectedModels[i] {
			t.Errorf("FetchModels() model[%d] = %v, want %v", i, model, expectedModels[i])
		}
	}
}
