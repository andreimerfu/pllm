package federation

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RunnerConfig governs the federation sync runner. Each peer has its own
// SyncIntervalMinutes; the runner uses TickInterval as the master tick
// frequency at which it re-evaluates which peers are due.
type RunnerConfig struct {
	// TickInterval is how often the runner wakes up to consider peers.
	// Defaults to 1 minute when zero.
	TickInterval time.Duration

	// DefaultSyncInterval is used when a peer leaves SyncIntervalMinutes
	// at zero. Defaults to 1 hour when zero.
	DefaultSyncInterval time.Duration
}

// Runner periodically pulls every enabled peer. One per process; started
// from router.go alongside enrichment / lifecycle.
type Runner struct {
	svc    *Service
	hooks  WebhookEmitter
	cfg    RunnerConfig
	cache  *peerCache
	logger *zap.Logger

	mu        sync.Mutex
	lastByID  map[string]time.Time // peer.PeerID -> last successful run
	inFlight  map[string]struct{}  // peer.PeerID -> currently running
}

// NewRunner builds a Runner. Defaults are applied here so callers can
// pass a zero-value RunnerConfig.
func NewRunner(svc *Service, hooks WebhookEmitter, cfg RunnerConfig, logger *zap.Logger) *Runner {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = time.Minute
	}
	if cfg.DefaultSyncInterval <= 0 {
		cfg.DefaultSyncInterval = time.Hour
	}
	return &Runner{
		svc:      svc,
		hooks:    hooks,
		cfg:      cfg,
		cache:    newPeerCache(),
		logger:   logger.With(zap.String("component", "federation-runner")),
		lastByID: map[string]time.Time{},
		inFlight: map[string]struct{}{},
	}
}

// Start kicks the runner loop. Returns immediately; goroutine exits on
// context cancellation.
func (r *Runner) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(r.cfg.TickInterval)
		defer t.Stop()
		// Refresh cache once at boot so the first tick has data.
		r.refresh(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				r.refresh(ctx)
				r.tick(ctx)
			}
		}
	}()
	r.logger.Info("federation runner started",
		zap.Duration("tick_interval", r.cfg.TickInterval),
		zap.Duration("default_sync_interval", r.cfg.DefaultSyncInterval))
}

// refresh re-reads peer rows into the cache. Errors are logged; the runner
// keeps using the previous snapshot rather than blanking the schedule.
func (r *Runner) refresh(ctx context.Context) {
	rows, err := r.svc.List(ctx)
	if err != nil {
		r.logger.Warn("federation runner: list peers failed", zap.Error(err))
		return
	}
	r.cache.replace(rows)
}

// tick selects peers that are due and runs them concurrently. We cap to
// one-in-flight-per-peer so a slow upstream can't pile up overlapping
// syncs.
func (r *Runner) tick(ctx context.Context) {
	now := time.Now()
	for _, p := range r.cache.snapshot() {
		if !p.Enabled {
			continue
		}
		interval := peerEnabledRunInterval(&p, r.cfg.DefaultSyncInterval)
		r.mu.Lock()
		last, ok := r.lastByID[p.PeerID]
		_, busy := r.inFlight[p.PeerID]
		due := !ok || now.Sub(last) >= interval
		r.mu.Unlock()
		if busy || !due {
			continue
		}
		r.mu.Lock()
		r.inFlight[p.PeerID] = struct{}{}
		r.mu.Unlock()
		peer := p
		go func() {
			defer func() {
				r.mu.Lock()
				delete(r.inFlight, peer.PeerID)
				r.lastByID[peer.PeerID] = time.Now()
				r.mu.Unlock()
			}()
			_, err := r.svc.SyncOne(ctx, peer.ID, r.hooks)
			if err != nil {
				r.logger.Warn("federation: peer sync failed",
					zap.String("peer_id", peer.PeerID),
					zap.Error(err))
			}
		}()
	}
}
