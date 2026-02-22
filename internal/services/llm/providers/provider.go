package providers

import (
	"context"
	"io"
	"time"
)

type Provider interface {
	// Chat completions
	ChatCompletion(ctx context.Context, request *ChatRequest) (*ChatResponse, error)
	ChatCompletionStream(ctx context.Context, request *ChatRequest) (<-chan StreamResponse, error)

	// Completions (legacy)
	Completion(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error)
	CompletionStream(ctx context.Context, request *CompletionRequest) (<-chan StreamResponse, error)

	// Embeddings
	Embeddings(ctx context.Context, request *EmbeddingsRequest) (*EmbeddingsResponse, error)

	// Audio
	AudioTranscription(ctx context.Context, request *TranscriptionRequest) (*TranscriptionResponse, error)
	AudioSpeech(ctx context.Context, request *SpeechRequest) ([]byte, error)

	// Images
	ImageGeneration(ctx context.Context, request *ImageRequest) (*ImageResponse, error)

	// Provider info
	GetType() string
	GetName() string
	GetPriority() int
	IsHealthy() bool
	SupportsModel(model string) bool
	ListModels() []string

	// Health check
	HealthCheck(ctx context.Context) error
}

// Request/Response types matching OpenAI format

type ChatRequest struct {
	Model            string          `json:"model"`
	Messages         []Message       `json:"messages"`
	Temperature      *float32        `json:"temperature,omitempty"`
	TopP             *float32        `json:"top_p,omitempty"`
	N                *int            `json:"n,omitempty"`
	Stream           bool            `json:"stream,omitempty"`
	Stop             []string        `json:"stop,omitempty"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	PresencePenalty  *float32        `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float32        `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]int  `json:"logit_bias,omitempty"`
	User             string          `json:"user,omitempty"`
	ResponseFormat   *ResponseFormat `json:"response_format,omitempty"`
	Seed             *int            `json:"seed,omitempty"`
	Tools            []Tool          `json:"tools,omitempty"`
	ToolChoice       interface{}     `json:"tool_choice,omitempty"`
	ReasoningEffort  *string         `json:"reasoning_effort,omitempty"`
}

type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // Can be string or []MessageContent for vision
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// MessageContent represents individual content blocks for vision messages
type MessageContent struct {
	Type     string    `json:"type"`     // "text" or "image_url"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents image content
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low", "high", or "auto"
}

type ResponseFormat struct {
	Type string `json:"type"`
}

type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage,omitempty"`
	SystemFingerprint string   `json:"system_fingerprint,omitempty"`
}

type Choice struct {
	Index        int         `json:"index"`
	Message      Message     `json:"message,omitempty"`
	Delta        Message     `json:"delta,omitempty"`
	FinishReason string      `json:"finish_reason,omitempty"`
	LogProbs     interface{} `json:"logprobs,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

type StreamChoice struct {
	Index        int     `json:"index"`
	Delta        Message `json:"delta"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

type CompletionRequest struct {
	Model            string         `json:"model"`
	Prompt           interface{}    `json:"prompt"`
	Suffix           string         `json:"suffix,omitempty"`
	MaxTokens        *int           `json:"max_tokens,omitempty"`
	Temperature      *float32       `json:"temperature,omitempty"`
	TopP             *float32       `json:"top_p,omitempty"`
	N                *int           `json:"n,omitempty"`
	Stream           bool           `json:"stream,omitempty"`
	LogProbs         *int           `json:"logprobs,omitempty"`
	Echo             bool           `json:"echo,omitempty"`
	Stop             []string       `json:"stop,omitempty"`
	PresencePenalty  *float32       `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float32       `json:"frequency_penalty,omitempty"`
	BestOf           *int           `json:"best_of,omitempty"`
	LogitBias        map[string]int `json:"logit_bias,omitempty"`
	User             string         `json:"user,omitempty"`
}

type CompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
	Usage   Usage              `json:"usage,omitempty"`
}

type CompletionChoice struct {
	Text         string      `json:"text"`
	Index        int         `json:"index"`
	LogProbs     interface{} `json:"logprobs,omitempty"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

type EmbeddingsRequest struct {
	Model          string      `json:"model"`
	Input          interface{} `json:"input"`
	User           string      `json:"user,omitempty"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
}

type EmbeddingsResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  Usage       `json:"usage"`
}

type Embedding struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// Image generation types

type ImageRequest struct {
	Prompt         string `json:"prompt"`
	Model          string `json:"model,omitempty"`
	N              *int   `json:"n,omitempty"`
	Quality        string `json:"quality,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	Size           string `json:"size,omitempty"`
	Style          string `json:"style,omitempty"`
	User           string `json:"user,omitempty"`
}

type ImageResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
}

type ImageData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// Audio types

type TranscriptionRequest struct {
	File           io.Reader `json:"file"`
	Model          string    `json:"model"`
	Language       string    `json:"language,omitempty"`
	Prompt         string    `json:"prompt,omitempty"`
	ResponseFormat string    `json:"response_format,omitempty"`
	Temperature    *float32  `json:"temperature,omitempty"`
}

type TranscriptionResponse struct {
	Text string `json:"text"`
}

type TranslationResponse struct {
	Text string `json:"text"`
}

type SpeechRequest struct {
	Model          string   `json:"model"`
	Input          string   `json:"input"`
	Voice          string   `json:"voice"`
	ResponseFormat string   `json:"response_format,omitempty"`
	Speed          *float32 `json:"speed,omitempty"`
}

// Moderation types

type ModerationRequest struct {
	Input string `json:"input"`
	Model string `json:"model,omitempty"`
}

type ModerationResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Results []ModerationResult `json:"results"`
}

type ModerationResult struct {
	Flagged    bool               `json:"flagged"`
	Categories map[string]bool    `json:"categories"`
	Scores     map[string]float32 `json:"category_scores"`
}

// Error types

type APIError struct {
	Message string      `json:"message"`
	Type    string      `json:"type"`
	Param   string      `json:"param,omitempty"`
	Code    interface{} `json:"code,omitempty"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}

// Base provider implementation

type BaseProvider struct {
	name     string
	typ      string
	priority int
	healthy  bool
	models   []string
}

func NewBaseProvider(name, typ string, priority int, models []string) *BaseProvider {
	return &BaseProvider{
		name:     name,
		typ:      typ,
		priority: priority,
		healthy:  true,
		models:   models,
	}
}

func (p *BaseProvider) GetType() string {
	return p.typ
}

func (p *BaseProvider) GetName() string {
	return p.name
}

func (p *BaseProvider) GetPriority() int {
	return p.priority
}

func (p *BaseProvider) IsHealthy() bool {
	return p.healthy
}

func (p *BaseProvider) SetHealthy(healthy bool) {
	p.healthy = healthy
}

func (p *BaseProvider) SupportsModel(model string) bool {
	for _, m := range p.models {
		if m == model {
			return true
		}
	}
	return false
}

func (p *BaseProvider) ListModels() []string {
	return p.models
}

func GenerateID() string {
	return "chatcmpl-" + generateRandomString(29)
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// Anthropic Messages API format types for public endpoint

type MessagesAPIMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []MessagesAPIContent
}

type MessagesAPIContent struct {
	Type   string                   `json:"type"`
	Text   string                   `json:"text,omitempty"`
	Source *MessagesAPIImageSource `json:"source,omitempty"`
}

type MessagesAPIImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
	URL       string `json:"url,omitempty"`
}

type MessagesAPIRequest struct {
	Model         string                `json:"model"`
	MaxTokens     int                   `json:"max_tokens"`
	Messages      []MessagesAPIMessage  `json:"messages"`
	System        string                `json:"system,omitempty"`
	Temperature   *float32              `json:"temperature,omitempty"`
	TopP          *float32              `json:"top_p,omitempty"`
	TopK          *int                  `json:"top_k,omitempty"`
	Stream        bool                  `json:"stream,omitempty"`
	StopSequences []string              `json:"stop_sequences,omitempty"`
	Tools         []MessagesAPITool     `json:"tools,omitempty"`
	ToolChoice    interface{}           `json:"tool_choice,omitempty"`
}

type MessagesAPITool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema,omitempty"`
}

type MessagesAPIResponse struct {
	ID           string                `json:"id"`
	Type         string                `json:"type"`
	Role         string                `json:"role"`
	Content      []MessagesAPIContent  `json:"content"`
	Model        string                `json:"model"`
	StopReason   string                `json:"stop_reason,omitempty"`
	StopSequence string                `json:"stop_sequence,omitempty"`
	Usage        MessagesAPIUsage      `json:"usage"`
}

type MessagesAPIUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type MessagesAPIStreamResponse struct {
	Type  string              `json:"type"`
	Index int                 `json:"index,omitempty"`
	Delta interface{}         `json:"delta,omitempty"`
	Usage *MessagesAPIUsage   `json:"usage,omitempty"`
}

func GenerateMessagesAPIID() string {
	return "msg_" + generateRandomString(25)
}
