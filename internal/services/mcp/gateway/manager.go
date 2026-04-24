package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/mcp/protocol"
)

// Separator between backend slug and the backend-local tool name in the
// aggregated surface. Chosen to be URL-safe and unlikely in real names.
const NameSeparator = "__"

// Manager owns the registry of live Backends and aggregates them for the
// gateway endpoint. It is safe for concurrent use.
type Manager struct {
	logger *zap.Logger
	db     *gorm.DB

	mu        sync.RWMutex
	backends  map[uuid.UUID]*Backend
	bySlug    map[string]*Backend

	healthInterval time.Duration
	cancelHealth   context.CancelFunc
}

// NewManager creates a Manager. Call Load to hydrate from DB and Start to
// kick off the health loop.
func NewManager(logger *zap.Logger, db *gorm.DB) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		logger:         logger.With(zap.String("component", "mcp-manager")),
		db:             db,
		backends:       make(map[uuid.UUID]*Backend),
		bySlug:         make(map[string]*Backend),
		healthInterval: 30 * time.Second,
	}
}

// Load reads enabled MCP servers from the DB and starts them.
// Servers that fail to start are kept in the map (marked unhealthy)
// so admins see them in the UI.
func (m *Manager) Load(ctx context.Context) error {
	if m.db == nil {
		return nil
	}
	var rows []models.MCPServer
	if err := m.db.WithContext(ctx).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return fmt.Errorf("load mcp servers: %w", err)
	}
	for i := range rows {
		row := rows[i]
		info, err := RowToInfo(&row)
		if err != nil {
			m.logger.Warn("skipping mcp backend: bad row", zap.String("slug", row.Slug), zap.Error(err))
			continue
		}
		b := NewBackend(info, m.logger)
		m.mu.Lock()
		m.backends[info.ID] = b
		m.bySlug[info.Slug] = b
		m.mu.Unlock()

		go m.startOne(ctx, b)
	}
	return nil
}

// Start spawns a background loop that periodically re-checks backend health
// and refreshes tool catalogs.
func (m *Manager) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	m.cancelHealth = cancel
	go m.healthLoop(ctx)
}

// Stop halts the health loop and tears down all backends.
func (m *Manager) Stop() {
	if m.cancelHealth != nil {
		m.cancelHealth()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.backends {
		b.Stop()
	}
	m.backends = map[uuid.UUID]*Backend{}
	m.bySlug = map[string]*Backend{}
}

// AddBackend registers a backend and kicks off startup in the background.
// The caller's ctx is only used for synchronous bookkeeping; the actual
// connect / initialize runs on a context derived from the manager's
// lifetime so it survives the triggering HTTP request returning.
func (m *Manager) AddBackend(ctx context.Context, info BackendInfo) (*Backend, error) {
	if info.Slug == "" {
		return nil, fmt.Errorf("slug is required")
	}
	_ = ctx // preserved in signature for possible future synchronous validation
	b := NewBackend(info, m.logger)
	m.mu.Lock()
	if existing, ok := m.bySlug[info.Slug]; ok {
		m.mu.Unlock()
		existing.Stop()
		m.mu.Lock()
		delete(m.backends, existing.info.ID)
	}
	m.backends[info.ID] = b
	m.bySlug[info.Slug] = b
	m.mu.Unlock()
	// Detach from the caller's ctx: startOne applies its own timeout, and
	// we don't want the handler's request cancellation to kill a backend
	// that the user explicitly asked to register.
	go m.startOne(context.Background(), b)
	return b, nil
}

// RemoveBackend stops and removes a backend by ID.
func (m *Manager) RemoveBackend(id uuid.UUID) {
	m.mu.Lock()
	b, ok := m.backends[id]
	if ok {
		delete(m.backends, id)
		delete(m.bySlug, b.info.Slug)
	}
	m.mu.Unlock()
	if b != nil {
		b.Stop()
	}
}

// GetBackend returns a backend by slug.
func (m *Manager) GetBackend(slug string) (*Backend, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.bySlug[slug]
	return b, ok
}

// ListBackends returns a snapshot ordered by slug.
func (m *Manager) ListBackends() []*Backend {
	m.mu.RLock()
	out := make([]*Backend, 0, len(m.backends))
	for _, b := range m.backends {
		out = append(out, b)
	}
	m.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].info.Slug < out[j].info.Slug })
	return out
}

// AggregateTools returns the merged tool list with names prefixed by
// "<slug>__" so callers can disambiguate across backends.
func (m *Manager) AggregateTools() []protocol.Tool {
	var out []protocol.Tool
	for _, b := range m.ListBackends() {
		if !b.IsHealthy() {
			continue
		}
		slug := b.info.Slug
		for _, t := range b.Tools() {
			out = append(out, protocol.Tool{
				Name:        slug + NameSeparator + t.Name,
				Description: fmt.Sprintf("[%s] %s", slug, t.Description),
				InputSchema: t.InputSchema,
			})
		}
	}
	return out
}

// AggregatePrompts returns merged prompts with "<slug>__" prefix.
func (m *Manager) AggregatePrompts() []protocol.Prompt {
	var out []protocol.Prompt
	for _, b := range m.ListBackends() {
		if !b.IsHealthy() {
			continue
		}
		slug := b.info.Slug
		for _, p := range b.Prompts() {
			out = append(out, protocol.Prompt{
				Name:        slug + NameSeparator + p.Name,
				Description: fmt.Sprintf("[%s] %s", slug, p.Description),
				Arguments:   p.Arguments,
			})
		}
	}
	return out
}

// AggregateResources returns merged resources (URIs passed through verbatim).
func (m *Manager) AggregateResources() []protocol.Resource {
	var out []protocol.Resource
	for _, b := range m.ListBackends() {
		if !b.IsHealthy() {
			continue
		}
		out = append(out, b.Resources()...)
	}
	return out
}

// ResolveTool splits "<slug>__<tool>" and returns the backend + local name.
func (m *Manager) ResolveTool(qualified string) (*Backend, string, error) {
	slug, name, ok := strings.Cut(qualified, NameSeparator)
	if !ok {
		return nil, "", fmt.Errorf("tool name missing %q prefix", NameSeparator)
	}
	b, ok := m.GetBackend(slug)
	if !ok {
		return nil, "", fmt.Errorf("unknown backend %q", slug)
	}
	return b, name, nil
}

// ResolvePrompt works like ResolveTool but for prompts.
func (m *Manager) ResolvePrompt(qualified string) (*Backend, string, error) {
	return m.ResolveTool(qualified)
}

// --- internal ---

func (m *Manager) startOne(ctx context.Context, b *Backend) {
	startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := b.Start(startCtx); err != nil {
		m.logger.Warn("mcp backend start failed",
			zap.String("slug", b.info.Slug),
			zap.Error(err))
		m.persistHealth(b)
		return
	}
	m.logger.Info("mcp backend connected",
		zap.String("slug", b.info.Slug),
		zap.Int("tools", len(b.Tools())))
	m.persistHealth(b)
	m.persistTools(b)
}

func (m *Manager) healthLoop(ctx context.Context) {
	t := time.NewTicker(m.healthInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, b := range m.ListBackends() {
				b := b
				go func() {
					hctx, cancel := context.WithTimeout(ctx, 10*time.Second)
					defer cancel()
					if err := b.HealthCheck(hctx); err != nil {
						m.logger.Debug("mcp backend unhealthy",
							zap.String("slug", b.info.Slug), zap.Error(err))
					} else {
						_ = b.RefreshCatalog(hctx)
						m.persistTools(b)
					}
					m.persistHealth(b)
				}()
			}
		}
	}
}

func (m *Manager) persistHealth(b *Backend) {
	if m.db == nil {
		return
	}
	status := models.MCPHealthUnhealthy
	if b.IsHealthy() {
		status = models.MCPHealthHealthy
	}
	last := b.LastSeen()
	updates := map[string]any{
		"health_status": status,
		"last_error":    b.LastError(),
	}
	if !last.IsZero() {
		updates["last_seen_at"] = last
	}
	if err := m.db.Model(&models.MCPServer{}).
		Where("id = ?", b.info.ID).Updates(updates).Error; err != nil {
		m.logger.Debug("persist health failed", zap.Error(err))
	}
}

func (m *Manager) persistTools(b *Backend) {
	if m.db == nil {
		return
	}
	tools := b.Tools()
	// Replace all cached tools for this backend.
	if err := m.db.Where("mcp_server_id = ?", b.info.ID).Delete(&models.MCPServerTool{}).Error; err != nil {
		m.logger.Debug("delete cached tools failed", zap.Error(err))
		return
	}
	rows := make([]models.MCPServerTool, 0, len(tools))
	for _, t := range tools {
		rows = append(rows, models.MCPServerTool{
			MCPServerID: b.info.ID,
			Name:        t.Name,
			Description: t.Description,
			InputSchema: datatypes.JSON(t.InputSchema),
		})
	}
	if len(rows) > 0 {
		if err := m.db.Create(&rows).Error; err != nil {
			m.logger.Debug("insert cached tools failed", zap.Error(err))
		}
	}
}

// RowToInfo translates a DB row into a runtime BackendInfo.
func RowToInfo(row *models.MCPServer) (BackendInfo, error) {
	info := BackendInfo{
		ID:          row.ID,
		Slug:        row.Slug,
		Name:        row.Name,
		Description: row.Description,
		Transport:   row.Transport,
		Endpoint:    row.Endpoint,
		Command:     row.Command,
		Args:        []string(row.Args),
		WorkingDir:  row.WorkingDir,
	}
	if len(row.Headers) > 0 {
		var hdrs map[string]string
		if err := json.Unmarshal(row.Headers, &hdrs); err != nil {
			return info, fmt.Errorf("headers: %w", err)
		}
		info.Headers = hdrs
	}
	if len(row.Env) > 0 {
		var envMap map[string]string
		if err := json.Unmarshal(row.Env, &envMap); err != nil {
			return info, fmt.Errorf("env: %w", err)
		}
		env := make([]string, 0, len(envMap))
		for k, v := range envMap {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		info.Env = env
	}
	return info, nil
}
