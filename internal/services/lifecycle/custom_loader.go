package lifecycle

import (
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// NewDBCustomLoader returns a CustomRuleLoader that reads enabled
// LifecycleCustomRule rows from the database and compiles each into a
// runnable CustomRule. Compilation failures are logged and the rule
// is silently skipped — a single bad rule must not poison the tick.
//
// Compilation happens per-tick. CEL programs are sub-microsecond to
// build, and per-tick rebuild lets UI-edited rules take effect on the
// next interval without process state to invalidate.
func NewDBCustomLoader(db *gorm.DB, logger *zap.Logger) CustomRuleLoader {
	if logger == nil {
		logger = zap.NewNop()
	}
	return func(ctx context.Context) []Rule {
		var specs []models.LifecycleCustomRule
		if err := db.WithContext(ctx).
			Where("enabled = ?", true).
			Order("name ASC").
			Find(&specs).Error; err != nil {
			logger.Warn("custom rule load failed", zap.Error(err))
			return nil
		}
		out := make([]Rule, 0, len(specs))
		for i := range specs {
			s := specs[i]
			cr, err := NewCustomRule(&s)
			if err != nil {
				logger.Warn("custom rule compile failed",
					zap.String("rule", s.Name),
					zap.Error(err))
				continue
			}
			out = append(out, cr)
		}
		return out
	}
}
