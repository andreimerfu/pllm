package circuitbreaker

import (
	"sync"
	"time"
)

// AdaptiveBreaker is a circuit breaker that considers both failures and latency
type AdaptiveBreaker struct {
	mu sync.RWMutex
	
	// Failure tracking
	failures        int
	lastFailureTime time.Time
	
	// Latency tracking
	latencyWindow   []time.Duration
	windowSize      int
	slowRequests    int
	
	// Circuit state
	state State // CLOSED, OPEN, HALF_OPEN
	
	// Configuration
	failureThreshold   int
	latencyThreshold   time.Duration  // Requests slower than this count as "slow"
	slowRequestLimit   int             // Number of slow requests before opening
	cooldownPeriod     time.Duration
	halfOpenRequests   int             // Requests allowed in half-open state
	halfOpenSuccesses  int             // Successes needed to close circuit
	
	// Metrics
	totalRequests      int64
	currentConcurrent  int32
	maxConcurrent      int32
}

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// NewAdaptiveBreaker creates a new adaptive circuit breaker
func NewAdaptiveBreaker(failureThreshold int, latencyThreshold time.Duration, slowRequestLimit int) *AdaptiveBreaker {
	return &AdaptiveBreaker{
		failureThreshold:  failureThreshold,
		latencyThreshold:  latencyThreshold,
		slowRequestLimit:  slowRequestLimit,
		cooldownPeriod:    30 * time.Second,
		windowSize:        100,
		latencyWindow:     make([]time.Duration, 0, 100),
		halfOpenRequests:  3,
		halfOpenSuccesses: 2,
		state:            StateClosed,
	}
}

// CanRequest checks if a request should be allowed
func (ab *AdaptiveBreaker) CanRequest() bool {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	
	switch ab.state {
	case StateClosed:
		return true
		
	case StateOpen:
		// Check if cooldown has passed
		if time.Since(ab.lastFailureTime) > ab.cooldownPeriod {
			ab.state = StateHalfOpen
			ab.halfOpenRequests = 3
			ab.halfOpenSuccesses = 0
			return true
		}
		return false
		
	case StateHalfOpen:
		// Allow limited requests in half-open state
		if ab.halfOpenRequests > 0 {
			ab.halfOpenRequests--
			return true
		}
		return false
		
	default:
		return false
	}
}

// RecordSuccess records a successful request with its latency
func (ab *AdaptiveBreaker) RecordSuccess(latency time.Duration) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	
	ab.totalRequests++
	
	// Track latency
	ab.addLatency(latency)
	
	// Check if this was a slow request
	if latency > ab.latencyThreshold {
		ab.slowRequests++
		
		// Check if we should open due to slow requests
		if ab.slowRequests >= ab.slowRequestLimit && ab.state == StateClosed {
			ab.openCircuit("too many slow requests")
			return
		}
	} else {
		// Reset slow request counter on fast request
		if ab.slowRequests > 0 {
			ab.slowRequests--
		}
	}
	
	// Handle state transitions
	switch ab.state {
	case StateHalfOpen:
		ab.halfOpenSuccesses++
		if ab.halfOpenSuccesses >= 2 {
			// Circuit has recovered
			ab.state = StateClosed
			ab.failures = 0
			ab.slowRequests = 0
		}
		
	case StateClosed:
		// Reset failure counter on success
		if ab.failures > 0 {
			ab.failures--
		}
	}
}

// RecordFailure records a failed request
func (ab *AdaptiveBreaker) RecordFailure() {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	
	ab.totalRequests++
	ab.failures++
	ab.lastFailureTime = time.Now()
	
	switch ab.state {
	case StateClosed:
		if ab.failures >= ab.failureThreshold {
			ab.openCircuit("too many failures")
		}
		
	case StateHalfOpen:
		// Failed during recovery, reopen
		ab.openCircuit("failed in half-open state")
	}
}

// RecordTimeout records a timeout (counts as both failure and slow)
func (ab *AdaptiveBreaker) RecordTimeout() {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	
	ab.totalRequests++
	ab.failures++
	ab.slowRequests++
	ab.lastFailureTime = time.Now()
	
	// Timeouts are critical - open immediately in any state
	ab.openCircuit("timeout detected")
}

// StartRequest increments concurrent request counter
func (ab *AdaptiveBreaker) StartRequest() {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	
	ab.currentConcurrent++
	if ab.currentConcurrent > ab.maxConcurrent {
		ab.maxConcurrent = ab.currentConcurrent
	}
}

// EndRequest decrements concurrent request counter
func (ab *AdaptiveBreaker) EndRequest() {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	
	if ab.currentConcurrent > 0 {
		ab.currentConcurrent--
	}
}

// GetConcurrent returns the current number of concurrent requests
func (ab *AdaptiveBreaker) GetConcurrent() int32 {
	ab.mu.RLock()
	defer ab.mu.RUnlock()
	return ab.currentConcurrent
}

// GetAverageLatency returns the average latency from the window
func (ab *AdaptiveBreaker) GetAverageLatency() time.Duration {
	ab.mu.RLock()
	defer ab.mu.RUnlock()
	
	if len(ab.latencyWindow) == 0 {
		return 0
	}
	
	var total time.Duration
	for _, lat := range ab.latencyWindow {
		total += lat
	}
	
	return total / time.Duration(len(ab.latencyWindow))
}

// GetP95Latency returns the 95th percentile latency
func (ab *AdaptiveBreaker) GetP95Latency() time.Duration {
	ab.mu.RLock()
	defer ab.mu.RUnlock()
	
	if len(ab.latencyWindow) == 0 {
		return 0
	}
	
	// Simple P95 calculation (not perfectly accurate but fast)
	index := int(float64(len(ab.latencyWindow)) * 0.95)
	if index >= len(ab.latencyWindow) {
		index = len(ab.latencyWindow) - 1
	}
	
	return ab.latencyWindow[index]
}

// GetState returns current circuit state and metrics
func (ab *AdaptiveBreaker) GetState() map[string]interface{} {
	ab.mu.RLock()
	defer ab.mu.RUnlock()
	
	return map[string]interface{}{
		"state":             ab.state.String(),
		"failures":          ab.failures,
		"slow_requests":     ab.slowRequests,
		"avg_latency":       ab.GetAverageLatency().String(),
		"p95_latency":       ab.GetP95Latency().String(),
		"concurrent":        ab.currentConcurrent,
		"max_concurrent":    ab.maxConcurrent,
		"total_requests":    ab.totalRequests,
	}
}

// Reset manually resets the circuit breaker
func (ab *AdaptiveBreaker) Reset() {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	
	ab.state = StateClosed
	ab.failures = 0
	ab.slowRequests = 0
	ab.latencyWindow = make([]time.Duration, 0, ab.windowSize)
}

// Private methods

func (ab *AdaptiveBreaker) openCircuit(reason string) {
	ab.state = StateOpen
	ab.lastFailureTime = time.Now()
	// Log reason if needed
}

func (ab *AdaptiveBreaker) addLatency(latency time.Duration) {
	// Maintain a sliding window of latencies
	if len(ab.latencyWindow) >= ab.windowSize {
		// Remove oldest
		ab.latencyWindow = ab.latencyWindow[1:]
	}
	ab.latencyWindow = append(ab.latencyWindow, latency)
}

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}