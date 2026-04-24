// Package webhooks delivers pllm events to external HTTP endpoints with
// HMAC signing, bounded retries, and a best-effort "last seen" health
// marker per webhook. No external queue; we run a simple in-process
// buffered-channel worker pool. Events that can't be delivered in the
// retry budget are dropped and counted in FailureCount.
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// Event is the envelope pushed to every subscribing endpoint.
// Data is opaque on our side — callers marshal their own shape.
type Event struct {
	ID        string                  `json:"id"`
	Type      models.WebhookEventType `json:"type"`
	CreatedAt time.Time               `json:"created_at"`
	// TeamID scopes delivery: when set, only webhooks with matching or
	// nil TeamID receive this event. Nil = fire for every webhook.
	TeamID *uuid.UUID              `json:"-"`
	Data   any                     `json:"data"`
}

// Dispatcher buffers events, matches them against active webhooks, and
// delivers to each with retries.
type Dispatcher struct {
	db     *gorm.DB
	logger *zap.Logger
	client *http.Client

	queue   chan Event
	workers int

	// attemptTimeout bounds each single HTTP call. Total per-event time
	// is capped by the retry schedule below.
	attemptTimeout time.Duration
	// retrySchedule is the delay between attempts. Length of the slice =
	// total attempts (first attempt fires immediately).
	retrySchedule []time.Duration

	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewDispatcher builds a Dispatcher with sensible defaults. Call Start
// to spawn the worker pool; Stop for graceful shutdown.
func NewDispatcher(db *gorm.DB, logger *zap.Logger) *Dispatcher {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Dispatcher{
		db:             db,
		logger:         logger.With(zap.String("component", "webhook-dispatcher")),
		client:         &http.Client{Timeout: 15 * time.Second},
		queue:          make(chan Event, 1024),
		workers:        4,
		attemptTimeout: 10 * time.Second,
		retrySchedule: []time.Duration{
			0,               // first attempt — fire immediately
			2 * time.Second, // +2s
			10 * time.Second,
			60 * time.Second, // last chance
		},
		stopCh: make(chan struct{}),
	}
}

// Start spawns the worker goroutines. Safe to call once.
func (d *Dispatcher) Start(ctx context.Context) {
	for i := 0; i < d.workers; i++ {
		go d.worker(ctx)
	}
	d.logger.Info("webhook dispatcher started", zap.Int("workers", d.workers))
}

// Stop closes the event queue so workers drain and exit. Best-effort;
// in-flight deliveries continue to completion or retry exhaustion.
func (d *Dispatcher) Stop() {
	d.stopOnce.Do(func() {
		close(d.stopCh)
	})
}

// Emit enqueues an event for delivery. Non-blocking: if the buffer is
// full we drop and log — we never block an admin write on webhook health.
func (d *Dispatcher) Emit(evt Event) {
	if d == nil || d.db == nil {
		return
	}
	if evt.ID == "" {
		evt.ID = uuid.New().String()
	}
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = time.Now().UTC()
	}
	select {
	case d.queue <- evt:
	default:
		d.logger.Warn("webhook queue full, dropping event",
			zap.String("type", string(evt.Type)))
	}
}

// --- internals -----------------------------------------------------------

func (d *Dispatcher) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case evt := <-d.queue:
			d.fanOut(ctx, evt)
		}
	}
}

// fanOut loads matching webhooks and delivers to each in parallel.
func (d *Dispatcher) fanOut(ctx context.Context, evt Event) {
	var hooks []models.Webhook
	q := d.db.WithContext(ctx).Where("enabled = ?", true)
	if evt.TeamID != nil {
		q = q.Where("team_id IS NULL OR team_id = ?", *evt.TeamID)
	}
	if err := q.Find(&hooks).Error; err != nil {
		d.logger.Warn("load webhooks failed", zap.Error(err))
		return
	}
	for i := range hooks {
		h := hooks[i]
		if !h.Subscribes(evt.Type) {
			continue
		}
		go d.deliver(ctx, &h, evt)
	}
}

// deliver attempts the full retry schedule against a single webhook,
// persisting the last delivery status + error for the admin UI.
func (d *Dispatcher) deliver(ctx context.Context, hook *models.Webhook, evt Event) {
	body, err := json.Marshal(evt)
	if err != nil {
		d.logger.Warn("marshal event failed", zap.Error(err))
		return
	}
	sig := sign(hook.Secret, body)

	var lastStatus int
	var lastErr string
	success := false

	for attempt, backoff := range d.retrySchedule {
		if backoff > 0 {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			case <-d.stopCh:
				return
			}
		}
		status, err := d.attempt(ctx, hook, evt, body, sig, attempt)
		lastStatus = status
		if err != nil {
			lastErr = err.Error()
		} else {
			lastErr = ""
		}
		if err == nil && status >= 200 && status < 300 {
			success = true
			break
		}
	}
	d.recordDelivery(hook, lastStatus, lastErr, success)
}

func (d *Dispatcher) attempt(
	ctx context.Context,
	hook *models.Webhook,
	evt Event,
	body []byte,
	sig string,
	attempt int,
) (int, error) {
	reqCtx, cancel := context.WithTimeout(ctx, d.attemptTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "pllm-webhook/1")
	req.Header.Set("X-PLLM-Event", string(evt.Type))
	req.Header.Set("X-PLLM-Delivery-ID", evt.ID)
	req.Header.Set("X-PLLM-Signature", "sha256="+sig)
	req.Header.Set("X-PLLM-Attempt", fmt.Sprintf("%d", attempt+1))

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	// Drain a small portion so connections can be reused; ignore content.
	_, _ = io.CopyN(io.Discard, resp.Body, 8*1024)
	return resp.StatusCode, nil
}

func (d *Dispatcher) recordDelivery(hook *models.Webhook, status int, errMsg string, success bool) {
	now := time.Now()
	updates := map[string]any{
		"last_delivery_status": status,
		"last_delivery_error":  errMsg,
		"last_delivery_at":     &now,
	}
	if success {
		updates["failure_count"] = 0
	} else {
		updates["failure_count"] = gorm.Expr("failure_count + 1")
	}
	if err := d.db.Model(&models.Webhook{}).Where("id = ?", hook.ID).Updates(updates).Error; err != nil {
		d.logger.Debug("persist webhook delivery failed", zap.Error(err))
	}
}

// sign computes the HMAC-SHA256 of body with the webhook's secret and
// returns the hex digest. Subscribers verify by re-computing.
func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
