package providers

import (
	"context"
	"fmt"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// RealtimeProvider extends Provider interface with realtime capabilities
type RealtimeProvider interface {
	Provider
	
	// ConnectRealtime establishes a WebSocket connection to the provider's realtime API
	ConnectRealtime(ctx context.Context, config *RealtimeConnectionConfig) (*websocket.Conn, error)
	
	// SupportsRealtime indicates if the provider supports realtime API
	SupportsRealtime() bool
}

// RealtimeConnectionConfig holds configuration for realtime connection
type RealtimeConnectionConfig struct {
	Model       string                        `json:"model"`
	APIKey      string                        `json:"-"` // Don't serialize API keys
	BaseURL     string                        `json:"base_url,omitempty"`
	Session     *models.RealtimeSessionConfig `json:"session,omitempty"`
	Headers     map[string]string             `json:"headers,omitempty"`
	QueryParams map[string]string             `json:"query_params,omitempty"`
}



// RealtimeMessageHandler handles bidirectional message routing between client and provider
type RealtimeMessageHandler struct {
	logger         *zap.Logger
	clientConn     *websocket.Conn
	providerConn   *websocket.Conn
	sessionConfig  *models.RealtimeSessionConfig
	stopChan       chan struct{}
	model          string
}

// NewRealtimeMessageHandler creates a new message handler
func NewRealtimeMessageHandler(
	logger *zap.Logger,
	clientConn *websocket.Conn,
	providerConn *websocket.Conn,
	sessionConfig *models.RealtimeSessionConfig,
	model string,
) *RealtimeMessageHandler {
	return &RealtimeMessageHandler{
		logger:        logger,
		clientConn:    clientConn,
		providerConn:  providerConn,
		sessionConfig: sessionConfig,
		stopChan:      make(chan struct{}),
		model:         model,
	}
}

// Start begins message routing between client and provider
func (h *RealtimeMessageHandler) Start(ctx context.Context) error {
	// Start client->provider message routing
	go h.routeClientToProvider(ctx)
	
	// Start provider->client message routing
	go h.routeProviderToClient(ctx)

	// Send initial session update to provider
	if h.sessionConfig != nil {
		sessionUpdate := &models.SessionUpdateEvent{
			Session: *h.sessionConfig,
		}
		
		event, err := models.NewRealtimeEvent("session.update", sessionUpdate)
		if err != nil {
			return fmt.Errorf("failed to create session.update event: %w", err)
		}

		if err := h.providerConn.WriteJSON(event); err != nil {
			return fmt.Errorf("failed to send initial session update: %w", err)
		}
	}

	return nil
}

// Stop stops message routing
func (h *RealtimeMessageHandler) Stop() {
	close(h.stopChan)
}

// routeClientToProvider routes messages from client to provider
func (h *RealtimeMessageHandler) routeClientToProvider(ctx context.Context) {
	defer h.logger.Debug("Client->Provider routing stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopChan:
			return
		default:
		}

		// Read message from client
		var event models.RealtimeEvent
		if err := h.clientConn.ReadJSON(&event); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error("Client WebSocket error", zap.Error(err))
			}
			return
		}

		h.logger.Debug("Routing client event to provider", 
			zap.String("type", event.Type),
			zap.String("event_id", event.EventID))

		// Process event if needed (middleware, validation, etc.)
		if err := h.processClientEvent(&event); err != nil {
			h.logger.Error("Failed to process client event", 
				zap.String("type", event.Type),
				zap.Error(err))
			continue
		}

		// Forward to provider
		if err := h.providerConn.WriteJSON(&event); err != nil {
			h.logger.Error("Failed to send event to provider", 
				zap.String("type", event.Type),
				zap.Error(err))
			return
		}
	}
}

// routeProviderToClient routes messages from provider to client
func (h *RealtimeMessageHandler) routeProviderToClient(ctx context.Context) {
	defer h.logger.Debug("Provider->Client routing stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopChan:
			return
		default:
		}

		// Read message from provider
		var event models.RealtimeEvent
		if err := h.providerConn.ReadJSON(&event); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error("Provider WebSocket error", zap.Error(err))
			}
			return
		}

		h.logger.Debug("Routing provider event to client", 
			zap.String("type", event.Type),
			zap.String("event_id", event.EventID))

		// Process event if needed (metrics, logging, etc.)
		if err := h.processProviderEvent(&event); err != nil {
			h.logger.Error("Failed to process provider event", 
				zap.String("type", event.Type),
				zap.Error(err))
			continue
		}

		// Forward to client
		if err := h.clientConn.WriteJSON(&event); err != nil {
			h.logger.Error("Failed to send event to client", 
				zap.String("type", event.Type),
				zap.Error(err))
			return
		}
	}
}

// processClientEvent processes events from client before forwarding to provider
func (h *RealtimeMessageHandler) processClientEvent(event *models.RealtimeEvent) error {
	// Add any client-side processing here
	// Examples: rate limiting, validation, logging, metrics
	
	h.logger.Debug("Processing client event",
		zap.String("type", event.Type),
		zap.String("model", h.model))

	return nil
}

// processProviderEvent processes events from provider before forwarding to client
func (h *RealtimeMessageHandler) processProviderEvent(event *models.RealtimeEvent) error {
	// Add any provider-side processing here
	// Examples: token counting, cost calculation, metrics, logging

	h.logger.Debug("Processing provider event",
		zap.String("type", event.Type),
		zap.String("model", h.model))

	return nil
}

// Utility functions for checking provider support

// ProviderSupportsRealtime checks if a provider supports realtime API
func ProviderSupportsRealtime(provider Provider) bool {
	if rtProvider, ok := provider.(RealtimeProvider); ok {
		return rtProvider.SupportsRealtime()
	}
	return false
}

// GetRealtimeProvider safely casts a provider to RealtimeProvider
func GetRealtimeProvider(provider Provider) (RealtimeProvider, error) {
	if rtProvider, ok := provider.(RealtimeProvider); ok {
		if rtProvider.SupportsRealtime() {
			return rtProvider, nil
		}
		return nil, fmt.Errorf("provider %s does not support realtime API", provider.GetName())
	}
	return nil, fmt.Errorf("provider %s does not implement RealtimeProvider interface", provider.GetName())
}