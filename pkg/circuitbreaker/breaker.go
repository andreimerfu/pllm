package circuitbreaker

import (
	"sync"
	"time"
)

// SimpleBreaker is a basic circuit breaker that tracks failures and opens after a threshold
type SimpleBreaker struct {
	mu              sync.RWMutex
	failures        int
	lastFailureTime time.Time
	isOpen          bool

	// Configuration
	threshold int
	cooldown  time.Duration
}

// New creates a new circuit breaker
func New(threshold int, cooldown time.Duration) *SimpleBreaker {
	if threshold <= 0 {
		threshold = 5 // Default: 5 failures
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second // Default: 30 seconds
	}

	return &SimpleBreaker{
		threshold: threshold,
		cooldown:  cooldown,
	}
}

// IsOpen checks if the circuit is open (blocking requests)
func (b *SimpleBreaker) IsOpen() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.isOpen {
		return false
	}

	// Check if cooldown period has passed
	if time.Since(b.lastFailureTime) > b.cooldown {
		// Reset the circuit breaker
		b.mu.RUnlock()
		b.mu.Lock()
		b.isOpen = false
		b.failures = 0
		b.mu.Unlock()
		b.mu.RLock()
		return false
	}

	return true
}

// RecordSuccess resets the failure counter
func (b *SimpleBreaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures = 0
	b.isOpen = false
}

// RecordFailure increments the failure counter and opens the circuit if threshold is reached
func (b *SimpleBreaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures++
	b.lastFailureTime = time.Now()

	if b.failures >= b.threshold {
		b.isOpen = true
	}
}

// Reset manually resets the circuit breaker
func (b *SimpleBreaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures = 0
	b.isOpen = false
}

// GetState returns current state for monitoring
func (b *SimpleBreaker) GetState() (isOpen bool, failures int) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.isOpen, b.failures
}

// Manager manages circuit breakers for multiple models
type Manager struct {
	mu       sync.RWMutex
	breakers map[string]*SimpleBreaker

	// Default configuration
	defaultThreshold int
	defaultCooldown  time.Duration
}

// NewManager creates a new circuit breaker manager
func NewManager(threshold int, cooldown time.Duration) *Manager {
	return &Manager{
		breakers:         make(map[string]*SimpleBreaker),
		defaultThreshold: threshold,
		defaultCooldown:  cooldown,
	}
}

// GetBreaker gets or creates a circuit breaker for a model
func (m *Manager) GetBreaker(model string) *SimpleBreaker {
	m.mu.RLock()
	breaker, exists := m.breakers[model]
	m.mu.RUnlock()

	if exists {
		return breaker
	}

	// Create new breaker
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if breaker, exists = m.breakers[model]; exists {
		return breaker
	}

	breaker = New(m.defaultThreshold, m.defaultCooldown)
	m.breakers[model] = breaker
	return breaker
}

// IsOpen checks if circuit is open for a model
func (m *Manager) IsOpen(model string) bool {
	return m.GetBreaker(model).IsOpen()
}

// RecordSuccess records a success for a model
func (m *Manager) RecordSuccess(model string) {
	m.GetBreaker(model).RecordSuccess()
}

// RecordFailure records a failure for a model
func (m *Manager) RecordFailure(model string) {
	m.GetBreaker(model).RecordFailure()
}

// Reset resets a specific model's circuit breaker
func (m *Manager) Reset(model string) {
	m.GetBreaker(model).Reset()
}

// ResetAll resets all circuit breakers
func (m *Manager) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, breaker := range m.breakers {
		breaker.Reset()
	}
}

// GetAllStates returns the state of all circuit breakers for monitoring
func (m *Manager) GetAllStates() map[string]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make(map[string]map[string]interface{})
	for model, breaker := range m.breakers {
		isOpen, failures := breaker.GetState()
		states[model] = map[string]interface{}{
			"is_open":  isOpen,
			"failures": failures,
		}
	}

	return states
}
