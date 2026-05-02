// Package lifecycle implements automated hygiene rules that run on a
// periodic ticker (and on-demand via the admin handler). Each Rule is a
// small struct that scans the database for stale state and either logs
// (DryRun) or mutates rows + writes Audit + emits a webhook.
package lifecycle

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Config is the runtime view of internal/core/config.LifecycleConfig.
// Kept here as a plain struct so this package doesn't import the global
// config package (matches the rest of internal/services).
type Config struct {
	DryRun                bool
	Interval              time.Duration
	InactiveKeyDays       int
	OwnerlessKeysEnabled  bool
	RiskyServerScoreFloor float64
}

// WebhookEmitter is the small surface this package needs from
// webhooks.Dispatcher. Defined here so we don't import the concrete
// dispatcher type — same pattern as use_cases.WebhookEmitter.
type WebhookEmitter interface {
	Emit(typ string, data any)
}

// Action is one mutation taken by a rule, recorded in the run summary
// and returned to the admin UI. ResourceID is the row that was changed.
type Action struct {
	Rule       string    `json:"rule"`
	ResourceID string    `json:"resource_id"`
	Subject    string    `json:"subject"`
	Reason     string    `json:"reason"`
	DryRun     bool      `json:"dry_run"`
	At         time.Time `json:"at"`
}

// RuleResult is what a single Rule returns from Run.
type RuleResult struct {
	Rule    string   `json:"rule"`
	Scanned int      `json:"scanned"`
	Actions []Action `json:"actions"`
	Err     string   `json:"error,omitempty"`
}

// RunSummary is the response to admin "run now" + the value cached on
// the service for "last status" queries.
type RunSummary struct {
	RanAt        time.Time     `json:"ran_at"`
	Duration     time.Duration `json:"duration"`
	TotalActions int           `json:"total_actions"`
	Rules        []RuleResult  `json:"rules"`
}

// Rule is a single hygiene check. Implementations live in their own
// files in this package (inactive_keys.go, ownerless_keys.go, etc.).
type Rule interface {
	Name() string
	Run(ctx context.Context, db *gorm.DB, cfg Config, hooks WebhookEmitter) RuleResult
}

// Service holds the rule list and orchestrates ticker-driven runs.
// One instance per process; safe to call RunOnce concurrently with the
// background ticker (each run is its own DB transaction-per-action).
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
	cfg    Config
	hooks  WebhookEmitter
	rules  []Rule

	mu          sync.RWMutex
	lastSummary RunSummary
}

// NewService builds a service. Rules are added with Register; this lets
// tests construct a service with a single rule.
func NewService(db *gorm.DB, logger *zap.Logger, cfg Config) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{db: db, logger: logger.With(zap.String("component", "lifecycle")), cfg: cfg}
}

// WithWebhooks attaches an emitter. Returns the receiver for chaining.
func (s *Service) WithWebhooks(e WebhookEmitter) *Service {
	s.hooks = e
	return s
}

// Register appends a Rule to the run order. Idempotent across names —
// last registration wins.
func (s *Service) Register(r Rule) {
	for i, existing := range s.rules {
		if existing.Name() == r.Name() {
			s.rules[i] = r
			return
		}
	}
	s.rules = append(s.rules, r)
}

// RunOnce executes every rule in order, captures the summary, and
// returns it. Errors from a single rule are recorded on the result but
// don't abort other rules.
func (s *Service) RunOnce(ctx context.Context) (RunSummary, error) {
	start := time.Now()
	summary := RunSummary{RanAt: start, Rules: make([]RuleResult, 0, len(s.rules))}
	for _, r := range s.rules {
		res := r.Run(ctx, s.db, s.cfg, s.hooks)
		summary.Rules = append(summary.Rules, res)
		summary.TotalActions += len(res.Actions)
	}
	summary.Duration = time.Since(start)
	s.mu.Lock()
	s.lastSummary = summary
	s.mu.Unlock()
	s.logger.Info("lifecycle run complete",
		zap.Int("rules", len(summary.Rules)),
		zap.Int("actions", summary.TotalActions),
		zap.Duration("duration", summary.Duration),
		zap.Bool("dry_run", s.cfg.DryRun))
	return summary, nil
}

// LastSummary returns the most recent run summary (zero value if never run).
func (s *Service) LastSummary() RunSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSummary
}

// Start spawns the periodic ticker. Runs once immediately, then on
// Config.Interval. Stops when ctx is cancelled.
func (s *Service) Start(ctx context.Context) {
	if s.cfg.Interval <= 0 {
		s.logger.Warn("lifecycle disabled — Interval <= 0")
		return
	}
	go func() {
		t := time.NewTicker(s.cfg.Interval)
		defer t.Stop()
		_, _ = s.RunOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_, _ = s.RunOnce(ctx)
			}
		}
	}()
	s.logger.Info("lifecycle daemon started",
		zap.Duration("interval", s.cfg.Interval),
		zap.Bool("dry_run", s.cfg.DryRun))
}
