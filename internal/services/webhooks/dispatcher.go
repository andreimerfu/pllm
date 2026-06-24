// Package webhooks delivers pllm events to external HTTP endpoints with
// vendor-specific formatting, bounded retries, and a best-effort "last
// seen" health marker per webhook. No external queue; we run a simple
// in-process buffered-channel worker pool. Events that can't be
// delivered in the retry budget are dropped and counted in
// FailureCount.
//
// Per-vendor request formatting and success criteria live in the
// adapters sub-package — this file owns queueing, fan-out, retry
// scheduling, and the "last delivery" persistence shape.
package webhooks

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/webhooks/adapters"
)

// Event is the dispatcher-level envelope. Type-aliased from the adapter
// package so existing callers keep importing webhooks.Event without
// caring that the canonical type lives one package down.
type Event = adapters.Event

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
	if evt.TeamID() != nil {
		q = q.Where("team_id IS NULL OR team_id = ?", *evt.TeamID())
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
// Vendor-specific request shape and success criteria are delegated to
// the adapter selected by the webhook's destination_type.
func (d *Dispatcher) deliver(ctx context.Context, hook *models.Webhook, evt Event) {
	adapter := adapters.For(hook.DestinationType)

	var (
		lastStatus int
		lastErr    string
		success    bool
	)

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
		status, body, err := d.attempt(ctx, adapter, hook, evt, attempt)
		lastStatus = status
		if err != nil {
			lastErr = err.Error()
		} else {
			lastErr = ""
		}
		if err == nil && adapter.Success(status, body) {
			success = true
			break
		}
		// On a 2xx that the adapter rejects (e.g. Slack returning
		// 200 + "no_service"), keep the body in lastErr so operators
		// see why we treated the delivery as failed even though the
		// status looks fine.
		if err == nil && lastErr == "" && len(body) > 0 {
			lastErr = "adapter rejected response: " + truncate(string(body), 200)
		}
	}
	d.recordDelivery(hook, lastStatus, lastErr, success)
}

func (d *Dispatcher) attempt(
	ctx context.Context,
	adapter adapters.Adapter,
	hook *models.Webhook,
	evt Event,
	attempt int,
) (int, []byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, d.attemptTimeout)
	defer cancel()

	req, err := adapter.BuildRequest(evt, hook)
	if err != nil {
		return 0, nil, err
	}
	req = req.WithContext(reqCtx)
	// Standard pllm headers: still added on top of whatever the adapter
	// set so receivers (esp. generic relays in the chain) can correlate
	// deliveries even for vendor-formatted payloads.
	req.Header.Set("X-PLLM-Event", string(evt.Type))
	req.Header.Set("X-PLLM-Delivery-ID", evt.ID)
	req.Header.Set("X-PLLM-Attempt", itoa(attempt+1))

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	// Vendor success bodies are small (Splunk returns ~30 bytes, Slack
	// "ok" or "no_service"). 8 KiB is plenty and bounds memory.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	return resp.StatusCode, body, nil
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

// --- helpers --------------------------------------------------------------

// itoa avoids pulling in fmt for the per-attempt header value.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// truncate keeps adapter-rejection details readable in last_delivery_error
// without ballooning the column with an entire vendor body.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
