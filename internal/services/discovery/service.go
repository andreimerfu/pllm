package discovery

import (
	"context"
	"errors"
	"time"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// PolicyResult is consumed by Service.upsertOne to update the call row.
// Defined here (not in policy.go) so it's in scope for the
// PolicyEvaluator interface even when policy.go isn't compiled in.
type PolicyResult struct {
	Outcome models.ScopeEventOutcome
	Reason  string
}

// PolicyEvaluator decides whether an observed call is allowed for the
// agent's bound use case. Implementations live in policy.go.
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, agent *models.DiscoveredAgent, call *models.DiscoveredCall) PolicyResult
}

// FlagReactor reacts to flagged-count changes on an agent.
// Implementations live in revoker.go. nil => no auto-revoke.
type FlagReactor interface {
	OnFlagged(ctx context.Context, agent *models.DiscoveredAgent)
}

// AgentResolver looks up the RegistryAgent service when promoting.
// We keep it abstract so the discovery package doesn't pull in the
// registry service directly. The router wires the real adapter.
type AgentResolver interface {
	PublishFromDiscovery(ctx context.Context, in PublishInput) (uuid.UUID, error)
}

type PublishInput struct {
	Name        string
	Version     string
	Title       string
	Description string
}

// Service is the top-level discovery API. It is goroutine-safe: GORM
// connection pool serializes inserts and we use ON CONFLICT for upserts.
type Service struct {
	db      *gorm.DB
	logger  *zap.Logger
	policy  PolicyEvaluator
	revoker FlagReactor
	agents  AgentResolver
}

func NewService(
	db *gorm.DB,
	logger *zap.Logger,
	policy PolicyEvaluator,
	revoker FlagReactor,
	agents AgentResolver,
) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{db: db, logger: logger, policy: policy, revoker: revoker, agents: agents}
}

// Ingest persists one OTLP/HTTP request. Errors during a single
// ResourceSpans block are logged but don't fail the whole batch — OTel
// exporters retry the entire envelope, which would amplify partial
// failures.
func (s *Service) Ingest(ctx context.Context, req ExportTraceServiceRequest) error {
	for _, rs := range req.ResourceSpans {
		processed := ProcessResourceSpans(rs)
		if processed.Agent.ServiceName == "" {
			continue
		}
		if err := s.upsertOne(ctx, processed); err != nil {
			s.logger.Warn("discovery upsert failed",
				zap.String("service", processed.Agent.ServiceName),
				zap.Error(err))
		}
	}
	return nil
}

func (s *Service) upsertOne(ctx context.Context, p ProcessedSpans) error {
	now := time.Now()

	var keyID *uuid.UUID
	if p.Agent.KeyIDRaw != "" {
		if id, err := uuid.Parse(p.Agent.KeyIDRaw); err == nil {
			keyID = &id
		}
	}

	// Find-or-create on the unique service_name. We don't use OnConflict
	// here because BaseModel.BeforeCreate generates a fresh UUID, which
	// makes the upsert RETURNING ambiguous about which row was touched.
	var agent models.DiscoveredAgent
	err := s.db.WithContext(ctx).
		Where("service_name = ?", p.Agent.ServiceName).
		First(&agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		agent = models.DiscoveredAgent{
			ServiceName:    p.Agent.ServiceName,
			ServiceVersion: p.Agent.ServiceVersion,
			KeyID:          keyID,
			Status:         models.DiscoveredStatusPending,
			FirstSeenAt:    now,
			LastSeenAt:     now,
		}
		if cerr := s.db.WithContext(ctx).Create(&agent).Error; cerr != nil {
			return cerr
		}
	} else if err != nil {
		return err
	} else {
		updates := map[string]any{
			"last_seen_at":    now,
			"service_version": p.Agent.ServiceVersion,
			"updated_at":      now,
		}
		if keyID != nil {
			updates["key_id"] = keyID
		}
		if uerr := s.db.WithContext(ctx).Model(&agent).Updates(updates).Error; uerr != nil {
			return uerr
		}
	}

	for _, c := range p.Calls {
		var existing models.DiscoveredCall
		err := s.db.WithContext(ctx).
			Where("discovered_agent_id = ? AND kind = ? AND target_name = ?",
				agent.ID, models.DiscoveredCallKind(c.Kind), c.TargetName).
			First(&existing).Error

		var call *models.DiscoveredCall
		if errors.Is(err, gorm.ErrRecordNotFound) {
			call = &models.DiscoveredCall{
				DiscoveredAgentID: agent.ID,
				Kind:              models.DiscoveredCallKind(c.Kind),
				TargetName:        c.TargetName,
				Count:             c.Count,
				FirstSeenAt:       now,
				LastSeenAt:        now,
			}
			if cerr := s.db.WithContext(ctx).Create(call).Error; cerr != nil {
				s.logger.Warn("discovery call insert failed", zap.Error(cerr))
				continue
			}
		} else if err != nil {
			s.logger.Warn("discovery call lookup failed", zap.Error(err))
			continue
		} else {
			existing.Count += c.Count
			existing.LastSeenAt = now
			if uerr := s.db.WithContext(ctx).Save(&existing).Error; uerr != nil {
				s.logger.Warn("discovery call update failed", zap.Error(uerr))
				continue
			}
			call = &existing
		}

		if s.policy == nil {
			continue
		}
		res := s.policy.Evaluate(ctx, &agent, call)
		updates := map[string]any{
			"last_outcome": string(res.Outcome),
			"last_reason":  res.Reason,
		}
		if err := s.db.WithContext(ctx).Model(call).Updates(updates).Error; err != nil {
			s.logger.Warn("discovery call outcome update failed", zap.Error(err))
		}
		if res.Outcome == models.ScopeOutcomeFlagged {
			if err := s.db.WithContext(ctx).
				Model(&models.DiscoveredAgent{}).
				Where("id = ?", agent.ID).
				UpdateColumn("flagged_count", gorm.Expr("flagged_count + 1")).
				Error; err != nil {
				s.logger.Warn("discovery flagged_count bump failed", zap.Error(err))
			}
			if err := s.db.WithContext(ctx).First(&agent, "id = ?", agent.ID).Error; err == nil && s.revoker != nil {
				s.revoker.OnFlagged(ctx, &agent)
			}
		}
	}
	return nil
}

// List returns all DiscoveredAgent rows filtered by status.
// Empty status returns all.
func (s *Service) List(ctx context.Context, status models.DiscoveredAgentStatus) ([]models.DiscoveredAgent, error) {
	q := s.db.WithContext(ctx).Order("last_seen_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.DiscoveredAgent
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Get returns one agent + its call rollups.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.DiscoveredAgent, []models.DiscoveredCall, error) {
	var agent models.DiscoveredAgent
	if err := s.db.WithContext(ctx).First(&agent, "id = ?", id).Error; err != nil {
		return nil, nil, err
	}
	var calls []models.DiscoveredCall
	if err := s.db.WithContext(ctx).
		Where("discovered_agent_id = ?", id).
		Order("last_seen_at DESC").
		Find(&calls).Error; err != nil {
		return &agent, nil, err
	}
	return &agent, calls, nil
}

// Promote turns a discovered agent into a RegistryAgent and marks the
// row promoted. Returns the new RegistryAgent ID.
func (s *Service) Promote(ctx context.Context, id uuid.UUID, version string) (uuid.UUID, error) {
	if s.agents == nil {
		return uuid.Nil, errors.New("agent resolver not configured")
	}
	var agent models.DiscoveredAgent
	if err := s.db.WithContext(ctx).First(&agent, "id = ?", id).Error; err != nil {
		return uuid.Nil, err
	}
	if version == "" {
		version = "0.1.0"
	}
	regID, err := s.agents.PublishFromDiscovery(ctx, PublishInput{
		Name:        "discovered/" + agent.ServiceName,
		Version:     version,
		Title:       agent.ServiceName,
		Description: "Auto-published from OTel discovery on " + time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return uuid.Nil, err
	}
	now := time.Now()
	updates := map[string]any{
		"status":            models.DiscoveredStatusPromoted,
		"promoted_agent_id": regID,
		"promoted_at":       now,
		"updated_at":        now,
	}
	if err := s.db.WithContext(ctx).Model(&agent).Updates(updates).Error; err != nil {
		return uuid.Nil, err
	}
	return regID, nil
}

// Ignore marks an agent so it stops appearing in the pending list.
func (s *Service) Ignore(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.DiscoveredAgent{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     models.DiscoveredStatusIgnored,
			"ignored_at": now,
			"updated_at": now,
		}).Error
}
