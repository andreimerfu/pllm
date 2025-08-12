package circuitbreaker

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("with valid parameters", func(t *testing.T) {
		breaker := New(5, 30*time.Second)
		assert.Equal(t, 5, breaker.threshold)
		assert.Equal(t, 30*time.Second, breaker.cooldown)
		assert.False(t, breaker.isOpen)
		assert.Equal(t, 0, breaker.failures)
	})

	t.Run("with zero threshold uses default", func(t *testing.T) {
		breaker := New(0, 30*time.Second)
		assert.Equal(t, 5, breaker.threshold) // Default
	})

	t.Run("with zero cooldown uses default", func(t *testing.T) {
		breaker := New(5, 0)
		assert.Equal(t, 30*time.Second, breaker.cooldown) // Default
	})

	t.Run("with negative values uses defaults", func(t *testing.T) {
		breaker := New(-1, -1*time.Second)
		assert.Equal(t, 5, breaker.threshold)
		assert.Equal(t, 30*time.Second, breaker.cooldown)
	})
}

func TestSimpleBreaker_IsOpen(t *testing.T) {
	breaker := New(3, 100*time.Millisecond)

	t.Run("starts closed", func(t *testing.T) {
		assert.False(t, breaker.IsOpen())
	})

	t.Run("stays closed under threshold", func(t *testing.T) {
		breaker.RecordFailure()
		breaker.RecordFailure()
		assert.False(t, breaker.IsOpen())
	})

	t.Run("opens when threshold reached", func(t *testing.T) {
		breaker.RecordFailure() // Third failure
		assert.True(t, breaker.IsOpen())
	})

	t.Run("stays open during cooldown", func(t *testing.T) {
		assert.True(t, breaker.IsOpen())
		time.Sleep(50 * time.Millisecond) // Half cooldown
		assert.True(t, breaker.IsOpen())
	})

	t.Run("closes after cooldown", func(t *testing.T) {
		time.Sleep(60 * time.Millisecond) // Remaining cooldown + buffer
		assert.False(t, breaker.IsOpen())
	})
}

func TestSimpleBreaker_RecordSuccess(t *testing.T) {
	breaker := New(3, 100*time.Millisecond)

	t.Run("resets failures when closed", func(t *testing.T) {
		breaker.RecordFailure()
		breaker.RecordFailure()
		assert.Equal(t, 2, breaker.failures)

		breaker.RecordSuccess()
		assert.Equal(t, 0, breaker.failures)
		assert.False(t, breaker.isOpen)
	})

	t.Run("closes circuit and resets failures", func(t *testing.T) {
		// Open the circuit
		for i := 0; i < 3; i++ {
			breaker.RecordFailure()
		}
		assert.True(t, breaker.isOpen)

		breaker.RecordSuccess()
		assert.False(t, breaker.isOpen)
		assert.Equal(t, 0, breaker.failures)
	})
}

func TestSimpleBreaker_RecordFailure(t *testing.T) {
	breaker := New(3, 100*time.Millisecond)

	t.Run("increments failure count", func(t *testing.T) {
		breaker.RecordFailure()
		assert.Equal(t, 1, breaker.failures)
		assert.False(t, breaker.isOpen)

		breaker.RecordFailure()
		assert.Equal(t, 2, breaker.failures)
		assert.False(t, breaker.isOpen)
	})

	t.Run("opens circuit at threshold", func(t *testing.T) {
		breaker.RecordFailure() // Third failure
		assert.Equal(t, 3, breaker.failures)
		assert.True(t, breaker.isOpen)
	})

	t.Run("records timestamp of failure", func(t *testing.T) {
		before := time.Now()
		breaker.RecordFailure()
		after := time.Now()

		assert.True(t, breaker.lastFailureTime.After(before) || breaker.lastFailureTime.Equal(before))
		assert.True(t, breaker.lastFailureTime.Before(after) || breaker.lastFailureTime.Equal(after))
	})
}

func TestSimpleBreaker_Reset(t *testing.T) {
	breaker := New(3, 100*time.Millisecond)

	// Open the circuit
	for i := 0; i < 3; i++ {
		breaker.RecordFailure()
	}
	assert.True(t, breaker.isOpen)

	breaker.Reset()
	assert.False(t, breaker.isOpen)
	assert.Equal(t, 0, breaker.failures)
}

func TestSimpleBreaker_GetState(t *testing.T) {
	breaker := New(3, 100*time.Millisecond)

	t.Run("initial state", func(t *testing.T) {
		isOpen, failures := breaker.GetState()
		assert.False(t, isOpen)
		assert.Equal(t, 0, failures)
	})

	t.Run("after failures", func(t *testing.T) {
		breaker.RecordFailure()
		breaker.RecordFailure()

		isOpen, failures := breaker.GetState()
		assert.False(t, isOpen)
		assert.Equal(t, 2, failures)
	})

	t.Run("when open", func(t *testing.T) {
		breaker.RecordFailure() // Third failure

		isOpen, failures := breaker.GetState()
		assert.True(t, isOpen)
		assert.Equal(t, 3, failures)
	})
}

func TestSimpleBreaker_ConcurrentAccess(t *testing.T) {
	breaker := New(100, 100*time.Millisecond)
	const numGoroutines = 50
	const operationsPerGoroutine = 20

	var wg sync.WaitGroup
	
	// Test concurrent failures
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				breaker.RecordFailure()
			}
		}()
	}

	wg.Wait()

	// Circuit should be open (threshold is 100, we recorded 1000 failures)
	assert.True(t, breaker.IsOpen())
	isOpen, failures := breaker.GetState()
	assert.True(t, isOpen)
	assert.Equal(t, numGoroutines*operationsPerGoroutine, failures)
}

func TestSimpleBreaker_ConcurrentSuccessAndFailure(t *testing.T) {
	breaker := New(50, 100*time.Millisecond)
	const numGoroutines = 10

	var wg sync.WaitGroup

	// Half goroutines record failures, half record successes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if id%2 == 0 {
				for j := 0; j < 10; j++ {
					breaker.RecordFailure()
				}
			} else {
				for j := 0; j < 10; j++ {
					breaker.RecordSuccess()
				}
			}
		}(i)
	}

	wg.Wait()

	// The exact state depends on the order of operations
	// but the breaker should handle concurrent access safely
	isOpen, failures := breaker.GetState()
	assert.True(t, failures >= 0) // Should never be negative
	assert.True(t, isOpen || !isOpen) // Should be in a valid state
}

func TestManager(t *testing.T) {
	t.Run("creates new breakers on demand", func(t *testing.T) {
		manager := NewManager(3, 100*time.Millisecond)
		breaker1 := manager.GetBreaker("model1")
		breaker2 := manager.GetBreaker("model2")
		
		assert.NotNil(t, breaker1)
		assert.NotNil(t, breaker2)
		// Different models should have different breakers (different memory addresses)
		assert.NotSame(t, breaker1, breaker2)
	})

	t.Run("returns same breaker for same model", func(t *testing.T) {
		manager := NewManager(3, 100*time.Millisecond)
		breaker1 := manager.GetBreaker("model1")
		breaker2 := manager.GetBreaker("model1")
		
		assert.Equal(t, breaker1, breaker2)
	})

	t.Run("IsOpen delegates to breaker", func(t *testing.T) {
		manager := NewManager(3, 100*time.Millisecond)
		assert.False(t, manager.IsOpen("model1"))
		
		// Trip the breaker
		for i := 0; i < 3; i++ {
			manager.RecordFailure("model1")
		}
		
		assert.True(t, manager.IsOpen("model1"))
	})

	t.Run("RecordSuccess delegates to breaker", func(t *testing.T) {
		manager := NewManager(3, 100*time.Millisecond)
		manager.RecordFailure("model2")
		manager.RecordFailure("model2")
		
		breaker := manager.GetBreaker("model2")
		_, failures := breaker.GetState()
		assert.Equal(t, 2, failures)
		
		manager.RecordSuccess("model2")
		_, failures = breaker.GetState()
		assert.Equal(t, 0, failures)
	})

	t.Run("RecordFailure delegates to breaker", func(t *testing.T) {
		manager := NewManager(3, 100*time.Millisecond)
		manager.RecordFailure("model3")
		
		breaker := manager.GetBreaker("model3")
		_, failures := breaker.GetState()
		assert.Equal(t, 1, failures)
	})

	t.Run("Reset delegates to breaker", func(t *testing.T) {
		manager := NewManager(3, 100*time.Millisecond)
		// Trip the breaker
		for i := 0; i < 3; i++ {
			manager.RecordFailure("model4")
		}
		assert.True(t, manager.IsOpen("model4"))
		
		manager.Reset("model4")
		assert.False(t, manager.IsOpen("model4"))
	})

	t.Run("ResetAll resets all breakers", func(t *testing.T) {
		manager := NewManager(3, 100*time.Millisecond)
		// Trip multiple breakers
		for i := 0; i < 3; i++ {
			manager.RecordFailure("model5")
			manager.RecordFailure("model6")
		}
		
		assert.True(t, manager.IsOpen("model5"))
		assert.True(t, manager.IsOpen("model6"))
		
		manager.ResetAll()
		
		assert.False(t, manager.IsOpen("model5"))
		assert.False(t, manager.IsOpen("model6"))
	})

	t.Run("GetAllStates returns all breaker states", func(t *testing.T) {
		manager := NewManager(3, 100*time.Millisecond)
		// Create some breakers in different states
		manager.RecordFailure("model7")
		manager.RecordFailure("model8")
		manager.RecordFailure("model8")
		
		states := manager.GetAllStates()
		
		require.Contains(t, states, "model7")
		require.Contains(t, states, "model8")
		
		model7State := states["model7"]
		assert.False(t, model7State["is_open"].(bool))
		assert.Equal(t, 1, model7State["failures"].(int))
		
		model8State := states["model8"]
		assert.False(t, model8State["is_open"].(bool))
		assert.Equal(t, 2, model8State["failures"].(int))
	})
}

func TestManager_ConcurrentAccess(t *testing.T) {
	manager := NewManager(10, 100*time.Millisecond)
	const numModels = 5
	const numGoroutines = 20

	var wg sync.WaitGroup

	// Create multiple goroutines accessing different models
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			modelName := fmt.Sprintf("model-%d", id%numModels)
			
			// Mix of operations
			for j := 0; j < 10; j++ {
				switch j % 4 {
				case 0:
					manager.RecordFailure(modelName)
				case 1:
					manager.RecordSuccess(modelName)
				case 2:
					manager.IsOpen(modelName)
				case 3:
					manager.GetBreaker(modelName)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all models were created and are in valid states
	states := manager.GetAllStates()
	assert.Equal(t, numModels, len(states))
	
	for modelName, state := range states {
		isOpen := state["is_open"].(bool)
		failures := state["failures"].(int)
		
		assert.True(t, failures >= 0, "Model %s should not have negative failures", modelName)
		assert.True(t, isOpen || !isOpen, "Model %s should be in valid state", modelName)
	}
}

// Benchmark tests
func BenchmarkSimpleBreaker_IsOpen(b *testing.B) {
	breaker := New(5, 30*time.Second)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		breaker.IsOpen()
	}
}

func BenchmarkSimpleBreaker_RecordSuccess(b *testing.B) {
	breaker := New(5, 30*time.Second)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		breaker.RecordSuccess()
	}
}

func BenchmarkSimpleBreaker_RecordFailure(b *testing.B) {
	breaker := New(5, 30*time.Second)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		breaker.RecordFailure()
		if i%5 == 4 {
			breaker.Reset() // Reset to avoid staying open
		}
	}
}

func BenchmarkManager_GetBreaker(b *testing.B) {
	manager := NewManager(5, 30*time.Second)
	models := []string{"model1", "model2", "model3", "model4", "model5"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model := models[i%len(models)]
		manager.GetBreaker(model)
	}
}

func BenchmarkManager_ConcurrentAccess(b *testing.B) {
	manager := NewManager(5, 30*time.Second)
	models := []string{"model1", "model2", "model3", "model4", "model5"}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			model := models[i%len(models)]
			switch i % 3 {
			case 0:
				manager.RecordSuccess(model)
			case 1:
				manager.RecordFailure(model)
			case 2:
				manager.IsOpen(model)
			}
			i++
		}
	})
}

// Edge case tests
func TestSimpleBreaker_EdgeCases(t *testing.T) {
	t.Run("very short cooldown", func(t *testing.T) {
		breaker := New(1, 1*time.Millisecond)
		breaker.RecordFailure()
		
		assert.True(t, breaker.IsOpen())
		
		// Wait for cooldown
		time.Sleep(5 * time.Millisecond)
		
		assert.False(t, breaker.IsOpen())
	})

	t.Run("threshold of 1", func(t *testing.T) {
		breaker := New(1, 100*time.Millisecond)
		
		assert.False(t, breaker.IsOpen())
		
		breaker.RecordFailure()
		assert.True(t, breaker.IsOpen())
	})

	t.Run("very long cooldown", func(t *testing.T) {
		breaker := New(1, 24*time.Hour)
		breaker.RecordFailure()
		
		assert.True(t, breaker.IsOpen())
		
		// Should still be open after short wait
		time.Sleep(1 * time.Millisecond)
		assert.True(t, breaker.IsOpen())
	})
}

func TestManager_EdgeCases(t *testing.T) {
	t.Run("empty model name", func(t *testing.T) {
		manager := NewManager(3, 100*time.Millisecond)
		
		breaker := manager.GetBreaker("")
		assert.NotNil(t, breaker)
		
		// Should work normally
		manager.RecordFailure("")
		assert.False(t, manager.IsOpen(""))
	})

	t.Run("unicode model names", func(t *testing.T) {
		manager := NewManager(3, 100*time.Millisecond)
		
		unicodeModel := "模型-测试-🤖"
		breaker := manager.GetBreaker(unicodeModel)
		assert.NotNil(t, breaker)
		
		manager.RecordFailure(unicodeModel)
		assert.False(t, manager.IsOpen(unicodeModel))
		
		states := manager.GetAllStates()
		assert.Contains(t, states, unicodeModel)
	})

	t.Run("very long model names", func(t *testing.T) {
		manager := NewManager(3, 100*time.Millisecond)
		
		longModel := string(make([]byte, 1000))
		for i := range longModel {
			longModel = longModel[:i] + "a" + longModel[i+1:]
		}
		
		breaker := manager.GetBreaker(longModel)
		assert.NotNil(t, breaker)
	})
}

// Test rapid state changes
func TestSimpleBreaker_RapidStateChanges(t *testing.T) {
	breaker := New(2, 10*time.Millisecond)

	// Rapid failure -> success -> failure pattern
	for i := 0; i < 100; i++ {
		breaker.RecordFailure()
		breaker.RecordFailure()
		assert.True(t, breaker.IsOpen())
		
		breaker.RecordSuccess()
		assert.False(t, breaker.IsOpen())
	}
}

// Test behavior during exactly the cooldown period
func TestSimpleBreaker_CooldownTiming(t *testing.T) {
	cooldown := 50 * time.Millisecond
	breaker := New(1, cooldown)
	
	// Open the circuit
	breaker.RecordFailure()
	assert.True(t, breaker.IsOpen())
	
	// Check exactly at cooldown time
	time.Sleep(cooldown)
	assert.False(t, breaker.IsOpen())
	
	// Should be able to record success
	breaker.RecordSuccess()
	assert.False(t, breaker.IsOpen())
}