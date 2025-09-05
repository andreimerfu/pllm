package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenRouterProvider implements OpenRouter API provider
type OpenRouterProvider struct {
	*BaseProvider
	apiKey      string
	baseURL     string
	client      *http.Client
	httpReferer string // Required by OpenRouter
	xTitle      string // Optional title for OpenRouter dashboard
	appName     string // App name for tracking
}

// OpenRouterError represents OpenRouter-specific error response
type OpenRouterError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// OpenRouterModelsResponse represents the response from /models endpoint
type OpenRouterModelsResponse struct {
	Data []OpenRouterModel `json:"data"`
}

type OpenRouterModel struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	Pricing          OpenRouterPricing      `json:"pricing"`
	ContextLength    int                    `json:"context_length"`
	Architecture     OpenRouterArchitecture `json:"architecture"`
	TopProvider      OpenRouterTopProvider  `json:"top_provider"`
	PerRequestLimits map[string]int         `json:"per_request_limits"`
}

type OpenRouterPricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Image      string `json:"image"`
	Request    string `json:"request"`
}

type OpenRouterArchitecture struct {
	Modality     string `json:"modality"`
	Tokenizer    string `json:"tokenizer"`
	InstructType string `json:"instruct_type"`
}

type OpenRouterTopProvider struct {
	MaxCompletionTokens int  `json:"max_completion_tokens"`
	IsFree              bool `json:"is_free"`
}

// NewOpenRouterProvider creates a new OpenRouter provider
func NewOpenRouterProvider(name string, cfg ProviderConfig) (*OpenRouterProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OpenRouter API key is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}

	// Default models - we'll fetch actual models dynamically
	models := cfg.Models
	if len(models) == 0 {
		models = []string{
			"openai/gpt-4-turbo",
			"openai/gpt-4",
			"openai/gpt-3.5-turbo",
			"anthropic/claude-3-opus",
			"anthropic/claude-3-sonnet",
			"anthropic/claude-3-haiku",
			"meta-llama/llama-2-70b-chat",
			"mistralai/mixtral-8x7b-instruct",
			"google/gemini-pro",
		}
	}

	// Extract OpenRouter-specific config from Extra
	httpReferer := ""
	xTitle := ""
	appName := "pllm-gateway"

	if cfg.Extra != nil {
		if referer, ok := cfg.Extra["http_referer"].(string); ok {
			httpReferer = referer
		}
		if title, ok := cfg.Extra["x_title"].(string); ok {
			xTitle = title
		}
		if app, ok := cfg.Extra["app_name"].(string); ok {
			appName = app
		}
	}

	// Default HTTP-Referer if not provided
	if httpReferer == "" {
		httpReferer = "http://localhost:8080" // Default referer
	}

	client := &http.Client{
		Timeout: 120 * time.Second, // OpenRouter can be slower due to routing
	}

	return &OpenRouterProvider{
		BaseProvider: NewBaseProvider(name, "openrouter", cfg.Priority, models),
		apiKey:       cfg.APIKey,
		baseURL:      baseURL,
		client:       client,
		httpReferer:  httpReferer,
		xTitle:       xTitle,
		appName:      appName,
	}, nil
}

// ChatCompletion implements the Provider interface
func (p *OpenRouterProvider) ChatCompletion(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	// Prepare the request body - OpenRouter uses same format as OpenAI
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set OpenRouter-specific headers
	p.setHeaders(req)

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		// Try to parse OpenRouter error format first
		var orErr OpenRouterError
		if err := json.Unmarshal(body, &orErr); err == nil && orErr.Error.Message != "" {
			return nil, fmt.Errorf("OpenRouter API error (%s): %s", orErr.Error.Code, orErr.Error.Message)
		}

		// Fallback to standard error format
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("OpenRouter API error: %s", errResp.Error.Message)
	}

	// Parse successful response
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &chatResp, nil
}

// ChatCompletionStream implements streaming for the Provider interface
func (p *OpenRouterProvider) ChatCompletionStream(ctx context.Context, request *ChatRequest) (<-chan StreamResponse, error) {
	// Create the stream channel
	streamChan := make(chan StreamResponse, 100)

	go func() {
		defer close(streamChan)

		// Enable streaming in request
		request.Stream = true

		// Prepare the request body
		reqBody, err := json.Marshal(request)
		if err != nil {
			// Send error to channel before closing
			streamChan <- StreamResponse{
				ID:      "error",
				Object:  "error",
				Choices: []StreamChoice{{Index: 0, Delta: Message{Role: "assistant", Content: fmt.Sprintf("Error marshaling request: %v", err)}}},
			}
			return
		}

		// Create HTTP request
		req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(reqBody))
		if err != nil {
			streamChan <- StreamResponse{
				ID:      "error",
				Object:  "error",
				Choices: []StreamChoice{{Index: 0, Delta: Message{Role: "assistant", Content: fmt.Sprintf("Error creating request: %v", err)}}},
			}
			return
		}

		// Set headers
		p.setHeaders(req)
		req.Header.Set("Accept", "text/event-stream")

		// Make the request
		resp, err := p.client.Do(req)
		if err != nil {
			streamChan <- StreamResponse{
				ID:      "error",
				Object:  "error",
				Choices: []StreamChoice{{Index: 0, Delta: Message{Role: "assistant", Content: fmt.Sprintf("Error making request: %v", err)}}},
			}
			return
		}
		defer func() { _ = resp.Body.Close() }()

		// Check for errors
		if resp.StatusCode != http.StatusOK {
			// Read error body
			body, _ := io.ReadAll(resp.Body)
			streamChan <- StreamResponse{
				ID:      "error",
				Object:  "error",
				Choices: []StreamChoice{{Index: 0, Delta: Message{Role: "assistant", Content: fmt.Sprintf("OpenRouter API error %d: %s", resp.StatusCode, string(body))}}},
			}
			return
		}

		// Parse SSE stream
		p.parseStreamResponse(resp.Body, streamChan)
	}()

	return streamChan, nil
}

// setHeaders sets the appropriate headers for OpenRouter
func (p *OpenRouterProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Required by OpenRouter
	req.Header.Set("HTTP-Referer", p.httpReferer)

	// Optional headers for better tracking
	if p.xTitle != "" {
		req.Header.Set("X-Title", p.xTitle)
	}

	// Add User-Agent for identification
	req.Header.Set("User-Agent", p.appName+"/1.0")
}

// parseStreamResponse parses the SSE stream response from OpenRouter
func (p *OpenRouterProvider) parseStreamResponse(body io.Reader, streamChan chan<- StreamResponse) {
	bufReader := bufio.NewReader(body)
	chunkCount := 0

	for {
		line, err := bufReader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Normal end of stream
				return
			}
			// Log error but don't send to client
			fmt.Printf("OpenRouter stream read error after %d chunks: %v\n", chunkCount, err)
			return
		}

		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip SSE comment lines (starting with :)
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Only process data lines
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		// Extract data
		data := strings.TrimPrefix(line, "data: ")

		// Check for end of stream
		if data == "[DONE]" {
			return
		}

		// Parse JSON data
		var streamResp StreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			fmt.Printf("OpenRouter parse error for data '%s': %v\n", data, err)
			continue
		}

		chunkCount++
		// Send valid response to channel
		streamChan <- streamResp
	}
}

// Completion implements the Provider interface (legacy)
func (p *OpenRouterProvider) Completion(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error) {
	// Prepare the request body
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	p.setHeaders(req)

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var orErr OpenRouterError
		if err := json.Unmarshal(body, &orErr); err == nil && orErr.Error.Message != "" {
			return nil, fmt.Errorf("OpenRouter API error (%s): %s", orErr.Error.Code, orErr.Error.Message)
		}
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var compResp CompletionResponse
	if err := json.Unmarshal(body, &compResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &compResp, nil
}

// CompletionStream implements streaming completions
func (p *OpenRouterProvider) CompletionStream(ctx context.Context, request *CompletionRequest) (<-chan StreamResponse, error) {
	streamChan := make(chan StreamResponse, 100)

	go func() {
		defer close(streamChan)

		// Enable streaming
		request.Stream = true

		reqBody, err := json.Marshal(request)
		if err != nil {
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/completions", bytes.NewBuffer(reqBody))
		if err != nil {
			return
		}

		p.setHeaders(req)
		req.Header.Set("Accept", "text/event-stream")

		resp, err := p.client.Do(req)
		if err != nil {
			return
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return
		}

		p.parseStreamResponse(resp.Body, streamChan)
	}()

	return streamChan, nil
}

// Embeddings implements the Provider interface
func (p *OpenRouterProvider) Embeddings(ctx context.Context, request *EmbeddingsRequest) (*EmbeddingsResponse, error) {
	// OpenRouter may not support embeddings for all models
	return nil, fmt.Errorf("embeddings endpoint not supported by OpenRouter provider")
}

// HealthCheck implements health checking
func (p *OpenRouterProvider) HealthCheck(ctx context.Context) error {
	// Use the models endpoint to check health
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("HTTP-Referer", p.httpReferer)

	resp, err := p.client.Do(req)
	if err != nil {
		p.SetHealthy(false)
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		p.SetHealthy(false)
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	p.SetHealthy(true)
	return nil
}

// FetchModels fetches available models from OpenRouter
func (p *OpenRouterProvider) FetchModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("HTTP-Referer", p.httpReferer)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch models, status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read models response: %w", err)
	}

	var modelsResp OpenRouterModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	// Extract model IDs
	models := make([]string, len(modelsResp.Data))
	for i, model := range modelsResp.Data {
		models[i] = model.ID
	}

	return models, nil
}

// UpdateModels updates the provider's model list by fetching from OpenRouter
func (p *OpenRouterProvider) UpdateModels(ctx context.Context) error {
	models, err := p.FetchModels(ctx)
	if err != nil {
		return err
	}

	// Update the model list
	p.models = models
	return nil
}

func (p *OpenRouterProvider) AudioTranscription(ctx context.Context, request *TranscriptionRequest) (*TranscriptionResponse, error) {
	return nil, fmt.Errorf("audio transcription not currently supported through OpenRouter - use OpenAI directly")
}

func (p *OpenRouterProvider) AudioSpeech(ctx context.Context, request *SpeechRequest) ([]byte, error) {
	return nil, fmt.Errorf("text-to-speech not currently supported through OpenRouter - use OpenAI directly")
}

func (p *OpenRouterProvider) ImageGeneration(ctx context.Context, request *ImageRequest) (*ImageResponse, error) {
	return nil, fmt.Errorf("image generation not currently supported through OpenRouter - use OpenAI directly")
}
