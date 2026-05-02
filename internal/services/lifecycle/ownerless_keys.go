package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// OwnerlessKeysRule revokes Keys whose owning User has IsActive=false OR
// has been soft-deleted. Team-owned keys with no UserID are NOT touched —
// they're owned by the team. Master keys are skipped.
type OwnerlessKeysRule struct{}

func (r *OwnerlessKeysRule) Name() string { return "ownerless_keys" }

func (r *OwnerlessKeysRule) Run(ctx context.Context, db *gorm.DB, cfg Config, hooks WebhookEmitter) RuleResult {
	res := RuleResult{Rule: r.Name(), Actions: []Action{}}
	if !cfg.OwnerlessKeysEnabled {
		return res
	}

	// Use a Raw query with LEFT JOIN against the unscoped users table so we
	// can detect soft-deleted users (default GORM filtering would exclude
	// them entirely and we'd never see them as ownerless).
	type joined struct {
		KeyID         string     `gorm:"column:key_id"`
		IsActive      bool       `gorm:"column:is_active"`
		KeyName       string     `gorm:"column:name"`
		KeyPrefix     string     `gorm:"column:key_prefix"`
		UserID        *string    `gorm:"column:user_id"`
		TeamID        *string    `gorm:"column:team_id"`
		UserActive    *bool      `gorm:"column:user_active"`
		UserDeletedAt *time.Time `gorm:"column:user_deleted_at"`
	}
	var rows []joined
	err := db.WithContext(ctx).Raw(`
		SELECT k.id::text AS key_id, k.is_active, k.name, k.key_prefix,
		       k.user_id::text AS user_id, k.team_id::text AS team_id,
		       u.is_active AS user_active, u.deleted_at AS user_deleted_at
		FROM keys k
		LEFT JOIN users u ON u.id = k.user_id
		WHERE k.is_active = true
		  AND k.revoked_at IS NULL
		  AND k.type <> ?
		  AND k.user_id IS NOT NULL
	`, models.KeyTypeMaster).Scan(&rows).Error
	if err != nil {
		res.Err = err.Error()
		return res
	}
	res.Scanned = len(rows)

	for _, r2 := range rows {
		var why string
		switch {
		case r2.UserDeletedAt != nil:
			why = "owner user soft-deleted"
		case r2.UserActive != nil && !*r2.UserActive:
			why = "owner user inactive"
		case r2.UserActive == nil:
			why = "owner user missing"
		default:
			// owner is healthy — left join found an active, non-deleted user
			continue
		}

		keyID, err := uuid.Parse(r2.KeyID)
		if err != nil {
			continue
		}
		k := models.Key{
			BaseModel: models.BaseModel{ID: keyID},
			Name:      r2.KeyName,
			KeyPrefix: r2.KeyPrefix,
		}
		if r2.UserID != nil {
			if uid, err := uuid.Parse(*r2.UserID); err == nil {
				k.UserID = &uid
			}
		}
		if r2.TeamID != nil {
			if tid, err := uuid.Parse(*r2.TeamID); err == nil {
				k.TeamID = &tid
			}
		}

		now := time.Now()
		reason := fmt.Sprintf("ownerless: %s", why)
		res.Actions = append(res.Actions, Action{
			Rule:       r.Name(),
			ResourceID: k.ID.String(),
			Subject:    fmt.Sprintf("%s (%s)", k.Name, k.KeyPrefix),
			Reason:     reason,
			DryRun:     cfg.DryRun,
			At:         now,
		})
		if cfg.DryRun {
			continue
		}
		revokedAt, mutated, err := revokeKey(ctx, db, &k, reason)
		if err != nil {
			res.Actions[len(res.Actions)-1].Reason = "revoke failed: " + err.Error()
			continue
		}
		if !mutated {
			res.Actions[len(res.Actions)-1].Reason = "already revoked (race)"
			continue
		}
		writeKeyAudit(ctx, db, &k, "key_revoked_ownerless", reason, revokedAt, nil)
		if hooks != nil {
			hooks.Emit(WebhookEventEnvelope{
				Type: models.WebhookEventKeyRevokedOwnerless,
				Data: map[string]any{
					"key_id":      k.ID,
					"key_prefix":  k.KeyPrefix,
					"name":        k.Name,
					"user_id":     k.UserID,
					"team_id":     k.TeamID,
					"reason":      reason,
					"occurred_at": revokedAt.UTC(),
				},
			})
		}
	}
	return res
}
