package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
	lcel "github.com/amerfu/pllm/internal/services/lifecycle/cel"
)

// scanPageSize bounds the page each table walk pulls per round. CEL
// evaluation is microsecond-fast so a large page is fine; we paginate
// mainly so the daemon doesn't hold a 50k-row result set in memory.
const scanPageSize = 500

// maxActionsPerRun is the hard cap on actions a single custom rule
// can take in one tick. Stops a runaway rule (predicate matches
// everything) from revoking the entire keys table before the operator
// notices. Operators can keep clicking "Run now" if they really want
// more.
const maxActionsPerRun = 1000

// CustomRule is the runtime view of a LifecycleCustomRule row. The
// daemon's customLoader produces these by compiling the stored Match
// expression and pre-resolving the action dispatcher.
type CustomRule struct {
	Spec     *models.LifecycleCustomRule
	Compiled *lcel.CompiledRule
}

// NewCustomRule compiles the spec's CEL expression and returns a
// runnable rule. Errors include the rule name so failed config loads
// point operators at the offending entry directly.
func NewCustomRule(spec *models.LifecycleCustomRule) (*CustomRule, error) {
	if spec == nil {
		return nil, errors.New("nil spec")
	}
	prog, err := lcel.Compile(spec.Scan, spec.Match)
	if err != nil {
		return nil, fmt.Errorf("rule %q: %w", spec.Name, err)
	}
	return &CustomRule{Spec: spec, Compiled: prog}, nil
}

// Name satisfies the Rule interface; uses the spec's stable Name.
func (r *CustomRule) Name() string { return r.Spec.Name }

// Run walks the rule's scan target with stable pagination, evaluates
// the CEL predicate per row using a single `now`, and dispatches the
// configured action for each match. Honors the rule-level DryRun
// override on top of the global cfg.DryRun.
func (r *CustomRule) Run(ctx context.Context, db *gorm.DB, cfg Config, hooks WebhookEmitter) RuleResult {
	res := RuleResult{Rule: r.Name(), Actions: []Action{}}
	if !r.Spec.Enabled {
		return res
	}
	dryRun := cfg.DryRun
	if r.Spec.DryRun != nil {
		dryRun = *r.Spec.DryRun
	}
	now := time.Now()

	scan := func(handle func(input map[string]any, subject string, mutate func(reason string) (uuid.UUID, bool, error), audit func(at time.Time, reason string), event func(at time.Time, reason string) (models.WebhookEventType, map[string]any)) error) error {
		switch r.Spec.Scan {
		case models.LifecycleScanKeys:
			return r.scanKeys(ctx, db, &res, handle)
		case models.LifecycleScanServers:
			return r.scanServers(ctx, db, &res, handle)
		case models.LifecycleScanUseCases:
			return r.scanUseCases(ctx, db, &res, handle)
		case models.LifecycleScanUsers:
			return r.scanUsers(ctx, db, &res, handle)
		}
		return fmt.Errorf("unknown scan target: %s", r.Spec.Scan)
	}

	type pendingAction struct {
		at      time.Time
		subject string
		reason  string
		mutate  func(reason string) (uuid.UUID, bool, error)
		audit   func(at time.Time, reason string)
		event   func(at time.Time, reason string) (models.WebhookEventType, map[string]any)
		input   map[string]any
	}

	processRow := func(input map[string]any, subject string, mutate func(reason string) (uuid.UUID, bool, error), audit func(at time.Time, reason string), event func(at time.Time, reason string) (models.WebhookEventType, map[string]any)) error {
		match, err := r.Compiled.Eval(ctx, input, now)
		if err != nil {
			res.Err = err.Error()
			return err
		}
		if !match {
			return nil
		}
		if len(res.Actions) >= maxActionsPerRun {
			res.Err = fmt.Sprintf("hit per-run cap of %d actions; aborting further matches", maxActionsPerRun)
			return errStopScan
		}
		reason := r.Spec.Reason
		if reason == "" {
			reason = fmt.Sprintf("custom rule %q matched", r.Spec.Name)
		}
		act := Action{
			Rule:       r.Name(),
			ResourceID: stringID(input),
			Subject:    subject,
			Reason:     reason,
			DryRun:     dryRun,
			At:         now,
		}
		res.Actions = append(res.Actions, act)
		if dryRun || r.Spec.Action == models.LifecycleActionNotify {
			// notify never mutates; dry-run never mutates
			if !dryRun && r.Spec.Action == models.LifecycleActionNotify {
				// notify still emits webhook + audit — operators rely on it.
				audit(now, reason)
				if hooks != nil && event != nil {
					et, payload := event(now, reason)
					hooks.Emit(WebhookEventEnvelope{Type: et, Data: payload})
				}
			}
			return nil
		}
		_, mutated, err := mutate(reason)
		if err != nil {
			res.Actions[len(res.Actions)-1].Reason = "action failed: " + err.Error()
			return nil
		}
		if !mutated {
			res.Actions[len(res.Actions)-1].Reason = "already in target state (race)"
			return nil
		}
		audit(now, reason)
		if hooks != nil && event != nil {
			et, payload := event(now, reason)
			hooks.Emit(WebhookEventEnvelope{Type: et, Data: payload})
		}
		_ = pendingAction{} // keep struct around for future batched dispatch
		return nil
	}

	if err := scan(processRow); err != nil && !errors.Is(err, errStopScan) {
		if res.Err == "" {
			res.Err = err.Error()
		}
	}
	return res
}

// errStopScan is the sentinel a row handler returns to stop the page
// loop early — used when we've hit maxActionsPerRun and don't want to
// keep evaluating.
var errStopScan = errors.New("stop scan")

// stringID pulls the "id" field from a CEL input map. Inputs always
// include id (see input.go), so a nil/missing return signals an
// upstream bug.
func stringID(input map[string]any) string {
	if v, ok := input["id"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// rowHandler is the per-row callback the scanner invokes. The
// scanner knows how to mutate / audit / build a webhook payload for
// its specific resource type; the handler decides whether to apply.
type rowHandler func(
	input map[string]any,
	subject string,
	mutate func(reason string) (uuid.UUID, bool, error),
	audit func(at time.Time, reason string),
	event func(at time.Time, reason string) (models.WebhookEventType, map[string]any),
) error

// scanKeys walks the keys table in pages, calling handle once per row.
// The mutate callback resolves to revokeKey for action=revoke; for
// other actions on keys we still return a no-op closure so the row
// handler logic stays uniform.
func (r *CustomRule) scanKeys(ctx context.Context, db *gorm.DB, res *RuleResult, handle rowHandler) error {
	offset := 0
	for {
		var batch []models.Key
		err := db.WithContext(ctx).
			Where("revoked_at IS NULL").
			Order("created_at ASC").
			Offset(offset).
			Limit(scanPageSize).
			Find(&batch).Error
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		res.Scanned += len(batch)
		for i := range batch {
			k := batch[i]
			input := lcel.KeyToInput(&k)
			subject := fmt.Sprintf("%s (%s)", k.Name, k.KeyPrefix)
			mutate := func(reason string) (uuid.UUID, bool, error) {
				if r.Spec.Action != models.LifecycleActionRevoke {
					return uuid.Nil, false, fmt.Errorf("action %s not supported on keys", r.Spec.Action)
				}
				_, mutated, err := revokeKey(ctx, db, &k, reason)
				return k.ID, mutated, err
			}
			audit := func(at time.Time, reason string) {
				writeCustomKeyAudit(ctx, db, &k, r.Spec, reason, at)
			}
			event := func(at time.Time, reason string) (models.WebhookEventType, map[string]any) {
				return models.WebhookEventLifecycleCustom, map[string]any{
					"rule_name":   r.Spec.Name,
					"action":      string(r.Spec.Action),
					"target":      "keys",
					"resource_id": k.ID,
					"subject":     subject,
					"reason":      reason,
					"occurred_at": at.UTC(),
				}
			}
			if err := handle(input, subject, mutate, audit, event); err != nil {
				if errors.Is(err, errStopScan) {
					return err
				}
			}
		}
		if len(batch) < scanPageSize {
			break
		}
		offset += len(batch)
	}
	return nil
}

func (r *CustomRule) scanServers(ctx context.Context, db *gorm.DB, res *RuleResult, handle rowHandler) error {
	offset := 0
	for {
		var batch []models.RegistryServer
		err := db.WithContext(ctx).
			Order("created_at ASC").
			Offset(offset).
			Limit(scanPageSize).
			Find(&batch).Error
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		res.Scanned += len(batch)
		for i := range batch {
			s := batch[i]
			input := lcel.ServerToInput(&s)
			subject := fmt.Sprintf("%s@%s", s.Name, s.Version)
			mutate := func(reason string) (uuid.UUID, bool, error) {
				if r.Spec.Action != models.LifecycleActionBlock {
					return uuid.Nil, false, fmt.Errorf("action %s not supported on servers", r.Spec.Action)
				}
				upd := db.WithContext(ctx).Model(&models.RegistryServer{}).
					Where("id = ? AND status = ?", s.ID, models.RegistryStatusActive).
					Update("status", models.RegistryStatusBlocked)
				if upd.Error != nil {
					return uuid.Nil, false, upd.Error
				}
				return s.ID, upd.RowsAffected > 0, nil
			}
			audit := func(at time.Time, reason string) {
				writeCustomServerAudit(ctx, db, &s, r.Spec, reason, at)
			}
			event := func(at time.Time, reason string) (models.WebhookEventType, map[string]any) {
				return models.WebhookEventLifecycleCustom, map[string]any{
					"rule_name":   r.Spec.Name,
					"action":      string(r.Spec.Action),
					"target":      "registry_servers",
					"resource_id": s.ID,
					"subject":     subject,
					"reason":      reason,
					"occurred_at": at.UTC(),
				}
			}
			if err := handle(input, subject, mutate, audit, event); err != nil {
				if errors.Is(err, errStopScan) {
					return err
				}
			}
		}
		if len(batch) < scanPageSize {
			break
		}
		offset += len(batch)
	}
	return nil
}

func (r *CustomRule) scanUseCases(ctx context.Context, db *gorm.DB, res *RuleResult, handle rowHandler) error {
	offset := 0
	for {
		var batch []models.UseCase
		err := db.WithContext(ctx).
			Order("created_at ASC").
			Offset(offset).
			Limit(scanPageSize).
			Find(&batch).Error
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		res.Scanned += len(batch)
		for i := range batch {
			u := batch[i]
			input := lcel.UseCaseToInput(&u)
			subject := fmt.Sprintf("%s (%s)", u.Name, u.Slug)
			mutate := func(reason string) (uuid.UUID, bool, error) {
				if r.Spec.Action != models.LifecycleActionFlag {
					return uuid.Nil, false, fmt.Errorf("action %s not supported on use_cases", r.Spec.Action)
				}
				upd := db.WithContext(ctx).Model(&models.UseCase{}).
					Where("id = ? AND compliance_flag = ?", u.ID, false).
					Updates(map[string]any{
						"compliance_flag":        true,
						"compliance_flag_reason": reason,
					})
				if upd.Error != nil {
					return uuid.Nil, false, upd.Error
				}
				return u.ID, upd.RowsAffected > 0, nil
			}
			audit := func(at time.Time, reason string) {
				writeCustomUseCaseAudit(ctx, db, &u, r.Spec, reason, at)
			}
			event := func(at time.Time, reason string) (models.WebhookEventType, map[string]any) {
				return models.WebhookEventLifecycleCustom, map[string]any{
					"rule_name":   r.Spec.Name,
					"action":      string(r.Spec.Action),
					"target":      "use_cases",
					"resource_id": u.ID,
					"subject":     subject,
					"reason":      reason,
					"occurred_at": at.UTC(),
				}
			}
			if err := handle(input, subject, mutate, audit, event); err != nil {
				if errors.Is(err, errStopScan) {
					return err
				}
			}
		}
		if len(batch) < scanPageSize {
			break
		}
		offset += len(batch)
	}
	return nil
}

func (r *CustomRule) scanUsers(ctx context.Context, db *gorm.DB, res *RuleResult, handle rowHandler) error {
	offset := 0
	for {
		var batch []models.User
		err := db.WithContext(ctx).
			Order("created_at ASC").
			Offset(offset).
			Limit(scanPageSize).
			Find(&batch).Error
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		res.Scanned += len(batch)
		for i := range batch {
			u := batch[i]
			input := lcel.UserToInput(&u)
			subject := fmt.Sprintf("%s <%s>", u.Username, u.Email)
			mutate := func(reason string) (uuid.UUID, bool, error) {
				// users only support notify in v1 — flag/revoke would be a
				// destructive HR-level action, intentionally out of scope.
				return uuid.Nil, false, fmt.Errorf("action %s not supported on users", r.Spec.Action)
			}
			audit := func(at time.Time, reason string) {
				writeCustomUserAudit(ctx, db, &u, r.Spec, reason, at)
			}
			event := func(at time.Time, reason string) (models.WebhookEventType, map[string]any) {
				return models.WebhookEventLifecycleCustom, map[string]any{
					"rule_name":   r.Spec.Name,
					"action":      string(r.Spec.Action),
					"target":      "users",
					"resource_id": u.ID,
					"subject":     subject,
					"reason":      reason,
					"occurred_at": at.UTC(),
				}
			}
			if err := handle(input, subject, mutate, audit, event); err != nil {
				if errors.Is(err, errStopScan) {
					return err
				}
			}
		}
		if len(batch) < scanPageSize {
			break
		}
		offset += len(batch)
	}
	return nil
}

// writeCustomKeyAudit records a custom-rule key action with the rule's
// name in NewValues so auditors can filter on it later. Action is the
// canonical verb ("custom_revoke" / "custom_notify").
func writeCustomKeyAudit(ctx context.Context, db *gorm.DB, k *models.Key, spec *models.LifecycleCustomRule, reason string, at time.Time) {
	row := &models.Audit{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		EventType:    models.AuditEventLifecycle,
		EventAction:  fmt.Sprintf("custom_%s", spec.Action),
		EventResult:  models.AuditResult("success"),
		KeyID:        &k.ID,
		ResourceType: "key",
		ResourceID:   &k.ID,
		Message:      reason,
		Timestamp:    at,
	}
	if k.UserID != nil {
		row.UserID = k.UserID
	}
	if b, err := json.Marshal(map[string]any{
		"rule_name":   spec.Name,
		"rule_id":     spec.ID,
		"action":      string(spec.Action),
		"is_active":   false,
		"revoked_at":  at,
	}); err == nil {
		row.NewValues = b
	}
	_ = db.WithContext(ctx).Create(row).Error
}

func writeCustomServerAudit(ctx context.Context, db *gorm.DB, s *models.RegistryServer, spec *models.LifecycleCustomRule, reason string, at time.Time) {
	row := &models.Audit{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		EventType:    models.AuditEventLifecycle,
		EventAction:  fmt.Sprintf("custom_%s", spec.Action),
		EventResult:  models.AuditResult("success"),
		ResourceType: "registry_server",
		ResourceID:   &s.ID,
		Message:      reason,
		Timestamp:    at,
	}
	if b, err := json.Marshal(map[string]any{
		"rule_name": spec.Name,
		"rule_id":   spec.ID,
		"action":    string(spec.Action),
		"name":      s.Name,
		"version":   s.Version,
		"status":    string(models.RegistryStatusBlocked),
	}); err == nil {
		row.NewValues = b
	}
	_ = db.WithContext(ctx).Create(row).Error
}

func writeCustomUseCaseAudit(ctx context.Context, db *gorm.DB, u *models.UseCase, spec *models.LifecycleCustomRule, reason string, at time.Time) {
	row := &models.Audit{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		EventType:    models.AuditEventLifecycle,
		EventAction:  fmt.Sprintf("custom_%s", spec.Action),
		EventResult:  models.AuditResult("success"),
		ResourceType: "use_case",
		ResourceID:   &u.ID,
		Message:      reason,
		Timestamp:    at,
	}
	if b, err := json.Marshal(map[string]any{
		"rule_name":       spec.Name,
		"rule_id":         spec.ID,
		"action":          string(spec.Action),
		"compliance_flag": true,
		"slug":            u.Slug,
	}); err == nil {
		row.NewValues = b
	}
	_ = db.WithContext(ctx).Create(row).Error
}

func writeCustomUserAudit(ctx context.Context, db *gorm.DB, u *models.User, spec *models.LifecycleCustomRule, reason string, at time.Time) {
	row := &models.Audit{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		EventType:    models.AuditEventLifecycle,
		EventAction:  fmt.Sprintf("custom_%s", spec.Action),
		EventResult:  models.AuditResult("success"),
		UserID:       &u.ID,
		ResourceType: "user",
		ResourceID:   &u.ID,
		Message:      reason,
		Timestamp:    at,
	}
	if b, err := json.Marshal(map[string]any{
		"rule_name": spec.Name,
		"rule_id":   spec.ID,
		"action":    string(spec.Action),
		"email":     u.Email,
	}); err == nil {
		row.NewValues = b
	}
	_ = db.WithContext(ctx).Create(row).Error
}

// avoid unused-import warning when zap isn't used yet.
var _ = zap.NewNop
