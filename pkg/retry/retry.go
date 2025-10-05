package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Config defines retry behavior
type Config struct {
	MaxAttempts  int           // Maximum number of attempts (including initial)
	InitialDelay time.Duration // Initial delay between retries
	MaxDelay     time.Duration // Maximum delay between retries
	Multiplier   float64       // Backoff multiplier
	Jitter       bool          // Add jitter to delays
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func(ctx context.Context) error

// IsRetryable determines if an error should trigger a retry
type IsRetryable func(error) bool

// DefaultIsRetryable returns true for common retryable errors
func DefaultIsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for common retryable error patterns
	errStr := err.Error()
	retryablePatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"429", // Rate limit
		"500", // Internal server error
		"502", // Bad gateway
		"503", // Service unavailable
		"504", // Gateway timeout
	}

	for _, pattern := range retryablePatterns {
		if containsString(errStr, pattern) {
			return true
		}
	}

	// Check if error is context.DeadlineExceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}

// Do executes the function with retry logic
func Do(ctx context.Context, config *Config, fn RetryableFunc, isRetryable IsRetryable) error {
	if config == nil {
		config = DefaultConfig()
	}

	if isRetryable == nil {
		isRetryable = DefaultIsRetryable
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Execute the function
		err := fn(ctx)

		// Success!
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !isRetryable(err) {
			return err // Non-retryable error
		}

		// Check if this was the last attempt
		if attempt == config.MaxAttempts-1 {
			break
		}

		// Calculate delay with exponential backoff
		if attempt > 0 {
			delay = time.Duration(float64(delay) * config.Multiplier)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}

		// Add jitter if enabled
		actualDelay := delay
		if config.Jitter {
			jitter := time.Duration(rand.Float64() * float64(delay) * 0.3)
			actualDelay = delay + jitter
		}

		// Wait before next attempt
		select {
		case <-time.After(actualDelay):
			// Continue to next attempt
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}

// DoWithBackoff is a simplified version with exponential backoff
func DoWithBackoff(ctx context.Context, maxAttempts int, fn RetryableFunc) error {
	config := &Config{
		MaxAttempts:  maxAttempts,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}

	return Do(ctx, config, fn, DefaultIsRetryable)
}

// Simple is the simplest retry with fixed delay
func Simple(ctx context.Context, attempts int, delay time.Duration, fn RetryableFunc) error {
	config := &Config{
		MaxAttempts:  attempts,
		InitialDelay: delay,
		MaxDelay:     delay,
		Multiplier:   1.0,
		Jitter:       false,
	}

	return Do(ctx, config, fn, DefaultIsRetryable)
}

// CalculateBackoff calculates the delay for a given attempt
func CalculateBackoff(attempt int, config *Config) time.Duration {
	if config == nil {
		config = DefaultConfig()
	}

	if attempt <= 0 {
		return config.InitialDelay
	}

	delay := config.InitialDelay * time.Duration(math.Pow(config.Multiplier, float64(attempt-1)))
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	return delay
}

// containsString checks if a string contains a substring (case-insensitive)
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && contains(s, substr)
}

// contains is a simple substring check
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
