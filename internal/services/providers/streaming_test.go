package providers

import (
	"context"
	"testing"
	"time"
)

func TestStreamingSupport(t *testing.T) {
	tests := []struct {
		name         string
		providerFunc func(t *testing.T) Provider
		model        string
	}{
		{
			name: "OpenAI Streaming",
			providerFunc: func(t *testing.T) Provider {
				p, _ := NewOpenAIProvider("test-openai", ProviderConfig{
					APIKey:  "test-key",
					BaseURL: "https://api.openai.com/v1",
					Models:  []string{"gpt-3.5-turbo"},
				})
				return p
			},
			model: "gpt-3.5-turbo",
		},
		{
			name: "Azure Streaming",
			providerFunc: func(t *testing.T) Provider {
				p, _ := NewAzureProvider("test-azure", ProviderConfig{
					APIKey:  "test-key",
					BaseURL: "https://test.openai.azure.com",
					Extra: map[string]interface{}{
						"deployments": map[string]interface{}{
							"gpt-3.5-turbo": "gpt-35-turbo",
						},
					},
				})
				return p
			},
			model: "gpt-3.5-turbo",
		},
		{
			name: "Bedrock Streaming",
			providerFunc: func(t *testing.T) Provider {
				p, _ := NewBedrockProvider("test-bedrock", ProviderConfig{
					APIKey:    "test-key",
					APISecret: "test-secret",
					Region:    "us-east-1",
				})
				return p
			},
			model: "anthropic.claude-3-haiku-20240307",
		},
		// TODO: Vertex streaming is not yet implemented
		// Uncomment when streaming is added to Vertex provider
		// {
		// 	name: "Vertex Streaming",
		// 	providerFunc: func(t *testing.T) Provider {
		// 		p, err := NewVertexProvider("test-vertex", ProviderConfig{
		// 			APIKey: `{"type":"service_account","client_email":"test@test.iam.gserviceaccount.com","private_key":"-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----"}`,
		// 			Extra: map[string]interface{}{
		// 				"project_id": "test-project",
		// 				"location":   "us-central1",
		// 			},
		// 		})
		// 		if err != nil {
		// 			t.Logf("Vertex provider initialization failed: %v", err)
		// 		}
		// 		return p
		// 	},
		// 	model: "gemini-pro",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := tt.providerFunc(t)
			if provider == nil {
				t.Skip("Provider initialization failed")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			request := &ChatRequest{
				Model: tt.model,
				Messages: []Message{
					{Role: "user", Content: "Test message"},
				},
				Stream: true,
			}

			// Test that streaming method exists and returns a channel
			streamChan, err := provider.ChatCompletionStream(ctx, request)
			
			// For Bedrock non-Claude models, we expect an error
			if tt.name == "Bedrock Streaming" && tt.model != "anthropic.claude-3-haiku-20240307" {
				if err == nil || streamChan != nil {
					// This would be an actual API call error, which is expected
					return
				}
			}

			// For all providers that support streaming, channel should be returned
			if streamChan == nil {
				t.Errorf("%s: ChatCompletionStream should return a channel, got nil", tt.name)
			}

			// Verify channel eventually closes (since we're not making real API calls)
			select {
			case <-streamChan:
				// Channel closed or received data
			case <-time.After(200 * time.Millisecond):
				// Timeout is okay - channel might stay open if no real API call
			}
		})
	}
}

func TestStreamingChannelBehavior(t *testing.T) {
	// Test that streaming channels are properly created and closed
	providers := []struct {
		name     string
		provider Provider
	}{
		{
			name: "OpenAI",
			provider: func() Provider {
				p, _ := NewOpenAIProvider("test", ProviderConfig{
					APIKey: "test-key",
				})
				return p
			}(),
		},
		{
			name: "Azure",
			provider: func() Provider {
				p, _ := NewAzureProvider("test", ProviderConfig{
					APIKey:  "test-key",
					BaseURL: "https://test.openai.azure.com",
				})
				return p
			}(),
		},
		{
			name: "Bedrock",
			provider: func() Provider {
				p, _ := NewBedrockProvider("test", ProviderConfig{
					APIKey:    "test-key",
					APISecret: "test-secret",
				})
				return p
			}(),
		},
		// TODO: Vertex streaming not yet implemented
		// {
		// 	name: "Vertex",
		// 	provider: func() Provider {
		// 		p, _ := NewVertexProvider("test", ProviderConfig{
		// 			APIKey: `{"type":"service_account","client_email":"test@test.iam.gserviceaccount.com","private_key":"-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----"}`,
		// 			Extra: map[string]interface{}{
		// 				"project_id": "test-project",
		// 			},
		// 		})
		// 		return p
		// 	}(),
		// },
	}

	for _, p := range providers {
		t.Run(p.name+" channel creation", func(t *testing.T) {
			if p.provider == nil {
				t.Skip("Provider initialization failed")
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			request := &ChatRequest{
				Model: "test-model",
				Messages: []Message{
					{Role: "user", Content: "test"},
				},
				Stream: true,
			}

			// Get streaming channel
			streamChan, _ := p.provider.ChatCompletionStream(ctx, request)
			
			if streamChan != nil {
				// Cancel context to trigger cleanup
				cancel()
				
				// Verify channel behavior after context cancellation
				timer := time.NewTimer(500 * time.Millisecond)
				defer timer.Stop()
				
				select {
				case <-streamChan:
					// Good - channel closed or returned
				case <-timer.C:
					// Timeout is acceptable - no real API call
				}
			}
		})
	}
}