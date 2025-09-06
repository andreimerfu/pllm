package realtime

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/amerfu/pllm/internal/models"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SessionManager manages active realtime sessions
type SessionManager struct {
	sessions map[string]*Session
	mutex    sync.RWMutex
	logger   *zap.Logger
	db       *gorm.DB
	
	// Configuration
	maxSessions     int
	sessionTimeout  time.Duration
	cleanupInterval time.Duration
}

// Session represents an active WebSocket session
type Session struct {
	ID          string
	ModelName   string
	UserID      *uint
	TeamID      *uint  
	KeyID       *uint
	WebSocket   *websocket.Conn
	Events      chan *models.RealtimeEvent
	Stop        chan struct{}
	Config      *models.RealtimeSessionConfig
	CreatedAt   time.Time
	LastActivity time.Time
	
	// Provider connection
	ProviderConn *websocket.Conn
	
	// Internal state
	mutex sync.RWMutex
}

// SessionConfig holds session manager configuration
type SessionConfig struct {
	MaxSessions     int           `yaml:"max_sessions" mapstructure:"max_sessions"`
	SessionTimeout  time.Duration `yaml:"session_timeout" mapstructure:"session_timeout"`
	CleanupInterval time.Duration `yaml:"cleanup_interval" mapstructure:"cleanup_interval"`
}

// NewSessionManager creates a new session manager
func NewSessionManager(logger *zap.Logger, db *gorm.DB, config *SessionConfig) *SessionManager {
	if config == nil {
		config = &SessionConfig{
			MaxSessions:     100,
			SessionTimeout:  30 * time.Minute,
			CleanupInterval: 5 * time.Minute,
		}
	}

	sm := &SessionManager{
		sessions:        make(map[string]*Session),
		logger:          logger,
		db:              db,
		maxSessions:     config.MaxSessions,
		sessionTimeout:  config.SessionTimeout,
		cleanupInterval: config.CleanupInterval,
	}

	// Start cleanup goroutine
	go sm.cleanupSessions()

	return sm
}

// CreateSession creates a new realtime session
func (sm *SessionManager) CreateSession(
	sessionID, modelName string,
	ws *websocket.Conn,
	userID, teamID, keyID *uint,
) (*Session, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Check session limits
	if len(sm.sessions) >= sm.maxSessions {
		return nil, fmt.Errorf("maximum number of sessions reached")
	}

	// Check if session already exists
	if _, exists := sm.sessions[sessionID]; exists {
		return nil, fmt.Errorf("session with ID %s already exists", sessionID)
	}

	now := time.Now()
	session := &Session{
		ID:           sessionID,
		ModelName:    modelName,
		UserID:       userID,
		TeamID:       teamID,
		KeyID:        keyID,
		WebSocket:    ws,
		Events:       make(chan *models.RealtimeEvent, 100),
		Stop:         make(chan struct{}),
		CreatedAt:    now,
		LastActivity: now,
		Config: &models.RealtimeSessionConfig{
			Modalities:        []string{"text", "audio"},
			Voice:            "alloy",
			InputAudioFormat: "pcm16",
			OutputAudioFormat: "pcm16",
			TurnDetection: &models.TurnDetectionConfig{
				Type: "server_vad",
			},
		},
	}

	sm.sessions[sessionID] = session

	// Persist session to database if available
	if sm.db != nil {
		dbSession := models.NewRealtimeSession(modelName, userID, teamID, keyID)
		dbSession.ID = sessionID
		
		if configBytes, err := json.Marshal(session.Config); err == nil {
			dbSession.Config = configBytes
		}

		if err := sm.db.Create(dbSession).Error; err != nil {
			sm.logger.Error("Failed to persist session to database", 
				zap.String("session_id", sessionID),
				zap.Error(err))
		}
	}

	sm.logger.Info("Created realtime session",
		zap.String("session_id", sessionID),
		zap.String("model", modelName),
		zap.Uintp("user_id", userID),
		zap.Uintp("team_id", teamID))

	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*Session, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	session, exists := sm.sessions[sessionID]
	if exists {
		session.updateActivity()
	}
	return session, exists
}

// RemoveSession removes a session
func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	session, exists := sm.sessions[sessionID]
	if !exists {
		return
	}

	// Close session
	session.Close()
	
	// Remove from map
	delete(sm.sessions, sessionID)

	// Update database status
	if sm.db != nil {
		sm.db.Model(&models.RealtimeSession{}).
			Where("id = ?", sessionID).
			Update("status", "closed")
	}

	sm.logger.Info("Removed realtime session",
		zap.String("session_id", sessionID))
}

// ListSessions returns all active sessions
func (sm *SessionManager) ListSessions() map[string]*Session {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	// Return a copy to avoid concurrent map access
	sessions := make(map[string]*Session)
	for id, session := range sm.sessions {
		sessions[id] = session
	}
	return sessions
}

// GetSessionCount returns the number of active sessions
func (sm *SessionManager) GetSessionCount() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return len(sm.sessions)
}

// cleanupSessions removes expired sessions
func (sm *SessionManager) cleanupSessions() {
	ticker := time.NewTicker(sm.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		sm.mutex.Lock()
		
		var toRemove []string
		cutoff := time.Now().Add(-sm.sessionTimeout)
		
		for id, session := range sm.sessions {
			if session.LastActivity.Before(cutoff) {
				toRemove = append(toRemove, id)
			}
		}
		
		for _, id := range toRemove {
			session := sm.sessions[id]
			session.Close()
			delete(sm.sessions, id)
			
			sm.logger.Info("Cleaned up expired session",
				zap.String("session_id", id))
		}
		
		sm.mutex.Unlock()

		// Update database for expired sessions
		if sm.db != nil && len(toRemove) > 0 {
			sm.db.Model(&models.RealtimeSession{}).
				Where("id IN ? AND status = ?", toRemove, "active").
				Update("status", "expired")
		}
	}
}

// Close closes all sessions and stops the manager
func (sm *SessionManager) Close() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	for id, session := range sm.sessions {
		session.Close()
		delete(sm.sessions, id)
	}
	
	sm.logger.Info("Session manager closed")
}

// Session methods

// updateActivity updates the last activity timestamp
func (s *Session) updateActivity() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.LastActivity = time.Now()
}

// SendEvent sends an event to the client WebSocket
func (s *Session) SendEvent(event *models.RealtimeEvent) error {
	s.updateActivity()
	
	if s.WebSocket == nil {
		return fmt.Errorf("WebSocket connection is nil")
	}

	return s.WebSocket.WriteJSON(event)
}

// SendToProvider sends an event to the provider WebSocket
func (s *Session) SendToProvider(event *models.RealtimeEvent) error {
	if s.ProviderConn == nil {
		return fmt.Errorf("provider connection is nil")
	}

	return s.ProviderConn.WriteJSON(event)
}

// UpdateConfig updates session configuration
func (s *Session) UpdateConfig(config *models.RealtimeSessionConfig) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.Config = config
	s.updateActivity()
}

// GetConfig returns current session configuration
func (s *Session) GetConfig() *models.RealtimeSessionConfig {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.Config
}

// IsActive checks if session is still active
func (s *Session) IsActive() bool {
	select {
	case <-s.Stop:
		return false
	default:
		return true
	}
}

// Close closes the session and all connections
func (s *Session) Close() {
	// Close stop channel
	select {
	case <-s.Stop:
		// Already closed
		return
	default:
		close(s.Stop)
	}

	// Close WebSocket connections
	if s.WebSocket != nil {
		_ = s.WebSocket.Close()
	}
	if s.ProviderConn != nil {
		_ = s.ProviderConn.Close()
	}

	// Close events channel
	close(s.Events)
}