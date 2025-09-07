package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/services/guardrails/types"
	"github.com/amerfu/pllm/internal/services/providers"
)

// PresidioGuardrail implements PII detection and masking using Presidio
type PresidioGuardrail struct {
	name          string
	config        *config.PresidioConfig
	mode          types.GuardrailMode
	enabled       bool
	logger        *zap.Logger
	httpClient    *http.Client
	
	// Presidio specific config
	language      string
	entityTypes   []string
	scoreThreshold float64
}

// PresidioAnalyzeRequest represents the request to Presidio Analyzer
type PresidioAnalyzeRequest struct {
	Text        string   `json:"text"`
	Language    string   `json:"language"`
	EntityTypes []string `json:"entities,omitempty"`
	ScoreThreshold float64 `json:"score_threshold,omitempty"`
}

// PresidioEntity represents a detected PII entity
type PresidioEntity struct {
	EntityType     string  `json:"entity_type"`
	Start          int     `json:"start"`
	End            int     `json:"end"`
	Score          float64 `json:"score"`
	RecognitionMetadata interface{} `json:"recognition_metadata,omitempty"`
}

// PresidioAnonymizeRequest represents the request to Presidio Anonymizer
type PresidioAnonymizeRequest struct {
	Text      string                    `json:"text"`
	Analyzer  []PresidioEntity         `json:"analyzer_results"`
	Operators map[string]OperatorConfig `json:"operators"`
}

// PresidioAnonymizeResponse represents the response from Presidio Anonymizer
type PresidioAnonymizeResponse struct {
	Text  string            `json:"text"`
	Items []AnonymizedItem  `json:"items"`
}

// OperatorConfig defines how to anonymize specific entity types
type OperatorConfig struct {
	Type   string                 `json:"type"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// AnonymizedItem represents an anonymized entity
type AnonymizedItem struct {
	EntityType string `json:"entity_type"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	Text       string `json:"text"`
	Operator   string `json:"operator"`
}

// NewPresidioGuardrail creates a new Presidio guardrail
func NewPresidioGuardrail(name string, config *config.PresidioConfig, mode types.GuardrailMode, enabled bool, logger *zap.Logger) *PresidioGuardrail {
	return &PresidioGuardrail{
		name:    name,
		config:  config,
		mode:    mode,
		enabled: enabled,
		logger:  logger.Named("presidio"),
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		language:       config.Language,
		entityTypes:    []string{}, // Empty means detect all supported types
		scoreThreshold: 0.35,       // Default threshold
	}
}

// Execute implements the Guardrail interface
func (p *PresidioGuardrail) Execute(ctx context.Context, input *types.GuardrailInput) (*types.GuardrailResult, error) {
	start := time.Now()
	
	// Extract text to analyze
	texts := p.extractTexts(input)
	if len(texts) == 0 {
		return &types.GuardrailResult{
			Passed:    true,
			Blocked:   false,
			Modified:  false,
			Reason:    "No text content to analyze",
		}, nil
	}
	
	// Process each text
	hasEntities := false
	totalEntities := 0
	var modifiedRequest *providers.ChatRequest
	var modifiedResponse *providers.ChatResponse
	
	if input.Request != nil {
		if req, ok := input.Request.(*providers.ChatRequest); ok {
			modifiedRequest = req
		}
	}
	
	if input.Response != nil {
		if resp, ok := input.Response.(*providers.ChatResponse); ok {
			modifiedResponse = resp
		}
	}
	
	for _, text := range texts {
		entities, err := p.analyzeText(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze text: %w", err)
		}
		
		if len(entities) > 0 {
			hasEntities = true
			totalEntities += len(entities)
			
			// If this is pre_call or logging_only, mask the PII
			if p.mode == types.PreCall || p.mode == types.LoggingOnly {
				maskedText, err := p.anonymizeText(ctx, text, entities)
				if err != nil {
					p.logger.Error("Failed to mask PII", zap.Error(err))
					// Continue without masking rather than fail
				} else {
					// Apply masking to request/response
					modifiedRequest = p.applyMasking(modifiedRequest, text, maskedText)
				}
			}
		}
	}
	
	// Determine result based on mode
	result := &types.GuardrailResult{
		Passed:        true,
		Blocked:       false,
		Modified:      false,
		ExecutionTime: time.Since(start),
		Details: map[string]interface{}{
			"total_entities": totalEntities,
			"has_pii":       hasEntities,
		},
	}
	
	if hasEntities {
		if p.mode == types.PreCall {
			// For pre_call, mask PII but allow request
			result.Modified = true
			result.ModifiedRequest = modifiedRequest
			result.Reason = fmt.Sprintf("PII detected and masked (%d entities)", totalEntities)
		} else if p.mode == types.PostCall {
			// For post_call, we could either block or just log
			// For now, just log the detection
			result.Reason = fmt.Sprintf("PII detected in response (%d entities)", totalEntities)
		} else if p.mode == types.LoggingOnly {
			// For logging_only, always mask
			result.Modified = true
			result.ModifiedRequest = modifiedRequest
			result.ModifiedResponse = modifiedResponse
			result.Reason = fmt.Sprintf("PII masked for logging (%d entities)", totalEntities)
		}
	}
	
	return result, nil
}

// extractTexts extracts all text content from the input for analysis
func (p *PresidioGuardrail) extractTexts(input *types.GuardrailInput) []string {
	var texts []string
	
	// Extract from request messages
	if input.Request != nil {
		if req, ok := input.Request.(*providers.ChatRequest); ok {
			for _, msg := range req.Messages {
				if content, ok := msg.Content.(string); ok {
					texts = append(texts, content)
				}
			}
		}
	}
	
	// Extract from response
	if input.Response != nil {
		if resp, ok := input.Response.(*providers.ChatResponse); ok {
			for _, choice := range resp.Choices {
				if choice.Message.Content != nil {
					if content, ok := choice.Message.Content.(string); ok {
						texts = append(texts, content)
					}
				}
			}
		}
	}
	
	return texts
}

// analyzeText calls Presidio Analyzer to detect PII entities
func (p *PresidioGuardrail) analyzeText(ctx context.Context, text string) ([]PresidioEntity, error) {
	if strings.TrimSpace(text) == "" {
		return []PresidioEntity{}, nil
	}
	
	request := PresidioAnalyzeRequest{
		Text:           text,
		Language:       p.language,
		EntityTypes:    p.entityTypes,
		ScoreThreshold: p.scoreThreshold,
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analyze request: %w", err)
	}
	
	url := strings.TrimSuffix(p.config.AnalyzerURL, "/") + "/analyze"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create analyze request: %w", err)
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call analyzer: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("analyzer returned status %d", resp.StatusCode)
	}
	
	var entities []PresidioEntity
	if err := json.NewDecoder(resp.Body).Decode(&entities); err != nil {
		return nil, fmt.Errorf("failed to decode analyze response: %w", err)
	}
	
	return entities, nil
}

// anonymizeText calls Presidio Anonymizer to mask PII entities
func (p *PresidioGuardrail) anonymizeText(ctx context.Context, text string, entities []PresidioEntity) (string, error) {
	if len(entities) == 0 {
		return text, nil
	}
	
	// Default operators for common entity types
	operators := map[string]OperatorConfig{
		"DEFAULT": {
			Type: "replace",
			Params: map[string]interface{}{
				"new_value": "[REDACTED]",
			},
		},
		"PERSON": {
			Type: "replace",
			Params: map[string]interface{}{
				"new_value": "[PERSON]",
			},
		},
		"EMAIL_ADDRESS": {
			Type: "replace",
			Params: map[string]interface{}{
				"new_value": "[EMAIL]",
			},
		},
		"PHONE_NUMBER": {
			Type: "replace",
			Params: map[string]interface{}{
				"new_value": "[PHONE]",
			},
		},
		"CREDIT_CARD": {
			Type: "replace",
			Params: map[string]interface{}{
				"new_value": "[CREDIT_CARD]",
			},
		},
	}
	
	request := PresidioAnonymizeRequest{
		Text:      text,
		Analyzer:  entities,
		Operators: operators,
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return text, fmt.Errorf("failed to marshal anonymize request: %w", err)
	}
	
	url := strings.TrimSuffix(p.config.AnonymizerURL, "/") + "/anonymize"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return text, fmt.Errorf("failed to create anonymize request: %w", err)
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return text, fmt.Errorf("failed to call anonymizer: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return text, fmt.Errorf("anonymizer returned status %d", resp.StatusCode)
	}
	
	var anonymizeResp PresidioAnonymizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&anonymizeResp); err != nil {
		return text, fmt.Errorf("failed to decode anonymize response: %w", err)
	}
	
	return anonymizeResp.Text, nil
}

// applyMasking applies masked text back to the request structure
func (p *PresidioGuardrail) applyMasking(request *providers.ChatRequest, originalText, maskedText string) *providers.ChatRequest {
	if request == nil || originalText == maskedText {
		return request
	}
	
	// Create a copy of the request
	modifiedRequest := *request
	modifiedRequest.Messages = make([]providers.Message, len(request.Messages))
	copy(modifiedRequest.Messages, request.Messages)
	
	// Apply masking to matching messages
	for i, msg := range modifiedRequest.Messages {
		if content, ok := msg.Content.(string); ok && content == originalText {
			modifiedRequest.Messages[i].Content = maskedText
		}
	}
	
	return &modifiedRequest
}

// GetName implements the Guardrail interface
func (p *PresidioGuardrail) GetName() string {
	return p.name
}

// GetType implements the Guardrail interface
func (p *PresidioGuardrail) GetType() types.GuardrailType {
	return types.PII
}

// GetMode implements the Guardrail interface
func (p *PresidioGuardrail) GetMode() types.GuardrailMode {
	return p.mode
}

// IsEnabled implements the Guardrail interface
func (p *PresidioGuardrail) IsEnabled() bool {
	return p.enabled
}

// HealthCheck implements the Guardrail interface
func (p *PresidioGuardrail) HealthCheck(ctx context.Context) error {
	// Check analyzer
	analyzerURL := strings.TrimSuffix(p.config.AnalyzerURL, "/") + "/health"
	if err := p.checkEndpoint(ctx, analyzerURL); err != nil {
		return fmt.Errorf("analyzer health check failed: %w", err)
	}
	
	// Check anonymizer
	anonymizerURL := strings.TrimSuffix(p.config.AnonymizerURL, "/") + "/health"
	if err := p.checkEndpoint(ctx, anonymizerURL); err != nil {
		return fmt.Errorf("anonymizer health check failed: %w", err)
	}
	
	return nil
}

// checkEndpoint performs a health check on an endpoint
func (p *PresidioGuardrail) checkEndpoint(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("endpoint returned status %d", resp.StatusCode)
	}
	
	return nil
}