package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// RiskyServersRule sets RegistryServer.Status to "blocked" when the
// latest OSV EnrichmentScore is below Config.RiskyServerScoreFloor.
// Operators clear the block manually; we never auto-unblock.
type RiskyServersRule struct{}

func (r *RiskyServersRule) Name() string { return "risky_servers" }

func (r *RiskyServersRule) Run(ctx context.Context, db *gorm.DB, cfg Config, hooks WebhookEmitter) RuleResult {
	res := RuleResult{Rule: r.Name()}
	if cfg.RiskyServerScoreFloor <= 0 {
		return res
	}

	type row struct {
		ServerID uuid.UUID `gorm:"column:server_id"`
		Name     string    `gorm:"column:name"`
		Version  string    `gorm:"column:version"`
		Score    float64   `gorm:"column:score"`
		Summary  string    `gorm:"column:summary"`
	}
	var rows []row
	err := db.WithContext(ctx).Raw(`
		SELECT s.id AS server_id, s.name, s.version, e.score, e.summary
		FROM registry_servers s
		JOIN enrichment_scores e
		  ON e.resource_kind = ?
		 AND e.resource_id   = s.id
		 AND e.type          = ?
		WHERE s.status = ?
		  AND e.score < ?
	`, models.RegistryKindServer, models.EnrichmentTypeOSV,
		models.RegistryStatusActive, cfg.RiskyServerScoreFloor).
		Scan(&rows).Error
	if err != nil {
		res.Err = err.Error()
		return res
	}
	res.Scanned = len(rows)

	for _, x := range rows {
		now := time.Now()
		reason := fmt.Sprintf("OSV score %.1f below floor %.1f: %s",
			x.Score, cfg.RiskyServerScoreFloor, x.Summary)
		res.Actions = append(res.Actions, Action{
			Rule:       r.Name(),
			ResourceID: x.ServerID.String(),
			Subject:    fmt.Sprintf("%s@%s", x.Name, x.Version),
			Reason:     reason,
			DryRun:     cfg.DryRun,
			At:         now,
		})
		if cfg.DryRun {
			continue
		}
		// Idempotent: only block if status is still active. Mirrors revokeKey.
		updRes := db.WithContext(ctx).Model(&models.RegistryServer{}).
			Where("id = ? AND status = ?", x.ServerID, models.RegistryStatusActive).
			Update("status", models.RegistryStatusBlocked)
		if updRes.Error != nil {
			res.Actions[len(res.Actions)-1].Reason = "block failed: " + updRes.Error.Error()
			continue
		}
		if updRes.RowsAffected == 0 {
			// Already blocked between scan and update.
			res.Actions[len(res.Actions)-1].Reason = "already blocked (race)"
			continue
		}
		writeServerAudit(ctx, db, x.ServerID, x.Name, x.Version, x.Score, cfg.RiskyServerScoreFloor, reason, now, nil)
		if hooks != nil {
			hooks.Emit(WebhookEventEnvelope{
				Type: models.WebhookEventRegistryServerBlockedRisky,
				Data: map[string]any{
					"server_id":   x.ServerID,
					"name":        x.Name,
					"version":     x.Version,
					"score":       x.Score,
					"floor":       cfg.RiskyServerScoreFloor,
					"reason":      reason,
					"occurred_at": now.UTC(),
				},
			})
		}
	}
	return res
}

// writeServerAudit persists an AuditEventLifecycle row keyed to the
// affected RegistryServer. NewValues captures the post-state plus the
// score and floor that triggered the block — operators clearing a block
// can read this row to understand exactly why the daemon acted.
func writeServerAudit(ctx context.Context, db *gorm.DB, sid uuid.UUID, name, version string, score, floor float64, reason string, blockedAt time.Time, logger *zap.Logger) {
	row := &models.Audit{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		EventType:    models.AuditEventLifecycle,
		EventAction:  "server_blocked_risky",
		EventResult:  models.AuditResult("success"),
		ResourceType: "registry_server",
		ResourceID:   &sid,
		Message:      reason,
		Timestamp:    blockedAt,
	}
	if b, err := json.Marshal(map[string]any{
		"name":    name,
		"version": version,
		"status":  string(models.RegistryStatusBlocked),
		"score":   score,
		"floor":   floor,
	}); err == nil {
		row.NewValues = b
	}
	if err := db.WithContext(ctx).Create(row).Error; err != nil {
		if logger != nil {
			logger.Warn("lifecycle server-audit write failed", zap.Error(err))
		}
	}
}
