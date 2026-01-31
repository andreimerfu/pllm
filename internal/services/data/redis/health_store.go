package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// HealthCheckResult represents the result of a single instance health check.
type HealthCheckResult struct {
	InstanceID   string    `json:"instance_id"`
	ModelName    string    `json:"model_name"`
	ProviderType string    `json:"provider_type"`
	Healthy      bool      `json:"healthy"`
	LatencyMs    int64     `json:"latency_ms"`
	Error        string    `json:"error,omitempty"`
	CheckedAt    time.Time `json:"checked_at"`
}

// ModelHealthSummary aggregates health results for all instances of a model.
type ModelHealthSummary struct {
	ModelName     string              `json:"model_name"`
	Healthy       bool                `json:"healthy"`
	HealthyCount  int                 `json:"healthy_count"`
	TotalCount    int                 `json:"total_count"`
	AvgLatencyMs  int64               `json:"avg_latency_ms"`
	LastCheckedAt time.Time           `json:"last_checked_at"`
	Instances     []HealthCheckResult `json:"instances"`
}

// HealthStore persists and retrieves health check results in Redis.
type HealthStore struct {
	client *redis.Client
	logger *zap.Logger
	ttl    time.Duration
}

// NewHealthStore creates a new HealthStore.
func NewHealthStore(client *redis.Client, logger *zap.Logger) *HealthStore {
	return &HealthStore{
		client: client,
		logger: logger,
		ttl:    5 * time.Minute,
	}
}

// StoreResult persists a single health check result.
func (s *HealthStore) StoreResult(ctx context.Context, result HealthCheckResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal health result: %w", err)
	}

	instanceKey := s.instanceKey(result.InstanceID)
	modelSetKey := s.modelSetKey(result.ModelName)

	pipe := s.client.Pipeline()
	pipe.Set(ctx, instanceKey, data, s.ttl)
	pipe.SAdd(ctx, modelSetKey, result.InstanceID)
	pipe.Expire(ctx, modelSetKey, s.ttl)

	_, err = pipe.Exec(ctx)
	if err != nil {
		s.logger.Error("Failed to store health check result",
			zap.String("instance", result.InstanceID),
			zap.Error(err))
		return err
	}
	return nil
}

// GetResult returns the health check result for a single instance.
func (s *HealthStore) GetResult(ctx context.Context, instanceID string) (*HealthCheckResult, error) {
	data, err := s.client.Get(ctx, s.instanceKey(instanceID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var result HealthCheckResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetModelHealth returns aggregated health for all instances of a model.
func (s *HealthStore) GetModelHealth(ctx context.Context, modelName string) (*ModelHealthSummary, error) {
	instanceIDs, err := s.client.SMembers(ctx, s.modelSetKey(modelName)).Result()
	if err != nil {
		return nil, err
	}

	summary := &ModelHealthSummary{
		ModelName: modelName,
		Healthy:   true,
		Instances: make([]HealthCheckResult, 0, len(instanceIDs)),
	}

	if len(instanceIDs) == 0 {
		return summary, nil
	}

	// Fetch all instance results via pipeline
	pipe := s.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(instanceIDs))
	for i, id := range instanceIDs {
		cmds[i] = pipe.Get(ctx, s.instanceKey(id))
	}
	_, _ = pipe.Exec(ctx) // some keys may have expired

	var totalLatency int64
	var latestCheck time.Time

	for _, cmd := range cmds {
		data, err := cmd.Bytes()
		if err != nil {
			continue // expired or missing
		}

		var result HealthCheckResult
		if err := json.Unmarshal(data, &result); err != nil {
			continue
		}

		summary.Instances = append(summary.Instances, result)
		summary.TotalCount++
		if result.Healthy {
			summary.HealthyCount++
		}
		totalLatency += result.LatencyMs
		if result.CheckedAt.After(latestCheck) {
			latestCheck = result.CheckedAt
		}
	}

	if summary.TotalCount > 0 {
		summary.AvgLatencyMs = totalLatency / int64(summary.TotalCount)
		summary.Healthy = summary.HealthyCount > 0
	}
	summary.LastCheckedAt = latestCheck

	return summary, nil
}

// GetAllModelsHealth returns health summaries for the given model names.
func (s *HealthStore) GetAllModelsHealth(ctx context.Context, modelNames []string) (map[string]*ModelHealthSummary, error) {
	result := make(map[string]*ModelHealthSummary, len(modelNames))
	for _, name := range modelNames {
		summary, err := s.GetModelHealth(ctx, name)
		if err != nil {
			s.logger.Warn("Failed to get health for model",
				zap.String("model", name),
				zap.Error(err))
			continue
		}
		result[name] = summary
	}
	return result, nil
}

func (s *HealthStore) instanceKey(instanceID string) string {
	return fmt.Sprintf("pllm:health:instance:%s", instanceID)
}

func (s *HealthStore) modelSetKey(modelName string) string {
	return fmt.Sprintf("pllm:health:model:%s:instances", modelName)
}
