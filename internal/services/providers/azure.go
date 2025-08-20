package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AzureProvider implements Azure OpenAI provider
type AzureProvider struct {
	mu          sync.RWMutex
	name        string
	config      ProviderConfig
	client      *http.Client
	healthy     bool
	deployments map[string]string // model -> deployment name mapping
	apiVersion  string
}

// AzureConfig contains Azure-specific configuration
type AzureConfig struct {
	Endpoint    string            `mapstructure:"endpoint"`
	APIVersion  string            `mapstructure:"api_version"`
	Deployments map[string]string `mapstructure:"deployments"` // model -> deployment mapping
}

// NewAzureProvider creates a new Azure OpenAI provider
func NewAzureProvider(name string, config ProviderConfig) (*AzureProvider, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("Azure endpoint URL is required")
	}

	// Ensure endpoint doesn't have trailing slash
	config.BaseURL = strings.TrimSuffix(config.BaseURL, "/")

	apiVersion := config.APIVersion
	if apiVersion == "" {
		apiVersion = "2024-02-01" // Default API version
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Parse deployments from config
	deployments := make(map[string]string)
	if config.Extra != nil {
		if deps, ok := config.Extra["deployments"].(map[string]interface{}); ok {
			for model, deployment := range deps {
				if depStr, ok := deployment.(string); ok {
					deployments[model] = depStr
				}
			}
		}
	}

	p := &AzureProvider{
		name:        name,
		config:      config,
		client:      client,
		healthy:     true,
		deployments: deployments,
		apiVersion:  apiVersion,
	}

	return p, nil
}

// ChatCompletion implements the Provider interface
func (p *AzureProvider) ChatCompletion(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	// Get deployment name for the model
	deployment := p.getDeploymentName(request.Model)
	if deployment == "" {
		return nil, fmt.Errorf("no deployment configured for model: %s", request.Model)
	}

	// Build URL
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		p.config.BaseURL, deployment, p.apiVersion)

	// Transform request to Azure format (mostly same as OpenAI)
	azureRequest := p.transformRequest(request)

	body, err := json.Marshal(azureRequest)
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Set headers
	p.setHeaders(req, ctx)

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Azure OpenAI API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var azureResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&azureResp); err != nil {
		return nil, err
	}

	// Azure response is already in OpenAI format
	return &azureResp, nil
}

// ChatCompletionStream implements streaming for Provider interface
func (p *AzureProvider) ChatCompletionStream(ctx context.Context, request *ChatRequest) (<-chan StreamResponse, error) {
	// Get deployment name first - return error immediately if not found
	deployment := p.getDeploymentName(request.Model)
	if deployment == "" {
		return nil, fmt.Errorf("no deployment configured for model: %s", request.Model)
	}

	streamChan := make(chan StreamResponse, 100)

	go func() {
		defer close(streamChan)

		// Build URL
		url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
			p.config.BaseURL, deployment, p.apiVersion)

		// Enable streaming
		request.Stream = true
		azureRequest := p.transformRequest(request)

		body, err := json.Marshal(azureRequest)
		if err != nil {
			return // Just close the channel on error
		}

		// Create HTTP request
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return // Just close the channel on error
		}

		// Set headers
		p.setHeaders(req, ctx)
		req.Header.Set("Accept", "text/event-stream")

		// Send request
		resp, err := p.client.Do(req)
		if err != nil {
			return // Just close the channel on error
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// Log error but just close channel
			return
		}

		// Parse SSE stream
		p.parseStreamResponse(resp.Body, streamChan)
	}()

	return streamChan, nil
}

// transformRequest transforms the request to Azure format
func (p *AzureProvider) transformRequest(request *ChatRequest) map[string]interface{} {
	// Azure uses the same format as OpenAI
	azureReq := map[string]interface{}{
		"messages": request.Messages,
		"stream":   request.Stream,
	}

	// Add optional parameters
	if request.Temperature != nil {
		azureReq["temperature"] = *request.Temperature
	}

	if request.TopP != nil {
		azureReq["top_p"] = *request.TopP
	}

	if request.MaxTokens != nil {
		azureReq["max_tokens"] = *request.MaxTokens
	}

	if request.N != nil {
		azureReq["n"] = *request.N
	}

	if len(request.Stop) > 0 {
		azureReq["stop"] = request.Stop
	}

	if request.PresencePenalty != nil {
		azureReq["presence_penalty"] = *request.PresencePenalty
	}

	if request.FrequencyPenalty != nil {
		azureReq["frequency_penalty"] = *request.FrequencyPenalty
	}

	if request.ResponseFormat != nil {
		azureReq["response_format"] = request.ResponseFormat
	}

	if len(request.Tools) > 0 {
		azureReq["tools"] = request.Tools
	}

	if request.ToolChoice != nil {
		azureReq["tool_choice"] = request.ToolChoice
	}

	if request.Seed != nil {
		azureReq["seed"] = *request.Seed
	}

	if request.User != "" {
		azureReq["user"] = request.User
	}

	return azureReq
}

// setHeaders sets the appropriate headers for Azure
func (p *AzureProvider) setHeaders(req *http.Request, ctx context.Context) {
	req.Header.Set("Content-Type", "application/json")

	// Check for bearer token in context (for Azure AD auth)
	if token := ctx.Value("AzureAuthorizationToken"); token != nil {
		if tokenStr, ok := token.(string); ok && tokenStr != "" {
			req.Header.Set("Authorization", "Bearer "+tokenStr)
			return
		}
	}

	// Fall back to API key auth
	if p.config.APIKey != "" {
		req.Header.Set("api-key", p.config.APIKey)
	}
}

// getDeploymentName gets the deployment name for a model
func (p *AzureProvider) getDeploymentName(model string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check if there's a specific deployment mapping
	if deployment, ok := p.deployments[model]; ok {
		return deployment
	}

	// Some common default mappings
	defaultMappings := map[string]string{
		"gpt-4":                  "gpt-4",
		"gpt-4-turbo":            "gpt-4-turbo",
		"gpt-3.5-turbo":          "gpt-35-turbo", // Azure uses different naming
		"text-embedding-ada-002": "text-embedding-ada-002",
		"text-embedding-3-small": "text-embedding-3-small",
		"text-embedding-3-large": "text-embedding-3-large",
	}

	if deployment, ok := defaultMappings[model]; ok {
		return deployment
	}

	// If no mapping found, try using the model name directly
	return model
}

// parseStreamResponse parses the SSE stream response
func (p *AzureProvider) parseStreamResponse(body io.Reader, streamChan chan<- StreamResponse) {
	// Read the response body
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return // Just close on error
	}

	// Split by double newline (SSE format)
	events := strings.Split(string(bodyBytes), "\n\n")

	for _, event := range events {
		if event == "" {
			continue
		}

		// Parse SSE event
		lines := strings.Split(event, "\n")
		var data string

		for _, line := range lines {
			if strings.HasPrefix(line, "data: ") {
				data = strings.TrimPrefix(line, "data: ")
				break
			}
		}

		if data == "" || data == "[DONE]" {
			continue
		}

		// Parse JSON data
		var streamResp StreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue // Skip malformed data
		}

		streamChan <- streamResp
	}
}

// Completion implements the Provider interface (legacy)
func (p *AzureProvider) Completion(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error) {
	// Get deployment name
	deployment := p.getDeploymentName(request.Model)
	if deployment == "" {
		return nil, fmt.Errorf("no deployment configured for model: %s", request.Model)
	}

	// Build URL
	url := fmt.Sprintf("%s/openai/deployments/%s/completions?api-version=%s",
		p.config.BaseURL, deployment, p.apiVersion)

	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Set headers
	p.setHeaders(req, ctx)

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Azure OpenAI API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var azureResp CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&azureResp); err != nil {
		return nil, err
	}

	return &azureResp, nil
}

// CompletionStream implements the Provider interface (legacy)
func (p *AzureProvider) CompletionStream(ctx context.Context, request *CompletionRequest) (<-chan StreamResponse, error) {
	return nil, fmt.Errorf("completion stream not yet implemented for Azure provider")
}

// Embeddings implements the Provider interface
func (p *AzureProvider) Embeddings(ctx context.Context, request *EmbeddingsRequest) (*EmbeddingsResponse, error) {
	// Get deployment name
	deployment := p.getDeploymentName(request.Model)
	if deployment == "" {
		return nil, fmt.Errorf("no deployment configured for model: %s", request.Model)
	}

	// Build URL
	url := fmt.Sprintf("%s/openai/deployments/%s/embeddings?api-version=%s",
		p.config.BaseURL, deployment, p.apiVersion)

	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Set headers
	p.setHeaders(req, ctx)

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Azure OpenAI API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var azureResp EmbeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&azureResp); err != nil {
		return nil, err
	}

	return &azureResp, nil
}

// Provider info methods
func (p *AzureProvider) GetType() string {
	return "azure"
}

func (p *AzureProvider) GetName() string {
	return p.name
}

func (p *AzureProvider) GetPriority() int {
	return 60
}

func (p *AzureProvider) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.healthy
}

func (p *AzureProvider) SupportsModel(model string) bool {
	// Check if we have a deployment for this model
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, hasDeployment := p.deployments[model]
	if hasDeployment {
		return true
	}

	// Check common models
	supportedModels := []string{
		"gpt-4",
		"gpt-4-turbo",
		"gpt-3.5-turbo",
		"text-embedding-ada-002",
		"text-embedding-3-small",
		"text-embedding-3-large",
		"dall-e-3",
		"dall-e-2",
		"whisper-1",
	}

	for _, m := range supportedModels {
		if m == model {
			return true
		}
	}

	return false
}

func (p *AzureProvider) ListModels() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	models := make([]string, 0, len(p.deployments))
	for model := range p.deployments {
		models = append(models, model)
	}

	// Add default supported models if not in deployments
	defaultModels := []string{
		"gpt-4",
		"gpt-4-turbo",
		"gpt-3.5-turbo",
	}

	for _, model := range defaultModels {
		found := false
		for _, m := range models {
			if m == model {
				found = true
				break
			}
		}
		if !found {
			models = append(models, model)
		}
	}

	return models
}

func (p *AzureProvider) HealthCheck(ctx context.Context) error {
	// Try to list deployments or make a simple API call
	testModel := "gpt-3.5-turbo"
	if len(p.deployments) > 0 {
		for model := range p.deployments {
			testModel = model
			break
		}
	}

	deployment := p.getDeploymentName(testModel)
	url := fmt.Sprintf("%s/openai/deployments/%s?api-version=%s",
		p.config.BaseURL, deployment, p.apiVersion)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	p.setHeaders(req, ctx)

	resp, err := p.client.Do(req)
	if err != nil {
		p.mu.Lock()
		p.healthy = false
		p.mu.Unlock()
		return err
	}
	defer resp.Body.Close()

	p.mu.Lock()
	p.healthy = resp.StatusCode < 500
	p.mu.Unlock()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
