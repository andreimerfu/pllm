// Package prompts is the gateway-side prompt fetch endpoint. Agents
// (via pllm_sdk.get_prompt) call GET /v1/prompts/{name}/versions/{v}
// to retrieve a registered prompt template. Auth uses the bearer key,
// and the bound use case's scope determines whether the prompt is
// reachable. Out-of-scope fetches return 403 and write a ScopeEvent
// (same shape as the LLM/MCP gateway denials).
package prompts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	registryservice "github.com/amerfu/pllm/internal/services/registry/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ScopeChecker is the narrow interface this handler needs from the
// use_cases service. The router bridges it to *use_cases.ScopeResolver.
type ScopeChecker interface {
	AllowPrompt(ctx context.Context, useCaseID *uuid.UUID, name string) ScopeDecision
}

type ScopeDecision struct {
	Allowed     bool
	EnforceMode bool
	Reason      string
}

// ScopeEventRecorder writes one prompt-denial event for the audit log.
type ScopeEventRecorder interface {
	RecordPromptDenial(
		ctx context.Context,
		useCaseID uuid.UUID,
		keyID *uuid.UUID, keyName, target, reason string, enforced bool,
		remoteAddr, requestID string,
	)
}

type Handler struct {
	logger *zap.Logger
	svc    *registryservice.PromptService
	scope  ScopeChecker       // optional
	rec    ScopeEventRecorder // optional
}

func New(logger *zap.Logger, svc *registryservice.PromptService) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, svc: svc}
}

// WithScope attaches the scope checker; without it, all prompt fetches
// are allowed (back-compat with installs that haven't enabled MRM).
func (h *Handler) WithScope(s ScopeChecker) *Handler { h.scope = s; return h }

// WithRecorder attaches the audit-log recorder.
func (h *Handler) WithRecorder(r ScopeEventRecorder) *Handler { h.rec = r; return h }

// Get is GET /v1/prompts/{name}/versions/{version}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	rawName := chi.URLParam(r, "name")
	name, err := url.PathUnescape(rawName)
	if err != nil {
		name = rawName
	}
	version := chi.URLParam(r, "version")

	key, _ := middleware.GetKey(r.Context())

	// Scope check first — same as LLM/MCP gateways. Master-key callers
	// (no Key on context) bypass scope; the auth layer has already
	// vetted them.
	if h.scope != nil && key != nil && key.UseCaseID != nil {
		dec := h.scope.AllowPrompt(r.Context(), key.UseCaseID, name)
		if !dec.Allowed && dec.EnforceMode {
			if h.rec != nil {
				keyID := key.ID
				h.rec.RecordPromptDenial(
					r.Context(), *key.UseCaseID,
					&keyID, key.Name, name, dec.Reason, true,
					r.RemoteAddr, "",
				)
			}
			http.Error(w, "forbidden: "+dec.Reason, http.StatusForbidden)
			return
		}
		if !dec.Allowed && h.rec != nil {
			// Flag mode — call proceeds but is recorded.
			keyID := key.ID
			h.rec.RecordPromptDenial(
				r.Context(), *key.UseCaseID,
				&keyID, key.Name, name, dec.Reason, false,
				r.RemoteAddr, "",
			)
		}
	}

	row, err := h.svc.Get(r.Context(), name, version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=0, no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"name":      row.Name,
		"version":   row.Version,
		"title":     row.Title,
		"template":  row.Template,
		"arguments": row.Arguments,
	})
}

// Suppress unused imports when scope/recorder paths aren't wired.
var _ = models.RegistryPrompt{}
