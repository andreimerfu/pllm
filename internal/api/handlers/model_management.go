package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/amerfu/pllm/internal/core/config"
)

// ModelManagementHandler handles model pricing and configuration endpoints
type ModelManagementHandler struct {
	pricingManager *config.ModelPricingManager
}

// NewModelManagementHandler creates a new model management handler
func NewModelManagementHandler(pricingManager *config.ModelPricingManager) *ModelManagementHandler {
	return &ModelManagementHandler{
		pricingManager: pricingManager,
	}
}

// GetModelInfo returns detailed model information (like LiteLLM's /model/info)
// GET /v1/model/info
func (h *ModelManagementHandler) GetModelInfo(w http.ResponseWriter, r *http.Request) {
	modelName := r.URL.Query().Get("model")
	if modelName == "" {
		// Return all models
		allModels := h.pricingManager.ListAllModels()
		response := make(map[string]interface{})
		
		for model := range allModels {
			if info := h.pricingManager.GetModelInfo(model); info != nil {
				response[model] = info
			}
		}
		
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}
	
	// Return specific model info
	modelInfo := h.pricingManager.GetModelInfo(modelName)
	if modelInfo == nil {
		http.Error(w, "Model not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(modelInfo); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// RegisterModelRequest represents a request to register a new model
type RegisterModelRequest struct {
	ModelName string                        `json:"model_name" validate:"required"`
	Pricing   *config.ModelPricingInfo      `json:"pricing" validate:"required"`
	ModelInfo map[string]interface{}        `json:"model_info,omitempty"`
}

// RegisterModel allows runtime registration of new models (like LiteLLM's /model/new)
// POST /v1/model/register
func (h *ModelManagementHandler) RegisterModel(w http.ResponseWriter, r *http.Request) {
	var req RegisterModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	// Validate required fields
	if req.ModelName == "" {
		http.Error(w, "model_name is required", http.StatusBadRequest)
		return
	}
	
	if req.Pricing == nil {
		http.Error(w, "pricing information is required", http.StatusBadRequest)
		return
	}
	
	// Validate pricing has at least input cost
	if req.Pricing.InputCostPerToken <= 0 {
		http.Error(w, "input_cost_per_token must be greater than 0", http.StatusBadRequest)
		return
	}
	
	// Register the model
	h.pricingManager.RegisterModel(req.ModelName, req.Pricing)
	
	// Return success response
	response := map[string]interface{}{
		"message":    "Model registered successfully",
		"model_name": req.ModelName,
		"source":     "runtime_override",
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// CalculateCostRequest represents a request to calculate cost
type CalculateCostRequest struct {
	ModelName    string `json:"model_name" validate:"required"`
	InputTokens  int    `json:"input_tokens" validate:"min=0"`
	OutputTokens int    `json:"output_tokens" validate:"min=0"`
}

// CalculateCost calculates the cost for a hypothetical request
// POST /v1/model/calculate-cost
func (h *ModelManagementHandler) CalculateCost(w http.ResponseWriter, r *http.Request) {
	var req CalculateCostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	// Validate
	if req.ModelName == "" {
		http.Error(w, "model_name is required", http.StatusBadRequest)
		return
	}
	
	if req.InputTokens < 0 || req.OutputTokens < 0 {
		http.Error(w, "token counts must be non-negative", http.StatusBadRequest)
		return
	}
	
	// Calculate cost
	cost, err := h.pricingManager.CalculateCost(req.ModelName, req.InputTokens, req.OutputTokens)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cost); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// GetModelCost returns cost per token for a specific model
// GET /v1/model/{model_name}/cost
func (h *ModelManagementHandler) GetModelCost(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model_name")
	
	pricing := h.pricingManager.GetPricing(modelName)
	if pricing == nil {
		http.Error(w, "Model not found", http.StatusNotFound)
		return
	}
	
	response := map[string]interface{}{
		"model_name":           modelName,
		"input_cost_per_token": pricing.InputCostPerToken,
		"output_cost_per_token": pricing.OutputCostPerToken,
		"currency":             "USD",
		"source":               pricing.Source,
		"last_updated":         pricing.LastUpdated,
	}
	
	// Add additional cost fields if present
	if pricing.OutputCostPerReasoningToken > 0 {
		response["output_cost_per_reasoning_token"] = pricing.OutputCostPerReasoningToken
	}
	if pricing.InputCostPerSecond > 0 {
		response["input_cost_per_second"] = pricing.InputCostPerSecond
	}
	if pricing.OutputCostPerSecond > 0 {
		response["output_cost_per_second"] = pricing.OutputCostPerSecond
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// UpdateModelPricingRequest represents a request to update model pricing
type UpdateModelPricingRequest struct {
	InputCostPerToken  *float64 `json:"input_cost_per_token,omitempty"`
	OutputCostPerToken *float64 `json:"output_cost_per_token,omitempty"`
	MaxTokens          *int     `json:"max_tokens,omitempty"`
	MaxInputTokens     *int     `json:"max_input_tokens,omitempty"`
	MaxOutputTokens    *int     `json:"max_output_tokens,omitempty"`
}

// UpdateModelPricing updates pricing for an existing model
// PATCH /v1/model/{model_name}/pricing
func (h *ModelManagementHandler) UpdateModelPricing(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model_name")
	
	var req UpdateModelPricingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	// Get existing pricing
	existing := h.pricingManager.GetPricing(modelName)
	if existing == nil {
		http.Error(w, "Model not found", http.StatusNotFound)
		return
	}
	
	// Create updated pricing info
	updated := *existing // Copy existing
	
	// Update only provided fields
	if req.InputCostPerToken != nil {
		updated.InputCostPerToken = *req.InputCostPerToken
	}
	if req.OutputCostPerToken != nil {
		updated.OutputCostPerToken = *req.OutputCostPerToken
	}
	if req.MaxTokens != nil {
		updated.MaxTokens = *req.MaxTokens
	}
	if req.MaxInputTokens != nil {
		updated.MaxInputTokens = *req.MaxInputTokens
	}
	if req.MaxOutputTokens != nil {
		updated.MaxOutputTokens = *req.MaxOutputTokens
	}
	
	// Register as override
	h.pricingManager.RegisterModel(modelName, &updated)
	
	response := map[string]interface{}{
		"message":    "Model pricing updated successfully",
		"model_name": modelName,
		"pricing":    h.pricingManager.GetModelInfo(modelName),
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// ListModels returns a list of all available models with their sources
// GET /v1/models
func (h *ModelManagementHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	source := r.URL.Query().Get("source")      // Filter by source
	includeInfo := r.URL.Query().Get("info")   // Include full info or just names
	
	allModels := h.pricingManager.ListAllModels()
	
	response := make(map[string]interface{})
	
	for modelName, modelSource := range allModels {
		// Filter by source if specified
		if source != "" && modelSource != source {
			continue
		}
		
		if includeInfo == "true" {
			// Include full model info
			if info := h.pricingManager.GetModelInfo(modelName); info != nil {
				response[modelName] = info
			}
		} else {
			// Just include source
			response[modelName] = map[string]interface{}{
				"source": modelSource,
			}
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"models": response,
		"total":  len(response),
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}