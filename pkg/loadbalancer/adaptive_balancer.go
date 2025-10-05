package loadbalancer

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ModelHealth tracks the health and performance of a model
type ModelHealth struct {
	mu sync.RWMutex

	// Identity
	ModelName string

	// Performance metrics
	ResponseTimes   []time.Duration // Sliding window of response times
	AvgResponseTime time.Duration
	P95ResponseTime time.Duration
	P99ResponseTime time.Duration

	// Load metrics
	ActiveRequests  int32
	TotalRequests   int64
	FailedRequests  int64
	TimeoutRequests int64

	// Health score (0-100)
	HealthScore float64

	// Rate limiting
	RequestsPerMin  int32
	TokensPerMin    int32
	LastMinuteReset time.Time

	// Circuit state
	IsCircuitOpen   bool
	LastFailureTime time.Time

	// Configuration
	MaxResponseTime time.Duration
	WindowSize      int
}

// AdaptiveLoadBalancer manages load distribution based on real-time performance
type AdaptiveLoadBalancer struct {
	mu        sync.RWMutex
	models    map[string]*ModelHealth
	fallbacks map[string][]string

	// Configuration
	maxConcurrent int32
	latencyWeight float64 // Weight for latency in scoring (0-1)
	loadWeight    float64 // Weight for current load in scoring (0-1)
	errorWeight   float64 // Weight for error rate in scoring (0-1)

	// Global metrics
	totalRequests int64
	totalFailures int64
}

// NewAdaptiveLoadBalancer creates a new adaptive load balancer
func NewAdaptiveLoadBalancer() *AdaptiveLoadBalancer {
	return &AdaptiveLoadBalancer{
		models:        make(map[string]*ModelHealth),
		fallbacks:     make(map[string][]string),
		maxConcurrent: 1000,
		latencyWeight: 0.4,
		loadWeight:    0.3,
		errorWeight:   0.3,
	}
}

// RegisterModel registers a model with the load balancer
func (alb *AdaptiveLoadBalancer) RegisterModel(modelName string, maxResponseTime time.Duration) {
	alb.mu.Lock()
	defer alb.mu.Unlock()

	if _, exists := alb.models[modelName]; !exists {
		alb.models[modelName] = &ModelHealth{
			ModelName:       modelName,
			ResponseTimes:   make([]time.Duration, 0, 100),
			MaxResponseTime: maxResponseTime,
			WindowSize:      100,
			HealthScore:     100.0,
			LastMinuteReset: time.Now(),
		}
	}
}

// SetFallbacks sets the fallback chain for a model
func (alb *AdaptiveLoadBalancer) SetFallbacks(model string, fallbacks []string) {
	alb.mu.Lock()
	defer alb.mu.Unlock()
	alb.fallbacks[model] = fallbacks
}

// SelectModel selects the best available model considering load and performance
func (alb *AdaptiveLoadBalancer) SelectModel(ctx context.Context, requestedModel string) (string, error) {
	alb.mu.RLock()
	defer alb.mu.RUnlock()

	// Build candidate list (requested model + fallbacks)
	candidates := []string{requestedModel}
	if fallbacks, exists := alb.fallbacks[requestedModel]; exists {
		candidates = append(candidates, fallbacks...)
	}

	// Find the best available model
	var bestModel string
	bestScore := -1.0

	// First try the primary model
	if primaryHealth, exists := alb.models[requestedModel]; exists {
		if !primaryHealth.IsCircuitOpen || time.Since(primaryHealth.LastFailureTime) >= 30*time.Second {
			// Primary is available, check its score
			primaryScore := alb.calculateScore(primaryHealth)
			// Use primary unless it's significantly degraded (< 50% health)
			if primaryScore >= 50 {
				bestModel = requestedModel
				bestScore = primaryScore
			}
		}
	}

	// Only check fallbacks if primary is not good enough
	if bestModel == "" && len(candidates) > 1 {
		for _, modelName := range candidates[1:] { // Skip primary, already checked
			health, exists := alb.models[modelName]
			if !exists {
				continue
			}

			// Skip if circuit is open
			if health.IsCircuitOpen {
				if time.Since(health.LastFailureTime) < 30*time.Second {
					continue
				}
				// Try to close circuit
				health.IsCircuitOpen = false
			}

			// Calculate current score
			score := alb.calculateScore(health)

			if score > bestScore {
				bestScore = score
				bestModel = modelName
			}
		}
	}

	if bestModel == "" {
		return "", fmt.Errorf("no available models for %s", requestedModel)
	}

	// Increment active requests
	if health, exists := alb.models[bestModel]; exists {
		health.mu.Lock()
		health.ActiveRequests++
		health.mu.Unlock()
	}

	return bestModel, nil
}

// RecordRequestStart marks the start of a request
func (alb *AdaptiveLoadBalancer) RecordRequestStart(modelName string) {
	alb.mu.RLock()
	health, exists := alb.models[modelName]
	alb.mu.RUnlock()

	if !exists {
		return
	}

	health.mu.Lock()
	defer health.mu.Unlock()

	health.ActiveRequests++
	health.TotalRequests++
	alb.totalRequests++

	// Reset per-minute counters if needed
	if time.Since(health.LastMinuteReset) > time.Minute {
		health.RequestsPerMin = 0
		health.TokensPerMin = 0
		health.LastMinuteReset = time.Now()
	}
	health.RequestsPerMin++
}

// RecordRequestEnd marks the end of a request
func (alb *AdaptiveLoadBalancer) RecordRequestEnd(modelName string, latency time.Duration, success bool) {
	alb.mu.RLock()
	health, exists := alb.models[modelName]
	alb.mu.RUnlock()

	if !exists {
		return
	}

	health.mu.Lock()
	defer health.mu.Unlock()

	// Decrement active requests
	if health.ActiveRequests > 0 {
		health.ActiveRequests--
	}

	if success {
		// Update latency metrics
		health.addResponseTime(latency)
		health.updateLatencyMetrics()

		// Check if response was slow
		if latency > health.MaxResponseTime {
			// Degrade health score for slow response
			health.HealthScore *= 0.95
		} else {
			// Improve health score for fast response
			health.HealthScore = math.Min(100, health.HealthScore*1.01)
		}
	} else {
		// Record failure
		health.FailedRequests++
		health.LastFailureTime = time.Now()
		alb.totalFailures++

		// Degrade health score
		health.HealthScore *= 0.9

		// Open circuit if too many failures
		failureRate := float64(health.FailedRequests) / float64(health.TotalRequests)
		if failureRate > 0.5 && health.TotalRequests > 10 {
			health.IsCircuitOpen = true
		}
	}

	// Ensure health score stays in bounds
	if health.HealthScore < 0 {
		health.HealthScore = 0
	}
}

// RecordTimeout records a timeout for a model
func (alb *AdaptiveLoadBalancer) RecordTimeout(modelName string) {
	alb.mu.RLock()
	health, exists := alb.models[modelName]
	alb.mu.RUnlock()

	if !exists {
		return
	}

	health.mu.Lock()
	defer health.mu.Unlock()

	health.TimeoutRequests++
	health.FailedRequests++
	health.LastFailureTime = time.Now()

	// Severely degrade health score for timeouts
	health.HealthScore *= 0.5

	// Open circuit immediately on timeout
	health.IsCircuitOpen = true
}

// GetModelStats returns statistics for all models
func (alb *AdaptiveLoadBalancer) GetModelStats() map[string]map[string]interface{} {
	alb.mu.RLock()
	defer alb.mu.RUnlock()

	stats := make(map[string]map[string]interface{})

	for name, health := range alb.models {
		health.mu.RLock()
		stats[name] = map[string]interface{}{
			"health_score":     health.HealthScore,
			"active_requests":  health.ActiveRequests,
			"total_requests":   health.TotalRequests,
			"failed_requests":  health.FailedRequests,
			"timeout_requests": health.TimeoutRequests,
			"avg_latency":      health.AvgResponseTime.String(),
			"p95_latency":      health.P95ResponseTime.String(),
			"p99_latency":      health.P99ResponseTime.String(),
			"circuit_open":     health.IsCircuitOpen,
			"requests_per_min": health.RequestsPerMin,
		}
		health.mu.RUnlock()
	}

	return stats
}

// Private methods

func (alb *AdaptiveLoadBalancer) calculateScore(health *ModelHealth) float64 {
	health.mu.RLock()
	defer health.mu.RUnlock()

	// Base score from health
	score := health.HealthScore

	// Penalize based on current load (0-1, where 0 is no load)
	loadFactor := 1.0
	if health.ActiveRequests > 0 {
		loadFactor = 1.0 / (1.0 + float64(health.ActiveRequests)/10.0)
	}

	// Penalize based on latency
	latencyFactor := 1.0
	if health.AvgResponseTime > 0 {
		// Normalize latency (assume 10s is terrible, 100ms is excellent)
		normalizedLatency := float64(health.AvgResponseTime) / float64(10*time.Second)
		latencyFactor = 1.0 - math.Min(1.0, normalizedLatency)
	}

	// Penalize based on error rate
	errorFactor := 1.0
	if health.TotalRequests > 0 {
		errorRate := float64(health.FailedRequests) / float64(health.TotalRequests)
		errorFactor = 1.0 - errorRate
	}

	// Weighted combination
	finalScore := score * (alb.loadWeight*loadFactor +
		alb.latencyWeight*latencyFactor +
		alb.errorWeight*errorFactor)

	return finalScore
}

func (health *ModelHealth) addResponseTime(latency time.Duration) {
	if len(health.ResponseTimes) >= health.WindowSize {
		health.ResponseTimes = health.ResponseTimes[1:]
	}
	health.ResponseTimes = append(health.ResponseTimes, latency)
}

func (health *ModelHealth) updateLatencyMetrics() {
	if len(health.ResponseTimes) == 0 {
		return
	}

	// Calculate average
	var total time.Duration
	for _, rt := range health.ResponseTimes {
		total += rt
	}
	health.AvgResponseTime = total / time.Duration(len(health.ResponseTimes))

	// Calculate percentiles
	sorted := make([]time.Duration, len(health.ResponseTimes))
	copy(sorted, health.ResponseTimes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	p95Index := int(float64(len(sorted)) * 0.95)
	p99Index := int(float64(len(sorted)) * 0.99)

	if p95Index < len(sorted) {
		health.P95ResponseTime = sorted[p95Index]
	}
	if p99Index < len(sorted) {
		health.P99ResponseTime = sorted[p99Index]
	}
}

// ShouldShedLoad returns true if the system should start shedding load
func (alb *AdaptiveLoadBalancer) ShouldShedLoad() bool {
	alb.mu.RLock()
	defer alb.mu.RUnlock()

	// Count total active requests
	var totalActive int32
	var healthyModels int

	for _, health := range alb.models {
		health.mu.RLock()
		totalActive += health.ActiveRequests
		if health.HealthScore > 50 && !health.IsCircuitOpen {
			healthyModels++
		}
		health.mu.RUnlock()
	}

	// Shed load if:
	// 1. Too many concurrent requests
	// 2. Too few healthy models
	// 3. High global failure rate
	return totalActive > alb.maxConcurrent ||
		healthyModels < 2 ||
		(alb.totalFailures > 100 && float64(alb.totalFailures)/float64(alb.totalRequests) > 0.1)
}
