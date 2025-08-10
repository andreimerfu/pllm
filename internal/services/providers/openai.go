package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	
)

type OpenAIProvider struct {
	*BaseProvider
	apiKey     string
	baseURL    string
	orgID      string
	client     *http.Client
}

func NewOpenAIProvider(name string, cfg ProviderConfig) (*OpenAIProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	
	models := cfg.Models
	if len(models) == 0 {
		models = []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo", "gpt-3.5-turbo-16k"}
	}
	
	return &OpenAIProvider{
		BaseProvider: NewBaseProvider(name, "openai", cfg.Priority, models),
		apiKey:      cfg.APIKey,
		baseURL:     baseURL,
		orgID:       cfg.OrgID,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (p *OpenAIProvider) ChatCompletion(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	// Prepare the request body
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	// Only set organization header if it's explicitly provided and not empty
	// Some API keys don't require an org ID and sending one causes an error
	if p.orgID != "" && p.orgID != "0" && p.orgID != "null" {
		req.Header.Set("OpenAI-Organization", p.orgID)
	}

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("OpenAI API error: %s", errResp.Error.Message)
	}

	// Parse successful response
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &chatResp, nil
}

func (p *OpenAIProvider) ChatCompletionStream(ctx context.Context, request *ChatRequest) (<-chan StreamResponse, error) {
	// Not implemented yet
	return nil, fmt.Errorf("streaming not implemented")
}

func (p *OpenAIProvider) Completion(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error) {
	// Not implemented yet
	return nil, fmt.Errorf("completion endpoint not implemented")
}

func (p *OpenAIProvider) CompletionStream(ctx context.Context, request *CompletionRequest) (<-chan StreamResponse, error) {
	// Not implemented yet
	return nil, fmt.Errorf("completion streaming not implemented")
}

func (p *OpenAIProvider) Embeddings(ctx context.Context, request *EmbeddingsRequest) (*EmbeddingsResponse, error) {
	// Not implemented yet
	return nil, fmt.Errorf("embeddings endpoint not implemented")
}

func (p *OpenAIProvider) HealthCheck(ctx context.Context) error {
	// Try a simple API call to check health
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.orgID != "" {
		req.Header.Set("OpenAI-Organization", p.orgID)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.SetHealthy(false)
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.SetHealthy(false)
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	p.SetHealthy(true)
	return nil
}