package enrichment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// Runner polls EnrichmentJob rows and dispatches them to the registered
// Scanners. One Runner per process is enough; it claims jobs atomically.
type Runner struct {
	db       *gorm.DB
	logger   *zap.Logger
	scanners map[models.EnrichmentType]Scanner
	// How often the runner wakes to look for new jobs.
	pollInterval time.Duration
	// Max concurrent scanner invocations.
	concurrency int
}

// NewRunner builds a runner with the supplied scanners.
// If `scanners` is empty, the runner defaults to OSV only.
func NewRunner(db *gorm.DB, logger *zap.Logger, scanners ...Scanner) *Runner {
	if logger == nil {
		logger = zap.NewNop()
	}
	byType := map[models.EnrichmentType]Scanner{}
	if len(scanners) == 0 {
		s := &OSVScanner{}
		byType[s.Type()] = s
	}
	for _, s := range scanners {
		byType[s.Type()] = s
	}
	return &Runner{
		db:           db,
		logger:       logger.With(zap.String("component", "enrichment-runner")),
		scanners:     byType,
		pollInterval: 15 * time.Second,
		concurrency:  4,
	}
}

// Enqueue records a job to scan `resource` with the given type.
// If an identical pending job exists, we don't duplicate it.
func (r *Runner) Enqueue(ctx context.Context, kind models.RegistryKind, id uuid.UUID, t models.EnrichmentType) error {
	var existing models.EnrichmentJob
	err := r.db.WithContext(ctx).
		Where("resource_kind = ? AND resource_id = ? AND type = ? AND status IN ?",
			kind, id, t, []models.EnrichmentJobStatus{models.JobStatusPending, models.JobStatusRunning}).
		First(&existing).Error
	if err == nil {
		return nil // already queued / running
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.WithContext(ctx).Create(&models.EnrichmentJob{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		ResourceKind: kind,
		ResourceID:   id,
		Type:         t,
		Status:       models.JobStatusPending,
	}).Error
}

// Start runs the poll loop until ctx is canceled.
func (r *Runner) Start(ctx context.Context) {
	t := time.NewTicker(r.pollInterval)
	defer t.Stop()
	r.logger.Info("enrichment runner started",
		zap.Duration("poll", r.pollInterval),
		zap.Int("concurrency", r.concurrency))
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.tick(ctx)
		}
	}
}

// ScoresFor returns every EnrichmentScore for the given resource,
// ordered newest-first. Safe to call with a nil Runner — returns nil.
func (r *Runner) ScoresFor(ctx context.Context, kind models.RegistryKind, id uuid.UUID) ([]models.EnrichmentScore, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	var out []models.EnrichmentScore
	err := r.db.WithContext(ctx).
		Where("resource_kind = ? AND resource_id = ?", kind, id).
		Order("scanned_at desc").
		Find(&out).Error
	return out, err
}

// RunOnce processes all currently-pending jobs synchronously. Useful for
// tests and for a manual "scan now" admin button.
func (r *Runner) RunOnce(ctx context.Context) (int, error) {
	var jobs []models.EnrichmentJob
	if err := r.db.WithContext(ctx).
		Where("status = ?", models.JobStatusPending).
		Order("created_at asc").
		Limit(50).
		Find(&jobs).Error; err != nil {
		return 0, err
	}
	for i := range jobs {
		r.runJob(ctx, &jobs[i])
	}
	return len(jobs), nil
}

func (r *Runner) tick(ctx context.Context) {
	sem := make(chan struct{}, r.concurrency)
	var jobs []models.EnrichmentJob
	if err := r.db.WithContext(ctx).
		Where("status = ?", models.JobStatusPending).
		Order("created_at asc").
		Limit(50).
		Find(&jobs).Error; err != nil {
		r.logger.Warn("poll jobs failed", zap.Error(err))
		return
	}
	for i := range jobs {
		job := jobs[i]
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			r.runJob(ctx, &job)
		}()
	}
	// Drain
	for i := 0; i < r.concurrency; i++ {
		sem <- struct{}{}
	}
}

func (r *Runner) runJob(ctx context.Context, job *models.EnrichmentJob) {
	// Claim atomically: only proceed if still pending.
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&models.EnrichmentJob{}).
		Where("id = ? AND status = ?", job.ID, models.JobStatusPending).
		Updates(map[string]any{
			"status":     models.JobStatusRunning,
			"started_at": now,
			"attempts":   gorm.Expr("attempts + 1"),
		})
	if res.Error != nil || res.RowsAffected == 0 {
		return // another worker got it, or gone
	}

	scanner, ok := r.scanners[job.Type]
	if !ok {
		r.markFailed(ctx, job, fmt.Sprintf("no scanner for type %q", job.Type))
		return
	}

	// Resolve the resource. Phase 2 scanners only handle servers.
	if job.ResourceKind != models.RegistryKindServer {
		r.markFailed(ctx, job, "scanners only support servers in phase 2")
		return
	}
	var server models.RegistryServer
	if err := r.db.WithContext(ctx).First(&server, "id = ?", job.ResourceID).Error; err != nil {
		r.markFailed(ctx, job, "resource missing: "+err.Error())
		return
	}

	result, err := scanner.Scan(ctx, &server)
	if err != nil {
		r.markFailed(ctx, job, err.Error())
		return
	}
	if err := r.writeScore(ctx, job, result); err != nil {
		r.markFailed(ctx, job, "write score: "+err.Error())
		return
	}
	done := time.Now()
	r.db.WithContext(ctx).Model(&models.EnrichmentJob{}).
		Where("id = ?", job.ID).
		Updates(map[string]any{
			"status":       models.JobStatusSucceeded,
			"completed_at": done,
			"error":        "",
		})
	r.logger.Info("enrichment succeeded",
		zap.String("job", job.ID.String()),
		zap.String("type", string(job.Type)),
		zap.Float64("score", result.Score))
}

func (r *Runner) writeScore(ctx context.Context, job *models.EnrichmentJob, result *ScanResult) error {
	var findings datatypes.JSON
	if result.Findings != nil {
		b, err := json.Marshal(result.Findings)
		if err != nil {
			return err
		}
		findings = b
	}
	score := models.EnrichmentScore{
		ResourceKind: job.ResourceKind,
		ResourceID:   job.ResourceID,
		Type:         job.Type,
		Score:        result.Score,
		Summary:      result.Summary,
		Findings:     findings,
		ScannedAt:    time.Now(),
	}
	// Upsert on (resource_kind, resource_id, type).
	var existing models.EnrichmentScore
	err := r.db.WithContext(ctx).
		Where("resource_kind = ? AND resource_id = ? AND type = ?",
			job.ResourceKind, job.ResourceID, job.Type).
		First(&existing).Error
	if err == nil {
		existing.Score = score.Score
		existing.Summary = score.Summary
		existing.Findings = score.Findings
		existing.ScannedAt = score.ScannedAt
		return r.db.WithContext(ctx).Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	score.ID = uuid.New()
	return r.db.WithContext(ctx).Create(&score).Error
}

func (r *Runner) markFailed(ctx context.Context, job *models.EnrichmentJob, msg string) {
	now := time.Now()
	r.db.WithContext(ctx).Model(&models.EnrichmentJob{}).
		Where("id = ?", job.ID).
		Updates(map[string]any{
			"status":       models.JobStatusFailed,
			"completed_at": now,
			"error":        msg,
		})
	r.logger.Warn("enrichment failed",
		zap.String("job", job.ID.String()),
		zap.String("type", string(job.Type)),
		zap.String("error", msg))
}
