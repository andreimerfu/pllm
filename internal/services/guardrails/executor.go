package guardrails

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/services/providers"
)

// Executor manages and executes guardrails
type Executor struct {
	mu              sync.RWMutex
	config          *config.GuardrailsConfig
	logger          *zap.Logger
	
	// Organized by execution mode for efficiency
	preCallRails    []Guardrail
	postCallRails   []Guardrail
	duringCallRails []Guardrail
	loggingRails    []Guardrail
	
	// All guardrails by name for lookups
	guardrails map[string]Guardrail
	
	// Statistics
	stats map[string]*GuardrailStats
}

// NewExecutor creates a new guardrails executor
func NewExecutor(config *config.GuardrailsConfig, logger *zap.Logger) *Executor {
	return &Executor{
		config:     config,
		logger:     logger.Named("guardrails"),
		guardrails: make(map[string]Guardrail),
		stats:      make(map[string]*GuardrailStats),
	}
}

// RegisterGuardrail adds a guardrail to the executor
func (e *Executor) RegisterGuardrail(guardrail Guardrail) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	name := guardrail.GetName()
	if _, exists := e.guardrails[name]; exists {
		return fmt.Errorf("guardrail %s already registered", name)
	}
	
	e.guardrails[name] = guardrail
	
	// Add to appropriate execution mode lists
	mode := guardrail.GetMode()
	switch mode {
	case PreCall:
		e.preCallRails = append(e.preCallRails, guardrail)
	case PostCall:
		e.postCallRails = append(e.postCallRails, guardrail)
	case DuringCall:
		e.duringCallRails = append(e.duringCallRails, guardrail)
	case LoggingOnly:
		e.loggingRails = append(e.loggingRails, guardrail)
	}
	
	// Initialize stats
	e.stats[name] = &GuardrailStats{
		LastExecuted: time.Now(),
	}
	
	e.logger.Info("Registered guardrail",
		zap.String("name", name),
		zap.String("type", guardrail.GetType().String()),
		zap.String("mode", mode.String()))
	
	return nil
}

// ExecutePreCall runs all pre-call guardrails
func (e *Executor) ExecutePreCall(ctx context.Context, request *providers.ChatRequest, userID, teamID, keyID string) error {
	if !e.config.Enabled {
		return nil
	}
	
	e.mu.RLock()
	rails := make([]Guardrail, len(e.preCallRails))
	copy(rails, e.preCallRails)
	e.mu.RUnlock()
	
	if len(rails) == 0 {
		return nil
	}
	
	input := &GuardrailInput{
		Request:   request,
		UserID:    userID,
		TeamID:    teamID,
		KeyID:     keyID,
		Timestamp: time.Now(),
		RequestID: fmt.Sprintf("req_%d", time.Now().UnixNano()),
	}
	
	err := e.executeGuardrails(ctx, rails, input)
	
	// Apply any modifications back to the original request
	if input.Request != request {
		*request = *input.Request.(*providers.ChatRequest)
	}
	
	return err
}

// ExecutePostCall runs all post-call guardrails
func (e *Executor) ExecutePostCall(ctx context.Context, request *providers.ChatRequest, response *providers.ChatResponse, userID, teamID, keyID string) error {
	if !e.config.Enabled {
		return nil
	}
	
	e.mu.RLock()
	rails := make([]Guardrail, len(e.postCallRails))
	copy(rails, e.postCallRails)
	e.mu.RUnlock()
	
	if len(rails) == 0 {
		return nil
	}
	
	input := &GuardrailInput{
		Request:   request,
		Response:  response,
		UserID:    userID,
		TeamID:    teamID,
		KeyID:     keyID,
		Timestamp: time.Now(),
		RequestID: fmt.Sprintf("req_%d", time.Now().UnixNano()),
	}
	
	return e.executeGuardrails(ctx, rails, input)
}

// StartDuringCall starts during-call guardrails (async)
func (e *Executor) StartDuringCall(ctx context.Context, request *providers.ChatRequest, userID, teamID, keyID string) context.Context {
	if !e.config.Enabled {
		return ctx
	}
	
	e.mu.RLock()
	rails := make([]Guardrail, len(e.duringCallRails))
	copy(rails, e.duringCallRails)
	e.mu.RUnlock()
	
	if len(rails) == 0 {
		return ctx
	}
	
	input := &GuardrailInput{
		Request:   request,
		UserID:    userID,
		TeamID:    teamID,
		KeyID:     keyID,
		Timestamp: time.Now(),
		RequestID: fmt.Sprintf("req_%d", time.Now().UnixNano()),
	}
	
	// Start async execution
	go func() {
		if err := e.executeGuardrails(ctx, rails, input); err != nil {
			e.logger.Error("During-call guardrail failed", zap.Error(err))
		}
	}()
	
	return ctx
}

// ExecuteLoggingOnly runs logging-only guardrails for masking sensitive data before logging
func (e *Executor) ExecuteLoggingOnly(ctx context.Context, request *providers.ChatRequest, response *providers.ChatResponse) (*GuardrailInput, error) {
	if !e.config.Enabled {
		return &GuardrailInput{Request: request, Response: response}, nil
	}
	
	e.mu.RLock()
	rails := make([]Guardrail, len(e.loggingRails))
	copy(rails, e.loggingRails)
	e.mu.RUnlock()
	
	if len(rails) == 0 {
		return &GuardrailInput{Request: request, Response: response}, nil
	}
	
	input := &GuardrailInput{
		Request:   request,
		Response:  response,
		Timestamp: time.Now(),
		RequestID: fmt.Sprintf("req_%d", time.Now().UnixNano()),
	}
	
	// Execute guardrails and apply modifications
	for _, rail := range rails {
		if !rail.IsEnabled() {
			continue
		}
		
		result, err := e.executeGuardrail(ctx, rail, input)
		if err != nil {
			e.logger.Error("Logging guardrail failed", 
				zap.String("guardrail", rail.GetName()),
				zap.Error(err))
			continue
		}
		
		// Apply modifications for logging
		if result.Modified {
			if result.ModifiedRequest != nil {
				input.Request = result.ModifiedRequest
			}
			if result.ModifiedResponse != nil {
				input.Response = result.ModifiedResponse
			}
		}
	}
	
	return input, nil
}

// executeGuardrails runs a list of guardrails and returns first blocking error
func (e *Executor) executeGuardrails(ctx context.Context, rails []Guardrail, input *GuardrailInput) error {
	for _, rail := range rails {
		if !rail.IsEnabled() {
			continue
		}
		
		result, err := e.executeGuardrail(ctx, rail, input)
		if err != nil {
			return err
		}
		
		// If blocked, return error immediately
		if result.Blocked {
			return &GuardrailError{
				GuardrailName: rail.GetName(),
				GuardrailType: rail.GetType().String(),
				Reason:        result.Reason,
				Details:       result.Details,
				Blocked:       true,
			}
		}
		
		// Apply modifications to input for next guardrail
		if result.Modified {
			if result.ModifiedRequest != nil {
				input.Request = result.ModifiedRequest
			}
			if result.ModifiedResponse != nil {
				input.Response = result.ModifiedResponse
			}
		}
	}
	
	return nil
}

// executeGuardrail runs a single guardrail with timeout and stats tracking
func (e *Executor) executeGuardrail(ctx context.Context, rail Guardrail, input *GuardrailInput) (*GuardrailResult, error) {
	start := time.Now()
	name := rail.GetName()
	
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second) // Default timeout
	defer cancel()
	
	// Execute guardrail
	result, err := rail.Execute(timeoutCtx, input)
	executionTime := time.Since(start)
	
	// Update statistics
	e.updateStats(name, executionTime, err)
	
	if err != nil {
		e.logger.Error("Guardrail execution failed",
			zap.String("name", name),
			zap.String("type", rail.GetType().String()),
			zap.Duration("execution_time", executionTime),
			zap.Error(err))
		return nil, fmt.Errorf("guardrail %s failed: %w", name, err)
	}
	
	// Set execution metadata
	if result != nil {
		result.ExecutionTime = executionTime
		result.GuardrailName = name
		result.GuardrailType = rail.GetType().String()
	}
	
	e.logger.Debug("Guardrail executed",
		zap.String("name", name),
		zap.String("type", rail.GetType().String()),
		zap.Duration("execution_time", executionTime),
		zap.Bool("passed", result.Passed),
		zap.Bool("blocked", result.Blocked),
		zap.Bool("modified", result.Modified))
	
	return result, nil
}

// updateStats updates guardrail execution statistics
func (e *Executor) updateStats(name string, executionTime time.Duration, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	stats := e.stats[name]
	if stats == nil {
		stats = &GuardrailStats{}
		e.stats[name] = stats
	}
	
	stats.TotalExecutions++
	stats.LastExecuted = time.Now()
	
	if err != nil {
		stats.TotalErrors++
	} else {
		stats.TotalPassed++
	}
	
	// Update average latency
	if stats.TotalExecutions == 1 {
		stats.AverageLatency = executionTime
	} else {
		stats.AverageLatency = time.Duration(
			(int64(stats.AverageLatency)*(stats.TotalExecutions-1) + int64(executionTime)) / 
			stats.TotalExecutions,
		)
	}
}

// GetStats returns statistics for all guardrails
func (e *Executor) GetStats() map[string]*GuardrailStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	// Return copy to avoid race conditions
	result := make(map[string]*GuardrailStats)
	for name, stats := range e.stats {
		statsCopy := *stats
		result[name] = &statsCopy
	}
	
	return result
}

// HealthCheck verifies all guardrails are healthy
func (e *Executor) HealthCheck(ctx context.Context) map[string]error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	results := make(map[string]error)
	for name, rail := range e.guardrails {
		if rail.IsEnabled() {
			results[name] = rail.HealthCheck(ctx)
		}
	}
	
	return results
}

// IsEnabled returns whether the executor is enabled
func (e *Executor) IsEnabled() bool {
	return e.config.Enabled
}