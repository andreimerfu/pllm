package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type OpenAIProvider struct {
	*BaseProvider
	apiKey  string
	baseURL string
	orgID   string
	client  *http.Client
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
		apiKey:       cfg.APIKey,
		baseURL:      baseURL,
		orgID:        cfg.OrgID,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (p *OpenAIProvider) ChatCompletion(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	// Debug logging for vision content
	for i, msg := range request.Messages {
		if msg.Content != nil {
			log.Printf("Message %d (%s): content type = %T", i, msg.Role, msg.Content)
			if contentArray, ok := msg.Content.([]interface{}); ok {
				log.Printf("Message %d has %d content parts", i, len(contentArray))
				for j, part := range contentArray {
					log.Printf("  Part %d: %+v", j, part)
				}
			}
		}
	}

	// Prepare the request body
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	log.Printf("Sending to OpenAI: %s", string(reqBody))

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
	defer func() { _ = resp.Body.Close() }()

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
	// Create the stream channel
	streamChan := make(chan StreamResponse, 100)

	go func() {
		defer close(streamChan)

		// Enable streaming in request
		request.Stream = true

		// Debug logging for vision content in streaming
		log.Printf("Streaming request for model: %s", request.Model)
		for i, msg := range request.Messages {
			if msg.Content != nil {
				log.Printf("Stream Message %d (%s): content type = %T", i, msg.Role, msg.Content)
			}
		}

		// Prepare the request body
		reqBody, err := json.Marshal(request)
		if err != nil {
			return // Just close channel on error
		}
		
		log.Printf("Streaming to OpenAI: %s", string(reqBody))

		// Create HTTP request
		req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(reqBody))
		if err != nil {
			return // Just close channel on error
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Accept", "text/event-stream")

		// Only set organization header if it's explicitly provided and not empty
		if p.orgID != "" && p.orgID != "0" && p.orgID != "null" {
			req.Header.Set("OpenAI-Organization", p.orgID)
		}

		// Make the request
		resp, err := p.client.Do(req)
		if err != nil {
			return // Just close channel on error
		}
		defer func() { _ = resp.Body.Close() }()

		// Check for errors
		if resp.StatusCode != http.StatusOK {
			return // Just close channel on error
		}

		// Parse SSE stream
		p.parseStreamResponse(resp.Body, streamChan)
	}()

	return streamChan, nil
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
	// Prepare the request body
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/embeddings", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.orgID != "" && p.orgID != "0" && p.orgID != "null" {
		req.Header.Set("OpenAI-Organization", p.orgID)
	}

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
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("OpenAI API error: %s", errResp.Error.Message)
	}

	// Parse successful response
	var embResp EmbeddingsResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &embResp, nil
}

func (p *OpenAIProvider) AudioTranscription(ctx context.Context, request *TranscriptionRequest) (*TranscriptionResponse, error) {
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file field
	fileWriter, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(fileWriter, request.File); err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	// Add other fields
	if err := writer.WriteField("model", request.Model); err != nil {
		return nil, fmt.Errorf("failed to write model field: %w", err)
	}
	if request.Language != "" {
		if err := writer.WriteField("language", request.Language); err != nil {
			return nil, fmt.Errorf("failed to write language field: %w", err)
		}
	}
	if request.Prompt != "" {
		if err := writer.WriteField("prompt", request.Prompt); err != nil {
			return nil, fmt.Errorf("failed to write prompt field: %w", err)
		}
	}
	if request.ResponseFormat != "" {
		if err := writer.WriteField("response_format", request.ResponseFormat); err != nil {
			return nil, fmt.Errorf("failed to write response_format field: %w", err)
		}
	}
	if request.Temperature != nil {
		if err := writer.WriteField("temperature", fmt.Sprintf("%.2f", *request.Temperature)); err != nil {
			return nil, fmt.Errorf("failed to write temperature field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.orgID != "" && p.orgID != "0" && p.orgID != "null" {
		req.Header.Set("OpenAI-Organization", p.orgID)
	}

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
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("OpenAI API error: %s", errResp.Error.Message)
	}

	// Parse successful response
	var transcResp TranscriptionResponse
	if err := json.Unmarshal(body, &transcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &transcResp, nil
}

func (p *OpenAIProvider) AudioSpeech(ctx context.Context, request *SpeechRequest) ([]byte, error) {
	// Prepare the request body
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/audio/speech", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.orgID != "" && p.orgID != "0" && p.orgID != "null" {
		req.Header.Set("OpenAI-Organization", p.orgID)
	}

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for errors first
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("OpenAI API error: %s", errResp.Error.Message)
	}

	// Read audio data
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	return audioData, nil
}

func (p *OpenAIProvider) ImageGeneration(ctx context.Context, request *ImageRequest) (*ImageResponse, error) {
	// Prepare the request body
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/images/generations", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.orgID != "" && p.orgID != "0" && p.orgID != "null" {
		req.Header.Set("OpenAI-Organization", p.orgID)
	}

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
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("OpenAI API error: %s", errResp.Error.Message)
	}

	// Parse successful response
	var imgResp ImageResponse
	if err := json.Unmarshal(body, &imgResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &imgResp, nil
}

// parseStreamResponse parses the SSE stream response from OpenAI
func (p *OpenAIProvider) parseStreamResponse(body io.Reader, streamChan chan<- StreamResponse) {
	bufReader := bufio.NewReader(body)

	for {
		line, err := bufReader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				// Log error but continue
				log.Printf("Error reading OpenAI response: %v", err)
			}
			return
		}

		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check for SSE data prefix
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
			// Skip malformed data
			continue
		}

		// Send to channel
		streamChan <- streamResp
	}
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		p.SetHealthy(false)
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	p.SetHealthy(true)
	return nil
}
