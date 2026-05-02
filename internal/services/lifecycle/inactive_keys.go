package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// InactiveKeysRule revokes Keys whose LastUsedAt (or CreatedAt for
// never-used keys) is older than Config.InactiveKeyDays. Master keys
// are exempt — operators rotate them by hand.
type InactiveKeysRule struct{}

// Name is the canonical identifier in the run summary + audit row.
func (r *InactiveKeysRule) Name() string { return "inactive_keys" }

func (r *InactiveKeysRule) Run(ctx context.Context, db *gorm.DB, cfg Config, hooks WebhookEmitter) RuleResult {
	res := RuleResult{Rule: r.Name()}
	if cfg.InactiveKeyDays <= 0 {
		return res
	}

	cutoff := time.Now().Add(-time.Duration(cfg.InactiveKeyDays) * 24 * time.Hour)

	var keys []models.Key
	err := db.WithContext(ctx).
		Where("is_active = ? AND revoked_at IS NULL AND type <> ?", true, models.KeyTypeMaster).
		Where("(last_used_at IS NOT NULL AND last_used_at < ?) OR (last_used_at IS NULL AND created_at < ?)",
			cutoff, cutoff).
		Find(&keys).Error
	if err != nil {
		res.Err = err.Error()
		return res
	}
	res.Scanned = len(keys)

	for i := range keys {
		k := keys[i]
		reason := fmt.Sprintf("no activity since %s (threshold=%d days)",
			lastSeen(&k), cfg.InactiveKeyDays)
		act := Action{
			Rule:       r.Name(),
			ResourceID: k.ID.String(),
			Subject:    fmt.Sprintf("%s (%s)", k.Name, k.KeyPrefix),
			Reason:     reason,
			DryRun:     cfg.DryRun,
			At:         time.Now(),
		}
		res.Actions = append(res.Actions, act)
		if cfg.DryRun {
			continue
		}
		if err := revokeKey(ctx, db, &k, reason); err != nil {
			res.Actions[len(res.Actions)-1].Reason = "revoke failed: " + err.Error()
			continue
		}
		writeKeyAudit(ctx, db, &k, r.Name(), reason)
		if hooks != nil {
			hooks.Emit(WebhookEventEnvelope{
				Type: models.WebhookEventKeyRevokedInactive,
				Data: map[string]any{
					"key_id":      k.ID,
					"key_prefix":  k.KeyPrefix,
					"name":        k.Name,
					"user_id":     k.UserID,
					"team_id":     k.TeamID,
					"reason":      reason,
					"occurred_at": time.Now().UTC(),
				},
			})
		}
	}
	return res
}

// revokeKey is the shared mutation used by all key-revoking rules.
// Lifecycle is system-driven, so RevokedBy is left nil — the audit row
// carries the system attribution via EventType=lifecycle.
func revokeKey(ctx context.Context, db *gorm.DB, k *models.Key, reason string) error {
	now := time.Now()
	return db.WithContext(ctx).Model(&models.Key{}).
		Where("id = ?", k.ID).
		Updates(map[string]any{
			"is_active":         false,
			"revoked_at":        now,
			"revocation_reason": reason,
		}).Error
}

// writeKeyAudit persists a single AuditEventLifecycle row keyed to the
// affected key. EventAction encodes the rule name so operators can
// filter the audit log per-rule.
func writeKeyAudit(ctx context.Context, db *gorm.DB, k *models.Key, action, reason string) {
	row := &models.Audit{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		EventType:    models.AuditEventLifecycle,
		EventAction:  fmt.Sprintf("key_%s", action),
		EventResult:  models.AuditResult("success"),
		KeyID:        &k.ID,
		ResourceType: "key",
		ResourceID:   &k.ID,
		Message:      reason,
		Timestamp:    time.Now(),
	}
	if k.UserID != nil {
		row.UserID = k.UserID
	}
	if b, err := json.Marshal(map[string]any{"is_active": false, "revoked_at": time.Now()}); err == nil {
		row.NewValues = b
	}
	if err := db.WithContext(ctx).Create(row).Error; err != nil {
		_ = err
	}
}

// lastSeen returns a human-readable last-activity hint for the audit message.
func lastSeen(k *models.Key) string {
	if k.LastUsedAt != nil {
		return k.LastUsedAt.UTC().Format(time.RFC3339)
	}
	return "never"
}
