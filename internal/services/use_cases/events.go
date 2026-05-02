package use_cases

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// WebhookEmitter is the minimum surface the recorder needs from the
// webhooks package. Matches webhooks.Dispatcher.Emit but typed loosely
// so we don't import the concrete dispatcher here (keeps the dep tree
// shallow and makes tests easy).
type WebhookEmitter interface {
	Emit(e WebhookEventEnvelope)
}

// WebhookEventEnvelope is what the recorder hands to the emitter. The
// router wires this into a webhooks.Event with the right type/data.
type WebhookEventEnvelope struct {
	Type string
	Data any
}

// EventRecorder persists scope-denial events for the audit log.
// Called from MCP + chat handlers via a small adapter in the router so
// the edge handlers don't import this package directly.
type EventRecorder struct {
	db     *gorm.DB
	logger *zap.Logger
	hooks  WebhookEmitter // optional; nil silently drops outbound hooks
}

// NewEventRecorder wires a recorder.
func NewEventRecorder(db *gorm.DB, logger *zap.Logger) *EventRecorder {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &EventRecorder{db: db, logger: logger}
}

// WithWebhooks attaches an outbound webhook emitter. Called by the
// router when a webhooks.Dispatcher is available.
func (r *EventRecorder) WithWebhooks(e WebhookEmitter) *EventRecorder {
	r.hooks = e
	return r
}

// RecordInput carries all the fields the edge handler has in hand at
// the moment of denial. No business rules: we just persist what the
// caller gives us.
type RecordInput struct {
	UseCaseID  uuid.UUID
	KeyID      *uuid.UUID
	KeyName    string
	UserID     *uuid.UUID
	Kind       models.ScopeEventKind
	TargetName string
	Outcome    models.ScopeEventOutcome
	Reason     string
	RemoteAddr string
	RequestID  string
}

// Record writes one ScopeEvent. Persistence failures are logged but not
// surfaced — audit is best-effort; we never fail a request because the
// audit write failed.
func (r *EventRecorder) Record(ctx context.Context, in RecordInput) {
	if r == nil || r.db == nil || in.UseCaseID == uuid.Nil {
		return
	}
	row := &models.ScopeEvent{
		BaseModel:  models.BaseModel{ID: uuid.New()},
		UseCaseID:  in.UseCaseID,
		KeyID:      in.KeyID,
		KeyName:    in.KeyName,
		UserID:     in.UserID,
		Kind:       in.Kind,
		TargetName: in.TargetName,
		Outcome:    in.Outcome,
		Reason:     in.Reason,
		RemoteAddr: in.RemoteAddr,
		RequestID:  in.RequestID,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		r.logger.Warn("scope-event persist failed", zap.Error(err))
	}
	// Fan out to external subscribers. The router translates the typed
	// string ("scope.denied" or "scope.flagged") into the correct
	// webhooks.WebhookEventType value.
	if r.hooks != nil {
		typ := "scope.flagged"
		if in.Outcome == models.ScopeOutcomeDenied {
			typ = "scope.denied"
		}
		r.hooks.Emit(WebhookEventEnvelope{
			Type: typ,
			Data: map[string]any{
				"event_id":     row.ID,
				"occurred_at":  row.CreatedAt,
				"use_case_id":  in.UseCaseID,
				"key_id":       in.KeyID,
				"key_name":     in.KeyName,
				"kind":         in.Kind,
				"target_name":  in.TargetName,
				"outcome":      in.Outcome,
				"reason":       in.Reason,
				"remote_addr":  in.RemoteAddr,
				"request_id":   in.RequestID,
			},
		})
	}
}

// BulkRecordInput is the seed-time variant of RecordInput. The seed
// caller chooses the timestamp; runtime callers do not.
type BulkRecordInput struct {
	UseCaseID  uuid.UUID
	KeyID      *uuid.UUID
	KeyName    string
	UserID     *uuid.UUID
	Kind       models.ScopeEventKind
	TargetName string
	Outcome    models.ScopeEventOutcome
	Reason     string
	RemoteAddr string
	RequestID  string
	OccurredAt time.Time
}

// BulkInsert writes scope events with caller-supplied timestamps. Used
// only by the demo seed (`pllm seed bank --with-events`). Webhook
// fan-out is intentionally skipped — backdated history would generate
// misleading "live" notifications for any subscribed endpoint.
func (r *EventRecorder) BulkInsert(ctx context.Context, in []BulkRecordInput) (int, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("event recorder not configured")
	}
	if len(in) == 0 {
		return 0, nil
	}
	rows := make([]models.ScopeEvent, 0, len(in))
	for _, e := range in {
		if e.UseCaseID == uuid.Nil {
			return 0, fmt.Errorf("bulk insert: use_case_id is required for every event")
		}
		ts := e.OccurredAt
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		rows = append(rows, models.ScopeEvent{
			BaseModel:  models.BaseModel{ID: uuid.New(), CreatedAt: ts, UpdatedAt: ts},
			UseCaseID:  e.UseCaseID,
			KeyID:      e.KeyID,
			KeyName:    e.KeyName,
			UserID:     e.UserID,
			Kind:       e.Kind,
			TargetName: e.TargetName,
			Outcome:    e.Outcome,
			Reason:     e.Reason,
			RemoteAddr: e.RemoteAddr,
			RequestID:  e.RequestID,
		})
	}
	if err := r.db.WithContext(ctx).Create(&rows).Error; err != nil {
		return 0, err
	}
	return len(rows), nil
}

// --- Query side ----------------------------------------------------------

// EventListFilter narrows an audit-log list query.
type EventListFilter struct {
	UseCaseID  *uuid.UUID
	Kind       models.ScopeEventKind
	Outcome    models.ScopeEventOutcome
	TargetName string
	Since      *time.Time
	Limit      int
	Offset     int
}

// ListEvents returns newest-first events for the admin UI / CSV export.
func (r *EventRecorder) ListEvents(ctx context.Context, f EventListFilter) ([]models.ScopeEvent, error) {
	q := r.db.WithContext(ctx).Model(&models.ScopeEvent{})
	if f.UseCaseID != nil {
		q = q.Where("use_case_id = ?", *f.UseCaseID)
	}
	if f.Kind != "" {
		q = q.Where("kind = ?", f.Kind)
	}
	if f.Outcome != "" {
		q = q.Where("outcome = ?", f.Outcome)
	}
	if f.TargetName != "" {
		q = q.Where("target_name = ?", f.TargetName)
	}
	if f.Since != nil {
		q = q.Where("created_at >= ?", *f.Since)
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var out []models.ScopeEvent
	err := q.Order("created_at desc").Limit(limit).Offset(f.Offset).Find(&out).Error
	return out, err
}

// EventStats is a tiny aggregate summary used by the UI header.
type EventStats struct {
	Last24hDenied  int64 `json:"last_24h_denied"`
	Last24hFlagged int64 `json:"last_24h_flagged"`
	Last7dDenied   int64 `json:"last_7d_denied"`
	Last7dFlagged  int64 `json:"last_7d_flagged"`
}

// Stats returns cheap counters for the audit dashboard.
func (r *EventRecorder) Stats(ctx context.Context) (EventStats, error) {
	now := time.Now()
	var s EventStats
	err := r.db.WithContext(ctx).Model(&models.ScopeEvent{}).
		Where("outcome = ? AND created_at >= ?", models.ScopeOutcomeDenied, now.Add(-24*time.Hour)).
		Count(&s.Last24hDenied).Error
	if err == nil {
		err = r.db.WithContext(ctx).Model(&models.ScopeEvent{}).
			Where("outcome = ? AND created_at >= ?", models.ScopeOutcomeFlagged, now.Add(-24*time.Hour)).
			Count(&s.Last24hFlagged).Error
	}
	if err == nil {
		err = r.db.WithContext(ctx).Model(&models.ScopeEvent{}).
			Where("outcome = ? AND created_at >= ?", models.ScopeOutcomeDenied, now.Add(-7*24*time.Hour)).
			Count(&s.Last7dDenied).Error
	}
	if err == nil {
		err = r.db.WithContext(ctx).Model(&models.ScopeEvent{}).
			Where("outcome = ? AND created_at >= ?", models.ScopeOutcomeFlagged, now.Add(-7*24*time.Hour)).
			Count(&s.Last7dFlagged).Error
	}
	return s, err
}
