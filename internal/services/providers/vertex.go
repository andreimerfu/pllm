package providers

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// VertexProvider implements Google Vertex AI provider
type VertexProvider struct {
	mu         sync.RWMutex
	name       string
	config     ProviderConfig
	client     *http.Client
	healthy    bool
	projectID  string
	region     string
	token      string
	tokenExp   time.Time
	serviceAcc *ServiceAccount
}

// ServiceAccount represents Google service account credentials
type ServiceAccount struct {
	Type                string `json:"type"`
	ProjectID           string `json:"project_id"`
	PrivateKeyID        string `json:"private_key_id"`
	PrivateKey          string `json:"private_key"`
	ClientEmail         string `json:"client_email"`
	ClientID            string `json:"client_id"`
	AuthURI             string `json:"auth_uri"`
	TokenURI            string `json:"token_uri"`
	AuthProviderCertURL string `json:"auth_provider_x509_cert_url"`
	ClientCertURL       string `json:"client_x509_cert_url"`
}

// JWT represents a JSON Web Token
type JWT struct {
	Header    JWTHeader
	Claims    JWTClaims
	Signature string
}

type JWTHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid,omitempty"`
}

type JWTClaims struct {
	Issuer   string `json:"iss"`
	Scope    string `json:"scope"`
	Audience string `json:"aud"`
	IssuedAt int64  `json:"iat"`
	Expiry   int64  `json:"exp"`
}

// NewVertexProvider creates a new Google Vertex AI provider
func NewVertexProvider(name string, config ProviderConfig) (*VertexProvider, error) {
	// Parse service account from config
	var serviceAcc *ServiceAccount

	// Check if service account JSON is provided in APIKey
	if config.APIKey != "" {
		serviceAcc = &ServiceAccount{}
		if err := json.Unmarshal([]byte(config.APIKey), serviceAcc); err != nil {
			// APIKey might be just the private key, try to build service account
			if config.Extra != nil {
				if projectID, ok := config.Extra["project_id"].(string); ok {
					if clientEmail, ok := config.Extra["client_email"].(string); ok {
						serviceAcc = &ServiceAccount{
							Type:        "service_account",
							ProjectID:   projectID,
							PrivateKey:  config.APIKey,
							ClientEmail: clientEmail,
							TokenURI:    "https://oauth2.googleapis.com/token",
						}
					}
				}
			}

			if serviceAcc.ProjectID == "" {
				return nil, fmt.Errorf("invalid service account configuration")
			}
		}
	} else {
		return nil, fmt.Errorf("service account credentials required for Vertex AI")
	}

	// Get project ID and region
	projectID := serviceAcc.ProjectID
	if config.Extra != nil {
		if pid, ok := config.Extra["project_id"].(string); ok && pid != "" {
			projectID = pid
		}
	}

	region := config.Region
	if region == "" {
		region = "us-central1" // Default region
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	p := &VertexProvider{
		name:       name,
		config:     config,
		client:     client,
		healthy:    true,
		projectID:  projectID,
		region:     region,
		serviceAcc: serviceAcc,
	}

	return p, nil
}

// getAccessToken gets or refreshes the access token
func (p *VertexProvider) getAccessToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if token is still valid
	if p.token != "" && time.Now().Before(p.tokenExp.Add(-5*time.Minute)) {
		return p.token, nil
	}

	// Generate JWT
	jwt, err := p.generateJWT()
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT: %w", err)
	}

	// Exchange JWT for access token
	token, expiry, err := p.exchangeJWT(ctx, jwt)
	if err != nil {
		return "", fmt.Errorf("failed to exchange JWT: %w", err)
	}

	p.token = token
	p.tokenExp = expiry

	return token, nil
}

// generateJWT generates a JWT for service account authentication
func (p *VertexProvider) generateJWT() (string, error) {
	now := time.Now()

	header := JWTHeader{
		Algorithm: "RS256",
		Type:      "JWT",
		KeyID:     p.serviceAcc.PrivateKeyID,
	}

	claims := JWTClaims{
		Issuer:   p.serviceAcc.ClientEmail,
		Scope:    "https://www.googleapis.com/auth/cloud-platform",
		Audience: "https://oauth2.googleapis.com/token",
		IssuedAt: now.Unix(),
		Expiry:   now.Add(1 * time.Hour).Unix(),
	}

	// Encode header and claims
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Create signature
	signatureInput := headerB64 + "." + claimsB64

	// Parse private key
	block, _ := pem.Decode([]byte(p.serviceAcc.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("failed to parse private key")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("private key is not RSA")
	}

	// Sign the input
	hash := sha256.Sum256([]byte(signatureInput))
	signature, err := rsa.SignPKCS1v15(nil, rsaKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}

	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)

	return signatureInput + "." + signatureB64, nil
}

// exchangeJWT exchanges a JWT for an access token
func (p *VertexProvider) exchangeJWT(ctx context.Context, jwt string) (string, time.Time, error) {
	reqBody := fmt.Sprintf("grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer&assertion=%s", jwt)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token",
		strings.NewReader(reqBody))
	if err != nil {
		return "", time.Time{}, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("token exchange failed: status %d, body: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", time.Time{}, err
	}

	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return tokenResp.AccessToken, expiry, nil
}

// ChatCompletion implements the Provider interface
func (p *VertexProvider) ChatCompletion(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	// Get access token
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// Determine endpoint based on model
	var url string
	var body []byte

	if strings.Contains(request.Model, "claude") {
		// Anthropic Claude models via Vertex
		url = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/anthropic/models/%s:streamRawPredict",
			p.region, p.projectID, p.region, request.Model)
		body, err = p.transformClaudeRequest(request)
	} else if strings.Contains(request.Model, "gemini") {
		// Google Gemini models
		url = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/endpoints/openapi/chat/completions",
			p.region, p.projectID, p.region)
		body, err = p.transformGeminiRequest(request)
	} else {
		// Default to OpenAI-compatible endpoint
		url = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/endpoints/openapi/chat/completions",
			p.region, p.projectID, p.region)
		body, err = json.Marshal(request)
	}

	if err != nil {
		return nil, err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Vertex AI API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response based on model type
	if strings.Contains(request.Model, "claude") {
		return p.parseClaudeResponse(resp.Body, request.Model)
	} else if strings.Contains(request.Model, "gemini") {
		return p.parseGeminiResponse(resp.Body, request.Model)
	}

	// Default OpenAI-compatible response
	var vertexResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&vertexResp); err != nil {
		return nil, err
	}

	return &vertexResp, nil
}

// transformClaudeRequest transforms request for Claude models
func (p *VertexProvider) transformClaudeRequest(request *ChatRequest) ([]byte, error) {
	claudeReq := map[string]interface{}{
		"anthropic_version": "vertex-2023-10-16",
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

// transformGeminiRequest transforms request for Gemini models
func (p *VertexProvider) transformGeminiRequest(request *ChatRequest) ([]byte, error) {
	geminiReq := map[string]interface{}{
		"model":    request.Model,
		"messages": request.Messages,
	}

	// Add generation config
	genConfig := map[string]interface{}{}

	if request.Temperature != nil {
		genConfig["temperature"] = *request.Temperature
	}

	if request.TopP != nil {
		genConfig["topP"] = *request.TopP
	}

	if request.MaxTokens != nil {
		genConfig["maxOutputTokens"] = *request.MaxTokens
	}

	if len(request.Stop) > 0 {
		genConfig["stopSequences"] = request.Stop
	}

	if len(genConfig) > 0 {
		geminiReq["generationConfig"] = genConfig
	}

	return json.Marshal(geminiReq)
}

// parseClaudeResponse parses Claude model response
func (p *VertexProvider) parseClaudeResponse(body io.Reader, model string) (*ChatResponse, error) {
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
		ID:      fmt.Sprintf("vertex-%d", time.Now().Unix()),
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

// parseGeminiResponse parses Gemini model response
func (p *VertexProvider) parseGeminiResponse(body io.Reader, model string) (*ChatResponse, error) {
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.NewDecoder(body).Decode(&geminiResp); err != nil {
		return nil, err
	}

	// Build response text
	var responseText strings.Builder
	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		for _, part := range geminiResp.Candidates[0].Content.Parts {
			responseText.WriteString(part.Text)
		}
	}

	finishReason := ""
	if len(geminiResp.Candidates) > 0 {
		finishReason = p.mapGeminiFinishReason(geminiResp.Candidates[0].FinishReason)
	}

	return &ChatResponse{
		ID:      fmt.Sprintf("vertex-%d", time.Now().Unix()),
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
				FinishReason: finishReason,
			},
		},
		Usage: Usage{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

func (p *VertexProvider) mapStopReason(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	default:
		return reason
	}
}

func (p *VertexProvider) mapGeminiFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	default:
		return reason
	}
}

// ChatCompletionStream implements streaming for Provider interface
func (p *VertexProvider) ChatCompletionStream(ctx context.Context, request *ChatRequest) (<-chan StreamResponse, error) {
	// Streaming implementation would be similar to ChatCompletion
	// but parsing SSE events. For now, return not implemented.
	return nil, fmt.Errorf("streaming not yet implemented for Vertex AI provider")
}

// Completion implements the Provider interface (legacy)
func (p *VertexProvider) Completion(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error) {
	return nil, fmt.Errorf("completion API not supported by Vertex AI provider")
}

// CompletionStream implements the Provider interface (legacy)
func (p *VertexProvider) CompletionStream(ctx context.Context, request *CompletionRequest) (<-chan StreamResponse, error) {
	return nil, fmt.Errorf("completion stream API not supported by Vertex AI provider")
}

// Embeddings implements the Provider interface
func (p *VertexProvider) Embeddings(ctx context.Context, request *EmbeddingsRequest) (*EmbeddingsResponse, error) {
	// Get access token
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// Build URL for embeddings
	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/endpoints/openapi/embeddings",
		p.region, p.projectID, p.region)

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
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Vertex AI API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var vertexResp EmbeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&vertexResp); err != nil {
		return nil, err
	}

	return &vertexResp, nil
}

// Provider info methods
func (p *VertexProvider) GetType() string {
	return "vertex"
}

func (p *VertexProvider) GetName() string {
	return p.name
}

func (p *VertexProvider) GetPriority() int {
	return 55
}

func (p *VertexProvider) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.healthy
}

func (p *VertexProvider) SupportsModel(model string) bool {
	supportedModels := []string{
		// Claude models via Vertex
		"claude-3-opus",
		"claude-3-sonnet",
		"claude-3-haiku",
		"claude-instant",
		// Gemini models
		"gemini-pro",
		"gemini-pro-vision",
		"gemini-ultra",
		// PaLM models
		"text-bison",
		"chat-bison",
		"code-bison",
	}

	for _, m := range supportedModels {
		if strings.Contains(model, m) {
			return true
		}
	}

	return false
}

func (p *VertexProvider) ListModels() []string {
	return []string{
		"claude-3-opus",
		"claude-3-sonnet",
		"claude-3-haiku",
		"gemini-pro",
		"gemini-pro-vision",
		"text-bison",
		"chat-bison",
	}
}

func (p *VertexProvider) HealthCheck(ctx context.Context) error {
	// Try to get an access token
	_, err := p.getAccessToken(ctx)
	if err != nil {
		p.mu.Lock()
		p.healthy = false
		p.mu.Unlock()
		return err
	}

	p.mu.Lock()
	p.healthy = true
	p.mu.Unlock()

	return nil
}
