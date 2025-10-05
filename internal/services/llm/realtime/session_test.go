package realtime

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// createTestWebSocket creates a mock WebSocket connection for testing
func createTestWebSocket(tb testing.TB) *websocket.Conn {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			tb.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		
		// Keep connection alive briefly for testing
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		tb.Fatalf("Failed to create test WebSocket connection: %v", err)
	}
	
	return conn
}

func TestSessionManager_Basic(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &SessionConfig{
		MaxSessions:     10,
		SessionTimeout:  5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	
	manager := NewSessionManager(logger, nil, config)
	defer manager.Close()

	// Test initial state
	assert.Equal(t, 0, manager.GetSessionCount())

	// Create test WebSocket connection
	ws := createTestWebSocket(t)
	defer func() { _ = ws.Close() }()

	// Test session creation
	userID := uint(123)
	teamID := uint(456)
	keyID := uint(789)

	session, err := manager.CreateSession(
		"test-session-1", 
		"gpt-4-realtime-preview", 
		ws, 
		&userID, 
		&teamID, 
		&keyID,
	)
	
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, "test-session-1", session.ID)
	assert.Equal(t, "gpt-4-realtime-preview", session.ModelName)
	assert.Equal(t, &userID, session.UserID)
	assert.Equal(t, &teamID, session.TeamID)
	assert.Equal(t, &keyID, session.KeyID)

	// Test session count
	assert.Equal(t, 1, manager.GetSessionCount())

	// Test session retrieval
	retrievedSession, exists := manager.GetSession("test-session-1")
	assert.True(t, exists)
	assert.Equal(t, session.ID, retrievedSession.ID)

	// Test non-existent session
	_, exists = manager.GetSession("non-existent")
	assert.False(t, exists)

	// Test session listing
	sessions := manager.ListSessions()
	assert.Len(t, sessions, 1)

	// Test session removal
	manager.RemoveSession("test-session-1")
	assert.Equal(t, 0, manager.GetSessionCount())

	_, exists = manager.GetSession("test-session-1")
	assert.False(t, exists)
}

func TestSessionManager_DuplicateSession(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &SessionConfig{
		MaxSessions:     10,
		SessionTimeout:  5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	
	manager := NewSessionManager(logger, nil, config)
	defer manager.Close()

	ws1 := createTestWebSocket(t)
	defer func() { _ = ws1.Close() }()
	
	ws2 := createTestWebSocket(t)
	defer func() { _ = ws2.Close() }()

	userID := uint(123)
	teamID := uint(456)
	keyID := uint(789)

	// Create first session
	_, err := manager.CreateSession("duplicate-test", "gpt-4-realtime-preview", ws1, &userID, &teamID, &keyID)
	require.NoError(t, err)

	// Try to create duplicate session
	_, err = manager.CreateSession("duplicate-test", "gpt-4-realtime-preview", ws2, &userID, &teamID, &keyID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestSessionManager_MaxSessions(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &SessionConfig{
		MaxSessions:     2, // Limit to 2 sessions for testing
		SessionTimeout:  5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	
	manager := NewSessionManager(logger, nil, config)
	defer manager.Close()

	userID := uint(123)
	teamID := uint(456)
	keyID := uint(789)

	// Create first session
	ws1 := createTestWebSocket(t)
	defer func() { _ = ws1.Close() }()
	_, err := manager.CreateSession("session-1", "gpt-4-realtime-preview", ws1, &userID, &teamID, &keyID)
	require.NoError(t, err)

	// Create second session
	ws2 := createTestWebSocket(t)
	defer func() { _ = ws2.Close() }()
	_, err = manager.CreateSession("session-2", "gpt-4-realtime-preview", ws2, &userID, &teamID, &keyID)
	require.NoError(t, err)

	// Try to create third session (should fail)
	ws3 := createTestWebSocket(t)
	defer func() { _ = ws3.Close() }()
	_, err = manager.CreateSession("session-3", "gpt-4-realtime-preview", ws3, &userID, &teamID, &keyID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum number of sessions reached")
}

func TestSession_UpdateActivity(t *testing.T) {
	ws := createTestWebSocket(t)
	defer func() { _ = ws.Close() }()

	userID := uint(123)
	now := time.Now()
	
	session := &Session{
		ID:           "test-session",
		ModelName:    "gpt-4-realtime-preview",
		UserID:       &userID,
		WebSocket:    ws,
		CreatedAt:    now,
		LastActivity: now,
	}

	// Wait a bit and update activity
	time.Sleep(10 * time.Millisecond)
	originalTime := session.LastActivity
	session.updateActivity()

	assert.True(t, session.LastActivity.After(originalTime))
}

func TestRedisSessionStore_Basic(t *testing.T) {
	// This is a unit test for the Redis session store without requiring actual Redis
	// In real scenarios, you'd use a Redis test container or mock
	
	sessionState := &SessionState{
		ID:           "test-session-id",
		UserID:       "user123",
		TeamID:       "team456", 
		Model:        "gpt-4-realtime-preview",
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		PodID:        "pod-abc123",
		Status:       "active",
	}

	// Test basic struct creation and field access
	assert.Equal(t, "test-session-id", sessionState.ID)
	assert.Equal(t, "user123", sessionState.UserID)
	assert.Equal(t, "gpt-4-realtime-preview", sessionState.Model)
	assert.Equal(t, "active", sessionState.Status)
}

// Benchmark tests
func BenchmarkSessionManager_CreateSession(b *testing.B) {
	logger := zaptest.NewLogger(b)
	config := &SessionConfig{
		MaxSessions:     10000,
		SessionTimeout:  30 * time.Minute,
		CleanupInterval: 5 * time.Minute,
	}
	
	manager := NewSessionManager(logger, nil, config)
	defer manager.Close()

	userID := uint(123)
	teamID := uint(456)
	keyID := uint(789)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID := "session-" + string(rune(i%1000)) // Reuse session IDs to avoid "already exists" errors
		
		// Remove session if it exists
		manager.RemoveSession(sessionID)
		
		ws := createTestWebSocket(b)
		_, err := manager.CreateSession(sessionID, "gpt-4-realtime-preview", ws, &userID, &teamID, &keyID)
		if err != nil {
			b.Fatalf("Failed to create session: %v", err)
		}
		_ = ws.Close()
	}
}

func BenchmarkSessionManager_GetSession(b *testing.B) {
	logger := zaptest.NewLogger(b)
	config := &SessionConfig{
		MaxSessions:     100,
		SessionTimeout:  30 * time.Minute,
		CleanupInterval: 5 * time.Minute,
	}
	
	manager := NewSessionManager(logger, nil, config)
	defer manager.Close()

	// Create a test session
	ws := createTestWebSocket(b)
	defer func() { _ = ws.Close() }()
	
	userID := uint(123)
	teamID := uint(456)
	keyID := uint(789)
	
	session, err := manager.CreateSession("benchmark-session", "gpt-4-realtime-preview", ws, &userID, &teamID, &keyID)
	if err != nil {
		b.Fatalf("Failed to create test session: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, exists := manager.GetSession(session.ID)
		if !exists {
			b.Fatal("Session not found")
		}
	}
}