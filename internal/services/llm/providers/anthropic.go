package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type AnthropicProvider struct {
	*BaseProvider
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewAnthropicProvider(name string, cfg ProviderConfig) (*AnthropicProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("anthropic API key is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	models := cfg.Models
	if len(models) == 0 {
		models = []string{
			"claude-3-5-sonnet-20241022",
			"claude-3-5-haiku-20241022",
			"claude-3-opus-20240229",
			"claude-3-sonnet-20240229",
			"claude-3-haiku-20240307",
		}
	}

	return &AnthropicProvider{
		BaseProvider: NewBaseProvider(name, "anthropic", cfg.Priority, models),
		apiKey:       cfg.APIKey,
		baseURL:      baseURL,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (p *AnthropicProvider) ChatCompletion(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	// Transform OpenAI format to Anthropic format
	antRequest, err := p.transformToAnthropicRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Prepare the request body
	reqBody, err := json.Marshal(antRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("Sending to Anthropic: %s", string(reqBody))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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
		var errResp map[string]interface{}
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("anthropic API error: %v", errResp)
	}

	// Parse Anthropic response
	var antResp AnthropicResponse
	if err := json.Unmarshal(body, &antResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Transform Anthropic format to OpenAI format
	chatResp := p.transformToOpenAIResponse(&antResp)
	return chatResp, nil
}

func (p *AnthropicProvider) ChatCompletionStream(ctx context.Context, request *ChatRequest) (<-chan StreamResponse, error) {
	// Create the stream channel
	streamChan := make(chan StreamResponse, 100)

	go func() {
		defer close(streamChan)

		// Transform OpenAI format to Anthropic format with streaming enabled
		antRequest, err := p.transformToAnthropicRequest(request)
		if err != nil {
			return // Just close channel on error
		}
		antRequest.Stream = true

		// Prepare the request body
		reqBody, err := json.Marshal(antRequest)
		if err != nil {
			return // Just close channel on error
		}

		log.Printf("Streaming to Anthropic: %s", string(reqBody))

		// Create HTTP request
		req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewBuffer(reqBody))
		if err != nil {
			return // Just close channel on error
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", p.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("Accept", "text/event-stream")

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

func (p *AnthropicProvider) Completion(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error) {
	// Not implemented yet
	return nil, fmt.Errorf("completion endpoint not implemented")
}

func (p *AnthropicProvider) CompletionStream(ctx context.Context, request *CompletionRequest) (<-chan StreamResponse, error) {
	// Not implemented yet
	return nil, fmt.Errorf("completion streaming not implemented")
}

func (p *AnthropicProvider) Embeddings(ctx context.Context, request *EmbeddingsRequest) (*EmbeddingsResponse, error) {
	return nil, fmt.Errorf("anthropic does not provide embeddings API - use OpenAI or Vertex AI instead")
}

func (p *AnthropicProvider) AudioTranscription(ctx context.Context, request *TranscriptionRequest) (*TranscriptionResponse, error) {
	return nil, fmt.Errorf("anthropic does not provide audio transcription API - use OpenAI or Vertex AI instead")
}

func (p *AnthropicProvider) AudioSpeech(ctx context.Context, request *SpeechRequest) ([]byte, error) {
	return nil, fmt.Errorf("anthropic does not provide text-to-speech API - use OpenAI or Vertex AI instead")
}

func (p *AnthropicProvider) ImageGeneration(ctx context.Context, request *ImageRequest) (*ImageResponse, error) {
	return nil, fmt.Errorf("anthropic does not provide image generation API - use OpenAI DALL-E instead")
}

func (p *AnthropicProvider) HealthCheck(ctx context.Context) error {
	// Try a simple API call to check health
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/v1/messages", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		p.SetHealthy(false)
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Anthropic returns 405 Method Not Allowed for GET on /v1/messages, which means the endpoint exists
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode < 500 {
		p.SetHealthy(true)
		return nil
	}

	p.SetHealthy(false)
	return fmt.Errorf("health check failed with status %d", resp.StatusCode)
}

// Anthropic API types
type AnthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Messages    []AnthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	Temperature *float32           `json:"temperature,omitempty"`
	TopP        *float32           `json:"top_p,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Stop        []string           `json:"stop_sequences,omitempty"`
}

type AnthropicMessage struct {
	Role    string             `json:"role"`
	Content []AnthropicContent `json:"content"`
}

type AnthropicContent struct {
	Type   string                `json:"type"`
	Text   string                `json:"text,omitempty"`
	Source *AnthropicImageSource `json:"source,omitempty"`
}

type AnthropicImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type AnthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []AnthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence string                  `json:"stop_sequence"`
	Usage        AnthropicUsage          `json:"usage"`
}

type AnthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type AnthropicStreamResponse struct {
	Type         string                 `json:"type"`
	Message      *AnthropicResponse     `json:"message,omitempty"`
	Index        int                    `json:"index,omitempty"`
	ContentBlock *AnthropicContentBlock `json:"content_block,omitempty"`
	Delta        *AnthropicDelta        `json:"delta,omitempty"`
}

type AnthropicDelta struct {
	Type         string `json:"type"`
	Text         string `json:"text"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
}

// transformToAnthropicRequest converts OpenAI format to Anthropic format
func (p *AnthropicProvider) transformToAnthropicRequest(req *ChatRequest) (*AnthropicRequest, error) {
	antReq := &AnthropicRequest{
		Model:     req.Model,
		MaxTokens: 4096, // Default max tokens
		Messages:  make([]AnthropicMessage, 0),
	}

	if req.MaxTokens != nil {
		antReq.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		antReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		antReq.TopP = req.TopP
	}
	if len(req.Stop) > 0 {
		antReq.Stop = req.Stop
	}

	// Convert messages and handle system messages
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Anthropic handles system messages differently
			if contentStr, ok := msg.Content.(string); ok {
				antReq.System = contentStr
			}
			continue
		}

		// Convert message content
		antMsg := AnthropicMessage{
			Role:    msg.Role,
			Content: make([]AnthropicContent, 0),
		}

		// Handle different content types
		switch content := msg.Content.(type) {
		case string:
			// Simple text content
			antMsg.Content = append(antMsg.Content, AnthropicContent{
				Type: "text",
				Text: content,
			})
		case []interface{}:
			// Multimodal content (text + images)
			for _, item := range content {
				if itemMap, ok := item.(map[string]interface{}); ok {
					itemType, _ := itemMap["type"].(string)
					
					switch itemType {
					case "text":
						if text, ok := itemMap["text"].(string); ok {
							antMsg.Content = append(antMsg.Content, AnthropicContent{
								Type: "text",
								Text: text,
							})
						}
					case "image_url":
						if imageURL, ok := itemMap["image_url"].(map[string]interface{}); ok {
							if url, ok := imageURL["url"].(string); ok {
								// Convert image URL to Anthropic format
								if strings.HasPrefix(url, "data:") {
									// Handle base64 data URLs
									parts := strings.SplitN(url, ",", 2)
									if len(parts) == 2 {
										headerParts := strings.Split(parts[0], ";")
										mediaType := strings.TrimPrefix(headerParts[0], "data:")
										
										antMsg.Content = append(antMsg.Content, AnthropicContent{
											Type: "image",
											Source: &AnthropicImageSource{
												Type:      "base64",
												MediaType: mediaType,
												Data:      parts[1],
											},
										})
									}
								}
								// Note: URL-based images would need to be downloaded and converted to base64
							}
						}
					}
				}
			}
		}

		antReq.Messages = append(antReq.Messages, antMsg)
	}

	return antReq, nil
}

// transformToOpenAIResponse converts Anthropic format to OpenAI format
func (p *AnthropicProvider) transformToOpenAIResponse(antResp *AnthropicResponse) *ChatResponse {
	content := ""
	if len(antResp.Content) > 0 {
		content = antResp.Content[0].Text
	}

	return &ChatResponse{
		ID:      antResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   antResp.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: antResp.StopReason,
			},
		},
		Usage: Usage{
			PromptTokens:     antResp.Usage.InputTokens,
			CompletionTokens: antResp.Usage.OutputTokens,
			TotalTokens:      antResp.Usage.InputTokens + antResp.Usage.OutputTokens,
		},
	}
}

// parseStreamResponse parses the SSE stream response from Anthropic
func (p *AnthropicProvider) parseStreamResponse(body io.Reader, streamChan chan<- StreamResponse) {
	bufReader := bufio.NewReader(body)

	for {
		line, err := bufReader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				// Log error but continue
				log.Printf("Error reading Anthropic response: %v", err)
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
		var antStreamResp AnthropicStreamResponse
		if err := json.Unmarshal([]byte(data), &antStreamResp); err != nil {
			// Skip malformed data
			continue
		}

		// Convert to OpenAI format and send
		if streamResp := p.convertStreamResponse(&antStreamResp); streamResp != nil {
			streamChan <- *streamResp
		}
	}
}

// convertStreamResponse converts Anthropic stream response to OpenAI format
func (p *AnthropicProvider) convertStreamResponse(antResp *AnthropicStreamResponse) *StreamResponse {
	if antResp.Type == "content_block_delta" && antResp.Delta != nil {
		return &StreamResponse{
			ID:      GenerateID(),
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "claude",
			Choices: []StreamChoice{
				{
					Index: 0,
					Delta: Message{
						Role:    "assistant",
						Content: antResp.Delta.Text,
					},
					FinishReason: antResp.Delta.StopReason,
				},
			},
		}
	}
	return nil
}