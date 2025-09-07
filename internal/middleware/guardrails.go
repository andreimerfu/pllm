package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/models"
	"github.com/amerfu/pllm/internal/services/guardrails"
	"github.com/amerfu/pllm/internal/services/providers"
)

// GuardrailsMiddleware handles guardrails execution in the request pipeline
type GuardrailsMiddleware struct {
	executor *guardrails.Executor
	logger   *zap.Logger
}

// NewGuardrailsMiddleware creates a new guardrails middleware
func NewGuardrailsMiddleware(executor *guardrails.Executor, logger *zap.Logger) *GuardrailsMiddleware {
	return &GuardrailsMiddleware{
		executor: executor,
		logger:   logger.Named("guardrails_middleware"),
	}
}

// Middleware returns the HTTP middleware function
func (m *GuardrailsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.logger.Debug("Guardrails middleware triggered", 
			zap.String("path", r.URL.Path),
			zap.String("method", r.Method))
		
		// Only apply to chat completions endpoint
		if r.URL.Path != "/v1/chat/completions" {
			m.logger.Debug("Skipping guardrails - not chat completions", zap.String("path", r.URL.Path))
			next.ServeHTTP(w, r)
			return
		}
		
		m.logger.Info("Processing guardrails for chat completions")
		
		// Skip if guardrails not enabled
		if !m.executor.IsEnabled() {
			m.logger.Warn("Guardrails executor is not enabled")
			next.ServeHTTP(w, r)
			return
		}
		
		m.logger.Info("Guardrails executor is enabled, executing pre-call checks")
		
		// Parse request body for guardrails processing
		var request providers.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			m.logger.Error("Failed to decode request for guardrails", zap.Error(err))
			next.ServeHTTP(w, r)
			return
		}
		
		// Extract auth context
		userID := m.extractUserID(r.Context())
		teamID := m.extractTeamID(r.Context())
		keyID := m.extractKeyID(r.Context())
		
		// Execute pre-call guardrails
		if err := m.executor.ExecutePreCall(r.Context(), &request, userID, teamID, keyID); err != nil {
			m.logger.Info("Request blocked by pre-call guardrail", 
				zap.String("user_id", userID),
				zap.String("team_id", teamID),
				zap.String("key_id", keyID),
				zap.Error(err))
			
			m.sendGuardrailError(w, err)
			return
		}
		
		// Reconstruct body with potentially modified request from guardrails
		requestBytes, _ := json.Marshal(request)
		r.Body = &readCloser{strings.NewReader(string(requestBytes))}
		
		// Start during-call guardrails (async)
		ctx := m.executor.StartDuringCall(r.Context(), &request, userID, teamID, keyID)
		r = r.WithContext(ctx)
		
		// Wrap response writer to capture response for post-call guardrails
		wrapper := &guardrailsResponseWriter{
			ResponseWriter: w,
			middleware:     m,
			request:        &request,
			userID:         userID,
			teamID:         teamID,
			keyID:          keyID,
		}
		
		// Continue to next handler
		next.ServeHTTP(wrapper, r)
	})
}

// guardrailsResponseWriter captures responses for post-call processing
type guardrailsResponseWriter struct {
	http.ResponseWriter
	middleware *GuardrailsMiddleware
	request    *providers.ChatRequest
	userID     string
	teamID     string
	keyID      string
	statusCode int
	body       []byte
}

func (w *guardrailsResponseWriter) Write(data []byte) (int, error) {
	// Capture response body
	w.body = append(w.body, data...)
	
	// Try to parse as chat response for post-call guardrails
	if w.statusCode == 200 && len(w.body) > 0 {
		var response providers.ChatResponse
		if err := json.Unmarshal(w.body, &response); err == nil {
			// Execute post-call guardrails
			if err := w.middleware.executor.ExecutePostCall(
				context.Background(), // Use background context to avoid request timeout
				w.request, 
				&response, 
				w.userID, 
				w.teamID, 
				w.keyID,
			); err != nil {
				w.middleware.logger.Info("Response flagged by post-call guardrail",
					zap.String("user_id", w.userID),
					zap.String("team_id", w.teamID), 
					zap.String("key_id", w.keyID),
					zap.Error(err))
				
				// For post-call, we typically log rather than block
				// But we could implement blocking behavior here if needed
			}
		}
	}
	
	return w.ResponseWriter.Write(data)
}

func (w *guardrailsResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// sendGuardrailError sends a guardrails error response
func (m *GuardrailsMiddleware) sendGuardrailError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	
	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"message": err.Error(),
			"type":    "guardrail_violation",
			"code":    "content_blocked",
		},
	}
	
	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}

// Extract auth context helpers
func (m *GuardrailsMiddleware) extractUserID(ctx context.Context) string {
	if user, ok := ctx.Value(UserContextKey).(*models.User); ok && user != nil {
		return user.ID.String()
	}
	return ""
}

func (m *GuardrailsMiddleware) extractTeamID(ctx context.Context) string {
	if team, ok := ctx.Value(TeamContextKey).(*models.Team); ok && team != nil {
		return team.ID.String()
	}
	return ""
}

func (m *GuardrailsMiddleware) extractKeyID(ctx context.Context) string {
	if key, ok := ctx.Value(KeyContextKey).(*models.Key); ok && key != nil {
		return key.ID.String()
	}
	return ""
}

// readCloser implements io.ReadCloser for reconstructing request body
type readCloser struct {
	*strings.Reader
}

func (r *readCloser) Close() error {
	return nil
}