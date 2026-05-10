package use_cases

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// ScopeResolver answers "is this target reachable from a key's use case?"
// with a short TTL cache so the per-request cost is O(map lookup).
//
// We cache the resolved (llm set, server-slug set, block-mode flag) keyed
// by use-case-id. On use case update/delete, the service layer calls
// Invalidate; on TTL expiry we refetch.
type ScopeResolver struct {
	db  *gorm.DB
	ttl time.Duration

	mu    sync.RWMutex
	cache map[uuid.UUID]cachedScope
}

type cachedScope struct {
	llms       map[string]struct{}
	serverSlug map[string]struct{}
	agents     map[string]struct{}
	prompts    map[string]struct{}
	enforce    bool
	expiresAt  time.Time
}

// NewScopeResolver builds a resolver with a 30-second cache TTL — enough
// to smooth burst traffic, short enough that a compliance edit lands fast.
func NewScopeResolver(db *gorm.DB) *ScopeResolver {
	return &ScopeResolver{db: db, ttl: 30 * time.Second, cache: map[uuid.UUID]cachedScope{}}
}

// Decision is the structured output of a scope check. The caller decides
// what to do with it (audit, deny, etc.) based on EnforceMode.
type Decision struct {
	Allowed     bool
	EnforceMode bool // true = block out-of-scope; false = flag/audit only
	Reason      string
}

// AllowLLM checks whether the key's use case declares `modelName` as an LLM ref.
// Unscoped keys (no UseCaseID) or missing use case rows always allow.
func (r *ScopeResolver) AllowLLM(ctx context.Context, useCaseID *uuid.UUID, modelName string) Decision {
	if useCaseID == nil {
		return Decision{Allowed: true}
	}
	s, err := r.load(ctx, *useCaseID)
	if err != nil {
		// If the use case has been deleted, err on the safe side: allow
		// but mark unenforced so the LLM path doesn't break under race.
		return Decision{Allowed: true, Reason: "use case lookup failed: " + err.Error()}
	}
	if _, ok := s.llms[modelName]; ok {
		return Decision{Allowed: true, EnforceMode: s.enforce}
	}
	return Decision{
		Allowed:     false,
		EnforceMode: s.enforce,
		Reason:      "model " + modelName + " is not declared by the bound use case",
	}
}

// AllowMCPTool checks whether the key's use case declares the server slug
// that a tool call is targeting. Tool names arrive as "<slug>__<tool>" so
// the caller splits before asking.
func (r *ScopeResolver) AllowMCPTool(ctx context.Context, useCaseID *uuid.UUID, serverSlug string) Decision {
	if useCaseID == nil {
		return Decision{Allowed: true}
	}
	s, err := r.load(ctx, *useCaseID)
	if err != nil {
		return Decision{Allowed: true, Reason: "use case lookup failed: " + err.Error()}
	}
	if _, ok := s.serverSlug[serverSlug]; ok {
		return Decision{Allowed: true, EnforceMode: s.enforce}
	}
	return Decision{
		Allowed:     false,
		EnforceMode: s.enforce,
		Reason:      "mcp server " + serverSlug + " is not declared by the bound use case",
	}
}

// AllowAgent checks whether the key's use case declares `agentName` as an
// agent ref. Accepts the full registry name, the last path segment, or any
// declared local_name alias (mirrors AllowMCPTool's matching).
// Unscoped keys (no UseCaseID) or missing use case rows always allow.
func (r *ScopeResolver) AllowAgent(ctx context.Context, useCaseID *uuid.UUID, agentName string) Decision {
	if useCaseID == nil {
		return Decision{Allowed: true}
	}
	s, err := r.load(ctx, *useCaseID)
	if err != nil {
		return Decision{Allowed: true, Reason: "use case lookup failed: " + err.Error()}
	}
	if _, ok := s.agents[agentName]; ok {
		return Decision{Allowed: true, EnforceMode: s.enforce}
	}
	return Decision{
		Allowed:     false,
		EnforceMode: s.enforce,
		Reason:      "agent " + agentName + " is not declared by the bound use case",
	}
}

// ScopeSnapshot is the read-only view of a use case's scope, returned
// by GetScope. Slices are sorted for stable comparisons in callers
// (e.g. the SDK policy endpoint can hash these for ETag-style caching).
type ScopeSnapshot struct {
	Enforce bool
	LLMs    []string
	Servers []string
	Agents  []string
	Prompts []string
}

// GetScope returns a stable snapshot of the cached scope for a use
// case. Returns nil if useCaseID is nil. Used by the SDK policy
// endpoint so client-side enforcers can mirror the gateway's policy.
func (r *ScopeResolver) GetScope(ctx context.Context, useCaseID *uuid.UUID) (*ScopeSnapshot, error) {
	if useCaseID == nil {
		return nil, nil
	}
	s, err := r.load(ctx, *useCaseID)
	if err != nil {
		return nil, err
	}
	out := &ScopeSnapshot{Enforce: s.enforce}
	for k := range s.llms {
		out.LLMs = append(out.LLMs, k)
	}
	for k := range s.serverSlug {
		out.Servers = append(out.Servers, k)
	}
	for k := range s.agents {
		out.Agents = append(out.Agents, k)
	}
	for k := range s.prompts {
		out.Prompts = append(out.Prompts, k)
	}
	sort.Strings(out.LLMs)
	sort.Strings(out.Servers)
	sort.Strings(out.Agents)
	sort.Strings(out.Prompts)
	return out, nil
}

// AllowPrompt checks whether the key's use case declares `name` as a
// prompt ref. Same matching as servers/agents (full name, last
// segment, or local alias).
func (r *ScopeResolver) AllowPrompt(ctx context.Context, useCaseID *uuid.UUID, name string) Decision {
	if useCaseID == nil {
		return Decision{Allowed: true}
	}
	s, err := r.load(ctx, *useCaseID)
	if err != nil {
		return Decision{Allowed: true, Reason: "use case lookup failed: " + err.Error()}
	}
	if _, ok := s.prompts[name]; ok {
		return Decision{Allowed: true, EnforceMode: s.enforce}
	}
	return Decision{
		Allowed:     false,
		EnforceMode: s.enforce,
		Reason:      "prompt " + name + " is not declared by the bound use case",
	}
}

// Invalidate drops the cached scope for a use case. Called by the service
// layer after any write that could change membership (refs, status, etc.).
func (r *ScopeResolver) Invalidate(id uuid.UUID) {
	r.mu.Lock()
	delete(r.cache, id)
	r.mu.Unlock()
}

// --- internal ---

func (r *ScopeResolver) load(ctx context.Context, id uuid.UUID) (cachedScope, error) {
	r.mu.RLock()
	if s, ok := r.cache[id]; ok && time.Now().Before(s.expiresAt) {
		r.mu.RUnlock()
		return s, nil
	}
	r.mu.RUnlock()

	fresh, err := r.fetch(ctx, id)
	if err != nil {
		return cachedScope{}, err
	}
	r.mu.Lock()
	r.cache[id] = fresh
	r.mu.Unlock()
	return fresh, nil
}

func (r *ScopeResolver) fetch(ctx context.Context, id uuid.UUID) (cachedScope, error) {
	var uc models.UseCase
	if err := r.db.WithContext(ctx).First(&uc, "id = ?", id).Error; err != nil {
		return cachedScope{}, err
	}
	var refs []models.RegistryRef
	if err := r.db.WithContext(ctx).
		Where("owner_kind = ? AND owner_id = ?", models.RegistryKindUseCase, id).
		Find(&refs).Error; err != nil {
		return cachedScope{}, err
	}

	out := cachedScope{
		llms:       map[string]struct{}{},
		serverSlug: map[string]struct{}{},
		agents:     map[string]struct{}{},
		prompts:    map[string]struct{}{},
		enforce:    uc.EnforceScope,
		expiresAt:  time.Now().Add(r.ttl),
	}
	for _, ref := range refs {
		switch ref.TargetKind {
		case models.RegistryKindLLM:
			out.llms[ref.TargetName] = struct{}{}
		case models.RegistryKindServer:
			// Registry server names are like "io.mcp/server-everything".
			// Gateway backend slugs are usually the last segment; but we
			// also accept exact matches against the full registry name so
			// operators can bind either way.
			out.serverSlug[ref.TargetName] = struct{}{}
			if i := strings.LastIndex(ref.TargetName, "/"); i >= 0 {
				out.serverSlug[ref.TargetName[i+1:]] = struct{}{}
			}
			// If the ref has a local_name alias, also accept that as the
			// slug — matches how auto-registered gateway backends are
			// named "reg-<workload>".
			if ref.LocalName != "" {
				out.serverSlug[ref.LocalName] = struct{}{}
			}
		case models.RegistryKindAgent:
			// Same matching rules as servers: accept full registry name,
			// last path segment, and any declared local_name alias.
			out.agents[ref.TargetName] = struct{}{}
			if i := strings.LastIndex(ref.TargetName, "/"); i >= 0 {
				out.agents[ref.TargetName[i+1:]] = struct{}{}
			}
			if ref.LocalName != "" {
				out.agents[ref.LocalName] = struct{}{}
			}
		case models.RegistryKindPrompt:
			// Prompts can be referenced by full name, last segment, or alias.
			out.prompts[ref.TargetName] = struct{}{}
			if i := strings.LastIndex(ref.TargetName, "/"); i >= 0 {
				out.prompts[ref.TargetName[i+1:]] = struct{}{}
			}
			if ref.LocalName != "" {
				out.prompts[ref.LocalName] = struct{}{}
			}
		}
	}
	return out, nil
}
