package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ModelPricingInfo contains pricing information for a model
type ModelPricingInfo struct {
	// Basic pricing
	MaxTokens          int     `json:"max_tokens"`
	MaxInputTokens     int     `json:"max_input_tokens"`
	MaxOutputTokens    int     `json:"max_output_tokens"`
	InputCostPerToken  float64 `json:"input_cost_per_token"`
	OutputCostPerToken float64 `json:"output_cost_per_token"`
	
	// Advanced pricing (following LiteLLM schema)
	OutputCostPerReasoningToken float64 `json:"output_cost_per_reasoning_token,omitempty"`
	InputCostPerTokenBatches    float64 `json:"input_cost_per_token_batches,omitempty"`
	OutputCostPerTokenBatches   float64 `json:"output_cost_per_token_batches,omitempty"`
	
	// Alternative pricing models
	InputCostPerSecond  float64 `json:"input_cost_per_second,omitempty"`  // For time-based billing
	OutputCostPerSecond float64 `json:"output_cost_per_second,omitempty"` // For time-based billing
	
	// Model metadata
	Provider         string   `json:"provider"`
	Mode            string   `json:"mode"` // chat, completion, embedding, etc.
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
	SearchContextCost map[string]float64 `json:"search_context_cost_per_query,omitempty"`
	FileSearchCostPer1kCalls    float64 `json:"file_search_cost_per_1k_calls,omitempty"`
	FileSearchCostPerGbPerDay   float64 `json:"file_search_cost_per_gb_per_day,omitempty"`
	VectorStoreCostPerGbPerDay  float64 `json:"vector_store_cost_per_gb_per_day,omitempty"`
	ComputerUseInputCostPer1k   float64 `json:"computer_use_input_cost_per_1k_tokens,omitempty"`
	ComputerUseOutputCostPer1k  float64 `json:"computer_use_output_cost_per_1k_tokens,omitempty"`
	CodeInterpreterCostPerSession float64 `json:"code_interpreter_cost_per_session,omitempty"`
	
	// Deprecation
	DeprecationDate string `json:"deprecation_date,omitempty"` // YYYY-MM-DD
	
	// Custom metadata
	Source      string                 `json:"source,omitempty"` // "default", "config_override", "database_override"
	LastUpdated time.Time              `json:"last_updated,omitempty"`
	CustomData  map[string]interface{} `json:"custom_data,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling to handle mixed data types
// for max_tokens fields that can be either integers (for real models) or strings (for documentation)
func (m *ModelPricingInfo) UnmarshalJSON(data []byte) error {
	// Create a temporary struct with interface{} types for problematic fields
	type Alias ModelPricingInfo
	aux := &struct {
		MaxTokens       interface{} `json:"max_tokens"`
		MaxInputTokens  interface{} `json:"max_input_tokens"`
		MaxOutputTokens interface{} `json:"max_output_tokens"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Helper function to convert interface{} to int, treating strings as 0
	convertToInt := func(val interface{}) int {
		switch v := val.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case string:
			// If it's a string (documentation), treat as 0
			return 0
		default:
			return 0
		}
	}

	// Convert the interface{} values to int
	m.MaxTokens = convertToInt(aux.MaxTokens)
	m.MaxInputTokens = convertToInt(aux.MaxInputTokens)
	m.MaxOutputTokens = convertToInt(aux.MaxOutputTokens)

	return nil
}

// ModelPricingManager handles model pricing information with override support
type ModelPricingManager struct {
	mu              sync.RWMutex
	defaultPricing  map[string]*ModelPricingInfo // From JSON file
	configOverrides map[string]*ModelPricingInfo // From config.yaml
	dbOverrides     map[string]*ModelPricingInfo // From database
	providerModelMap map[string]string           // Maps user-facing model name → provider model ID
	jsonFilePath    string
	lastJSONLoad    time.Time
	dbRepo          ModelPricingRepository // Database repository interface
}

// ModelPricingRepository interface for database operations
type ModelPricingRepository interface {
	GetEffectivePricing(modelName string, teamID *uint) (*ModelPricingInfo, error)
	CreatePricing(pricing *ModelPricingInfo) error
	UpdatePricing(modelName string, updates *ModelPricingInfo) error
	DeletePricing(modelName string) error
	ListPricing(offset, limit int, teamID *uint) ([]*ModelPricingInfo, error)
}

var (
	pricingManager *ModelPricingManager
	pricingOnce    sync.Once
)

// GetPricingManager returns the singleton pricing manager
func GetPricingManager() *ModelPricingManager {
	pricingOnce.Do(func() {
		pricingManager = &ModelPricingManager{
			defaultPricing:   make(map[string]*ModelPricingInfo),
			configOverrides:  make(map[string]*ModelPricingInfo),
			dbOverrides:      make(map[string]*ModelPricingInfo),
			providerModelMap: make(map[string]string),
		}
	})
	return pricingManager
}

// LoadDefaultPricing loads pricing from the JSON file
func (pm *ModelPricingManager) LoadDefaultPricing(configDir string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Default path
	jsonPath := filepath.Join(configDir, "model_prices_and_context_window.json")
	if configDir == "" {
		// Try current directory, then config directory
		candidates := []string{
			"model_prices_and_context_window.json",
			"internal/config/model_prices_and_context_window.json",
			"config/model_prices_and_context_window.json",
		}
		
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				jsonPath = candidate
				break
			}
		}
	}
	
	pm.jsonFilePath = jsonPath
	
	// Check if file exists
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		return fmt.Errorf("pricing file not found: %s", jsonPath)
	}
	
	// Read and parse JSON
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read pricing file: %w", err)
	}
	
	var pricingData map[string]*ModelPricingInfo
	if err := json.Unmarshal(data, &pricingData); err != nil {
		return fmt.Errorf("failed to parse pricing JSON: %w", err)
	}
	
	// Filter out documentation entries and set source for all default pricing
	for modelName, info := range pricingData {
		if info != nil {
			// Skip documentation entries
			if modelName == "sample_spec" {
				delete(pricingData, modelName)
				continue
			}
			info.Source = "default"
			info.LastUpdated = time.Now()
		}
	}
	
	pm.defaultPricing = pricingData
	pm.lastJSONLoad = time.Now()
	
	return nil
}

// AddConfigOverrides adds pricing overrides from model configuration
func (pm *ModelPricingManager) AddConfigOverrides(modelInstances []ModelInstance) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	for _, instance := range modelInstances {
		// Store mapping from user-facing model name to provider model ID
		if instance.Provider.Model != "" && instance.ModelName != instance.Provider.Model {
			pm.providerModelMap[instance.ModelName] = instance.Provider.Model
		}

		// Check if this instance has custom pricing
		if instance.InputCostPerToken > 0 || instance.OutputCostPerToken > 0 {
			// Create override pricing info
			pricingInfo := &ModelPricingInfo{
				InputCostPerToken:  instance.InputCostPerToken,
				OutputCostPerToken: instance.OutputCostPerToken,
				Source:             "config_override",
				LastUpdated:        time.Now(),
			}
			
			// Copy model info if available
			if instance.ModelInfo.MaxTokens > 0 {
				pricingInfo.MaxTokens = instance.ModelInfo.MaxTokens
			}
			if instance.ModelInfo.MaxInputTokens > 0 {
				pricingInfo.MaxInputTokens = instance.ModelInfo.MaxInputTokens
			}
			if instance.ModelInfo.MaxOutputTokens > 0 {
				pricingInfo.MaxOutputTokens = instance.ModelInfo.MaxOutputTokens
			}
			
			// Set provider type and mode
			pricingInfo.Provider = instance.Provider.Type
			pricingInfo.Mode = instance.ModelInfo.Mode
			
			// Store override using the user-facing model name
			pm.configOverrides[instance.ModelName] = pricingInfo
		}
	}
}

// SetDatabaseRepository sets the database repository for persistent overrides
func (pm *ModelPricingManager) SetDatabaseRepository(repo ModelPricingRepository) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.dbRepo = repo
}

// GetPricing returns pricing information for a model with override precedence
// Priority: database_override > config_override > default
func (pm *ModelPricingManager) GetPricing(modelName string) *ModelPricingInfo {
	return pm.GetPricingForTeam(modelName, nil)
}

// GetPricingForTeam returns pricing information for a model with team-specific overrides
// Priority: database_override (team-specific) > database_override (global) > config_override > default
func (pm *ModelPricingManager) GetPricingForTeam(modelName string, teamID *uint) *ModelPricingInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	// Check database overrides first if repository is available
	if pm.dbRepo != nil {
		if dbInfo, err := pm.dbRepo.GetEffectivePricing(modelName, teamID); err == nil {
			return dbInfo
		}
	}
	
	// Check in-memory database overrides
	if info, exists := pm.dbOverrides[modelName]; exists {
		return info
	}
	
	// Check config overrides second
	if info, exists := pm.configOverrides[modelName]; exists {
		// Merge with default if available to get missing fields
		if defaultInfo, hasDefault := pm.defaultPricing[modelName]; hasDefault {
			return pm.mergeWithDefault(info, defaultInfo)
		}
		return info
	}
	
	// Return default pricing
	if info, exists := pm.defaultPricing[modelName]; exists {
		return info
	}

	// Fallback: try looking up by provider model ID if user-facing name differs
	if providerModel, ok := pm.providerModelMap[modelName]; ok && providerModel != modelName {
		if pm.dbRepo != nil {
			if dbInfo, err := pm.dbRepo.GetEffectivePricing(providerModel, teamID); err == nil {
				return dbInfo
			}
		}
		if info, exists := pm.dbOverrides[providerModel]; exists {
			return info
		}
		if info, exists := pm.configOverrides[providerModel]; exists {
			if defaultInfo, hasDefault := pm.defaultPricing[providerModel]; hasDefault {
				return pm.mergeWithDefault(info, defaultInfo)
			}
			return info
		}
		if info, exists := pm.defaultPricing[providerModel]; exists {
			return info
		}
	}

	// Model not found
	return nil
}

// mergeWithDefault merges override pricing with default to fill missing fields
func (pm *ModelPricingManager) mergeWithDefault(override, defaultInfo *ModelPricingInfo) *ModelPricingInfo {
	merged := *override // Copy override
	
	// Fill in missing fields from default
	if merged.MaxTokens == 0 {
		merged.MaxTokens = defaultInfo.MaxTokens
	}
	if merged.MaxInputTokens == 0 {
		merged.MaxInputTokens = defaultInfo.MaxInputTokens
	}
	if merged.MaxOutputTokens == 0 {
		merged.MaxOutputTokens = defaultInfo.MaxOutputTokens
	}
	if merged.Provider == "" {
		merged.Provider = defaultInfo.Provider
	}
	if merged.Mode == "" {
		merged.Mode = defaultInfo.Mode
	}
	
	// Copy capabilities if not set
	if !merged.SupportsFunctionCalling && defaultInfo.SupportsFunctionCalling {
		merged.SupportsFunctionCalling = defaultInfo.SupportsFunctionCalling
	}
	if !merged.SupportsVision && defaultInfo.SupportsVision {
		merged.SupportsVision = defaultInfo.SupportsVision
	}
	// ... copy other capabilities as needed
	
	return &merged
}

// RegisterModel allows runtime registration of custom pricing (like LiteLLM)
func (pm *ModelPricingManager) RegisterModel(modelName string, pricingInfo *ModelPricingInfo) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pricingInfo.Source = "runtime_override"
	pricingInfo.LastUpdated = time.Now()
	
	// Store in config overrides for runtime registrations
	pm.configOverrides[modelName] = pricingInfo
}

// ListAllModels returns all known models with their pricing sources
func (pm *ModelPricingManager) ListAllModels() map[string]string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	result := make(map[string]string)
	
	// Add default models
	for modelName := range pm.defaultPricing {
		result[modelName] = "default"
	}
	
	// Override with config models
	for modelName := range pm.configOverrides {
		result[modelName] = "config_override"
	}
	
	// Override with database models
	for modelName := range pm.dbOverrides {
		result[modelName] = "database_override"
	}
	
	return result
}

// CalculateCost calculates the cost for a request
func (pm *ModelPricingManager) CalculateCost(modelName string, inputTokens, outputTokens int) (*CostCalculation, error) {
	pricingInfo := pm.GetPricing(modelName)
	if pricingInfo == nil {
		return nil, fmt.Errorf("pricing information not found for model: %s", modelName)
	}
	
	inputCost := float64(inputTokens) * pricingInfo.InputCostPerToken
	outputCost := float64(outputTokens) * pricingInfo.OutputCostPerToken
	totalCost := inputCost + outputCost
	
	return &CostCalculation{
		ModelName:    modelName,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		InputCost:    inputCost,
		OutputCost:   outputCost,
		TotalCost:    totalCost,
		Currency:     "USD",
		Source:       pricingInfo.Source,
		Timestamp:    time.Now(),
	}, nil
}

// CostCalculation represents a cost calculation result
type CostCalculation struct {
	ModelName    string    `json:"model_name"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	InputCost    float64   `json:"input_cost"`
	OutputCost   float64   `json:"output_cost"`
	TotalCost    float64   `json:"total_cost"`
	Currency     string    `json:"currency"`
	Source       string    `json:"source"` // Which pricing source was used
	Timestamp    time.Time `json:"timestamp"`
}

// GetModelInfo returns combined model information for API responses
func (pm *ModelPricingManager) GetModelInfo(modelName string) map[string]interface{} {
	pricingInfo := pm.GetPricing(modelName)
	if pricingInfo == nil {
		return nil
	}
	
	result := map[string]interface{}{
		"model_name":                    modelName,
		"max_tokens":                    pricingInfo.MaxTokens,
		"max_input_tokens":              pricingInfo.MaxInputTokens,
		"max_output_tokens":             pricingInfo.MaxOutputTokens,
		"input_cost_per_token":          pricingInfo.InputCostPerToken,
		"output_cost_per_token":         pricingInfo.OutputCostPerToken,
		"provider":                      pricingInfo.Provider,
		"mode":                          pricingInfo.Mode,
		"supports_streaming":            true, // Default to true for most models
		"source":                        pricingInfo.Source,
		"last_updated":                  pricingInfo.LastUpdated,
		
		// All capabilities
		"capabilities": map[string]interface{}{
			"function_calling":          pricingInfo.SupportsFunctionCalling,
			"parallel_function_calling": pricingInfo.SupportsParallelFunctionCalling,
			"vision":                    pricingInfo.SupportsVision,
			"audio_input":               pricingInfo.SupportsAudioInput,
			"audio_output":              pricingInfo.SupportsAudioOutput,
			"prompt_caching":            pricingInfo.SupportsPromptCaching,
			"response_schema":           pricingInfo.SupportsResponseSchema,
			"system_messages":           pricingInfo.SupportsSystemMessages,
			"reasoning":                 pricingInfo.SupportsReasoning,
			"web_search":                pricingInfo.SupportsWebSearch,
		},
		
		"supported_regions":             pricingInfo.SupportedRegions,
	}
	
	return result
}