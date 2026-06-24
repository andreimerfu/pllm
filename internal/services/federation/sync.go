package federation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// SyncResult is the outcome of pulling one peer. Counts let the runner
// write a human summary like "12 servers, 3 agents (1 orphaned)".
type SyncResult struct {
	PeerID     string `json:"peer_id"`
	Generation int    `json:"generation"`

	Servers SyncKindResult `json:"servers"`
	Agents  SyncKindResult `json:"agents"`
	Skills  SyncKindResult `json:"skills"`
	Prompts SyncKindResult `json:"prompts"`

	Orphaned int    `json:"orphaned"`
	Errors   []string `json:"errors,omitempty"`
}

// SyncKindResult breaks counts down per artifact kind.
type SyncKindResult struct {
	Fetched int `json:"fetched"`
	Saved   int `json:"saved"`
	Skipped int `json:"skipped"` // didn't pass whitelist / tag filter
}

// HumanSummary renders the result as a single line for the admin UI.
// Kept short so operators can scan a list of peers at a glance.
func (r SyncResult) HumanSummary() string {
	parts := []string{
		fmt.Sprintf("%d servers", r.Servers.Saved),
		fmt.Sprintf("%d agents", r.Agents.Saved),
		fmt.Sprintf("%d skills", r.Skills.Saved),
		fmt.Sprintf("%d prompts", r.Prompts.Saved),
	}
	if r.Orphaned > 0 {
		parts = append(parts, fmt.Sprintf("%d orphaned", r.Orphaned))
	}
	return strings.Join(parts, ", ")
}

// SyncOne pulls every kind from the peer and reconciles into the local
// registry tables. Errors are accumulated per-kind; the overall result is
// "partial" if any kind failed. Generation increments on every successful
// run regardless of partial state — orphans use the new generation as the
// "expected current" baseline.
func (s *Service) SyncOne(ctx context.Context, peerID uuid.UUID, hooks WebhookEmitter) (SyncResult, error) {
	peer, err := s.Get(ctx, peerID)
	if err != nil {
		return SyncResult{}, err
	}
	if !peer.Enabled {
		return SyncResult{PeerID: peer.PeerID}, fmt.Errorf("peer %q is disabled", peer.PeerID)
	}
	if err := s.markSyncStart(ctx, peer.ID); err != nil {
		s.logger.Warn("federation: mark start failed", zap.String("peer_id", peer.PeerID), zap.Error(err))
	}

	gen := peer.LastSyncGeneration + 1
	result := SyncResult{PeerID: peer.PeerID, Generation: gen}
	client := NewClient(peer.Endpoint, peer.FederationToken, peer.ExpectedClientID)

	// Health probe is best-effort — failure means the rest is going to
	// fail too, but we let downstream fetches surface their own error so
	// the operator sees which kind broke.
	if err := client.HealthOK(ctx); err != nil {
		s.logger.Warn("federation: peer health probe failed",
			zap.String("peer_id", peer.PeerID), zap.Error(err))
	}

	now := time.Now()

	// Servers
	if servers, err := client.FetchServers(ctx); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("servers: %v", err))
	} else {
		result.Servers.Fetched = len(servers)
		for i := range servers {
			r := servers[i]
			if !peer.MatchesWhitelist(models.RegistryKindServer, r.Name) {
				result.Servers.Skipped++
				continue
			}
			if !peer.MatchesTags(string(r.Metadata)) {
				result.Servers.Skipped++
				continue
			}
			if err := s.upsertFederatedServer(ctx, peer.PeerID, gen, now, r); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("server %s@%s: %v", r.Name, r.Version, err))
				continue
			}
			result.Servers.Saved++
		}
	}

	// Agents
	if agents, err := client.FetchAgents(ctx); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("agents: %v", err))
	} else {
		result.Agents.Fetched = len(agents)
		for i := range agents {
			r := agents[i]
			if !peer.MatchesWhitelist(models.RegistryKindAgent, r.Name) {
				result.Agents.Skipped++
				continue
			}
			if !peer.MatchesTags(string(r.Metadata)) {
				result.Agents.Skipped++
				continue
			}
			if err := s.upsertFederatedAgent(ctx, peer.PeerID, gen, now, r); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("agent %s@%s: %v", r.Name, r.Version, err))
				continue
			}
			result.Agents.Saved++
		}
	}

	// Skills
	if skills, err := client.FetchSkills(ctx); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("skills: %v", err))
	} else {
		result.Skills.Fetched = len(skills)
		for i := range skills {
			r := skills[i]
			if !peer.MatchesWhitelist(models.RegistryKindSkill, r.Name) {
				result.Skills.Skipped++
				continue
			}
			if !peer.MatchesTags(string(r.Metadata)) {
				result.Skills.Skipped++
				continue
			}
			if err := s.upsertFederatedSkill(ctx, peer.PeerID, gen, now, r); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("skill %s@%s: %v", r.Name, r.Version, err))
				continue
			}
			result.Skills.Saved++
		}
	}

	// Prompts
	if prompts, err := client.FetchPrompts(ctx); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("prompts: %v", err))
	} else {
		result.Prompts.Fetched = len(prompts)
		for i := range prompts {
			r := prompts[i]
			if !peer.MatchesWhitelist(models.RegistryKindPrompt, r.Name) {
				result.Prompts.Skipped++
				continue
			}
			if !peer.MatchesTags(string(r.Metadata)) {
				result.Prompts.Skipped++
				continue
			}
			if err := s.upsertFederatedPrompt(ctx, peer.PeerID, gen, now, r); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("prompt %s@%s: %v", r.Name, r.Version, err))
				continue
			}
			result.Prompts.Saved++
		}
	}

	// Generation-based orphan flagging — anything still at a lower
	// generation didn't appear in this run and is therefore stale.
	// Orphans are marked deprecated rather than deleted so an admin can
	// review before unmounting them from dependent agents / use cases.
	orphaned, orphanErr := s.flagOrphans(ctx, peer.PeerID, gen)
	if orphanErr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("orphan flag: %v", orphanErr))
	}
	result.Orphaned = orphaned

	status := models.FederationSyncStatusSuccess
	errStr := ""
	if len(result.Errors) > 0 {
		status = models.FederationSyncStatusPartial
		errStr = strings.Join(result.Errors, "; ")
		if isAllErrors(result) {
			status = models.FederationSyncStatusFailed
		}
	}
	if err := s.markSyncResult(ctx, peer.ID, status, result.HumanSummary(), errStr, gen); err != nil {
		s.logger.Warn("federation: mark result failed", zap.String("peer_id", peer.PeerID), zap.Error(err))
	}

	// Emit webhook
	if hooks != nil {
		evt := models.WebhookEventFederationSyncCompleted
		if status == models.FederationSyncStatusFailed {
			evt = models.WebhookEventFederationSyncFailed
		}
		hooks.Emit(WebhookEventEnvelope{Type: evt, Data: result})
		if result.Orphaned > 0 {
			hooks.Emit(WebhookEventEnvelope{Type: models.WebhookEventFederationItemOrphaned, Data: map[string]any{
				"peer_id":    peer.PeerID,
				"generation": gen,
				"orphaned":   result.Orphaned,
			}})
		}
	}

	if status == models.FederationSyncStatusFailed {
		return result, errors.New(errStr)
	}
	return result, nil
}

// isAllErrors reports whether every kind failed to produce anything.
// Used to escalate "partial" to "failed" when nothing actually synced.
func isAllErrors(r SyncResult) bool {
	if len(r.Errors) == 0 {
		return false
	}
	return r.Servers.Saved == 0 && r.Agents.Saved == 0 && r.Skills.Saved == 0 && r.Prompts.Saved == 0
}

// upsertFederatedServer reconciles one server row. Insert-or-update by
// (name, version) keyed on source_peer_id — a remote and a local with
// the same name can coexist if local was published first, but we never
// overwrite a local row with a federated one.
func (s *Service) upsertFederatedServer(ctx context.Context, peerID string, gen int, now time.Time, in models.RegistryServer) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.RegistryServer
		err := tx.Unscoped().
			Where("name = ? AND version = ?", in.Name, in.Version).First(&existing).Error
		if err == nil {
			if existing.SourcePeerID == "" {
				// Local row with same identity wins — skip.
				return nil
			}
			if existing.SourcePeerID != peerID {
				// Another peer already owns this row — skip to avoid flapping.
				return nil
			}
			existing.Title = in.Title
			existing.Description = in.Description
			existing.Status = in.Status
			existing.WebsiteURL = in.WebsiteURL
			existing.Repository = in.Repository
			existing.Packages = in.Packages
			existing.Remotes = in.Remotes
			existing.Metadata = in.Metadata
			existing.IsLatest = in.IsLatest
			existing.FederatedGeneration = gen
			existing.LastSyncedAt = &now
			existing.DeletedAt = gorm.DeletedAt{} // un-orphan if it was previously soft-deleted
			return tx.Unscoped().Save(&existing).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		in.ID = uuid.New()
		in.SourcePeerID = peerID
		in.FederatedGeneration = gen
		in.LastSyncedAt = &now
		in.PublishedByUserID = nil // local users don't own federated rows
		return tx.Create(&in).Error
	})
}

// upsertFederatedAgent mirrors the server path. Agent refs (the
// composition graph) are intentionally NOT synced — they reference local
// names that may not exist on this side, and importing them blindly
// would create dangling edges. Federated agents become read-only "cards"
// in the catalog; dependents are resolved when an operator re-publishes
// the agent locally.
func (s *Service) upsertFederatedAgent(ctx context.Context, peerID string, gen int, now time.Time, in models.RegistryAgent) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.RegistryAgent
		err := tx.Unscoped().Where("name = ? AND version = ?", in.Name, in.Version).First(&existing).Error
		if err == nil {
			if existing.SourcePeerID == "" || existing.SourcePeerID != peerID {
				return nil
			}
			existing.Title = in.Title
			existing.Description = in.Description
			existing.Status = in.Status
			existing.Language = in.Language
			existing.Framework = in.Framework
			existing.ModelProvider = in.ModelProvider
			existing.ModelName = in.ModelName
			existing.WebsiteURL = in.WebsiteURL
			existing.Repository = in.Repository
			existing.Packages = in.Packages
			existing.Remotes = in.Remotes
			existing.Metadata = in.Metadata
			existing.IsLatest = in.IsLatest
			existing.FederatedGeneration = gen
			existing.LastSyncedAt = &now
			existing.DeletedAt = gorm.DeletedAt{}
			return tx.Unscoped().Save(&existing).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		in.ID = uuid.New()
		in.SourcePeerID = peerID
		in.FederatedGeneration = gen
		in.LastSyncedAt = &now
		in.PublishedByUserID = nil
		return tx.Create(&in).Error
	})
}

// upsertFederatedSkill is structurally identical to server.
func (s *Service) upsertFederatedSkill(ctx context.Context, peerID string, gen int, now time.Time, in models.RegistrySkill) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.RegistrySkill
		err := tx.Unscoped().Where("name = ? AND version = ?", in.Name, in.Version).First(&existing).Error
		if err == nil {
			if existing.SourcePeerID == "" || existing.SourcePeerID != peerID {
				return nil
			}
			existing.Title = in.Title
			existing.Description = in.Description
			existing.Status = in.Status
			existing.Image = in.Image
			existing.Manifest = in.Manifest
			existing.Metadata = in.Metadata
			existing.IsLatest = in.IsLatest
			existing.FederatedGeneration = gen
			existing.LastSyncedAt = &now
			existing.DeletedAt = gorm.DeletedAt{}
			return tx.Unscoped().Save(&existing).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		in.ID = uuid.New()
		in.SourcePeerID = peerID
		in.FederatedGeneration = gen
		in.LastSyncedAt = &now
		in.PublishedByUserID = nil
		return tx.Create(&in).Error
	})
}

// upsertFederatedPrompt mirrors server/skill.
func (s *Service) upsertFederatedPrompt(ctx context.Context, peerID string, gen int, now time.Time, in models.RegistryPrompt) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.RegistryPrompt
		err := tx.Unscoped().Where("name = ? AND version = ?", in.Name, in.Version).First(&existing).Error
		if err == nil {
			if existing.SourcePeerID == "" || existing.SourcePeerID != peerID {
				return nil
			}
			existing.Title = in.Title
			existing.Description = in.Description
			existing.Status = in.Status
			existing.Template = in.Template
			existing.Arguments = in.Arguments
			existing.Metadata = in.Metadata
			existing.IsLatest = in.IsLatest
			existing.FederatedGeneration = gen
			existing.LastSyncedAt = &now
			existing.DeletedAt = gorm.DeletedAt{}
			return tx.Unscoped().Save(&existing).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		in.ID = uuid.New()
		in.SourcePeerID = peerID
		in.FederatedGeneration = gen
		in.LastSyncedAt = &now
		in.PublishedByUserID = nil
		return tx.Create(&in).Error
	})
}

// flagOrphans marks rows from this peer whose generation is less than
// the current run as deprecated. Returns the total count touched across
// all four kinds.
//
// We intentionally don't soft-delete here — an operator may want to keep
// a snapshot of "what was synced before the upstream removed it" for an
// audit. Status=deprecated keeps the row out of default lists while
// preserving the historical reference.
func (s *Service) flagOrphans(ctx context.Context, peerID string, currentGen int) (int, error) {
	total := 0
	tables := []string{
		"registry_servers",
		"registry_agents",
		"registry_skills",
		"registry_prompts",
	}
	for _, t := range tables {
		res := s.db.WithContext(ctx).Table(t).
			Where("source_peer_id = ? AND federated_generation > 0 AND federated_generation < ? AND status <> ?",
				peerID, currentGen, models.RegistryStatusDeprecated).
			Updates(map[string]any{
				"status":     models.RegistryStatusDeprecated,
				"is_latest":  false,
				"updated_at": time.Now(),
			})
		if res.Error != nil {
			return total, res.Error
		}
		total += int(res.RowsAffected)
	}
	return total, nil
}
