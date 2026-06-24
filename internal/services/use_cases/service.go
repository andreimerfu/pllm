// Package use_cases owns the UseCase entity + its component-edge
// management. A UseCase is a top-level approval unit: it references
// registry components (servers, agents, skills, prompts, LLMs) via the
// same RegistryRef table that agents use, with OwnerKind="use_case".
package use_cases

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// ErrNotFound is returned by Get when the requested use case is absent.
var ErrNotFound = errors.New("use_case: not found")

// ErrConflict is returned when a unique constraint would be violated
// (same name/slug already exists).
var ErrConflict = errors.New("use_case: conflict")

// Service owns all UseCase reads/writes.
type Service struct {
	db         *gorm.DB
	logger     *zap.Logger
	reconciler *Reconciler    // optional; nil = no auto-propagation
	scope      *ScopeResolver // shared with the edge handlers
	events     *EventRecorder // optional; nil silently drops audit writes
	hooks      WebhookEmitter // optional; nil disables lifecycle webhooks
}

// NewService constructs a Service.
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		db:         db,
		logger:     logger,
		reconciler: NewReconciler(db, logger),
		scope:      NewScopeResolver(db),
		events:     NewEventRecorder(db, logger),
	}
}

// Scope returns the shared resolver so the HTTP layer can plug it into
// the MCP + LLM handlers without the service leaking a concrete type.
func (s *Service) Scope() *ScopeResolver { return s.scope }

// Events returns the audit-log recorder so edge handlers can write
// scope events and the admin UI can read them.
func (s *Service) Events() *EventRecorder { return s.events }

// WithWebhooks attaches the outbound emitter to both the service
// (lifecycle events) and the underlying EventRecorder (scope events).
// Returns the receiver for chaining.
func (s *Service) WithWebhooks(e WebhookEmitter) *Service {
	s.hooks = e
	if s.events != nil {
		s.events = s.events.WithWebhooks(e)
	}
	return s
}

// emitLifecycle pushes a use_case.* event for CRUD operations.
// Swallows nil emitter — scope wiring is optional.
func (s *Service) emitLifecycle(kind string, uc *models.UseCase) {
	if s.hooks == nil || uc == nil {
		return
	}
	s.hooks.Emit(WebhookEventEnvelope{
		Type: kind,
		Data: map[string]any{
			"id":                   uc.ID,
			"slug":                 uc.Slug,
			"name":                 uc.Name,
			"risk_tier":            uc.RiskTier,
			"status":               uc.Status,
			"data_classifications": []string(uc.DataClassifications),
			"enforce_scope":        uc.EnforceScope,
			"compliance_flag":      uc.ComplianceFlag,
		},
	})
}

// TriggerRecompute schedules a reconcile for the given use case. Public
// so the router can expose it as an admin endpoint.
func (s *Service) TriggerRecompute(ctx context.Context, id uuid.UUID) error {
	if s.reconciler == nil {
		return nil
	}
	return s.reconciler.ComputeOne(ctx, id)
}

// WithRefs is the detailed response shape for a single use case:
// the row itself plus its component edges grouped by target kind so
// the UI doesn't have to re-bucket them.
type WithRefs struct {
	UseCase models.UseCase       `json:"use_case"`
	Servers []models.RegistryRef `json:"servers,omitempty"`
	Agents  []models.RegistryRef `json:"agents,omitempty"`
	Skills  []models.RegistryRef `json:"skills,omitempty"`
	Prompts []models.RegistryRef `json:"prompts,omitempty"`
	LLMs    []models.RegistryRef `json:"llms,omitempty"`
}

// ListFilter narrows a list query.
//
// Compliance-minded filters:
//   - Classification          — matches against declared data_classifications
//   - EffectiveClassification — matches against the computed set (includes
//                               transitively inherited tags)
//   - ComplianceFlag          — pointer so nil = "any", true/false = explicit
//   - HasThirdParty           — true = "at least one LLM dep is third-party"
type ListFilter struct {
	Search                  string
	Status                  models.UseCaseStatus
	RiskTier                models.UseCaseRiskTier
	TeamID                  *uuid.UUID
	Classification          string
	EffectiveClassification string
	ComplianceFlag          *bool
	HasThirdParty           *bool
	Limit                   int
	Offset                  int
}

// List returns use cases matching the filter, newest-first.
func (s *Service) List(ctx context.Context, f ListFilter) ([]models.UseCase, error) {
	q := s.db.WithContext(ctx).Model(&models.UseCase{})
	if f.Search != "" {
		pattern := "%" + strings.ToLower(f.Search) + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", pattern, pattern)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.RiskTier != "" {
		q = q.Where("risk_tier = ?", f.RiskTier)
	}
	if f.TeamID != nil {
		q = q.Where("team_id = ?", *f.TeamID)
	}
	// PostgreSQL array containment: "does the string column contain this value?"
	if f.Classification != "" {
		q = q.Where("? = ANY(data_classifications)", f.Classification)
	}
	if f.EffectiveClassification != "" {
		q = q.Where("? = ANY(effective_classifications)", f.EffectiveClassification)
	}
	if f.ComplianceFlag != nil {
		q = q.Where("compliance_flag = ?", *f.ComplianceFlag)
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []models.UseCase
	if err := q.Order("created_at desc").Limit(limit).Offset(f.Offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	// HasThirdParty is a post-filter — it requires walking refs to LLMs
	// and resolving them against UserModel.IsFirstParty. Avoid the SQL
	// complexity; this runs at most on ~200 rows per page.
	if f.HasThirdParty != nil {
		filtered := make([]models.UseCase, 0, len(rows))
		for _, uc := range rows {
			has, err := s.hasThirdPartyLLM(ctx, uc.ID)
			if err != nil {
				return nil, err
			}
			if has == *f.HasThirdParty {
				filtered = append(filtered, uc)
			}
		}
		rows = filtered
	}
	return rows, nil
}

// hasThirdPartyLLM returns true if any of the use case's LLM edges points
// at a UserModel marked is_first_party=false (the default — opt-in model).
// Unresolved LLM names (e.g. config-defined models with no DB row yet)
// are treated as third-party to err on the side of flagging them.
func (s *Service) hasThirdPartyLLM(ctx context.Context, useCaseID uuid.UUID) (bool, error) {
	var refs []models.RegistryRef
	err := s.db.WithContext(ctx).
		Where("owner_kind = ? AND owner_id = ? AND target_kind = ?",
			models.RegistryKindUseCase, useCaseID, models.RegistryKindLLM).
		Find(&refs).Error
	if err != nil {
		return false, err
	}
	if len(refs) == 0 {
		return false, nil
	}
	for _, ref := range refs {
		var um models.UserModel
		err := s.db.WithContext(ctx).
			Where("model_name = ?", ref.TargetName).
			First(&um).Error
		if err != nil {
			// Unresolved → assume third-party (safer default for compliance).
			return true, nil
		}
		if !um.IsFirstParty {
			return true, nil
		}
	}
	return false, nil
}

// Get returns a use case + its component refs.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*WithRefs, error) {
	var row models.UseCase
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	refs, err := s.loadRefs(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	return &WithRefs{
		UseCase: row,
		Servers: filter(refs, models.RegistryKindServer),
		Agents:  filter(refs, models.RegistryKindAgent),
		Skills:  filter(refs, models.RegistryKindSkill),
		Prompts: filter(refs, models.RegistryKindPrompt),
		LLMs:    filter(refs, models.RegistryKindLLM),
	}, nil
}

// GetBySlug is a convenience for the UI which navigates by slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*WithRefs, error) {
	var row models.UseCase
	if err := s.db.WithContext(ctx).First(&row, "slug = ?", slug).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.Get(ctx, row.ID)
}

// UpsertInput is what the handler hands to Create/Update.
// Refs replace the entire edge set for the use case.
type UpsertInput struct {
	Name                string
	Slug                string
	Description         string
	RiskTier            models.UseCaseRiskTier
	Status              models.UseCaseStatus
	BusinessOwnerUserID *uuid.UUID
	ApproverUserID      *uuid.UUID
	TeamID              *uuid.UUID
	DataClassifications []string
	ApprovalRecord      []byte
	Refs                []models.RegistryRef
	// EnforceScope is a pointer so we can distinguish "not provided"
	// (leave existing) from "explicitly false" (flag mode).
	EnforceScope *bool
}

// Create inserts a new use case. Slug defaults to a normalized Name.
func (s *Service) Create(ctx context.Context, in *UpsertInput) (*WithRefs, error) {
	if err := validateInput(in, true); err != nil {
		return nil, err
	}
	if in.Slug == "" {
		in.Slug = normalizeSlug(in.Name)
	}
	row := &models.UseCase{
		BaseModel:           models.BaseModel{ID: uuid.New()},
		Name:                in.Name,
		Slug:                in.Slug,
		Description:         in.Description,
		RiskTier:            defaultTier(in.RiskTier),
		Status:              defaultStatus(in.Status),
		BusinessOwnerUserID: in.BusinessOwnerUserID,
		ApproverUserID:      in.ApproverUserID,
		TeamID:              in.TeamID,
		DataClassifications: pq.StringArray(in.DataClassifications),
		ApprovalRecord:      in.ApprovalRecord,
	}
	if in.EnforceScope != nil {
		row.EnforceScope = *in.EnforceScope
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrConflict
			}
			return err
		}
		return s.writeRefs(tx, row.ID, in.Refs)
	})
	if err != nil {
		return nil, err
	}
	// Kick off classification propagation in the background so POST
	// responses stay snappy. Errors are logged but not surfaced.
	s.scheduleReconcile(row.ID)
	// Fresh row → no cache to drop, but keep the call for symmetry.
	s.scope.Invalidate(row.ID)
	s.emitLifecycle("use_case.created", row)
	return s.Get(ctx, row.ID)
}

// Update overwrites mutable fields + replaces the refs set.
// Slug is immutable after creation.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in *UpsertInput) (*WithRefs, error) {
	if err := validateInput(in, false); err != nil {
		return nil, err
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row models.UseCase
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		row.Name = in.Name
		row.Description = in.Description
		row.RiskTier = defaultTier(in.RiskTier)
		row.Status = defaultStatus(in.Status)
		row.BusinessOwnerUserID = in.BusinessOwnerUserID
		row.ApproverUserID = in.ApproverUserID
		row.TeamID = in.TeamID
		row.DataClassifications = pq.StringArray(in.DataClassifications)
		if in.ApprovalRecord != nil {
			row.ApprovalRecord = in.ApprovalRecord
		}
		if in.EnforceScope != nil {
			row.EnforceScope = *in.EnforceScope
		}
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		return s.writeRefs(tx, row.ID, in.Refs)
	})
	if err != nil {
		return nil, err
	}
	s.scheduleReconcile(id)
	s.scope.Invalidate(id)
	// Emit the lifecycle event after the transaction commits. Use the
	// freshly-re-read row so subscribers see the post-update state.
	if out, err := s.Get(ctx, id); err == nil {
		s.emitLifecycle("use_case.updated", &out.UseCase)
	}
	return s.Get(ctx, id)
}

// scheduleReconcile kicks off a background ComputeOne for the use case.
// Keeps the HTTP write path fast — effective classifications show up on
// the next read. Errors are logged only (transient DB issues retry on
// the next write).
func (s *Service) scheduleReconcile(id uuid.UUID) {
	if s.reconciler == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.reconciler.ComputeOne(ctx, id); err != nil {
			s.logger.Warn("use-case reconcile failed",
				zap.String("id", id.String()), zap.Error(err))
		}
	}()
}

// Delete removes the use case + cascades its refs. Hard delete — there's
// no soft-delete semantic here (soft-deletion of an approved use case is
// "retirement", a status change, not a removal).
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// Snapshot the row before deletion so the webhook carries useful
	// metadata (name, slug) — a subscriber receiving only "id" would be
	// useless for an audit trail.
	var snapshot models.UseCase
	_ = s.db.WithContext(ctx).First(&snapshot, "id = ?", id).Error

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", id).Delete(&models.UseCase{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return tx.Where("owner_kind = ? AND owner_id = ?",
			models.RegistryKindUseCase, id).Delete(&models.RegistryRef{}).Error
	})
	if err == nil {
		s.scope.Invalidate(id)
		if snapshot.ID != uuid.Nil {
			s.emitLifecycle("use_case.deleted", &snapshot)
		}
	}
	return err
}

// ListBindings returns use-case-owned registry refs — the
// (use_case → resource) edges. When useCaseID is non-nil it scopes to one
// use case; nil returns the edges for every use case. Used by the catalog
// facade to publish bindings to an external control plane.
func (s *Service) ListBindings(ctx context.Context, useCaseID *uuid.UUID) ([]models.RegistryRef, error) {
	q := s.db.WithContext(ctx).Where("owner_kind = ?", models.RegistryKindUseCase)
	if useCaseID != nil {
		q = q.Where("owner_id = ?", *useCaseID)
	}
	var refs []models.RegistryRef
	if err := q.Order("owner_id asc, target_kind asc, target_name asc").Find(&refs).Error; err != nil {
		return nil, err
	}
	return refs, nil
}

// --- internal ------------------------------------------------------------

func (s *Service) loadRefs(ctx context.Context, id uuid.UUID) ([]models.RegistryRef, error) {
	var refs []models.RegistryRef
	err := s.db.WithContext(ctx).
		Where("owner_kind = ? AND owner_id = ?", models.RegistryKindUseCase, id).
		Order("target_kind asc, target_name asc").
		Find(&refs).Error
	return refs, err
}

// writeRefs replaces the existing edge set wholesale. Matches the agent
// service's semantic so operators don't get surprised by diffing.
func (s *Service) writeRefs(tx *gorm.DB, ownerID uuid.UUID, refs []models.RegistryRef) error {
	if err := tx.Where("owner_kind = ? AND owner_id = ?",
		models.RegistryKindUseCase, ownerID).
		Delete(&models.RegistryRef{}).Error; err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}
	rows := make([]models.RegistryRef, 0, len(refs))
	for _, r := range refs {
		if !models.IsValidTargetKind(r.TargetKind) {
			return fmt.Errorf("invalid target_kind %q", r.TargetKind)
		}
		if r.TargetName == "" {
			return fmt.Errorf("target_name is required")
		}
		rows = append(rows, models.RegistryRef{
			BaseModel:     models.BaseModel{ID: uuid.New()},
			OwnerKind:     models.RegistryKindUseCase,
			OwnerID:       ownerID,
			TargetKind:    r.TargetKind,
			TargetName:    r.TargetName,
			TargetVersion: r.TargetVersion,
			LocalName:     r.LocalName,
		})
	}
	return tx.Create(&rows).Error
}

func filter(refs []models.RegistryRef, kind models.RegistryKind) []models.RegistryRef {
	out := make([]models.RegistryRef, 0, len(refs))
	for _, r := range refs {
		if r.TargetKind == kind {
			out = append(out, r)
		}
	}
	return out
}

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

func validateInput(in *UpsertInput, isCreate bool) error {
	if in == nil {
		return errors.New("input is nil")
	}
	if in.Name == "" {
		return errors.New("name is required")
	}
	if len(in.Name) > 200 {
		return errors.New("name is too long")
	}
	if isCreate && in.Slug != "" {
		if !slugRE.MatchString(in.Slug) && strings.ToLower(in.Slug) != in.Slug {
			return errors.New("slug must be lowercase; use letters, digits, '-' or '_'")
		}
	}
	return nil
}

func defaultTier(t models.UseCaseRiskTier) models.UseCaseRiskTier {
	if t == "" {
		return models.UseCaseRiskTier4
	}
	return t
}

func defaultStatus(s models.UseCaseStatus) models.UseCaseStatus {
	if s == "" {
		return models.UseCaseStatusDraft
	}
	return s
}

func isUniqueViolation(err error) bool {
	// GORM wraps Postgres errors; a unique violation contains code 23505.
	// We keep the check loose to avoid a hard dep on pgx types here.
	s := err.Error()
	return strings.Contains(s, "duplicate key") || strings.Contains(s, "unique constraint")
}
