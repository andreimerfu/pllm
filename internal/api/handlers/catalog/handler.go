package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/core/models"
)

// resourceKinds are the catalog resource kinds pllm publishes. LLMs are
// referenced by name in bindings but not yet listed as catalog resources.
var resourceKinds = []string{"agent", "server", "skill", "prompt"}

// ResourceLister lists registry resources of one kind as neutral rows.
type ResourceLister interface {
	ListResources(ctx context.Context, kind string, updatedSince *time.Time) ([]ResourceRow, error)
}

// UseCaseLister lists use cases, optionally filtered to those updated since.
type UseCaseLister interface {
	ListUseCases(ctx context.Context, updatedSince *time.Time) ([]models.UseCase, error)
}

// BindingLister lists use-case → resource edges, optionally scoped to one
// use case.
type BindingLister interface {
	ListBindings(ctx context.Context, useCaseID *uuid.UUID) ([]models.RegistryRef, error)
}

// Handler serves the read-only Catalog API.
type Handler struct {
	logger    *zap.Logger
	resources ResourceLister
	useCases  UseCaseLister
	bindings  BindingLister
}

// NewHandler builds the catalog handler from the three narrow listers.
func NewHandler(logger *zap.Logger, resources ResourceLister, useCases UseCaseLister, bindings BindingLister) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, resources: resources, useCases: useCases, bindings: bindings}
}

// ListResources serves GET .../catalog/resources?kind=&updated_since=
// Omitting kind returns all publishable kinds. Always latest-only.
func (h *Handler) ListResources(w http.ResponseWriter, r *http.Request) {
	since, err := parseUpdatedSince(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid updated_since: "+err.Error())
		return
	}
	kinds := resourceKinds
	if kind := r.URL.Query().Get("kind"); kind != "" {
		if !validResourceKind(kind) {
			writeErr(w, http.StatusBadRequest, "invalid kind: "+kind)
			return
		}
		kinds = []string{kind}
	}

	out := []Resource{}
	for _, k := range kinds {
		rows, err := h.resources.ListResources(r.Context(), k, since)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, row := range rows {
			out = append(out, toResourceDTO(row))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// ListUseCases serves GET .../catalog/use-cases?updated_since=
func (h *Handler) ListUseCases(w http.ResponseWriter, r *http.Request) {
	since, err := parseUpdatedSince(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid updated_since: "+err.Error())
		return
	}
	rows, err := h.useCases.ListUseCases(r.Context(), since)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]UseCase, 0, len(rows))
	for _, uc := range rows {
		out = append(out, toUseCaseDTO(uc))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// ListBindings serves GET .../catalog/bindings?use_case=
func (h *Handler) ListBindings(w http.ResponseWriter, r *http.Request) {
	var useCaseID *uuid.UUID
	if s := r.URL.Query().Get("use_case"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid use_case: "+err.Error())
			return
		}
		useCaseID = &id
	}
	refs, err := h.bindings.ListBindings(r.Context(), useCaseID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]Binding, 0, len(refs))
	for _, ref := range refs {
		out = append(out, toBindingDTO(ref))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// --- helpers ---

func validResourceKind(k string) bool {
	for _, v := range resourceKinds {
		if v == k {
			return true
		}
	}
	return false
}

func parseUpdatedSince(r *http.Request) (*time.Time, error) {
	s := r.URL.Query().Get("updated_since")
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
