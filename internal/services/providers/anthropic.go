package providers

import (
	"context"
	"fmt"
)

type AnthropicProvider struct {
	*BaseProvider
	apiKey  string
	baseURL string
}

func NewAnthropicProvider(name string, cfg ProviderConfig) (*AnthropicProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("Anthropic API key is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	models := cfg.Models
	if len(models) == 0 {
		models = []string{
			"claude-3-opus-20240229",
			"claude-3-sonnet-20240229",
			"claude-3-haiku-20240307",
		}
	}

	return &AnthropicProvider{
		BaseProvider: NewBaseProvider(name, "anthropic", cfg.Priority, models),
		apiKey:       cfg.APIKey,
		baseURL:      baseURL,
	}, nil
}

func (p *AnthropicProvider) ChatCompletion(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *AnthropicProvider) ChatCompletionStream(ctx context.Context, request *ChatRequest) (<-chan StreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *AnthropicProvider) Completion(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *AnthropicProvider) CompletionStream(ctx context.Context, request *CompletionRequest) (<-chan StreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *AnthropicProvider) Embeddings(ctx context.Context, request *EmbeddingsRequest) (*EmbeddingsResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *AnthropicProvider) HealthCheck(ctx context.Context) error {
	// For now, just return healthy
	p.SetHealthy(true)
	return nil
}
