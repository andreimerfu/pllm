package models

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/services/llm/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockFailingProvider simulates a provider that fails N times then succeeds
type MockFailingProvider struct {
	failCount     int
	currentFails  int
	responseDelay time.Duration
}

func (m *MockFailingProvider) ChatCompletion(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	if m.responseDelay > 0 {
		time.Sleep(m.responseDelay)
	}

	if m.currentFails < m.failCount {
		m.currentFails++
		return nil, errors.New("simulated provider failure")
	}

	return &providers.ChatResponse{
		ID:      "test-response",
		Model:   req.Model,
		Choices: []providers.Choice{{Message: providers.Message{Role: "assistant", Content: "Success!"}}},
		Usage:   providers.Usage{TotalTokens: 100},
	}, nil
}

func (m *MockFailingProvider) ChatCompletionStream(ctx context.Context, req *providers.ChatRequest) (<-chan providers.StreamResponse, error) {
	return nil, errors.New("streaming not implemented in mock")
}

func (m *MockFailingProvider) Completion(ctx context.Context, req *providers.CompletionRequest) (*providers.CompletionResponse, error) {
	return nil, errors.New("completion not implemented in mock")
}

func (m *MockFailingProvider) CompletionStream(ctx context.Context, req *providers.CompletionRequest) (<-chan providers.StreamResponse, error) {
	return nil, errors.New("completion stream not implemented in mock")
}

func (m *MockFailingProvider) Embeddings(ctx context.Context, req *providers.EmbeddingsRequest) (*providers.EmbeddingsResponse, error) {
	return nil, errors.New("embeddings not implemented in mock")
}

func (m *MockFailingProvider) AudioTranscription(ctx context.Context, req *providers.TranscriptionRequest) (*providers.TranscriptionResponse, error) {
	return nil, errors.New("audio transcription not implemented in mock")
}

func (m *MockFailingProvider) AudioSpeech(ctx context.Context, req *providers.SpeechRequest) ([]byte, error) {
	return nil, errors.New("audio speech not implemented in mock")
}

func (m *MockFailingProvider) ImageGeneration(ctx context.Context, req *providers.ImageRequest) (*providers.ImageResponse, error) {
	return nil, errors.New("image generation not implemented in mock")
}

func (m *MockFailingProvider) GetType() string        { return "mock" }
func (m *MockFailingProvider) GetName() string        { return "mock-provider" }
func (m *MockFailingProvider) GetPriority() int       { return 50 }
func (m *MockFailingProvider) IsHealthy() bool        { return true }
func (m *MockFailingProvider) SupportsModel(model string) bool { return true }
func (m *MockFailingProvider) ListModels() []string   { return []string{"mock-gpt-4", "mock-gpt-3.5"} }
func (m *MockFailingProvider) HealthCheck(ctx context.Context) error { return nil }

// TestInstanceLevelFailover tests automatic retry across multiple instances of the same model
func TestInstanceLevelFailover(t *testing.T) {
	logger := zap.NewNop()

	// Create router settings with failover enabled
	router := config.RouterSettings{
		RoutingStrategy:       "priority",
		EnableFailover:        true,
		InstanceRetryAttempts: 3,
		EnableModelFallback:   false,
	}

	manager := NewModelManager(logger, router, nil)

	// Create 3 mock instances directly (bypass provider factory)
	instance1 := &ModelInstance{
		Config: config.ModelInstance{
			ID:        "instance-1",
			ModelName: "test-model",
			Priority:  100,
			Enabled:   true,
			Provider:  config.ProviderParams{Type: "mock", Model: "mock-gpt-4"},
			Timeout:   5 * time.Second,
		},
		Provider: &MockFailingProvider{failCount: 2}, // Fails twice
	}
	instance1.Healthy.Store(true) // Mark as healthy
	
	instance2 := &ModelInstance{
		Config: config.ModelInstance{
			ID:        "instance-2",
			ModelName: "test-model",
			Priority:  90,
			Enabled:   true,
			Provider:  config.ProviderParams{Type: "mock", Model: "mock-gpt-4"},
			Timeout:   5 * time.Second,
		},
		Provider: &MockFailingProvider{failCount: 1}, // Fails once
	}
	instance2.Healthy.Store(true) // Mark as healthy
	
	instance3 := &ModelInstance{
		Config: config.ModelInstance{
			ID:        "instance-3",
			ModelName: "test-model",
			Priority:  80,
			Enabled:   true,
			Provider:  config.ProviderParams{Type: "mock", Model: "mock-gpt-4"},
			Timeout:   5 * time.Second,
		},
		Provider: &MockFailingProvider{failCount: 0}, // Always succeeds
	}
	instance3.Healthy.Store(true) // Mark as healthy

	// Register instances directly in registry
	manager.registry.mu.Lock()
	manager.registry.instances["instance-1"] = instance1
	manager.registry.instances["instance-2"] = instance2
	manager.registry.instances["instance-3"] = instance3
	manager.registry.modelMap["test-model"] = []*ModelInstance{instance1, instance2, instance3}
	manager.registry.mu.Unlock()

	// Execute with failover
	result, err := manager.ExecuteWithFailover(context.Background(), &FailoverRequest{
		ModelName: "test-model",
		ExecuteFunc: func(ctx context.Context, instance *ModelInstance) (interface{}, error) {
			req := &providers.ChatRequest{
				Model:    instance.Config.Provider.Model,
				Messages: []providers.Message{{Role: "user", Content: "test"}},
			}
			return instance.Provider.ChatCompletion(ctx, req)
		},
	})

	require.NoError(t, err, "Request should succeed after instance failover")
	assert.NotNil(t, result)
	assert.Greater(t, result.AttemptCount, 1, "Should have made multiple attempts")
	assert.NotEmpty(t, result.Failovers, "Should have recorded failovers")

	response := result.Response.(*providers.ChatResponse)
	assert.Equal(t, "Success!", response.Choices[0].Message.Content)
}

// TestModelLevelFailback tests falling back to a different model when all instances fail
func TestModelLevelFallback(t *testing.T) {
	logger := zap.NewNop()

	// Create router settings with model fallback enabled
	router := config.RouterSettings{
		RoutingStrategy:       "priority",
		EnableFailover:        true,
		InstanceRetryAttempts: 2,
		EnableModelFallback:   true,
		ModelFallbacks: map[string]string{
			"primary-model":  "fallback-model",
			"fallback-model": "last-resort-model",
		},
	}

	manager := NewModelManager(logger, router, nil)

	// Create mock instances directly
	primary1 := &ModelInstance{
		Config: config.ModelInstance{
			ID:        "primary-1",
			ModelName: "primary-model",
			Priority:  100,
			Provider:  config.ProviderParams{Type: "mock", Model: "mock-gpt-4"},
			Timeout:   5 * time.Second,
		},
		Provider: &MockFailingProvider{failCount: 999}, // Always fails
	}
	primary1.Healthy.Store(true)
	
	primary2 := &ModelInstance{
		Config: config.ModelInstance{
			ID:        "primary-2",
			ModelName: "primary-model",
			Priority:  90,
			Provider:  config.ProviderParams{Type: "mock", Model: "mock-gpt-4"},
			Timeout:   5 * time.Second,
		},
		Provider: &MockFailingProvider{failCount: 999}, // Always fails
	}
	primary2.Healthy.Store(true)
	
	fallback1 := &ModelInstance{
		Config: config.ModelInstance{
			ID:        "fallback-1",
			ModelName: "fallback-model",
			Priority:  100,
			Provider:  config.ProviderParams{Type: "mock", Model: "mock-gpt-3.5"},
			Timeout:   5 * time.Second,
		},
		Provider: &MockFailingProvider{failCount: 0}, // Always succeeds
	}
	fallback1.Healthy.Store(true)

	// Register instances
	manager.registry.mu.Lock()
	manager.registry.instances["primary-1"] = primary1
	manager.registry.instances["primary-2"] = primary2
	manager.registry.instances["fallback-1"] = fallback1
	manager.registry.modelMap["primary-model"] = []*ModelInstance{primary1, primary2}
	manager.registry.modelMap["fallback-model"] = []*ModelInstance{fallback1}
	manager.registry.mu.Unlock()

	// Execute with failover
	result, err := manager.ExecuteWithFailover(context.Background(), &FailoverRequest{
		ModelName: "primary-model",
		ExecuteFunc: func(ctx context.Context, instance *ModelInstance) (interface{}, error) {
			req := &providers.ChatRequest{
				Model:    instance.Config.Provider.Model,
				Messages: []providers.Message{{Role: "user", Content: "test"}},
			}
			return instance.Provider.ChatCompletion(ctx, req)
		},
	})

	require.NoError(t, err, "Request should succeed after model fallback")
	assert.NotNil(t, result)
	assert.Equal(t, "fallback-1", result.Instance.Config.ID, "Should have used fallback model")
	assert.Greater(t, len(result.Failovers), 2, "Should have recorded multiple failovers")

	// Check that failovers include both instance and model failures
	hasModelFailover := false
	for _, failover := range result.Failovers {
		if len(failover) > 5 && failover[:5] == "model" {
			hasModelFailover = true
			break
		}
	}
	assert.True(t, hasModelFailover, "Should have recorded model-level failover")
}

// TestFailoverDisabled tests that failover doesn't happen when disabled
func TestFailoverDisabled(t *testing.T) {
	logger := zap.NewNop()

	// Create router settings with failover disabled
	router := config.RouterSettings{
		RoutingStrategy: "priority",
		EnableFailover:  false,
	}

	manager := NewModelManager(logger, router, nil)

	// Create mock instance directly
	instance1 := &ModelInstance{
		Config: config.ModelInstance{
			ID:        "instance-1",
			ModelName: "test-model",
			Priority:  100,
			Provider:  config.ProviderParams{Type: "mock", Model: "mock-gpt-4"},
			Timeout:   5 * time.Second,
		},
		Provider: &MockFailingProvider{failCount: 999}, // Always fails
	}
	instance1.Healthy.Store(true)

	// Register instance
	manager.registry.mu.Lock()
	manager.registry.instances["instance-1"] = instance1
	manager.registry.modelMap["test-model"] = []*ModelInstance{instance1}
	manager.registry.mu.Unlock()

	// Execute with failover (but it's disabled)
	result, err := manager.ExecuteWithFailover(context.Background(), &FailoverRequest{
		ModelName: "test-model",
		ExecuteFunc: func(ctx context.Context, instance *ModelInstance) (interface{}, error) {
			req := &providers.ChatRequest{
				Model:    instance.Config.Provider.Model,
				Messages: []providers.Message{{Role: "user", Content: "test"}},
			}
			return instance.Provider.ChatCompletion(ctx, req)
		},
	})

	require.Error(t, err, "Request should fail when failover is disabled")
	assert.Nil(t, result)
}

// TestTransparentFailover simulates the end-user experience
// User doesn't see errors, just a successful response (albeit slower)
func TestTransparentFailover(t *testing.T) {
	logger := zap.NewNop()

	router := config.RouterSettings{
		RoutingStrategy:       "priority",
		EnableFailover:        true,
		InstanceRetryAttempts: 3,
	}

	manager := NewModelManager(logger, router, nil)

	// Create mock instances directly
	slowInstance := &ModelInstance{
		Config: config.ModelInstance{
			ID:        "slow-instance",
			ModelName: "test-model",
			Priority:  100,
			Provider:  config.ProviderParams{Type: "mock", Model: "mock-gpt-4"},
			Timeout:   5 * time.Second,
		},
		Provider: &MockFailingProvider{failCount: 999}, // Fails
	}
	slowInstance.Healthy.Store(true)
	
	fastInstance := &ModelInstance{
		Config: config.ModelInstance{
			ID:        "fast-instance",
			ModelName: "test-model",
			Priority:  90,
			Provider:  config.ProviderParams{Type: "mock", Model: "mock-gpt-4"},
			Timeout:   5 * time.Second,
		},
		Provider: &MockFailingProvider{failCount: 0}, // Succeeds
	}
	fastInstance.Healthy.Store(true)

	// Register instances
	manager.registry.mu.Lock()
	manager.registry.instances["slow-instance"] = slowInstance
	manager.registry.instances["fast-instance"] = fastInstance
	manager.registry.modelMap["test-model"] = []*ModelInstance{slowInstance, fastInstance}
	manager.registry.mu.Unlock()

	// Measure total time
	start := time.Now()

	// Execute - from user perspective, this should just work
	result, err := manager.ExecuteWithFailover(context.Background(), &FailoverRequest{
		ModelName: "test-model",
		ExecuteFunc: func(ctx context.Context, instance *ModelInstance) (interface{}, error) {
			req := &providers.ChatRequest{
				Model:    instance.Config.Provider.Model,
				Messages: []providers.Message{{Role: "user", Content: "Hello!"}},
			}
			return instance.Provider.ChatCompletion(ctx, req)
		},
	})

	elapsed := time.Since(start)

	// User gets a successful response
	require.NoError(t, err)
	assert.NotNil(t, result)

	response := result.Response.(*providers.ChatResponse)
	assert.Equal(t, "Success!", response.Choices[0].Message.Content)

	// Response is slower because we tried failing instance first
	// But user doesn't see any error - just gets the result
	t.Logf("Request completed in %v with %d attempts and %d failovers",
		elapsed, result.AttemptCount, len(result.Failovers))
	t.Logf("Failover chain: %v", result.Failovers)

	assert.Greater(t, result.AttemptCount, 1, "Should have retried")
}
