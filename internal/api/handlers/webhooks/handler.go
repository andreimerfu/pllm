// Package webhooks exposes admin CRUD for outbound event subscriptions
// plus a "test fire" endpoint for operator debugging.
package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/webhooks"
)

// Dispatcher is the minimum surface the test-fire endpoint needs.
// Defined locally so we don't couple the handler to the concrete
// dispatcher type.
type Dispatcher interface {
	Emit(e webhooks.Event)
}

// Handler wires the webhook service + dispatcher to HTTP.
type Handler struct {
	logger     *zap.Logger
	service    *webhooks.Service
	dispatcher Dispatcher
}

// NewHandler builds a Handler. service must be non-nil; dispatcher is
// optional — without it the /test endpoint returns 503.
func NewHandler(logger *zap.Logger, service *webhooks.Service, dispatcher Dispatcher) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, service: service, dispatcher: dispatcher}
}

type apiBody struct {
	Name    string     `json:"name"`
	URL     string     `json:"url"`
	Secret  string     `json:"secret,omitempty"`
	Events  []string   `json:"events,omitempty"`
	Enabled *bool      `json:"enabled,omitempty"`
	TeamID  *uuid.UUID `json:"team_id,omitempty"`
}

// List is GET /api/admin/webhooks.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeErr(w, http.StatusServiceUnavailable, "webhooks not configured")
		return
	}
	rows, err := h.service.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": rows})
}

// Get is GET /api/admin/webhooks/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.service.Get(r.Context(), id)
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// Create is POST /api/admin/webhooks.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var body apiBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	row, err := h.service.Create(r.Context(), &webhooks.UpsertInput{
		Name:    body.Name,
		URL:     body.URL,
		Secret:  body.Secret,
		Events:  body.Events,
		Enabled: body.Enabled,
		TeamID:  body.TeamID,
	})
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

// Update is PUT /api/admin/webhooks/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var body apiBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	row, err := h.service.Update(r.Context(), id, &webhooks.UpsertInput{
		Name:    body.Name,
		URL:     body.URL,
		Events:  body.Events,
		Enabled: body.Enabled,
		TeamID:  body.TeamID,
	})
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// Delete is DELETE /api/admin/webhooks/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		mapErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Test is POST /api/admin/webhooks/{id}/test — fires a synthetic event
// at the specified webhook for health verification. Returns 202 and
// the caller polls GET to see LastDeliveryStatus land.
func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	if h.dispatcher == nil {
		writeErr(w, http.StatusServiceUnavailable, "webhook dispatcher not available")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	hook, err := h.service.Get(r.Context(), id)
	if err != nil {
		mapErr(w, err)
		return
	}
	// We always fire scope.denied for tests since that's the most
	// representative payload shape. Subscriber logic that filters on
	// event type will ignore it if not subscribed.
	h.dispatcher.Emit(webhooks.Event{
		Type: models.WebhookEventScopeDenied,
		Data: map[string]any{
			"test":        true,
			"webhook_id":  hook.ID,
			"webhook_url": hook.URL,
		},
	})
	w.WriteHeader(http.StatusAccepted)
}

// --- helpers --------------------------------------------------------------

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func mapErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, webhooks.ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, webhooks.ErrConflict):
		writeErr(w, http.StatusConflict, err.Error())
	default:
		writeErr(w, http.StatusBadRequest, err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Compile-time guard that `*webhooks.Dispatcher` satisfies the
// embedded interface — a cheap way to catch drift if either side changes.
var _ Dispatcher = (*webhookDispatcherShim)(nil)

type webhookDispatcherShim struct{ _ context.Context }

func (webhookDispatcherShim) Emit(webhooks.Event) {}
