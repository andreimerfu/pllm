package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 3, config.MaxAttempts)
	assert.Equal(t, 1*time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
	assert.True(t, config.Jitter)
}

func TestDefaultIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"timeout error", errors.New("connection timeout"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"429 rate limit", errors.New("429 Too Many Requests"), true},
		{"500 internal server error", errors.New("500 Internal Server Error"), true},
		{"502 bad gateway", errors.New("502 Bad Gateway"), true},
		{"503 service unavailable", errors.New("503 Service Unavailable"), true},
		{"504 gateway timeout", errors.New("504 Gateway Timeout"), true},
		{"context deadline exceeded", context.DeadlineExceeded, true},
		{"non-retryable error", errors.New("400 Bad Request"), false},
		{"custom error", errors.New("something went wrong"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DefaultIsRetryable(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDo_Success(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return nil // Success on first attempt
	}

	err := Do(ctx, config, fn, DefaultIsRetryable)

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestDo_EventualSuccess(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount < 2 {
			return errors.New("timeout error") // Retryable
		}
		return nil // Success on second attempt
	}

	err := Do(ctx, config, fn, DefaultIsRetryable)

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestDo_MaxAttemptsReached(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	callCount := 0
	expectedErr := errors.New("persistent timeout")
	fn := func(ctx context.Context) error {
		callCount++
		return expectedErr // Always fails
	}

	err := Do(ctx, config, fn, DefaultIsRetryable)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, 3, callCount)
}

func TestDo_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	callCount := 0
	expectedErr := errors.New("400 Bad Request")
	fn := func(ctx context.Context) error {
		callCount++
		return expectedErr // Non-retryable error
	}

	err := Do(ctx, config, fn, DefaultIsRetryable)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, 1, callCount) // Should not retry
}

func TestDo_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := &Config{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond, // Longer delay to test cancellation
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	}

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount == 1 {
			// Cancel context after first failure
			go func() {
				time.Sleep(50 * time.Millisecond)
				cancel()
			}()
		}
		return errors.New("timeout error") // Retryable
	}

	err := Do(ctx, config, fn, DefaultIsRetryable)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Equal(t, 1, callCount) // Should only call once before cancellation
}

func TestDo_WithNilConfig(t *testing.T) {
	ctx := context.Background()

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return nil
	}

	err := Do(ctx, nil, fn, DefaultIsRetryable)

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestDo_WithNilIsRetryable(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount == 1 {
			return errors.New("timeout error") // Should be retryable with default function
		}
		return nil
	}

	err := Do(ctx, config, fn, nil)

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestDoWithBackoff(t *testing.T) {
	ctx := context.Background()

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount < 2 {
			return errors.New("timeout error")
		}
		return nil
	}

	err := DoWithBackoff(ctx, 3, fn)

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestSimple(t *testing.T) {
	ctx := context.Background()

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return errors.New("timeout error")
		}
		return nil
	}

	err := Simple(ctx, 3, 10*time.Millisecond, fn)

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestCalculateBackoff(t *testing.T) {
	config := &Config{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 30 * time.Second},  // Capped at MaxDelay
		{10, 30 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			delay := CalculateBackoff(tt.attempt, config)
			assert.Equal(t, tt.expected, delay)
		})
	}
}

func TestCalculateBackoff_WithNilConfig(t *testing.T) {
	delay := CalculateBackoff(1, nil)
	assert.Equal(t, 1*time.Second, delay) // Should use default config
}

func TestDo_ExponentialBackoff(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:  4,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	callTimes := make([]time.Time, 0)
	fn := func(ctx context.Context) error {
		callTimes = append(callTimes, time.Now())
		return errors.New("timeout error") // Always fail to test all delays
	}

	start := time.Now()
	err := Do(ctx, config, fn, DefaultIsRetryable)

	assert.Error(t, err)
	assert.Len(t, callTimes, 4)

	// Check that delays are approximately exponential
	// First call is immediate
	assert.WithinDuration(t, start, callTimes[0], 5*time.Millisecond)

	// Second call should be after InitialDelay (10ms)
	expectedDelay1 := 10 * time.Millisecond
	actualDelay1 := callTimes[1].Sub(callTimes[0])
	assert.InDelta(t, expectedDelay1.Nanoseconds(), actualDelay1.Nanoseconds(), float64(5*time.Millisecond.Nanoseconds()))

	// Third call should be after 20ms (10ms * 2.0)
	expectedDelay2 := 20 * time.Millisecond
	actualDelay2 := callTimes[2].Sub(callTimes[1])
	assert.InDelta(t, expectedDelay2.Nanoseconds(), actualDelay2.Nanoseconds(), float64(5*time.Millisecond.Nanoseconds()))
}

func TestDo_JitterEffect(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}

	var delays []time.Duration
	callTimes := make([]time.Time, 0)

	fn := func(ctx context.Context) error {
		callTimes = append(callTimes, time.Now())
		return errors.New("timeout error")
	}

	Do(ctx, config, fn, DefaultIsRetryable)

	// Calculate actual delays
	for i := 1; i < len(callTimes); i++ {
		delays = append(delays, callTimes[i].Sub(callTimes[i-1]))
	}

	// With jitter, delays should be greater than base delay but not too much greater
	baseDelay := config.InitialDelay
	for i, delay := range delays {
		expectedBase := time.Duration(float64(baseDelay) * math.Pow(2, float64(i))) // 2^i multiplier
		if expectedBase > config.MaxDelay {
			expectedBase = config.MaxDelay
		}

		// Jitter adds up to 30% of the delay
		minExpected := expectedBase
		maxExpected := expectedBase + time.Duration(float64(expectedBase)*0.3)

		assert.True(t, delay >= minExpected, "Delay %v should be at least %v", delay, minExpected)
		assert.True(t, delay <= maxExpected+10*time.Millisecond, "Delay %v should be at most %v", delay, maxExpected)
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "WORLD", false}, // Case sensitive
		{"timeout error", "timeout", true},
		{"connection refused", "refused", true},
		{"", "", true},
		{"test", "", true},
		{"", "test", false},
		{"short", "longer string", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s contains %s", tt.s, tt.substr), func(t *testing.T) {
			result := containsString(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "llo wo", true},
		{"hello world", "xyz", false},
		{"", "", true},
		{"test", "", true},
		{"", "test", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s contains %s", tt.s, tt.substr), func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Benchmark tests
func BenchmarkDo_Success(b *testing.B) {
	ctx := context.Background()
	config := DefaultConfig()
	fn := func(ctx context.Context) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Do(ctx, config, fn, DefaultIsRetryable)
	}
}

func BenchmarkDo_WithRetries(b *testing.B) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Microsecond, // Very small delay for benchmarking
		MaxDelay:     10 * time.Microsecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attempt := 0
		fn := func(ctx context.Context) error {
			attempt++
			if attempt < 2 {
				return errors.New("timeout")
			}
			return nil
		}
		Do(ctx, config, fn, DefaultIsRetryable)
	}
}

func BenchmarkDefaultIsRetryable(b *testing.B) {
	err := errors.New("connection timeout error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DefaultIsRetryable(err)
	}
}

func BenchmarkCalculateBackoff(b *testing.B) {
	config := DefaultConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateBackoff(5, config)
	}
}

// Test custom retry conditions
func TestDo_CustomRetryCondition(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()
	config.MaxAttempts = 3

	// Custom retry condition: only retry on specific error message
	customIsRetryable := func(err error) bool {
		return err != nil && strings.Contains(err.Error(), "retriable")
	}

	t.Run("retry on custom condition", func(t *testing.T) {
		callCount := 0
		fn := func(ctx context.Context) error {
			callCount++
			if callCount < 2 {
				return errors.New("retriable error")
			}
			return nil
		}

		err := Do(ctx, config, fn, customIsRetryable)
		assert.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})

	t.Run("don't retry on non-matching condition", func(t *testing.T) {
		callCount := 0
		fn := func(ctx context.Context) error {
			callCount++
			return errors.New("permanent error")
		}

		err := Do(ctx, config, fn, customIsRetryable)
		assert.Error(t, err)
		assert.Equal(t, 1, callCount)
	})
}

// Test edge cases
func TestDo_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("zero max attempts", func(t *testing.T) {
		config := &Config{MaxAttempts: 0}
		callCount := 0
		fn := func(ctx context.Context) error {
			callCount++
			return errors.New("error")
		}

		err := Do(ctx, config, fn, DefaultIsRetryable)
		assert.NoError(t, err) // No attempts means no error
		assert.Equal(t, 0, callCount)
	})

	t.Run("one max attempt", func(t *testing.T) {
		config := &Config{MaxAttempts: 1}
		callCount := 0
		fn := func(ctx context.Context) error {
			callCount++
			return errors.New("timeout")
		}

		err := Do(ctx, config, fn, DefaultIsRetryable)
		assert.Error(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("zero initial delay", func(t *testing.T) {
		config := &Config{
			MaxAttempts:  2,
			InitialDelay: 0,
			MaxDelay:     1 * time.Second,
			Multiplier:   2.0,
		}

		callTimes := make([]time.Time, 0)
		fn := func(ctx context.Context) error {
			callTimes = append(callTimes, time.Now())
			return errors.New("timeout")
		}

		Do(ctx, config, fn, DefaultIsRetryable)

		require.Len(t, callTimes, 2)
		// Should have minimal delay between calls
		delay := callTimes[1].Sub(callTimes[0])
		assert.True(t, delay < 10*time.Millisecond)
	})
}

// Test concurrent access safety
func TestDo_ConcurrentSafety(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:  2,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       true,
	}

	const numGoroutines = 100
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			fn := func(ctx context.Context) error {
				if id%2 == 0 {
					return nil // Half succeed
				}
				return errors.New("timeout") // Half fail
			}

			err := Do(ctx, config, fn, DefaultIsRetryable)
			results <- err
		}(i)
	}

	// Collect results
	successes := 0
	failures := 0
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		if err == nil {
			successes++
		} else {
			failures++
		}
	}

	assert.Equal(t, 50, successes)
	assert.Equal(t, 50, failures)
}
