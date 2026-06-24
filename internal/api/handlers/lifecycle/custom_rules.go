// Package lifecycle: custom-rule CRUD + preview endpoints. Each
// stored rule is a row in LifecycleCustomRule. Templates are read-only
// in-process catalog entries (services/lifecycle/templates.go).
package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
	lifesvc "github.com/amerfu/pllm/internal/services/lifecycle"
	lcel "github.com/amerfu/pllm/internal/services/lifecycle/cel"
)

// CustomRulesHandler manages the LifecycleCustomRule CRUD surface and
// the template catalog. Construction-time db is required; the lifecycle
// service is optional (templates + preview work without a running daemon).
type CustomRulesHandler struct {
	logger *zap.Logger
	db     *gorm.DB
}

// NewCustomRulesHandler builds a handler. nil db is permitted only for
// tests; production callers always pass a real db.
func NewCustomRulesHandler(logger *zap.Logger, db *gorm.DB) *CustomRulesHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CustomRulesHandler{logger: logger, db: db}
}

// ───────────────────────── DTOs ─────────────────────────

// templateOut mirrors lifecycle.Template but omits the unexported
// closures so the JSON shape is purely declarative for the UI.
type templateOut struct {
	ID          string                       `json:"id"`
	Title       string                       `json:"title"`
	Category    string                       `json:"category"`
	Description string                       `json:"description"`
	Scan        models.LifecycleScanTarget   `json:"scan"`
	Action      models.LifecycleAction       `json:"action"`
	Params      []lifesvc.TemplateParam      `json:"params"`
}

type ruleRequest struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	TemplateID  string                     `json:"template_id"`
	Params      map[string]any             `json:"params"`
	Enabled     *bool                      `json:"enabled,omitempty"`
	DryRun      *bool                      `json:"dry_run,omitempty"`
	// Reason is optional — falls back to template-generated reason
	Reason string `json:"reason,omitempty"`
}

type ruleResponse struct {
	*models.LifecycleCustomRule
	Template *templateOut `json:"template,omitempty"`
}

// ───────────────────────── Templates ─────────────────────────

// ListTemplates returns the in-process template catalog. Read-only.
func (h *CustomRulesHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	out := make([]templateOut, 0, len(lifesvc.Templates))
	for _, t := range lifesvc.Templates {
		out = append(out, templateOut{
			ID:          t.ID,
			Title:       t.Title,
			Category:    t.Category,
			Description: t.Description,
			Scan:        t.Scan,
			Action:      t.Action,
			Params:      t.Params,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": out})
}

// ───────────────────────── CRUD ─────────────────────────

// List returns every custom rule. Filters are deliberately not
// exposed — the operator-facing list rarely exceeds 50 rows and
// the UI sorts client-side.
func (h *CustomRulesHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeErr(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	var rules []models.LifecycleCustomRule
	if err := h.db.WithContext(r.Context()).
		Order("name ASC").
		Find(&rules).Error; err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]ruleResponse, 0, len(rules))
	for i := range rules {
		out = append(out, h.dressRule(&rules[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": out})
}

// Get returns one rule by ID.
func (h *CustomRulesHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeErr(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var rule models.LifecycleCustomRule
	if err := h.db.WithContext(r.Context()).First(&rule, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeErr(w, http.StatusNotFound, "rule not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.dressRule(&rule))
}

// Create persists a new rule. Validates that the resolved CEL
// expression compiles before saving; a save-then-fail-at-load loop is
// the worst possible UX for non-programmers, so we catch it here.
func (h *CustomRulesHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeErr(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	var req ruleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	rule, err := h.buildRule(r.Context(), &req, nil)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.WithContext(r.Context()).Create(rule).Error; err != nil {
		// Surface unique-name violations clearly.
		if isUniqueViolation(err) {
			writeErr(w, http.StatusConflict, "a rule with that name already exists")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, h.dressRule(rule))
}

// Update applies a partial-or-whole update. The CEL expression is
// re-compiled from the (possibly new) params.
func (h *CustomRulesHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeErr(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var existing models.LifecycleCustomRule
	if err := h.db.WithContext(r.Context()).First(&existing, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeErr(w, http.StatusNotFound, "rule not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var req ruleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	updated, err := h.buildRule(r.Context(), &req, &existing)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = existing.ID
	updated.CreatedAt = existing.CreatedAt
	updated.UpdatedAt = time.Now()
	if err := h.db.WithContext(r.Context()).Save(updated).Error; err != nil {
		if isUniqueViolation(err) {
			writeErr(w, http.StatusConflict, "a rule with that name already exists")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.dressRule(updated))
}

// Delete removes a rule. Idempotent — 404 if not found.
func (h *CustomRulesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeErr(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	res := h.db.WithContext(r.Context()).Delete(&models.LifecycleCustomRule{}, "id = ?", id)
	if res.Error != nil {
		writeErr(w, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		writeErr(w, http.StatusNotFound, "rule not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ───────────────────────── Preview ─────────────────────────

type previewRequest struct {
	TemplateID string         `json:"template_id"`
	Params     map[string]any `json:"params"`
	Limit      int            `json:"limit,omitempty"` // default 25, max 100
}

type previewMatch struct {
	ResourceID string `json:"resource_id"`
	Subject    string `json:"subject"`
	Detail     string `json:"detail,omitempty"`
}

type previewResponse struct {
	TemplateID string         `json:"template_id"`
	Scan       string         `json:"scan"`
	Action     string         `json:"action"`
	Match      string         `json:"match"`
	Reason     string         `json:"reason"`
	TotalRows  int            `json:"total_rows"`
	Matched    int            `json:"matched"`
	Capped     bool           `json:"capped"`
	Sample     []previewMatch `json:"sample"`
}

// Preview compiles the rule from a template + params and reports how
// many rows currently match — without mutating anything. The UI calls
// this from the rule builder so non-programmers can see exactly what
// their rule would do before clicking save.
func (h *CustomRulesHandler) Preview(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeErr(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	var req previewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	tmpl := lifesvc.FindTemplate(req.TemplateID)
	if tmpl == nil {
		writeErr(w, http.StatusBadRequest, "unknown template_id")
		return
	}
	expr, err := tmpl.BuildExpr(req.Params)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid params: "+err.Error())
		return
	}
	prog, err := lcel.Compile(tmpl.Scan, expr)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "compile failed: "+err.Error())
		return
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	resp := previewResponse{
		TemplateID: tmpl.ID,
		Scan:       string(tmpl.Scan),
		Action:     string(tmpl.Action),
		Match:      expr,
		Reason:     tmpl.BuildReason(req.Params),
		Sample:     []previewMatch{},
	}
	now := time.Now()

	// Per-target preview: walk the table once, evaluate per row, stop
	// when the sample limit is reached but keep counting matches.
	if err := h.previewScan(r.Context(), tmpl.Scan, prog, now, limit, &resp); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if resp.Matched > limit {
		resp.Capped = true
	}
	writeJSON(w, http.StatusOK, resp)
}

// previewScan walks the right table for a target and evaluates the
// program. We page over the whole table (even after the sample is
// full) because operators want the total count to inform their
// confidence in the rule.
func (h *CustomRulesHandler) previewScan(ctx context.Context, target models.LifecycleScanTarget, prog *lcel.CompiledRule, now time.Time, sampleLimit int, resp *previewResponse) error {
	const page = 500
	switch target {
	case models.LifecycleScanKeys:
		return scanForPreview(ctx, h.db, page, &resp.TotalRows, &resp.Matched, &resp.Sample, sampleLimit, prog, now,
			func(offset int, batch *[]models.Key) error {
				return h.db.WithContext(ctx).
					Where("revoked_at IS NULL").
					Order("created_at ASC").
					Offset(offset).Limit(page).
					Find(batch).Error
			},
			func(k *models.Key) (map[string]any, string) {
				return lcel.KeyToInput(k), fmt.Sprintf("%s (%s)", k.Name, k.KeyPrefix)
			},
			func(k *models.Key) string { return k.ID.String() })
	case models.LifecycleScanServers:
		return scanForPreview(ctx, h.db, page, &resp.TotalRows, &resp.Matched, &resp.Sample, sampleLimit, prog, now,
			func(offset int, batch *[]models.RegistryServer) error {
				return h.db.WithContext(ctx).
					Order("created_at ASC").
					Offset(offset).Limit(page).
					Find(batch).Error
			},
			func(s *models.RegistryServer) (map[string]any, string) {
				return lcel.ServerToInput(s), fmt.Sprintf("%s@%s", s.Name, s.Version)
			},
			func(s *models.RegistryServer) string { return s.ID.String() })
	case models.LifecycleScanUseCases:
		return scanForPreview(ctx, h.db, page, &resp.TotalRows, &resp.Matched, &resp.Sample, sampleLimit, prog, now,
			func(offset int, batch *[]models.UseCase) error {
				return h.db.WithContext(ctx).
					Order("created_at ASC").
					Offset(offset).Limit(page).
					Find(batch).Error
			},
			func(u *models.UseCase) (map[string]any, string) {
				return lcel.UseCaseToInput(u), fmt.Sprintf("%s (%s)", u.Name, u.Slug)
			},
			func(u *models.UseCase) string { return u.ID.String() })
	case models.LifecycleScanUsers:
		return scanForPreview(ctx, h.db, page, &resp.TotalRows, &resp.Matched, &resp.Sample, sampleLimit, prog, now,
			func(offset int, batch *[]models.User) error {
				return h.db.WithContext(ctx).
					Order("created_at ASC").
					Offset(offset).Limit(page).
					Find(batch).Error
			},
			func(u *models.User) (map[string]any, string) {
				return lcel.UserToInput(u), fmt.Sprintf("%s <%s>", u.Username, u.Email)
			},
			func(u *models.User) string { return u.ID.String() })
	}
	return fmt.Errorf("unsupported scan target: %s", target)
}

// scanForPreview is the generic page-and-evaluate loop shared by all
// preview targets. Generics keep the per-target switch above tiny.
func scanForPreview[T any](
	ctx context.Context,
	db *gorm.DB,
	page int,
	total, matched *int,
	sample *[]previewMatch,
	sampleLimit int,
	prog *lcel.CompiledRule,
	now time.Time,
	fetch func(offset int, batch *[]T) error,
	toInput func(*T) (map[string]any, string),
	toID func(*T) string,
) error {
	offset := 0
	for {
		var batch []T
		if err := fetch(offset, &batch); err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		*total += len(batch)
		for i := range batch {
			input, subject := toInput(&batch[i])
			ok, err := prog.Eval(ctx, input, now)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			*matched++
			if len(*sample) < sampleLimit {
				*sample = append(*sample, previewMatch{
					ResourceID: toID(&batch[i]),
					Subject:    subject,
				})
			}
		}
		if len(batch) < page {
			break
		}
		offset += len(batch)
	}
	return nil
}

// ───────────────────────── helpers ─────────────────────────

// buildRule compiles a request into a LifecycleCustomRule, validating
// fields along the way. existing is non-nil on update — used to
// preserve the row's ID/created_at.
func (h *CustomRulesHandler) buildRule(ctx context.Context, req *ruleRequest, existing *models.LifecycleCustomRule) (*models.LifecycleCustomRule, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	if !validRuleName(name) {
		return nil, errors.New("name must be lowercase letters, digits, underscore (3-64 chars)")
	}
	tmpl := lifesvc.FindTemplate(req.TemplateID)
	if tmpl == nil {
		return nil, fmt.Errorf("unknown template_id %q", req.TemplateID)
	}
	expr, err := tmpl.BuildExpr(req.Params)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if _, err := lcel.Compile(tmpl.Scan, expr); err != nil {
		// Should never happen if BuildExpr is well-formed, but fail
		// loud if it does — saving an uncompilable rule poisons every
		// future tick.
		return nil, fmt.Errorf("compile failed: %w", err)
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = tmpl.BuildReason(req.Params)
	}
	rule := &models.LifecycleCustomRule{
		Name:        name,
		Description: req.Description,
		Scan:        tmpl.Scan,
		Match:       expr,
		Action:      tmpl.Action,
		Reason:      reason,
		Enabled:     true,
		TemplateID:  tmpl.ID,
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.DryRun != nil {
		dr := *req.DryRun
		rule.DryRun = &dr
	}
	if req.Params != nil {
		if b, err := json.Marshal(req.Params); err == nil {
			rule.Params = datatypes.JSON(b)
		}
	}
	return rule, nil
}

// dressRule wraps a rule with its template metadata for richer UI
// rendering (icon, category, etc.). Template may be nil for legacy
// or hand-edited rules referencing a template that's been removed.
func (h *CustomRulesHandler) dressRule(rule *models.LifecycleCustomRule) ruleResponse {
	out := ruleResponse{LifecycleCustomRule: rule}
	if t := lifesvc.FindTemplate(rule.TemplateID); t != nil {
		out.Template = &templateOut{
			ID:          t.ID,
			Title:       t.Title,
			Category:    t.Category,
			Description: t.Description,
			Scan:        t.Scan,
			Action:      t.Action,
			Params:      t.Params,
		}
	}
	return out
}

func validRuleName(s string) bool {
	if len(s) < 3 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '_':
		default:
			return false
		}
	}
	return true
}

// isUniqueViolation peeks at the error string for the canonical
// Postgres "duplicate key value violates unique constraint" message.
// Saves us the lib/pq dependency in this handler.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate key value")
}
