package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/core/config"
)

// PricingCache provides Redis-based caching for model pricing information
type PricingCache struct {
	client         *redis.Client
	logger         *zap.Logger
	pricingManager *config.ModelPricingManager
	cachePrefix    string
	cacheTTL       time.Duration
}

// NewPricingCache creates a new pricing cache instance
func NewPricingCache(client *redis.Client, logger *zap.Logger, pricingManager *config.ModelPricingManager) *PricingCache {
	return &PricingCache{
		client:         client,
		logger:         logger,
		pricingManager: pricingManager,
		cachePrefix:    "pllm:pricing:",
		cacheTTL:       24 * time.Hour, // Cache for 24 hours
	}
}

// LoadAllPricingToCache loads all pricing data from the pricing manager into Redis cache
func (pc *PricingCache) LoadAllPricingToCache(ctx context.Context) error {
	pc.logger.Info("Loading all pricing data to Redis cache...")

	// Get all known models from the pricing manager
	allModels := pc.pricingManager.ListAllModels()
	if len(allModels) == 0 {
		pc.logger.Warn("No models found in pricing manager")
		return nil
	}

	// Use pipeline for batch operations
	pipe := pc.client.Pipeline()
	cachedCount := 0

	for modelName := range allModels {
		pricingInfo := pc.pricingManager.GetPricing(modelName)
		if pricingInfo == nil {
			pc.logger.Warn("No pricing info found for model", zap.String("model", modelName))
			continue
		}

		// Serialize pricing info to JSON
		data, err := json.Marshal(pricingInfo)
		if err != nil {
			pc.logger.Error("Failed to marshal pricing info", 
				zap.String("model", modelName), 
				zap.Error(err))
			continue
		}

		// Add to pipeline
		key := pc.cachePrefix + modelName
		pipe.Set(ctx, key, data, pc.cacheTTL)
		cachedCount++
	}

	// Execute all operations
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to execute pricing cache pipeline: %w", err)
	}

	pc.logger.Info("Successfully loaded pricing data to Redis cache", 
		zap.Int("models_cached", cachedCount))

	// Set a marker to indicate pricing cache is loaded
	pc.client.Set(ctx, pc.cachePrefix+"_loaded", "true", pc.cacheTTL)

	return nil
}

// GetPricing retrieves pricing information from cache, falling back to pricing manager if not found
func (pc *PricingCache) GetPricing(ctx context.Context, modelName string) *config.ModelPricingInfo {
	// Try to get from cache first
	key := pc.cachePrefix + modelName
	data, err := pc.client.Get(ctx, key).Result()
	
	if err == nil {
		// Found in cache, deserialize
		var pricingInfo config.ModelPricingInfo
		if err := json.Unmarshal([]byte(data), &pricingInfo); err == nil {
			return &pricingInfo
		} else {
			pc.logger.Error("Failed to unmarshal cached pricing info", 
				zap.String("model", modelName), 
				zap.Error(err))
		}
	} else if err != redis.Nil {
		// Redis error (not just key not found)
		pc.logger.Error("Redis error while getting pricing", 
			zap.String("model", modelName), 
			zap.Error(err))
	}

	// Cache miss or error - fallback to pricing manager
	pricingInfo := pc.pricingManager.GetPricing(modelName)
	if pricingInfo != nil {
		// Cache the result for future use
		go pc.cachePricingAsync(modelName, pricingInfo)
	}

	return pricingInfo
}

// GetPricingForTeam retrieves team-specific pricing, with cache fallback
func (pc *PricingCache) GetPricingForTeam(ctx context.Context, modelName string, teamID *uint) *config.ModelPricingInfo {
	// For team-specific pricing, we don't cache since it's more complex
	// and team-specific overrides are typically stored in database
	return pc.pricingManager.GetPricingForTeam(modelName, teamID)
}

// CalculateCost calculates cost using cached pricing data
func (pc *PricingCache) CalculateCost(ctx context.Context, modelName string, inputTokens, outputTokens int) (*config.CostCalculation, error) {
	pricingInfo := pc.GetPricing(ctx, modelName)
	if pricingInfo == nil {
		return nil, fmt.Errorf("pricing information not found for model: %s", modelName)
	}

	inputCost := float64(inputTokens) * pricingInfo.InputCostPerToken
	outputCost := float64(outputTokens) * pricingInfo.OutputCostPerToken
	totalCost := inputCost + outputCost

	return &config.CostCalculation{
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

// cachePricingAsync caches pricing info asynchronously (fire and forget)
func (pc *PricingCache) cachePricingAsync(modelName string, pricingInfo *config.ModelPricingInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := json.Marshal(pricingInfo)
	if err != nil {
		pc.logger.Error("Failed to marshal pricing info for caching", 
			zap.String("model", modelName), 
			zap.Error(err))
		return
	}

	key := pc.cachePrefix + modelName
	if err := pc.client.Set(ctx, key, data, pc.cacheTTL).Err(); err != nil {
		pc.logger.Error("Failed to cache pricing info", 
			zap.String("model", modelName), 
			zap.Error(err))
	}
}

// RefreshCache reloads all pricing data to cache
func (pc *PricingCache) RefreshCache(ctx context.Context) error {
	pc.logger.Info("Refreshing pricing cache...")

	// Clear existing cache
	pattern := pc.cachePrefix + "*"
	keys, err := pc.client.Keys(ctx, pattern).Result()
	if err != nil {
		pc.logger.Error("Failed to get cache keys for clearing", zap.Error(err))
	} else if len(keys) > 0 {
		if err := pc.client.Del(ctx, keys...).Err(); err != nil {
			pc.logger.Error("Failed to clear existing pricing cache", zap.Error(err))
		}
	}

	// Reload pricing data from files/config
	if err := pc.pricingManager.LoadDefaultPricing("internal/config"); err != nil {
		pc.logger.Error("Failed to reload pricing data from files", zap.Error(err))
		// Continue anyway - might have config overrides
	}

	// Load all to cache
	return pc.LoadAllPricingToCache(ctx)
}

// IsCacheLoaded checks if the pricing cache has been initialized
func (pc *PricingCache) IsCacheLoaded(ctx context.Context) bool {
	result, err := pc.client.Get(ctx, pc.cachePrefix+"_loaded").Result()
	return err == nil && result == "true"
}

// GetModelInfo returns combined model information for API responses (cached)
func (pc *PricingCache) GetModelInfo(ctx context.Context, modelName string) map[string]interface{} {
	pricingInfo := pc.GetPricing(ctx, modelName)
	if pricingInfo == nil {
		return nil
	}

	return map[string]interface{}{
		"model_name":                    modelName,
		"max_tokens":                    pricingInfo.MaxTokens,
		"max_input_tokens":              pricingInfo.MaxInputTokens,
		"max_output_tokens":             pricingInfo.MaxOutputTokens,
		"input_cost_per_token":          pricingInfo.InputCostPerToken,
		"output_cost_per_token":         pricingInfo.OutputCostPerToken,
		"provider":                      pricingInfo.Provider,
		"mode":                          pricingInfo.Mode,
		"supports_function_calling":     pricingInfo.SupportsFunctionCalling,
		"supports_vision":               pricingInfo.SupportsVision,
		"supports_streaming":            true, // Default to true for most models
		"source":                        pricingInfo.Source,
		"last_updated":                  pricingInfo.LastUpdated,
	}
}