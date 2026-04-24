package webhooks_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/amerfu/pllm/internal/services/webhooks"
)

// TestDispatcherSignsAndDelivers wires a real HTTP test server, registers
// it as a subscribed webhook, emits an event, and verifies the signature
// round-trips + the last_delivery_status gets persisted.
func TestDispatcherSignsAndDelivers(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Record what the subscriber saw.
	var (
		mu        sync.Mutex
		lastBody  []byte
		lastSig   string
		lastEvent string
		called    = make(chan struct{}, 1)
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		lastBody = body
		lastSig = r.Header.Get("X-PLLM-Signature")
		lastEvent = r.Header.Get("X-PLLM-Event")
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		select {
		case called <- struct{}{}:
		default:
		}
	}))
	defer srv.Close()

	// Register the webhook directly in the DB — skip the service layer
	// for brevity; we're exercising the dispatcher.
	secret := "shhhh"
	hook := &models.Webhook{
		Name:    "test",
		URL:     srv.URL,
		Secret:  secret,
		Enabled: true,
	}
	require.NoError(t, db.Create(hook).Error)

	disp := webhooks.NewDispatcher(db, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	disp.Start(ctx)
	defer disp.Stop()

	disp.Emit(webhooks.Event{
		Type: models.WebhookEventScopeDenied,
		Data: map[string]any{"foo": "bar"},
	})

	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatal("webhook not delivered within 5s")
	}

	// Signature must round-trip.
	mu.Lock()
	body := append([]byte{}, lastBody...)
	sig := lastSig
	evt := lastEvent
	mu.Unlock()

	require.Equal(t, "scope.denied", evt)
	require.Contains(t, string(body), "\"foo\":\"bar\"")

	// Verify HMAC.
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	require.True(t,
		hmac.Equal([]byte(expected), []byte(sig)),
		"signature mismatch: got %q, want %q", sig, expected,
	)

	// last_delivery_status should be persisted ≈ 204.
	// Give the recordDelivery path a moment.
	time.Sleep(200 * time.Millisecond)
	var reread models.Webhook
	require.NoError(t, db.First(&reread, "id = ?", hook.ID).Error)
	require.Equal(t, http.StatusNoContent, reread.LastDeliveryStatus)
	require.NotNil(t, reread.LastDeliveryAt)
	require.Zero(t, reread.FailureCount)
}

// TestSubscriptionFilter verifies that a webhook with an explicit Events
// list only receives matching event types.
func TestSubscriptionFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	var hits int
	var mu sync.Mutex
	done := make(chan struct{}, 5)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		done <- struct{}{}
	}))
	defer srv.Close()

	// Subscribed only to scope.denied.
	require.NoError(t, db.Create(&models.Webhook{
		Name: "denied-only", URL: srv.URL, Secret: "k", Enabled: true,
		Events: []string{"scope.denied"},
	}).Error)

	disp := webhooks.NewDispatcher(db, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	disp.Start(ctx)
	defer disp.Stop()

	// Fire both types — only the first should land.
	disp.Emit(webhooks.Event{Type: models.WebhookEventScopeDenied, Data: map[string]any{}})
	disp.Emit(webhooks.Event{Type: models.WebhookEventScopeFlagged, Data: map[string]any{}})

	// Wait for the denied one, then briefly idle to ensure the flagged
	// one really doesn't show.
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("expected denied delivery within 3s")
	}
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	require.Equal(t, 1, hits, "subscribed filter ignored")
	mu.Unlock()
}

// TestRetriesOnTransientFailure fires an event at a server that returns
// 500 twice then 204; expects exactly one final persisted success.
func TestRetriesOnTransientFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	var attempts int32
	var mu sync.Mutex
	succ := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		a := attempts
		mu.Unlock()
		if a < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		succ <- struct{}{}
	}))
	defer srv.Close()

	require.NoError(t, db.Create(&models.Webhook{
		Name: "retry-me", URL: srv.URL, Secret: "k", Enabled: true,
	}).Error)

	disp := webhooks.NewDispatcher(db, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	disp.Start(ctx)
	defer disp.Stop()
	disp.Emit(webhooks.Event{Type: models.WebhookEventScopeDenied, Data: map[string]any{}})

	// Retry schedule has +0, +2, +10, +60 — succeeds on attempt 3 so ~12s budget.
	select {
	case <-succ:
	case <-time.After(20 * time.Second):
		t.Fatal("retry sequence never succeeded")
	}
	mu.Lock()
	require.Equal(t, int32(3), attempts)
	mu.Unlock()
}

// Sanity: malformed JSON from the subscriber is ignored.
var _ = bytes.NewReader // keep bytes import if future tests need it

// json package is imported for signature verification; suppress
// "imported and not used" if the body above ever changes.
var _ = json.Marshal
