package use_cases

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/use_cases"
)

// ListScopeEvents is GET /api/admin/use-cases/events — the audit log
// across all use cases. Supports filters: use_case_id, kind (mcp_tool|llm),
// outcome (denied|flagged), target_name, since (RFC3339), limit/offset.
func (h *Handler) ListScopeEvents(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeErr(w, http.StatusServiceUnavailable, "use cases not configured")
		return
	}
	q := r.URL.Query()
	f := use_cases.EventListFilter{
		Kind:       models.ScopeEventKind(q.Get("kind")),
		Outcome:    models.ScopeEventOutcome(q.Get("outcome")),
		TargetName: q.Get("target_name"),
	}
	if v := q.Get("use_case_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.UseCaseID = &id
		}
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Since = &t
		}
	}
	if n, err := strconv.Atoi(q.Get("limit")); err == nil {
		f.Limit = n
	}
	if n, err := strconv.Atoi(q.Get("offset")); err == nil && n >= 0 {
		f.Offset = n
	}
	rec := h.service.Events()
	events, err := rec.ListEvents(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	stats, err := rec.Stats(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []models.ScopeEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events": events,
		"stats":  stats,
	})
}

// bulkSeedBody is the shape POST /api/admin/use-cases/events:bulk accepts.
// Each event must specify use_case_id; created_at is optional (defaults to
// now).
type bulkSeedBody struct {
	Events []bulkSeedEvent `json:"events"`
}

type bulkSeedEvent struct {
	UseCaseID  uuid.UUID  `json:"use_case_id"`
	KeyID      *uuid.UUID `json:"key_id,omitempty"`
	KeyName    string     `json:"key_name,omitempty"`
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	Kind       string     `json:"kind"`
	TargetName string     `json:"target_name"`
	Outcome    string     `json:"outcome"`
	Reason     string     `json:"reason,omitempty"`
	RemoteAddr string     `json:"remote_addr,omitempty"`
	RequestID  string     `json:"request_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at,omitempty"`
}

// BulkSeedScopeEvents is POST /api/admin/use-cases/events:bulk — a
// test/demo aid that backdates scope-event history. Master-key only
// (RequireAdmin upstream). Skips webhook fan-out so historical events
// don't generate live notifications to subscribers.
func (h *Handler) BulkSeedScopeEvents(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeErr(w, http.StatusServiceUnavailable, "use cases not configured")
		return
	}
	var body bulkSeedBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(body.Events) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"inserted": 0})
		return
	}
	in := make([]use_cases.BulkRecordInput, 0, len(body.Events))
	for _, e := range body.Events {
		in = append(in, use_cases.BulkRecordInput{
			UseCaseID:  e.UseCaseID,
			KeyID:      e.KeyID,
			KeyName:    e.KeyName,
			UserID:     e.UserID,
			Kind:       models.ScopeEventKind(e.Kind),
			TargetName: e.TargetName,
			Outcome:    models.ScopeEventOutcome(e.Outcome),
			Reason:     e.Reason,
			RemoteAddr: e.RemoteAddr,
			RequestID:  e.RequestID,
			OccurredAt: e.CreatedAt,
		})
	}
	n, err := h.service.Events().BulkInsert(r.Context(), in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"inserted": n})
}
