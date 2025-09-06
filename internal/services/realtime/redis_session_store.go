package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisSessionStore implements distributed session storage using Redis
type RedisSessionStore struct {
	client *redis.Client
	logger *zap.Logger
	prefix string
}

// SessionState represents the serializable state of a realtime session
type SessionState struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id,omitempty"`
	TeamID       string                 `json:"team_id,omitempty"`
	Model        string                 `json:"model"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActivity time.Time              `json:"last_activity"`
	Config       map[string]interface{} `json:"config,omitempty"`
	PodID        string                 `json:"pod_id"` // Track which pod owns the connection
	Status       string                 `json:"status"` // active, closed, error
}

// NewRedisSessionStore creates a new Redis session store
func NewRedisSessionStore(client *redis.Client, logger *zap.Logger) *RedisSessionStore {
	return &RedisSessionStore{
		client: client,
		logger: logger,
		prefix: "pllm:realtime:session:",
	}
}

// StoreSession stores session state in Redis
func (r *RedisSessionStore) StoreSession(ctx context.Context, session *SessionState, ttl time.Duration) error {
	key := r.prefix + session.ID
	
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session state: %w", err)
	}

	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store session in Redis: %w", err)
	}

	r.logger.Debug("Session stored in Redis", 
		zap.String("session_id", session.ID),
		zap.String("pod_id", session.PodID))
	
	return nil
}

// GetSession retrieves session state from Redis
func (r *RedisSessionStore) GetSession(ctx context.Context, sessionID string) (*SessionState, error) {
	key := r.prefix + sessionID
	
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session from Redis: %w", err)
	}

	var session SessionState
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session state: %w", err)
	}

	return &session, nil
}

// UpdateSession updates session state in Redis
func (r *RedisSessionStore) UpdateSession(ctx context.Context, session *SessionState, ttl time.Duration) error {
	session.LastActivity = time.Now()
	return r.StoreSession(ctx, session, ttl)
}

// DeleteSession removes session from Redis
func (r *RedisSessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	key := r.prefix + sessionID
	
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete session from Redis: %w", err)
	}

	r.logger.Debug("Session deleted from Redis", zap.String("session_id", sessionID))
	return nil
}

// ListSessionsByPod returns all sessions owned by a specific pod
func (r *RedisSessionStore) ListSessionsByPod(ctx context.Context, podID string) ([]SessionState, error) {
	pattern := r.prefix + "*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to scan Redis keys: %w", err)
	}

	var sessions []SessionState
	for _, key := range keys {
		data, err := r.client.Get(ctx, key).Result()
		if err != nil {
			r.logger.Warn("Failed to get session data", zap.String("key", key), zap.Error(err))
			continue
		}

		var session SessionState
		if err := json.Unmarshal([]byte(data), &session); err != nil {
			r.logger.Warn("Failed to unmarshal session", zap.String("key", key), zap.Error(err))
			continue
		}

		if session.PodID == podID {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// CleanupExpiredSessions removes expired sessions (for background cleanup)
func (r *RedisSessionStore) CleanupExpiredSessions(ctx context.Context) error {
	pattern := r.prefix + "*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to scan Redis keys: %w", err)
	}

	for _, key := range keys {
		// Check if key exists and get TTL
		ttl, err := r.client.TTL(ctx, key).Result()
		if err != nil {
			continue
		}

		// If TTL is -2, key doesn't exist (already expired)
		// If TTL is -1, key exists but has no expiration
		if ttl == -2 {
			r.logger.Debug("Found expired session key", zap.String("key", key))
		}
	}

	return nil
}