package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// ModelPricing represents custom pricing overrides stored in database
type ModelPricing struct {
	BaseModel
	
	// Model identification
	ModelName string `gorm:"uniqueIndex;not null" json:"model_name"`
	
	// Basic pricing
	InputCostPerToken  float64 `gorm:"type:decimal(20,10)" json:"input_cost_per_token"`
	OutputCostPerToken float64 `gorm:"type:decimal(20,10)" json:"output_cost_per_token"`
	
	// Advanced pricing
	OutputCostPerReasoningToken *float64 `gorm:"type:decimal(20,10)" json:"output_cost_per_reasoning_token,omitempty"`
	InputCostPerTokenBatches    *float64 `gorm:"type:decimal(20,10)" json:"input_cost_per_token_batches,omitempty"`
	OutputCostPerTokenBatches   *float64 `gorm:"type:decimal(20,10)" json:"output_cost_per_token_batches,omitempty"`
	
	// Time-based pricing
	InputCostPerSecond  *float64 `gorm:"type:decimal(20,10)" json:"input_cost_per_second,omitempty"`
	OutputCostPerSecond *float64 `gorm:"type:decimal(20,10)" json:"output_cost_per_second,omitempty"`
	
	// Model constraints
	MaxTokens       *int `json:"max_tokens,omitempty"`
	MaxInputTokens  *int `json:"max_input_tokens,omitempty"`
	MaxOutputTokens *int `json:"max_output_tokens,omitempty"`
	
	// Provider information
	Provider string `gorm:"index" json:"provider,omitempty"`
	Mode     string `json:"mode,omitempty"` // chat, completion, embedding, etc.
	
	// Capabilities (stored as JSON)
	Capabilities ModelCapabilities `gorm:"type:json" json:"capabilities,omitempty"`
	
	// Advanced features with costs (stored as JSON)
	AdvancedPricing AdvancedPricingData `gorm:"type:json" json:"advanced_pricing,omitempty"`
	
	// Metadata
	Source        string           `gorm:"default:'database_override'" json:"source"`
	CreatedByUser uint             `json:"created_by_user,omitempty"`
	TeamID        *uint            `gorm:"index" json:"team_id,omitempty"` // Team-specific pricing
	CustomData    CustomMetadata   `gorm:"type:json" json:"custom_data,omitempty"`
	
	// Lifecycle
	IsActive        bool       `gorm:"default:true" json:"is_active"`
	EffectiveFrom   *time.Time `json:"effective_from,omitempty"`
	EffectiveUntil  *time.Time `json:"effective_until,omitempty"`
	DeprecationDate *time.Time `json:"deprecation_date,omitempty"`
	
	// Relationships
	CreatedBy User  `gorm:"foreignKey:CreatedByUser" json:"created_by,omitempty"`
	Team      *Team `gorm:"foreignKey:TeamID" json:"team,omitempty"`
}

// ModelCapabilities stores model capability flags as JSON
type ModelCapabilities struct {
	SupportsFunctionCalling        bool     `json:"supports_function_calling,omitempty"`
	SupportsParallelFunctionCalling bool     `json:"supports_parallel_function_calling,omitempty"`
	SupportsVision                  bool     `json:"supports_vision,omitempty"`
	SupportsAudioInput              bool     `json:"supports_audio_input,omitempty"`
	SupportsAudioOutput             bool     `json:"supports_audio_output,omitempty"`
	SupportsPromptCaching           bool     `json:"supports_prompt_caching,omitempty"`
	SupportsResponseSchema          bool     `json:"supports_response_schema,omitempty"`
	SupportsSystemMessages          bool     `json:"supports_system_messages,omitempty"`
	SupportsReasoning               bool     `json:"supports_reasoning,omitempty"`
	SupportsWebSearch               bool     `json:"supports_web_search,omitempty"`
	SupportedRegions                []string `json:"supported_regions,omitempty"`
	SupportedLanguages              []string `json:"supported_languages,omitempty"`
}

// AdvancedPricingData stores advanced pricing features as JSON
type AdvancedPricingData struct {
	SearchContextCost             map[string]float64 `json:"search_context_cost_per_query,omitempty"`
	FileSearchCostPer1kCalls      float64            `json:"file_search_cost_per_1k_calls,omitempty"`
	FileSearchCostPerGbPerDay     float64            `json:"file_search_cost_per_gb_per_day,omitempty"`
	VectorStoreCostPerGbPerDay    float64            `json:"vector_store_cost_per_gb_per_day,omitempty"`
	ComputerUseInputCostPer1k     float64            `json:"computer_use_input_cost_per_1k_tokens,omitempty"`
	ComputerUseOutputCostPer1k    float64            `json:"computer_use_output_cost_per_1k_tokens,omitempty"`
	CodeInterpreterCostPerSession float64            `json:"code_interpreter_cost_per_session,omitempty"`
}

// CustomMetadata stores custom metadata as JSON
type CustomMetadata map[string]interface{}

// Implement Scanner and Valuer interfaces for GORM JSON handling
func (c *ModelCapabilities) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	
	return json.Unmarshal(bytes, c)
}

func (c ModelCapabilities) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (a *AdvancedPricingData) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	
	return json.Unmarshal(bytes, a)
}

func (a AdvancedPricingData) Value() (driver.Value, error) {
	return json.Marshal(a)
}

func (c *CustomMetadata) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	
	return json.Unmarshal(bytes, c)
}

func (c CustomMetadata) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// TableName returns the table name for ModelPricing
func (ModelPricing) TableName() string {
	return "model_pricing_overrides"
}

// IsEffective checks if the pricing is currently effective
func (mp *ModelPricing) IsEffective() bool {
	now := time.Now()
	
	if !mp.IsActive {
		return false
	}
	
	if mp.EffectiveFrom != nil && now.Before(*mp.EffectiveFrom) {
		return false
	}
	
	if mp.EffectiveUntil != nil && now.After(*mp.EffectiveUntil) {
		return false
	}
	
	if mp.DeprecationDate != nil && now.After(*mp.DeprecationDate) {
		return false
	}
	
	return true
}

// ToConfigPricing converts database model to config pricing format
func (mp *ModelPricing) ToConfigPricing() *ModelPricingInfo {
	info := &ModelPricingInfo{
		InputCostPerToken:  mp.InputCostPerToken,
		OutputCostPerToken: mp.OutputCostPerToken,
		Provider:           mp.Provider,
		Mode:               mp.Mode,
		Source:             mp.Source,
		LastUpdated:        mp.UpdatedAt,
	}
	
	// Copy optional fields
	if mp.MaxTokens != nil {
		info.MaxTokens = *mp.MaxTokens
	}
	if mp.MaxInputTokens != nil {
		info.MaxInputTokens = *mp.MaxInputTokens
	}
	if mp.MaxOutputTokens != nil {
		info.MaxOutputTokens = *mp.MaxOutputTokens
	}
	
	// Copy advanced pricing
	if mp.OutputCostPerReasoningToken != nil {
		info.OutputCostPerReasoningToken = *mp.OutputCostPerReasoningToken
	}
	if mp.InputCostPerSecond != nil {
		info.InputCostPerSecond = *mp.InputCostPerSecond
	}
	if mp.OutputCostPerSecond != nil {
		info.OutputCostPerSecond = *mp.OutputCostPerSecond
	}
	
	// Copy capabilities
	info.SupportsFunctionCalling = mp.Capabilities.SupportsFunctionCalling
	info.SupportsVision = mp.Capabilities.SupportsVision
	info.SupportsPromptCaching = mp.Capabilities.SupportsPromptCaching
	info.SupportsSystemMessages = mp.Capabilities.SupportsSystemMessages
	info.SupportsReasoning = mp.Capabilities.SupportsReasoning
	info.SupportsWebSearch = mp.Capabilities.SupportsWebSearch
	info.SupportedRegions = mp.Capabilities.SupportedRegions
	
	// Copy advanced pricing
	if len(mp.AdvancedPricing.SearchContextCost) > 0 {
		info.SearchContextCost = mp.AdvancedPricing.SearchContextCost
	}
	info.FileSearchCostPer1kCalls = mp.AdvancedPricing.FileSearchCostPer1kCalls
	info.FileSearchCostPerGbPerDay = mp.AdvancedPricing.FileSearchCostPerGbPerDay
	info.VectorStoreCostPerGbPerDay = mp.AdvancedPricing.VectorStoreCostPerGbPerDay
	info.ComputerUseInputCostPer1k = mp.AdvancedPricing.ComputerUseInputCostPer1k
	info.ComputerUseOutputCostPer1k = mp.AdvancedPricing.ComputerUseOutputCostPer1k
	info.CodeInterpreterCostPerSession = mp.AdvancedPricing.CodeInterpreterCostPerSession
	
	// Copy deprecation date
	if mp.DeprecationDate != nil {
		info.DeprecationDate = mp.DeprecationDate.Format("2006-01-02")
	}
	
	// Copy custom data
	if mp.CustomData != nil {
		info.CustomData = map[string]interface{}(mp.CustomData)
	}
	
	return info
}

// ModelPricingInfo represents pricing information (duplicated to avoid circular imports)
// This mirrors the struct in config package
type ModelPricingInfo struct {
	MaxTokens          int     `json:"max_tokens"`
	MaxInputTokens     int     `json:"max_input_tokens"`
	MaxOutputTokens    int     `json:"max_output_tokens"`
	InputCostPerToken  float64 `json:"input_cost_per_token"`
	OutputCostPerToken float64 `json:"output_cost_per_token"`
	
	// Advanced pricing
	OutputCostPerReasoningToken float64 `json:"output_cost_per_reasoning_token,omitempty"`
	InputCostPerTokenBatches    float64 `json:"input_cost_per_token_batches,omitempty"`
	OutputCostPerTokenBatches   float64 `json:"output_cost_per_token_batches,omitempty"`
	InputCostPerSecond          float64 `json:"input_cost_per_second,omitempty"`
	OutputCostPerSecond         float64 `json:"output_cost_per_second,omitempty"`
	
	// Model metadata
	Provider         string   `json:"provider"`
	Mode             string   `json:"mode"`
	SupportedRegions []string `json:"supported_regions,omitempty"`
	
	// Capabilities
	SupportsFunctionCalling        bool `json:"supports_function_calling,omitempty"`
	SupportsParallelFunctionCalling bool `json:"supports_parallel_function_calling,omitempty"`
	SupportsVision                  bool `json:"supports_vision,omitempty"`
	SupportsAudioInput              bool `json:"supports_audio_input,omitempty"`
	SupportsAudioOutput             bool `json:"supports_audio_output,omitempty"`
	SupportsPromptCaching           bool `json:"supports_prompt_caching,omitempty"`
	SupportsResponseSchema          bool `json:"supports_response_schema,omitempty"`
	SupportsSystemMessages          bool `json:"supports_system_messages,omitempty"`
	SupportsReasoning               bool `json:"supports_reasoning,omitempty"`
	SupportsWebSearch               bool `json:"supports_web_search,omitempty"`
	
	// Advanced features with costs
	SearchContextCost               map[string]float64 `json:"search_context_cost_per_query,omitempty"`
	FileSearchCostPer1kCalls        float64            `json:"file_search_cost_per_1k_calls,omitempty"`
	FileSearchCostPerGbPerDay       float64            `json:"file_search_cost_per_gb_per_day,omitempty"`
	VectorStoreCostPerGbPerDay      float64            `json:"vector_store_cost_per_gb_per_day,omitempty"`
	ComputerUseInputCostPer1k       float64            `json:"computer_use_input_cost_per_1k_tokens,omitempty"`
	ComputerUseOutputCostPer1k      float64            `json:"computer_use_output_cost_per_1k_tokens,omitempty"`
	CodeInterpreterCostPerSession   float64            `json:"code_interpreter_cost_per_session,omitempty"`
	
	// Metadata
	DeprecationDate string                 `json:"deprecation_date,omitempty"`
	Source          string                 `json:"source,omitempty"`
	LastUpdated     time.Time              `json:"last_updated,omitempty"`
	CustomData      map[string]interface{} `json:"custom_data,omitempty"`
}

// ModelPricingRepository provides database operations for model pricing
type ModelPricingRepository struct {
	db GormDB
}

// NewModelPricingRepository creates a new repository
func NewModelPricingRepository(db GormDB) *ModelPricingRepository {
	return &ModelPricingRepository{db: db}
}

// GetEffectivePricing returns the effective pricing for a model
func (r *ModelPricingRepository) GetEffectivePricing(modelName string, teamID *uint) (*ModelPricing, error) {
	var pricing ModelPricing
	
	query := r.db.Where("model_name = ? AND is_active = true", modelName)
	
	// Add team filter if provided (team-specific overrides)
	if teamID != nil {
		query = query.Where("(team_id IS NULL OR team_id = ?)", *teamID).Order("team_id DESC NULLS LAST")
	} else {
		query = query.Where("team_id IS NULL")
	}
	
	// Order by effective dates
	query = query.Order("effective_from DESC NULLS LAST, created_at DESC")
	
	if err := query.First(&pricing).Error; err != nil {
		return nil, err
	}
	
	// Check if effective
	if !pricing.IsEffective() {
		return nil, errors.New("no effective pricing found")
	}
	
	return &pricing, nil
}

// CreatePricing creates a new pricing override
func (r *ModelPricingRepository) CreatePricing(pricing *ModelPricing) error {
	return r.db.Create(pricing).Error
}

// UpdatePricing updates existing pricing
func (r *ModelPricingRepository) UpdatePricing(id uint, updates map[string]interface{}) error {
	return r.db.Model(&ModelPricing{}).Where("id = ?", id).Updates(updates).Error
}

// DeletePricing soft deletes pricing (sets is_active = false)
func (r *ModelPricingRepository) DeletePricing(id uint) error {
	return r.db.Model(&ModelPricing{}).Where("id = ?", id).Update("is_active", false).Error
}

// ListPricing returns all pricing records with pagination
func (r *ModelPricingRepository) ListPricing(offset, limit int, teamID *uint) ([]ModelPricing, error) {
	var pricings []ModelPricing
	
	query := r.db.Offset(offset).Limit(limit).Order("created_at DESC")
	
	if teamID != nil {
		query = query.Where("team_id IS NULL OR team_id = ?", *teamID)
	}
	
	err := query.Find(&pricings).Error
	return pricings, err
}