// Package use_cases exposes REST endpoints for the UseCase entity.
package use_cases

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/use_cases"
)

// Handler wires the use-case service to HTTP.
type Handler struct {
	logger  *zap.Logger
	service *use_cases.Service
}

// NewHandler constructs a Handler. nil service disables the endpoints
// (handlers respond 503) so the router can always mount them.
func NewHandler(logger *zap.Logger, service *use_cases.Service) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, service: service}
}

// apiBody is what create/update endpoints accept. Kept as a flat JSON
// body — refs are an inline array so the whole use case moves in one
// request/response.
type apiBody struct {
	Name                string                 `json:"name"`
	Slug                string                 `json:"slug,omitempty"`
	Description         string                 `json:"description,omitempty"`
	RiskTier            models.UseCaseRiskTier `json:"risk_tier,omitempty"`
	Status              models.UseCaseStatus   `json:"status,omitempty"`
	BusinessOwnerUserID *uuid.UUID             `json:"business_owner_user_id,omitempty"`
	ApproverUserID      *uuid.UUID             `json:"approver_user_id,omitempty"`
	TeamID              *uuid.UUID             `json:"team_id,omitempty"`
	DataClassifications []string               `json:"data_classifications,omitempty"`
	ApprovalRecord      json.RawMessage        `json:"approval_record,omitempty"`
	Refs                []refDTO               `json:"refs,omitempty"`
	EnforceScope        *bool                  `json:"enforce_scope,omitempty"`
}

type refDTO struct {
	TargetKind    models.RegistryKind `json:"target_kind"`
	TargetName    string              `json:"target_name"`
	TargetVersion string              `json:"target_version,omitempty"`
	LocalName     string              `json:"local_name,omitempty"`
}

func (b *apiBody) toInput() *use_cases.UpsertInput {
	in := &use_cases.UpsertInput{
		Name:                b.Name,
		Slug:                b.Slug,
		Description:         b.Description,
		RiskTier:            b.RiskTier,
		Status:              b.Status,
		BusinessOwnerUserID: b.BusinessOwnerUserID,
		ApproverUserID:      b.ApproverUserID,
		TeamID:              b.TeamID,
		DataClassifications: b.DataClassifications,
		EnforceScope:        b.EnforceScope,
	}
	if len(b.ApprovalRecord) > 0 {
		in.ApprovalRecord = []byte(b.ApprovalRecord)
	}
	if len(b.Refs) > 0 {
		refs := make([]models.RegistryRef, 0, len(b.Refs))
		for _, r := range b.Refs {
			refs = append(refs, models.RegistryRef{
				TargetKind:    r.TargetKind,
				TargetName:    r.TargetName,
				TargetVersion: r.TargetVersion,
				LocalName:     r.LocalName,
			})
		}
		in.Refs = refs
	}
	return in
}

// --- endpoints ------------------------------------------------------------
//
// Scope-event endpoints (ListScopeEvents, BulkSeedScopeEvents) live in
// scope_events.go.

// List is GET /api/admin/use-cases.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeErr(w, http.StatusServiceUnavailable, "use cases not configured")
		return
	}
	q := r.URL.Query()
	f := use_cases.ListFilter{
		Search:                  q.Get("search"),
		Status:                  models.UseCaseStatus(q.Get("status")),
		RiskTier:                models.UseCaseRiskTier(q.Get("risk_tier")),
		Classification:          q.Get("classification"),
		EffectiveClassification: q.Get("effective_classification"),
	}
	if v := q.Get("compliance_flag"); v != "" {
		b := v == "true" || v == "1"
		f.ComplianceFlag = &b
	}
	if v := q.Get("has_third_party"); v != "" {
		b := v == "true" || v == "1"
		f.HasThirdParty = &b
	}
	if n, err := strconv.Atoi(q.Get("limit")); err == nil {
		f.Limit = n
	}
	if n, err := strconv.Atoi(q.Get("offset")); err == nil && n >= 0 {
		f.Offset = n
	}
	rows, err := h.service.List(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"use_cases": rows})
}

// Get is GET /api/admin/use-cases/{id} — accepts a UUID or a slug.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeErr(w, http.StatusServiceUnavailable, "use cases not configured")
		return
	}
	idOrSlug := chi.URLParam(r, "id")
	var out any
	var err error
	if id, perr := uuid.Parse(idOrSlug); perr == nil {
		out, err = h.service.Get(r.Context(), id)
	} else {
		out, err = h.service.GetBySlug(r.Context(), idOrSlug)
	}
	if err != nil {
		if errors.Is(err, use_cases.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "use case not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// Create is POST /api/admin/use-cases.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeErr(w, http.StatusServiceUnavailable, "use cases not configured")
		return
	}
	var body apiBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.Create(r.Context(), body.toInput())
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// Update is PUT /api/admin/use-cases/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeErr(w, http.StatusServiceUnavailable, "use cases not configured")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body apiBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.Update(r.Context(), id, body.toInput())
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// RecomputeClassifications is POST /api/admin/use-cases/{id}/recompute.
// Synchronous re-walk of the dependency graph. Useful for "flush and
// refresh" after the operator has tagged a bunch of components.
func (h *Handler) RecomputeClassifications(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeErr(w, http.StatusServiceUnavailable, "use cases not configured")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.service.TriggerRecompute(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Read back so the caller sees fresh effective_classifications.
	out, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "use case not found")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// Delete is DELETE /api/admin/use-cases/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeErr(w, http.StatusServiceUnavailable, "use cases not configured")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		mapErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers --------------------------------------------------------------

func mapErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, use_cases.ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, use_cases.ErrConflict):
		writeErr(w, http.StatusConflict, err.Error())
	default:
		writeErr(w, http.StatusBadRequest, err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
