package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// RealtimeEvent represents a realtime API event
type RealtimeEvent struct {
	Type      string          `json:"type"`
	EventID   string          `json:"event_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp time.Time       `json:"timestamp,omitempty"`
}

// RealtimeSession represents an active realtime session
type RealtimeSession struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	UserID      *uint     `json:"user_id,omitempty" gorm:"index"`
	TeamID      *uint     `json:"team_id,omitempty" gorm:"index"`
	KeyID       *uint     `json:"key_id,omitempty" gorm:"index"`
	ModelName   string    `json:"model_name" gorm:"size:255;not null"`
	Status      string    `json:"status" gorm:"size:50;default:'active'"`
	Config      json.RawMessage `json:"config,omitempty" gorm:"type:jsonb"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty" gorm:"index"`
	
	// Relationships
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Team *Team `json:"team,omitempty" gorm:"foreignKey:TeamID"`
	Key  *Key  `json:"key,omitempty" gorm:"foreignKey:KeyID"`
}

// RealtimeSessionConfig represents session configuration
type RealtimeSessionConfig struct {
	Modalities                 []string               `json:"modalities,omitempty"`
	Instructions               string                 `json:"instructions,omitempty"`
	Voice                      string                 `json:"voice,omitempty"`
	InputAudioFormat           string                 `json:"input_audio_format,omitempty"`
	OutputAudioFormat          string                 `json:"output_audio_format,omitempty"`
	InputAudioTranscription    *AudioTranscriptionConfig `json:"input_audio_transcription,omitempty"`
	TurnDetection              *TurnDetectionConfig   `json:"turn_detection,omitempty"`
	Tools                      []RealtimeTool         `json:"tools,omitempty"`
	ToolChoice                 interface{}            `json:"tool_choice,omitempty"`
	Temperature                *float32               `json:"temperature,omitempty"`
	MaxResponseOutputTokens    *int                   `json:"max_response_output_tokens,omitempty"`
}

// AudioTranscriptionConfig for session configuration
type AudioTranscriptionConfig struct {
	Model string `json:"model"`
}

// TurnDetectionConfig for voice activity detection
type TurnDetectionConfig struct {
	Type               string   `json:"type"`
	Threshold          *float32 `json:"threshold,omitempty"`
	PrefixPaddingMs    *int     `json:"prefix_padding_ms,omitempty"`
	SilenceDurationMs  *int     `json:"silence_duration_ms,omitempty"`
}

// RealtimeTool represents a function tool for realtime API
type RealtimeTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// Client Events (sent from client to server)

// SessionUpdateEvent updates session configuration
type SessionUpdateEvent struct {
	Session RealtimeSessionConfig `json:"session"`
}

// InputAudioBufferAppendEvent adds audio data to input buffer
type InputAudioBufferAppendEvent struct {
	Audio string `json:"audio"` // base64 encoded audio
}

// InputAudioBufferCommitEvent commits pending audio
type InputAudioBufferCommitEvent struct{}

// InputAudioBufferClearEvent clears audio buffer  
type InputAudioBufferClearEvent struct{}

// ConversationItemCreateEvent creates a conversation item
type ConversationItemCreateEvent struct {
	PreviousItemID *string              `json:"previous_item_id,omitempty"`
	Item           RealtimeConversationItem `json:"item"`
}

// ConversationItemTruncateEvent truncates an item
type ConversationItemTruncateEvent struct {
	ItemID      string `json:"item_id"`
	ContentIndex int    `json:"content_index"`
	AudioEndMs  int    `json:"audio_end_ms"`
}

// ConversationItemDeleteEvent deletes an item
type ConversationItemDeleteEvent struct {
	ItemID string `json:"item_id"`
}

// ResponseCreateEvent requests a response
type ResponseCreateEvent struct {
	Response *RealtimeResponse `json:"response,omitempty"`
}

// ResponseCancelEvent cancels response generation
type ResponseCancelEvent struct{}

// RealtimeResponse represents a response configuration
type RealtimeResponse struct {
	Modalities              []string               `json:"modalities,omitempty"`
	Instructions            string                 `json:"instructions,omitempty"`
	Voice                   string                 `json:"voice,omitempty"`
	OutputAudioFormat       string                 `json:"output_audio_format,omitempty"`
	Tools                   []RealtimeTool         `json:"tools,omitempty"`
	ToolChoice              interface{}            `json:"tool_choice,omitempty"`
	Temperature             *float32               `json:"temperature,omitempty"`
	MaxOutputTokens         *int                   `json:"max_output_tokens,omitempty"`
}

// Server Events (sent from server to client)

// ErrorEvent represents an error from the server
type ErrorEvent struct {
	Type    string                 `json:"type"`
	Code    string                 `json:"code,omitempty"`
	Message string                 `json:"message"`
	Param   string                 `json:"param,omitempty"`
	EventID string                 `json:"event_id,omitempty"`
}

// SessionCreatedEvent sent when session is created
type SessionCreatedEvent struct {
	Session RealtimeSessionConfig `json:"session"`
}

// SessionUpdatedEvent sent when session is updated
type SessionUpdatedEvent struct {
	Session RealtimeSessionConfig `json:"session"`
}

// ConversationCreatedEvent sent when conversation starts
type ConversationCreatedEvent struct {
	Conversation RealtimeConversation `json:"conversation"`
}

// RealtimeConversation represents a conversation state
type RealtimeConversation struct {
	ID     string                      `json:"id"`
	Object string                      `json:"object"`
	Items  []RealtimeConversationItem  `json:"items"`
}

// ConversationItemCreatedEvent sent when item is created
type ConversationItemCreatedEvent struct {
	PreviousItemID *string                  `json:"previous_item_id,omitempty"`
	Item           RealtimeConversationItem `json:"item"`
}

// ConversationItemInputAudioTranscriptionCompletedEvent sent when transcription completes
type ConversationItemInputAudioTranscriptionCompletedEvent struct {
	ItemID       string `json:"item_id"`
	ContentIndex int    `json:"content_index"`
	Transcript   string `json:"transcript"`
}

// ConversationItemInputAudioTranscriptionFailedEvent sent when transcription fails
type ConversationItemInputAudioTranscriptionFailedEvent struct {
	ItemID       string                 `json:"item_id"`
	ContentIndex int                    `json:"content_index"`
	Error        map[string]interface{} `json:"error"`
}

// ConversationItemTruncatedEvent sent when item is truncated
type ConversationItemTruncatedEvent struct {
	ItemID       string `json:"item_id"`
	ContentIndex int    `json:"content_index"`
	AudioEndMs   int    `json:"audio_end_ms"`
}

// ConversationItemDeletedEvent sent when item is deleted
type ConversationItemDeletedEvent struct {
	ItemID string `json:"item_id"`
}

// InputAudioBufferCommittedEvent sent when audio buffer is committed
type InputAudioBufferCommittedEvent struct {
	PreviousItemID *string `json:"previous_item_id,omitempty"`
	ItemID         string  `json:"item_id"`
}

// InputAudioBufferClearedEvent sent when audio buffer is cleared
type InputAudioBufferClearedEvent struct{}

// InputAudioBufferSpeechStartedEvent sent when speech starts
type InputAudioBufferSpeechStartedEvent struct {
	AudioStartMs int    `json:"audio_start_ms"`
	ItemID       string `json:"item_id"`
}

// InputAudioBufferSpeechStoppedEvent sent when speech stops
type InputAudioBufferSpeechStoppedEvent struct {
	AudioEndMs int    `json:"audio_end_ms"`
	ItemID     string `json:"item_id"`
}

// ResponseCreatedEvent sent when response starts
type ResponseCreatedEvent struct {
	Response RealtimeResponseObject `json:"response"`
}

// ResponseDoneEvent sent when response completes
type ResponseDoneEvent struct {
	Response RealtimeResponseObject `json:"response"`
}

// ResponseOutputItemAddedEvent sent when output item is added
type ResponseOutputItemAddedEvent struct {
	ResponseID   string                   `json:"response_id"`
	OutputIndex  int                      `json:"output_index"`
	Item         RealtimeConversationItem `json:"item"`
}

// ResponseOutputItemDoneEvent sent when output item completes
type ResponseOutputItemDoneEvent struct {
	ResponseID   string                   `json:"response_id"`
	OutputIndex  int                      `json:"output_index"`
	Item         RealtimeConversationItem `json:"item"`
}

// ResponseContentPartAddedEvent sent when content part is added
type ResponseContentPartAddedEvent struct {
	ResponseID   string                     `json:"response_id"`
	ItemID       string                     `json:"item_id"`
	OutputIndex  int                        `json:"output_index"`
	ContentIndex int                        `json:"content_index"`
	Part         RealtimeConversationContent `json:"part"`
}

// ResponseContentPartDoneEvent sent when content part completes  
type ResponseContentPartDoneEvent struct {
	ResponseID   string                     `json:"response_id"`
	ItemID       string                     `json:"item_id"`
	OutputIndex  int                        `json:"output_index"`
	ContentIndex int                        `json:"content_index"`
	Part         RealtimeConversationContent `json:"part"`
}

// ResponseTextDeltaEvent sent for streaming text
type ResponseTextDeltaEvent struct {
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

// ResponseTextDoneEvent sent when text is complete
type ResponseTextDoneEvent struct {
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Text         string `json:"text"`
}

// ResponseAudioTranscriptDeltaEvent sent for streaming audio transcript
type ResponseAudioTranscriptDeltaEvent struct {
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

// ResponseAudioTranscriptDoneEvent sent when audio transcript is complete
type ResponseAudioTranscriptDoneEvent struct {
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Transcript   string `json:"transcript"`
}

// ResponseAudioDeltaEvent sent for streaming audio
type ResponseAudioDeltaEvent struct {
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"` // base64 encoded audio
}

// ResponseAudioDoneEvent sent when audio is complete
type ResponseAudioDoneEvent struct {
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
}

// ResponseFunctionCallArgumentsDeltaEvent sent for streaming function arguments
type ResponseFunctionCallArgumentsDeltaEvent struct {
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	CallID       string `json:"call_id"`
	Delta        string `json:"delta"`
}

// ResponseFunctionCallArgumentsDoneEvent sent when function arguments are complete
type ResponseFunctionCallArgumentsDoneEvent struct {
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	CallID       string `json:"call_id"`
	Arguments    string `json:"arguments"`
}

// RateLimitsUpdatedEvent sent when rate limits change
type RateLimitsUpdatedEvent struct {
	RateLimits []RateLimit `json:"rate_limits"`
}

// RateLimit represents a rate limit status
type RateLimit struct {
	Name      string `json:"name"`
	Limit     int    `json:"limit"`
	Remaining int    `json:"remaining"`
	ResetSeconds float32 `json:"reset_seconds"`
}

// RealtimeResponseObject represents a response object
type RealtimeResponseObject struct {
	ID                     string                     `json:"id"`
	Object                 string                     `json:"object"`
	Status                 string                     `json:"status"`
	StatusDetails          map[string]interface{}     `json:"status_details,omitempty"`
	Output                 []RealtimeConversationItem `json:"output,omitempty"`
	Usage                  *RealtimeUsage            `json:"usage,omitempty"`
}

// RealtimeUsage represents token usage for realtime
type RealtimeUsage struct {
	TotalTokens            int                        `json:"total_tokens"`
	InputTokens            int                        `json:"input_tokens"`
	OutputTokens           int                        `json:"output_tokens"`
	InputTokenDetails      *RealtimeInputTokenDetails `json:"input_token_details,omitempty"`
	OutputTokenDetails     *RealtimeOutputTokenDetails `json:"output_token_details,omitempty"`
}

// RealtimeInputTokenDetails provides detailed input token breakdown
type RealtimeInputTokenDetails struct {
	CachedTokens int `json:"cached_tokens"`
	TextTokens   int `json:"text_tokens"`
	AudioTokens  int `json:"audio_tokens"`
}

// RealtimeOutputTokenDetails provides detailed output token breakdown
type RealtimeOutputTokenDetails struct {
	TextTokens  int `json:"text_tokens"`
	AudioTokens int `json:"audio_tokens"`
}

// RealtimeConversationItem represents an item in conversation
type RealtimeConversationItem struct {
	ID       string                        `json:"id,omitempty"`
	Object   string                        `json:"object,omitempty"`
	Type     string                        `json:"type"`
	Status   string                        `json:"status,omitempty"`
	Role     string                        `json:"role,omitempty"`
	Content  []RealtimeConversationContent `json:"content,omitempty"`
	CallID   string                        `json:"call_id,omitempty"`
	Name     string                        `json:"name,omitempty"`
	Arguments string                       `json:"arguments,omitempty"`
	Output   string                        `json:"output,omitempty"`
}

// RealtimeConversationContent represents content within a conversation item
type RealtimeConversationContent struct {
	Type       string  `json:"type"`
	Text       string  `json:"text,omitempty"`
	Audio      string  `json:"audio,omitempty"` // base64 encoded
	Transcript string  `json:"transcript,omitempty"`
}

// NewRealtimeSession creates a new realtime session
func NewRealtimeSession(modelName string, userID, teamID, keyID *uint) *RealtimeSession {
	now := time.Now()
	expiresAt := now.Add(30 * time.Minute) // 30 minute default

	return &RealtimeSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		TeamID:    teamID,
		KeyID:     keyID,
		ModelName: modelName,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: &expiresAt,
	}
}

// IsExpired checks if the session has expired
func (rs *RealtimeSession) IsExpired() bool {
	if rs.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*rs.ExpiresAt)
}

// UpdateExpiry extends the session expiry time
func (rs *RealtimeSession) UpdateExpiry(duration time.Duration) {
	newExpiry := time.Now().Add(duration)
	rs.ExpiresAt = &newExpiry
	rs.UpdatedAt = time.Now()
}

// NewRealtimeEvent creates a new realtime event with ID and timestamp
func NewRealtimeEvent(eventType string, data interface{}) (*RealtimeEvent, error) {
	var jsonData json.RawMessage
	var err error
	
	if data != nil {
		jsonData, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}

	return &RealtimeEvent{
		Type:      eventType,
		EventID:   uuid.New().String(),
		Data:      jsonData,
		Timestamp: time.Now(),
	}, nil
}