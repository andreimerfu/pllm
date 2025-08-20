package providers

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// BedrockProvider implements AWS Bedrock LLM provider
type BedrockProvider struct {
	mu      sync.RWMutex
	name    string
	config  ProviderConfig
	client  *http.Client
	healthy bool
	models  []string
}

// BedrockAuth contains AWS authentication details
type BedrockAuth struct {
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	SessionToken    string `mapstructure:"session_token,omitempty"`
	Region          string `mapstructure:"region"`
}

// NewBedrockProvider creates a new AWS Bedrock provider
func NewBedrockProvider(name string, config ProviderConfig) (*BedrockProvider, error) {
	if config.APIKey == "" || config.APISecret == "" {
		return nil, fmt.Errorf("AWS access key and secret key are required")
	}

	region := config.Region
	if region == "" {
		region = "us-east-1"
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", region)
	}

	config.BaseURL = baseURL
	config.Region = region

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	p := &BedrockProvider{
		name:    name,
		config:  config,
		client:  client,
		healthy: true,
		models: []string{
			// Anthropic Claude models
			"anthropic.claude-3-opus-20240229",
			"anthropic.claude-3-sonnet-20240229",
			"anthropic.claude-3-haiku-20240307",
			"anthropic.claude-instant-v1",
			// Meta Llama models
			"meta.llama3-8b-instruct-v1:0",
			"meta.llama3-70b-instruct-v1:0",
			// Mistral models
			"mistral.mistral-7b-instruct-v0:2",
			"mistral.mixtral-8x7b-instruct-v0:1",
		},
	}

	return p, nil
}

// ChatCompletion implements the Provider interface
func (p *BedrockProvider) ChatCompletion(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	// Transform request based on model type
	var body []byte
	var err error
	var endpoint string

	if strings.HasPrefix(request.Model, "anthropic.") {
		body, err = p.transformClaudeRequest(request)
		endpoint = fmt.Sprintf("/model/%s/invoke", request.Model)
	} else if strings.HasPrefix(request.Model, "meta.") {
		body, err = p.transformLlamaRequest(request)
		endpoint = fmt.Sprintf("/model/%s/invoke", request.Model)
	} else if strings.HasPrefix(request.Model, "mistral.") {
		body, err = p.transformMistralRequest(request)
		endpoint = fmt.Sprintf("/model/%s/invoke", request.Model)
	} else {
		return nil, fmt.Errorf("unsupported model: %s", request.Model)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Sign request with AWS Signature V4
	if err := p.signRequest(req, body); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bedrock API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response based on model type
	if strings.HasPrefix(request.Model, "anthropic.") {
		return p.parseClaudeResponse(resp.Body, request.Model)
	} else if strings.HasPrefix(request.Model, "meta.") {
		return p.parseLlamaResponse(resp.Body, request.Model)
	} else if strings.HasPrefix(request.Model, "mistral.") {
		return p.parseMistralResponse(resp.Body, request.Model)
	}

	return nil, fmt.Errorf("unsupported model: %s", request.Model)
}

// ChatCompletionStream implements streaming for Provider interface
func (p *BedrockProvider) ChatCompletionStream(ctx context.Context, request *ChatRequest) (<-chan StreamResponse, error) {
	// Check if model supports streaming upfront
	if !strings.HasPrefix(request.Model, "anthropic.") {
		return nil, fmt.Errorf("streaming not supported for model: %s", request.Model)
	}

	streamChan := make(chan StreamResponse, 100)

	go func() {
		defer close(streamChan)

		// Transform request based on model type
		var body []byte
		var err error
		var endpoint string

		body, err = p.transformClaudeRequest(request)
		endpoint = fmt.Sprintf("/model/%s/invoke-with-response-stream", request.Model)

		if err != nil {
			return // Just close channel on error
		}

		// Create HTTP request
		req, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+endpoint, bytes.NewReader(body))
		if err != nil {
			return // Just close channel on error
		}

		// Sign request
		if err := p.signRequest(req, body); err != nil {
			return // Just close channel on error
		}

		// Send request
		resp, err := p.client.Do(req)
		if err != nil {
			return // Just close channel on error
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return // Just close channel on error
		}

		// Parse streaming response
		p.parseStreamResponse(resp.Body, streamChan)
	}()

	return streamChan, nil
}

// signRequest signs the HTTP request with AWS Signature V4
func (p *BedrockProvider) signRequest(req *http.Request, body []byte) error {
	// Set required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Calculate body hash
	bodyHash := sha256.Sum256(body)
	req.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(bodyHash[:]))

	// Set date
	now := time.Now().UTC()
	dateTime := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	req.Header.Set("X-Amz-Date", dateTime)

	// Add session token if present
	if p.config.OrgID != "" { // Using OrgID to store session token
		req.Header.Set("X-Amz-Security-Token", p.config.OrgID)
	}

	// Create canonical request
	canonicalRequest := p.createCanonicalRequest(req, hex.EncodeToString(bodyHash[:]))

	// Create string to sign
	stringToSign := p.createStringToSign(dateTime, date, canonicalRequest)

	// Calculate signature
	signature := p.calculateSignature(date, stringToSign)

	// Add authorization header
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/%s/bedrock/aws4_request, SignedHeaders=content-type;host;x-amz-content-sha256;x-amz-date, Signature=%s",
		p.config.APIKey, date, p.config.Region, signature)
	req.Header.Set("Authorization", authHeader)

	return nil
}

func (p *BedrockProvider) createCanonicalRequest(req *http.Request, bodyHash string) string {
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.Header.Get("Content-Type"),
		req.Host,
		bodyHash,
		req.Header.Get("X-Amz-Date"))

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		req.URL.Path,
		req.URL.RawQuery,
		canonicalHeaders,
		"content-type;host;x-amz-content-sha256;x-amz-date",
		bodyHash)
}

func (p *BedrockProvider) createStringToSign(dateTime, date, canonicalRequest string) string {
	requestHash := sha256.Sum256([]byte(canonicalRequest))
	return fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s/%s/bedrock/aws4_request\n%s",
		dateTime,
		date,
		p.config.Region,
		hex.EncodeToString(requestHash[:]))
}

func (p *BedrockProvider) calculateSignature(date, stringToSign string) string {
	key := []byte("AWS4" + p.config.APISecret)
	dateKey := hmacSHA256(key, date)
	regionKey := hmacSHA256(dateKey, p.config.Region)
	serviceKey := hmacSHA256(regionKey, "bedrock")
	signingKey := hmacSHA256(serviceKey, "aws4_request")
	signature := hmacSHA256(signingKey, stringToSign)
	return hex.EncodeToString(signature)
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// Transform request for Claude models
func (p *BedrockProvider) transformClaudeRequest(request *ChatRequest) ([]byte, error) {
	claudeReq := map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"messages":          []map[string]interface{}{},
	}

	// Extract system message if present
	var systemMsg string
	messages := []map[string]interface{}{}

	for _, msg := range request.Messages {
		if msg.Role == "system" {
			if content, ok := msg.Content.(string); ok {
				systemMsg = content
			}
			continue
		}

		claudeMsg := map[string]interface{}{
			"role": msg.Role,
		}

		// Handle content
		if content, ok := msg.Content.(string); ok {
			claudeMsg["content"] = content
		} else if contentArray, ok := msg.Content.([]interface{}); ok {
			claudeMsg["content"] = contentArray
		}

		messages = append(messages, claudeMsg)
	}

	if systemMsg != "" {
		claudeReq["system"] = systemMsg
	}
	claudeReq["messages"] = messages

	// Add optional parameters
	if request.MaxTokens != nil {
		claudeReq["max_tokens"] = *request.MaxTokens
	} else {
		claudeReq["max_tokens"] = 2000 // Default for Claude
	}

	if request.Temperature != nil {
		claudeReq["temperature"] = *request.Temperature
	}

	if request.TopP != nil {
		claudeReq["top_p"] = *request.TopP
	}

	if len(request.Stop) > 0 {
		claudeReq["stop_sequences"] = request.Stop
	}

	return json.Marshal(claudeReq)
}

// Transform request for Llama models
func (p *BedrockProvider) transformLlamaRequest(request *ChatRequest) ([]byte, error) {
	// Build prompt in Llama format
	var prompt strings.Builder

	for _, msg := range request.Messages {
		content := ""
		if str, ok := msg.Content.(string); ok {
			content = str
		}

		switch msg.Role {
		case "system":
			prompt.WriteString(fmt.Sprintf("<|begin_of_text|><|start_header_id|>system<|end_header_id|>\n\n%s<|eot_id|>", content))
		case "user":
			prompt.WriteString(fmt.Sprintf("<|start_header_id|>user<|end_header_id|>\n\n%s<|eot_id|>", content))
		case "assistant":
			prompt.WriteString(fmt.Sprintf("<|start_header_id|>assistant<|end_header_id|>\n\n%s<|eot_id|>", content))
		}
	}
	prompt.WriteString("<|start_header_id|>assistant<|end_header_id|>\n\n")

	llamaReq := map[string]interface{}{
		"prompt": prompt.String(),
	}

	// Add optional parameters
	if request.MaxTokens != nil {
		llamaReq["max_gen_len"] = *request.MaxTokens
	}

	if request.Temperature != nil {
		llamaReq["temperature"] = *request.Temperature
	}

	if request.TopP != nil {
		llamaReq["top_p"] = *request.TopP
	}

	return json.Marshal(llamaReq)
}

// Transform request for Mistral models
func (p *BedrockProvider) transformMistralRequest(request *ChatRequest) ([]byte, error) {
	// Build prompt in Mistral format
	var prompt strings.Builder

	for _, msg := range request.Messages {
		content := ""
		if str, ok := msg.Content.(string); ok {
			content = str
		}

		switch msg.Role {
		case "system":
			prompt.WriteString(fmt.Sprintf("<s>[INST] %s [/INST]", content))
		case "user":
			prompt.WriteString(fmt.Sprintf("[INST] %s [/INST]", content))
		case "assistant":
			prompt.WriteString(fmt.Sprintf(" %s </s>", content))
		}
	}

	mistralReq := map[string]interface{}{
		"prompt": prompt.String(),
	}

	// Add optional parameters
	if request.MaxTokens != nil {
		mistralReq["max_tokens"] = *request.MaxTokens
	}

	if request.Temperature != nil {
		mistralReq["temperature"] = *request.Temperature
	}

	if request.TopP != nil {
		mistralReq["top_p"] = *request.TopP
	}

	return json.Marshal(mistralReq)
}

// Parse Claude response
func (p *BedrockProvider) parseClaudeResponse(body io.Reader, model string) (*ChatResponse, error) {
	var claudeResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(body).Decode(&claudeResp); err != nil {
		return nil, err
	}

	// Build response text
	var responseText strings.Builder
	for _, content := range claudeResp.Content {
		if content.Type == "text" {
			responseText.WriteString(content.Text)
		}
	}

	return &ChatResponse{
		ID:      fmt.Sprintf("bedrock-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: responseText.String(),
				},
				FinishReason: p.mapStopReason(claudeResp.StopReason),
			},
		},
		Usage: Usage{
			PromptTokens:     claudeResp.Usage.InputTokens,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		},
	}, nil
}

// Parse Llama response
func (p *BedrockProvider) parseLlamaResponse(body io.Reader, model string) (*ChatResponse, error) {
	var llamaResp struct {
		Generation           string `json:"generation"`
		PromptTokenCount     int    `json:"prompt_token_count"`
		GenerationTokenCount int    `json:"generation_token_count"`
		StopReason           string `json:"stop_reason"`
	}

	if err := json.NewDecoder(body).Decode(&llamaResp); err != nil {
		return nil, err
	}

	return &ChatResponse{
		ID:      fmt.Sprintf("bedrock-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: llamaResp.Generation,
				},
				FinishReason: p.mapStopReason(llamaResp.StopReason),
			},
		},
		Usage: Usage{
			PromptTokens:     llamaResp.PromptTokenCount,
			CompletionTokens: llamaResp.GenerationTokenCount,
			TotalTokens:      llamaResp.PromptTokenCount + llamaResp.GenerationTokenCount,
		},
	}, nil
}

// Parse Mistral response
func (p *BedrockProvider) parseMistralResponse(body io.Reader, model string) (*ChatResponse, error) {
	var mistralResp struct {
		Outputs []struct {
			Text       string `json:"text"`
			StopReason string `json:"stop_reason"`
		} `json:"outputs"`
	}

	if err := json.NewDecoder(body).Decode(&mistralResp); err != nil {
		return nil, err
	}

	responseText := ""
	stopReason := ""
	if len(mistralResp.Outputs) > 0 {
		responseText = mistralResp.Outputs[0].Text
		stopReason = mistralResp.Outputs[0].StopReason
	}

	return &ChatResponse{
		ID:      fmt.Sprintf("bedrock-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: responseText,
				},
				FinishReason: p.mapStopReason(stopReason),
			},
		},
	}, nil
}

// Parse streaming response
func (p *BedrockProvider) parseStreamResponse(body io.Reader, streamChan chan<- StreamResponse) {
	scanner := bufio.NewScanner(body)

	for scanner.Scan() {
		line := scanner.Text()

		// AWS event stream format
		if strings.HasPrefix(line, ":event-type:") {
			// Skip metadata lines
			continue
		}

		// Parse JSON payload
		if strings.HasPrefix(line, "{") {
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				continue
			}

			// Extract content delta
			if delta, ok := event["delta"].(map[string]interface{}); ok {
				if text, ok := delta["text"].(string); ok {
					streamChan <- StreamResponse{
						Choices: []StreamChoice{
							{
								Index: 0,
								Delta: Message{
									Content: text,
								},
							},
						},
					}
				}
			}
		}
	}
}

func (p *BedrockProvider) mapStopReason(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	default:
		return reason
	}
}

// Completion implements the Provider interface (legacy)
func (p *BedrockProvider) Completion(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error) {
	return nil, fmt.Errorf("completion API not supported by Bedrock provider")
}

// CompletionStream implements the Provider interface (legacy)
func (p *BedrockProvider) CompletionStream(ctx context.Context, request *CompletionRequest) (<-chan StreamResponse, error) {
	return nil, fmt.Errorf("completion stream API not supported by Bedrock provider")
}

// Embeddings implements the Provider interface
func (p *BedrockProvider) Embeddings(ctx context.Context, request *EmbeddingsRequest) (*EmbeddingsResponse, error) {
	return nil, fmt.Errorf("embeddings not yet implemented for Bedrock provider")
}

// Provider info methods
func (p *BedrockProvider) GetType() string {
	return "bedrock"
}

func (p *BedrockProvider) GetName() string {
	return p.name
}

func (p *BedrockProvider) GetPriority() int {
	return 50
}

func (p *BedrockProvider) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.healthy
}

func (p *BedrockProvider) SupportsModel(model string) bool {
	for _, m := range p.models {
		if m == model {
			return true
		}
	}
	return false
}

func (p *BedrockProvider) ListModels() []string {
	return p.models
}

func (p *BedrockProvider) HealthCheck(ctx context.Context) error {
	// Simple health check - try to access the service
	req, err := http.NewRequestWithContext(ctx, "GET", p.config.BaseURL, nil)
	if err != nil {
		return err
	}

	// Sign the request
	if err := p.signRequest(req, []byte{}); err != nil {
		return err
	}

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

	return nil
}
