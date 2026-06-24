// Package federation implements peer-to-peer registry synchronization.
//
// Each pllm instance exposes its own catalog over a small read-only HTTP
// surface (handlers/federation export endpoints) and, optionally, pulls
// from one or more peers. Pulls are scheduled by Runner and reconciled
// into the local registry tables with provenance metadata
// (SourcePeerID, FederatedGeneration, LastSyncedAt) so operators can see
// where an artifact came from.
//
// The federation layer is intentionally a thin shell around the existing
// registry services — synced rows live in the same tables, gated against
// local mutation by the registry service's IsFederated check. This keeps
// downstream consumers (Use Cases, MCP gateway, deployment) unaware of
// whether an entry is local or federated.
package federation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// ErrNotFound is returned when a peer lookup misses.
var ErrNotFound = errors.New("federation: peer not found")

// WebhookEventEnvelope is what the federation runner hands to the emitter
// adapter wired in router.go. Mirrors the lifecycle / use_cases pattern.
type WebhookEventEnvelope struct {
	Type models.WebhookEventType
	Data any
}

// WebhookEmitter is the small surface the runner needs to publish events.
type WebhookEmitter interface {
	Emit(env WebhookEventEnvelope)
}

// Service is the CRUD layer over FederationPeer. The pull side lives on
// Runner; this type stays sync-free so the admin handler can call it
// from request goroutines.
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService builds a Service.
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{db: db, logger: logger.With(zap.String("component", "federation"))}
}

// CreateInput captures fields an operator can set on POST /peers.
type CreateInput struct {
	PeerID              string
	Name                string
	Endpoint            string
	FederationToken     string
	ExpectedClientID    string
	SyncMode            models.FederationSyncMode
	WhitelistServers    []string
	WhitelistAgents     []string
	WhitelistSkills     []string
	WhitelistPrompts    []string
	TagFilters          []string
	SyncIntervalMinutes int
	Enabled             bool
	CreatedByUserID     *uuid.UUID
}

// Create persists a new FederationPeer. Validates required fields and
// rejects duplicate PeerIDs up-front so the unique-index error doesn't
// leak through.
func (s *Service) Create(ctx context.Context, in CreateInput) (*models.FederationPeer, error) {
	if in.PeerID == "" {
		return nil, fmt.Errorf("peer_id is required")
	}
	if in.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if in.Name == "" {
		in.Name = in.PeerID
	}
	if in.SyncMode == "" {
		in.SyncMode = models.FederationSyncModeAll
	}
	if in.SyncIntervalMinutes <= 0 {
		in.SyncIntervalMinutes = 60
	}
	row := models.FederationPeer{
		PeerID:              in.PeerID,
		Name:                in.Name,
		Endpoint:            in.Endpoint,
		FederationToken:     in.FederationToken,
		ExpectedClientID:    in.ExpectedClientID,
		SyncMode:            in.SyncMode,
		WhitelistServers:    in.WhitelistServers,
		WhitelistAgents:     in.WhitelistAgents,
		WhitelistSkills:     in.WhitelistSkills,
		WhitelistPrompts:    in.WhitelistPrompts,
		TagFilters:          in.TagFilters,
		SyncIntervalMinutes: in.SyncIntervalMinutes,
		Enabled:             in.Enabled,
		CreatedByUserID:     in.CreatedByUserID,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// Get returns a peer by its UUID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.FederationPeer, error) {
	var row models.FederationPeer
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// GetByPeerID looks up by the operator-set slug. Useful for sync paths
// that want to start from a peer name rather than a UUID.
func (s *Service) GetByPeerID(ctx context.Context, peerID string) (*models.FederationPeer, error) {
	var row models.FederationPeer
	if err := s.db.WithContext(ctx).First(&row, "peer_id = ?", peerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// List returns every peer ordered by name for stable UI rendering.
func (s *Service) List(ctx context.Context) ([]models.FederationPeer, error) {
	var rows []models.FederationPeer
	if err := s.db.WithContext(ctx).Order("name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// UpdateInput is the PATCH-shaped struct: pointer fields denote
// "explicitly set, possibly to empty" vs "leave unchanged".
type UpdateInput struct {
	Name                *string
	Endpoint            *string
	FederationToken     *string
	ExpectedClientID    *string
	SyncMode            *models.FederationSyncMode
	WhitelistServers    *[]string
	WhitelistAgents     *[]string
	WhitelistSkills     *[]string
	WhitelistPrompts    *[]string
	TagFilters          *[]string
	SyncIntervalMinutes *int
	Enabled             *bool
}

// Update applies a sparse update to a peer.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*models.FederationPeer, error) {
	row, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		row.Name = *in.Name
	}
	if in.Endpoint != nil {
		row.Endpoint = *in.Endpoint
	}
	if in.FederationToken != nil {
		row.FederationToken = *in.FederationToken
	}
	if in.ExpectedClientID != nil {
		row.ExpectedClientID = *in.ExpectedClientID
	}
	if in.SyncMode != nil {
		row.SyncMode = *in.SyncMode
	}
	if in.WhitelistServers != nil {
		row.WhitelistServers = *in.WhitelistServers
	}
	if in.WhitelistAgents != nil {
		row.WhitelistAgents = *in.WhitelistAgents
	}
	if in.WhitelistSkills != nil {
		row.WhitelistSkills = *in.WhitelistSkills
	}
	if in.WhitelistPrompts != nil {
		row.WhitelistPrompts = *in.WhitelistPrompts
	}
	if in.TagFilters != nil {
		row.TagFilters = *in.TagFilters
	}
	if in.SyncIntervalMinutes != nil {
		row.SyncIntervalMinutes = *in.SyncIntervalMinutes
	}
	if in.Enabled != nil {
		row.Enabled = *in.Enabled
	}
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// Delete removes a peer. Synced rows it produced are left in place so
// the operator can review and drop them via the orphan flow.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	res := s.db.WithContext(ctx).Delete(&models.FederationPeer{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// markSyncStart resets the live status fields at the beginning of a run.
// The runner calls this before kicking off network IO so partial state is
// visible in the admin UI.
func (s *Service) markSyncStart(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.FederationPeer{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_sync_at":    now,
			"last_sync_error": "",
		}).Error
}

// markSyncResult writes terminal state for a run.
func (s *Service) markSyncResult(ctx context.Context, id uuid.UUID, status models.FederationSyncStatus, summary, errStr string, generation int) error {
	updates := map[string]any{
		"last_sync_status":  status,
		"last_sync_summary": summary,
		"last_sync_error":   errStr,
	}
	if generation > 0 {
		updates["last_sync_generation"] = generation
	}
	return s.db.WithContext(ctx).Model(&models.FederationPeer{}).
		Where("id = ?", id).Updates(updates).Error
}

// peerEnabledRunInterval picks the effective interval for a peer.
// Used by Runner to space out ticks per-peer instead of one global cadence.
func peerEnabledRunInterval(p *models.FederationPeer, fallback time.Duration) time.Duration {
	if p.SyncIntervalMinutes > 0 {
		return time.Duration(p.SyncIntervalMinutes) * time.Minute
	}
	return fallback
}

// --- helpers exposed for the runner ---------------------------------------

// peerCache is a thin cache that lets the runner re-read peer state
// without hammering the DB. Refreshed at the top of each tick.
type peerCache struct {
	mu    sync.RWMutex
	peers map[uuid.UUID]models.FederationPeer
}

func newPeerCache() *peerCache {
	return &peerCache{peers: map[uuid.UUID]models.FederationPeer{}}
}

func (c *peerCache) replace(rows []models.FederationPeer) {
	m := make(map[uuid.UUID]models.FederationPeer, len(rows))
	for _, r := range rows {
		m[r.ID] = r
	}
	c.mu.Lock()
	c.peers = m
	c.mu.Unlock()
}

func (c *peerCache) snapshot() []models.FederationPeer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]models.FederationPeer, 0, len(c.peers))
	for _, p := range c.peers {
		out = append(out, p)
	}
	return out
}
