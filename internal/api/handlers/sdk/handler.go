// Package sdk serves the policy snapshot consumed by client-side SDKs
// (e.g. examples/agents/fraud-investigator/pllm_sdk.py). The agent
// fetches the snapshot once on init, refreshes periodically, and uses
// it to block disallowed calls locally before they hit the network.
package sdk

import (
	"encoding/json"
	"net/http"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	"github.com/amerfu/pllm/internal/services/use_cases"
	"go.uber.org/zap"
)

type Handler struct {
	logger *zap.Logger
	scope  *use_cases.ScopeResolver
}

func New(logger *zap.Logger, scope *use_cases.ScopeResolver) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, scope: scope}
}

// PolicyResponse is the wire shape returned to the SDK. Slices are
// sorted by the resolver so a hash of this body can be used for
// stable ETag-style caching by the client.
type PolicyResponse struct {
	KeyID          string   `json:"key_id"`
	UseCaseID      string   `json:"use_case_id,omitempty"`
	Enforce        bool     `json:"enforce"`
	AllowedLLMs    []string `json:"allowed_llms"`
	AllowedMCP     []string `json:"allowed_mcp_servers"`
	AllowedAgents  []string `json:"allowed_agents"`
	AllowedPrompts []string `json:"allowed_prompts"`
	// AllowedExternal lists `host:port` pairs the SDK should let
	// through unconditionally. Currently empty by default — operators
	// will extend the use case model later. The SDK still always
	// allows the pllm gateway base URL it was initialized with.
	AllowedExternal []string `json:"allowed_external"`
	// TTLSeconds is how long the SDK should treat this snapshot as
	// fresh. Mirrors the resolver's internal cache TTL (30s).
	TTLSeconds int `json:"ttl_seconds"`
}

// Policy is GET /v1/sdk/policy. Reads the bearer key from the auth
// middleware and returns the scope resolved for its bound use case.
// Unscoped keys (no use_case_id) get a permissive response — the SDK
// fail-closes only when it can't reach this endpoint at all, not when
// the response says "no scope."
func (h *Handler) Policy(w http.ResponseWriter, r *http.Request) {
	key, ok := middleware.GetKey(r.Context())
	if !ok || key == nil {
		http.Error(w, "no api key on request", http.StatusUnauthorized)
		return
	}

	resp := PolicyResponse{
		KeyID:           key.ID.String(),
		Enforce:         false,
		AllowedLLMs:     []string{},
		AllowedMCP:      []string{},
		AllowedAgents:   []string{},
		AllowedPrompts:  []string{},
		AllowedExternal: []string{},
		TTLSeconds:      30,
	}

	if h.scope != nil && key.UseCaseID != nil {
		snap, err := h.scope.GetScope(r.Context(), key.UseCaseID)
		if err != nil {
			h.logger.Warn("sdk policy: scope load failed", zap.Error(err))
		} else if snap != nil {
			resp.UseCaseID = key.UseCaseID.String()
			resp.Enforce = snap.Enforce
			if snap.LLMs != nil {
				resp.AllowedLLMs = snap.LLMs
			}
			if snap.Servers != nil {
				resp.AllowedMCP = snap.Servers
			}
			if snap.Agents != nil {
				resp.AllowedAgents = snap.Agents
			}
			if snap.Prompts != nil {
				resp.AllowedPrompts = snap.Prompts
			}
		}
	}

	// Mark which we are: useful for the SDK to log which key it's
	// running under without extra round-trips.
	if key.Type == models.KeyTypeAPI {
		// No-op — kept here so the field can be added later without
		// re-shaping the JSON.
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=0, no-store")
	_ = json.NewEncoder(w).Encode(resp)
}
