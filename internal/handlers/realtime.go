package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/amerfu/pllm/internal/models"
	modelsService "github.com/amerfu/pllm/internal/services/models"
	"github.com/amerfu/pllm/internal/services/providers"
	"github.com/amerfu/pllm/internal/services/realtime"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// RealtimeHandler handles WebSocket realtime connections
type RealtimeHandler struct {
	logger         *zap.Logger
	sessionManager *realtime.SessionManager
	modelManager   *modelsService.ModelManager
	upgrader       websocket.Upgrader
}

// RealtimeHandlerConfig holds configuration for the realtime handler
type RealtimeHandlerConfig struct {
	ReadBufferSize    int           `yaml:"read_buffer_size" mapstructure:"read_buffer_size"`
	WriteBufferSize   int           `yaml:"write_buffer_size" mapstructure:"write_buffer_size"`
	HandshakeTimeout  time.Duration `yaml:"handshake_timeout" mapstructure:"handshake_timeout"`
	CheckOrigin       bool          `yaml:"check_origin" mapstructure:"check_origin"`
	EnableCompression bool          `yaml:"enable_compression" mapstructure:"enable_compression"`
}

// NewRealtimeHandler creates a new realtime handler
func NewRealtimeHandler(
	logger *zap.Logger,
	sessionManager *realtime.SessionManager,
	modelManager *modelsService.ModelManager,
	config *RealtimeHandlerConfig,
) *RealtimeHandler {
	if config == nil {
		config = &RealtimeHandlerConfig{
			ReadBufferSize:    4096,
			WriteBufferSize:   4096,
			HandshakeTimeout:  45 * time.Second,
			CheckOrigin:       false,
			EnableCompression: true,
		}
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:    config.ReadBufferSize,
		WriteBufferSize:   config.WriteBufferSize,
		HandshakeTimeout:  config.HandshakeTimeout,
		EnableCompression: config.EnableCompression,
		CheckOrigin: func(r *http.Request) bool {
			if config.CheckOrigin {
				// Implement origin checking logic here
				// For now, allow all origins in development
				return true
			}
			return true
		},
	}

	return &RealtimeHandler{
		logger:         logger,
		sessionManager: sessionManager,
		modelManager:   modelManager,
		upgrader:       upgrader,
	}
}

// ConnectRealtime handles WebSocket upgrade for realtime connections
// @Summary Connect to realtime API
// @Description Establish WebSocket connection for real-time conversation
// @Tags realtime
// @Accept json
// @Produce json
// @Param model query string true "Model name for realtime session"
// @Security BearerAuth
// @Security ApiKeyAuth
// @Success 101 {string} string "Switching Protocols"
// @Failure 400 {object} ErrorResponse "Bad Request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 500 {object} ErrorResponse "Internal Server Error"
// @Router /v1/realtime [get]
func (h *RealtimeHandler) ConnectRealtime(w http.ResponseWriter, r *http.Request) {
	// Get model from query parameters
	model := r.URL.Query().Get("model")
	if model == "" {
		http.Error(w, "model parameter is required", http.StatusBadRequest)
		return
	}

	// Extract authentication context (should be set by auth middleware)
	var userID, teamID, keyID *uint
	// TODO: Extract from auth context when auth is implemented
	// For now, allow anonymous connections for testing

	// Generate session ID
	sessionID := uuid.New().String()

	h.logger.Info("Realtime connection request",
		zap.String("session_id", sessionID),
		zap.String("model", model),
		zap.String("remote_addr", r.RemoteAddr))

	// Upgrade HTTP connection to WebSocket
	clientConn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade connection to WebSocket",
			zap.String("session_id", sessionID),
			zap.Error(err))
		return
	}
	defer func() { _ = clientConn.Close() }()

	// Get model instance for realtime
	instance, err := h.modelManager.GetBestInstanceAdaptive(r.Context(), model)
	if err != nil {
		h.logger.Error("Failed to get model instance for realtime",
			zap.String("session_id", sessionID),
			zap.String("model", model),
			zap.Error(err))
		h.sendErrorToClient(clientConn, "model_not_available", fmt.Sprintf("Model %s is not available", model), "")
		return
	}

	// Check if provider supports realtime
	realtimeProvider, err := providers.GetRealtimeProvider(instance.Provider)
	if err != nil {
		h.logger.Error("Provider does not support realtime API",
			zap.String("session_id", sessionID),
			zap.String("model", model),
			zap.String("provider", instance.Provider.GetType()),
			zap.Error(err))
		h.sendErrorToClient(clientConn, "realtime_not_supported", "Provider does not support realtime API", "")
		return
	}

	// Create session
	session, err := h.sessionManager.CreateSession(sessionID, model, clientConn, userID, teamID, keyID)
	if err != nil {
		h.logger.Error("Failed to create realtime session",
			zap.String("session_id", sessionID),
			zap.Error(err))
		h.sendErrorToClient(clientConn, "session_creation_failed", "Failed to create session", "")
		return
	}
	defer h.sessionManager.RemoveSession(sessionID)

	// Connect to provider's realtime API
	providerConfig := &providers.RealtimeConnectionConfig{
		Model:   instance.Config.Provider.Model, // Use provider's model name
		APIKey:  instance.Config.Provider.APIKey,
		BaseURL: instance.Config.Provider.BaseURL,
		Session: session.GetConfig(),
	}

	providerConn, err := realtimeProvider.ConnectRealtime(r.Context(), providerConfig)
	if err != nil {
		h.logger.Error("Failed to connect to provider realtime API",
			zap.String("session_id", sessionID),
			zap.String("provider", instance.Provider.GetType()),
			zap.Error(err))
		h.sendErrorToClient(clientConn, "provider_connection_failed", "Failed to connect to provider", "")
		return
	}
	defer func() { _ = providerConn.Close() }()

	// Store provider connection in session
	session.ProviderConn = providerConn

	h.logger.Info("Established realtime connections",
		zap.String("session_id", sessionID),
		zap.String("model", model),
		zap.String("provider", instance.Provider.GetType()))

	// Create message handler for bidirectional routing
	messageHandler := providers.NewRealtimeMessageHandler(
		h.logger,
		clientConn,
		providerConn,
		session.GetConfig(),
		model,
	)

	// Start message routing
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	if err := messageHandler.Start(ctx); err != nil {
		h.logger.Error("Failed to start message handler",
			zap.String("session_id", sessionID),
			zap.Error(err))
		h.sendErrorToClient(clientConn, "message_handler_failed", "Failed to start message handler", "")
		return
	}
	defer messageHandler.Stop()

	// Send session created event to client
	sessionCreated := &models.SessionCreatedEvent{
		Session: *session.GetConfig(),
	}
	
	event, err := models.NewRealtimeEvent("session.created", sessionCreated)
	if err != nil {
		h.logger.Error("Failed to create session.created event",
			zap.String("session_id", sessionID),
			zap.Error(err))
	} else {
		if err := session.SendEvent(event); err != nil {
			h.logger.Error("Failed to send session.created event",
				zap.String("session_id", sessionID),
				zap.Error(err))
		}
	}

	// Keep connection alive until context is cancelled or connection is closed
	<-ctx.Done()

	h.logger.Info("Realtime session ended",
		zap.String("session_id", sessionID),
		zap.String("model", model))
}

// CreateSession creates a new realtime session (HTTP endpoint)
// @Summary Create realtime session
// @Description Create a new realtime session with configuration
// @Tags realtime
// @Accept json
// @Produce json
// @Param request body CreateSessionRequest true "Session configuration"
// @Security BearerAuth
// @Security ApiKeyAuth
// @Success 201 {object} CreateSessionResponse "Session created"
// @Failure 400 {object} ErrorResponse "Bad Request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal Server Error"
// @Router /v1/realtime/sessions [post]
func (h *RealtimeHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Model == "" {
		http.Error(w, "model is required", http.StatusBadRequest)
		return
	}

	// Check if model exists and supports realtime
	instance, err := h.modelManager.GetBestInstanceAdaptive(r.Context(), req.Model)
	if err != nil {
		http.Error(w, fmt.Sprintf("Model %s is not available", req.Model), http.StatusBadRequest)
		return
	}

	if !providers.ProviderSupportsRealtime(instance.Provider) {
		http.Error(w, fmt.Sprintf("Model %s does not support realtime API", req.Model), http.StatusBadRequest)
		return
	}

	// Generate session ID
	sessionID := uuid.New().String()

	// Create session configuration
	config := &models.RealtimeSessionConfig{
		Modalities:        []string{"text", "audio"},
		Voice:            "alloy",
		InputAudioFormat: "pcm16",
		OutputAudioFormat: "pcm16",
		TurnDetection: &models.TurnDetectionConfig{
			Type: "server_vad",
		},
	}

	// Apply request overrides
	if req.Config != nil {
		if req.Config.Voice != "" {
			config.Voice = req.Config.Voice
		}
		if len(req.Config.Modalities) > 0 {
			config.Modalities = req.Config.Modalities
		}
		if req.Config.Instructions != "" {
			config.Instructions = req.Config.Instructions
		}
		if req.Config.TurnDetection != nil {
			config.TurnDetection = req.Config.TurnDetection
		}
		if req.Config.Temperature != nil {
			config.Temperature = req.Config.Temperature
		}
	}

	// Create response
	response := &CreateSessionResponse{
		ID:        sessionID,
		Model:     req.Model,
		CreatedAt: time.Now().Unix(),
		Config:    config,
		Status:    "created",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)

	h.logger.Info("Created realtime session via HTTP",
		zap.String("session_id", sessionID),
		zap.String("model", req.Model))
}

// GetSession retrieves session information
// @Summary Get realtime session
// @Description Get information about a realtime session
// @Tags realtime
// @Produce json
// @Param id path string true "Session ID"
// @Security BearerAuth
// @Security ApiKeyAuth
// @Success 200 {object} GetSessionResponse "Session information"
// @Failure 404 {object} ErrorResponse "Session not found"
// @Failure 500 {object} ErrorResponse "Internal Server Error"
// @Router /v1/realtime/sessions/{id} [get]
func (h *RealtimeHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		http.Error(w, "session ID is required", http.StatusBadRequest)
		return
	}

	session, exists := h.sessionManager.GetSession(sessionID)
	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	response := &GetSessionResponse{
		ID:           session.ID,
		Model:        session.ModelName,
		CreatedAt:    session.CreatedAt.Unix(),
		LastActivity: session.LastActivity.Unix(),
		Config:       session.GetConfig(),
		Status:       "active",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// ListSessions lists all active sessions
// @Summary List realtime sessions
// @Description List all active realtime sessions
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Security ApiKeyAuth
// @Success 200 {object} ListSessionsResponse "List of sessions"
// @Failure 500 {object} ErrorResponse "Internal Server Error"
// @Router /v1/realtime/sessions [get]
func (h *RealtimeHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := h.sessionManager.ListSessions()
	
	var sessionList []GetSessionResponse
	for _, session := range sessions {
		sessionList = append(sessionList, GetSessionResponse{
			ID:           session.ID,
			Model:        session.ModelName,
			CreatedAt:    session.CreatedAt.Unix(),
			LastActivity: session.LastActivity.Unix(),
			Config:       session.GetConfig(),
			Status:       "active",
		})
	}

	response := &ListSessionsResponse{
		Sessions: sessionList,
		Count:    len(sessionList),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// sendErrorToClient sends an error event to the WebSocket client
func (h *RealtimeHandler) sendErrorToClient(conn *websocket.Conn, code, message, param string) {
	errorEvent := &models.ErrorEvent{
		Type:    "error",
		Code:    code,
		Message: message,
		Param:   param,
		EventID: uuid.New().String(),
	}

	event, err := models.NewRealtimeEvent("error", errorEvent)
	if err != nil {
		h.logger.Error("Failed to create error event", zap.Error(err))
		return
	}

	if err := conn.WriteJSON(event); err != nil {
		h.logger.Error("Failed to send error to client", zap.Error(err))
	}
}

// Request/Response types

// CreateSessionRequest represents a request to create a realtime session
type CreateSessionRequest struct {
	Model  string                        `json:"model"`
	Config *models.RealtimeSessionConfig `json:"config,omitempty"`
}

// CreateSessionResponse represents the response after creating a session
type CreateSessionResponse struct {
	ID        string                        `json:"id"`
	Model     string                        `json:"model"`
	CreatedAt int64                         `json:"created_at"`
	Config    *models.RealtimeSessionConfig `json:"config"`
	Status    string                        `json:"status"`
}

// GetSessionResponse represents session information
type GetSessionResponse struct {
	ID           string                        `json:"id"`
	Model        string                        `json:"model"`
	CreatedAt    int64                         `json:"created_at"`
	LastActivity int64                         `json:"last_activity"`
	Config       *models.RealtimeSessionConfig `json:"config"`
	Status       string                        `json:"status"`
}

// ListSessionsResponse represents a list of sessions
type ListSessionsResponse struct {
	Sessions []GetSessionResponse `json:"sessions"`
	Count    int                  `json:"count"`
}