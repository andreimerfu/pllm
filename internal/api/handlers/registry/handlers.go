// Package registry exposes REST endpoints that wrap the registry services.
// All reads are public; writes require authentication + admin (wired in router).
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	"github.com/amerfu/pllm/internal/services/registry/enrichment"
	"github.com/amerfu/pllm/internal/services/registry/service"
)

// urlParam fetches a path parameter and percent-decodes it.
// Chi leaves %2F in path params untouched because encoded slashes would
// otherwise re-split the route. Registry names like
// "io.github.org/project" arrive double-encoded at the handler; this
// helper normalizes them before the service sees them.
func urlParam(r *http.Request, key string) string {
	raw := chi.URLParam(r, key)
	if dec, err := url.PathUnescape(raw); err == nil {
		return dec
	}
	return raw
}

// Handler aggregates the four per-kind services so one object can back
// the whole /v1/registry/* subtree.
type Handler struct {
	logger     *zap.Logger
	Servers    *service.ServerService
	Agents     *service.AgentService
	Skills     *service.SkillService
	Prompts    *service.PromptService
	Enrichment *enrichment.Runner // optional; nil disables auto-enqueue
}

// NewHandler builds a Handler. Pass nil logger to get a no-op.
func NewHandler(logger *zap.Logger, s *service.ServerService, a *service.AgentService,
	sk *service.SkillService, p *service.PromptService) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, Servers: s, Agents: a, Skills: sk, Prompts: p}
}

// enqueueServerEnrichment is called after a successful server upsert.
// Best-effort: any error is logged and swallowed so publish still succeeds.
func (h *Handler) enqueueServerEnrichment(ctx context.Context, id uuid.UUID) {
	if h.Enrichment == nil {
		return
	}
	for _, t := range []models.EnrichmentType{models.EnrichmentTypeOSV} {
		if err := h.Enrichment.Enqueue(ctx, models.RegistryKindServer, id, t); err != nil {
			h.logger.Warn("enqueue enrichment failed",
				zap.String("type", string(t)),
				zap.Error(err))
		}
	}
}

// --- shared ---------------------------------------------------------------

// authzWrite enforces resource-pattern permission on the authenticated key.
// Bypass rules:
//   - Master key (raw bearer) always allowed.
//   - JWT sessions (UI / admin login) are trusted because the upstream
//     RequireAdmin middleware already gated the route. Registry writes
//     are only mounted under /api/admin, so reaching this point on a JWT
//     means the caller is an admin.
// API keys must carry a matching "action:pattern" entry in RegistryPermissions.
func authzWrite(w http.ResponseWriter, r *http.Request, kind string, name string, action models.RegistryAction) bool {
	switch middleware.GetAuthType(r.Context()) {
	case middleware.AuthTypeMasterKey, middleware.AuthTypeJWT:
		return true
	}
	key, ok := middleware.GetKey(r.Context())
	if !ok || key == nil {
		writeErr(w, http.StatusForbidden, "registry writes require an API key with publish/edit/delete permissions")
		return false
	}
	if !key.HasRegistryPermission(kind, name, action) {
		writeErr(w, http.StatusForbidden,
			"key does not permit "+string(action)+" on "+kind+"/"+name)
		return false
	}
	return true
}

func parseFilter(r *http.Request) service.ListFilter {
	q := r.URL.Query()
	f := service.ListFilter{
		Search: q.Get("search"),
	}
	if v := q.Get("updated_since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.UpdatedSince = &t
		}
	}
	if q.Get("latest") == "true" {
		f.LatestOnly = true
	}
	if s := q.Get("status"); s != "" {
		f.Status = models.RegistryStatus(s)
	}
	if n, err := strconv.Atoi(q.Get("limit")); err == nil {
		f.Limit = n
	}
	if n, err := strconv.Atoi(q.Get("offset")); err == nil && n >= 0 {
		f.Offset = n
	}
	return f
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func mapServiceErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrConflict):
		writeErr(w, http.StatusConflict, err.Error())
	default:
		writeErr(w, http.StatusBadRequest, err.Error())
	}
}

// --- Servers --------------------------------------------------------------

func (h *Handler) ListServers(w http.ResponseWriter, r *http.Request) {
	out, err := h.Servers.List(r.Context(), parseFilter(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"servers": out.Items,
		"total":   out.Total,
		"next":    out.NextOffset,
	})
}

func (h *Handler) GetServer(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	row, err := h.Servers.Get(r.Context(), name, r.URL.Query().Get("version"))
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, withServerScores(r.Context(), h, row))
}

func (h *Handler) ListServerVersions(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	rows, err := h.Servers.ListVersions(r.Context(), name)
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": rows})
}

func (h *Handler) GetServerVersion(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	version := urlParam(r, "version")
	row, err := h.Servers.Get(r.Context(), name, version)
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, withServerScores(r.Context(), h, row))
}

// withServerScores fetches EnrichmentScore rows for the given server and
// returns a composite response. If no runner is configured or no scores
// exist, the envelope just echoes the server row.
func withServerScores(ctx context.Context, h *Handler, row *models.RegistryServer) any {
	if h.Enrichment == nil {
		return row
	}
	scores, _ := h.Enrichment.ScoresFor(ctx, models.RegistryKindServer, row.ID)
	return map[string]any{
		"server": row,
		"scores": scores,
	}
}

func (h *Handler) UpsertServer(w http.ResponseWriter, r *http.Request) {
	var body models.RegistryServer
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !authzWrite(w, r, "server", body.Name, models.RegistryActionPublish) {
		return
	}
	if uid, ok := middleware.GetUserID(r.Context()); ok {
		body.PublishedByUserID = &uid
	}
	out, err := h.Servers.Upsert(r.Context(), &body)
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	h.enqueueServerEnrichment(r.Context(), out.ID)
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) DeleteServerVersion(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	version := urlParam(r, "version")
	if !authzWrite(w, r, "server", name, models.RegistryActionDelete) {
		return
	}
	if err := h.Servers.SoftDelete(r.Context(), name, version); err != nil {
		mapServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Agents ---------------------------------------------------------------

func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	out, err := h.Agents.List(r.Context(), parseFilter(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agents": out.Items,
		"total":  out.Total,
		"next":   out.NextOffset,
	})
}

func (h *Handler) GetAgent(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	row, err := h.Agents.Get(r.Context(), name, r.URL.Query().Get("version"))
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handler) ListAgentVersions(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	rows, err := h.Agents.ListVersions(r.Context(), name)
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": rows})
}

func (h *Handler) GetAgentVersion(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	version := urlParam(r, "version")
	row, err := h.Agents.Get(r.Context(), name, version)
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// agentUpsertBody is the POST shape — an agent plus a flat list of refs.
type agentUpsertBody struct {
	models.RegistryAgent
	Refs []models.RegistryRef `json:"refs,omitempty"`
}

func (h *Handler) UpsertAgent(w http.ResponseWriter, r *http.Request) {
	var body agentUpsertBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !authzWrite(w, r, "agent", body.Name, models.RegistryActionPublish) {
		return
	}
	if uid, ok := middleware.GetUserID(r.Context()); ok {
		body.PublishedByUserID = &uid
	}
	out, err := h.Agents.Upsert(r.Context(), &service.UpsertInput{
		Agent: body.RegistryAgent,
		Refs:  body.Refs,
	})
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) DeleteAgentVersion(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	version := urlParam(r, "version")
	if !authzWrite(w, r, "agent", name, models.RegistryActionDelete) {
		return
	}
	if err := h.Agents.SoftDelete(r.Context(), name, version); err != nil {
		mapServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Skills ---------------------------------------------------------------

func (h *Handler) ListSkills(w http.ResponseWriter, r *http.Request) {
	out, err := h.Skills.List(r.Context(), parseFilter(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"skills": out.Items, "total": out.Total, "next": out.NextOffset,
	})
}

func (h *Handler) GetSkill(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	row, err := h.Skills.Get(r.Context(), name, r.URL.Query().Get("version"))
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handler) ListSkillVersions(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	rows, err := h.Skills.ListVersions(r.Context(), name)
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": rows})
}

func (h *Handler) GetSkillVersion(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	version := urlParam(r, "version")
	row, err := h.Skills.Get(r.Context(), name, version)
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handler) UpsertSkill(w http.ResponseWriter, r *http.Request) {
	var body models.RegistrySkill
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !authzWrite(w, r, "skill", body.Name, models.RegistryActionPublish) {
		return
	}
	if uid, ok := middleware.GetUserID(r.Context()); ok {
		body.PublishedByUserID = &uid
	}
	out, err := h.Skills.Upsert(r.Context(), &body)
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) DeleteSkillVersion(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	version := urlParam(r, "version")
	if !authzWrite(w, r, "skill", name, models.RegistryActionDelete) {
		return
	}
	if err := h.Skills.SoftDelete(r.Context(), name, version); err != nil {
		mapServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Prompts --------------------------------------------------------------

func (h *Handler) ListPrompts(w http.ResponseWriter, r *http.Request) {
	out, err := h.Prompts.List(r.Context(), parseFilter(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"prompts": out.Items, "total": out.Total, "next": out.NextOffset,
	})
}

func (h *Handler) GetPrompt(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	row, err := h.Prompts.Get(r.Context(), name, r.URL.Query().Get("version"))
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handler) ListPromptVersions(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	rows, err := h.Prompts.ListVersions(r.Context(), name)
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": rows})
}

func (h *Handler) GetPromptVersion(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	version := urlParam(r, "version")
	row, err := h.Prompts.Get(r.Context(), name, version)
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handler) UpsertPrompt(w http.ResponseWriter, r *http.Request) {
	var body models.RegistryPrompt
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !authzWrite(w, r, "prompt", body.Name, models.RegistryActionPublish) {
		return
	}
	if uid, ok := middleware.GetUserID(r.Context()); ok {
		body.PublishedByUserID = &uid
	}
	out, err := h.Prompts.Upsert(r.Context(), &body)
	if err != nil {
		mapServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) DeletePromptVersion(w http.ResponseWriter, r *http.Request) {
	name := urlParam(r, "name")
	version := urlParam(r, "version")
	if !authzWrite(w, r, "prompt", name, models.RegistryActionDelete) {
		return
	}
	if err := h.Prompts.SoftDelete(r.Context(), name, version); err != nil {
		mapServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Mount wires all registry routes onto a chi router at its current base.
// Reads are exposed publicly (caller controls auth at the router level);
// writes expect admin privileges enforced by outer middleware.
func (h *Handler) Mount(r chi.Router, readsPublic bool) {
	// Reads
	reads := func(r chi.Router) {
		r.Get("/servers", h.ListServers)
		r.Get("/servers/{name}", h.GetServer)
		r.Get("/servers/{name}/versions", h.ListServerVersions)
		r.Get("/servers/{name}/versions/{version}", h.GetServerVersion)

		r.Get("/agents", h.ListAgents)
		r.Get("/agents/{name}", h.GetAgent)
		r.Get("/agents/{name}/versions", h.ListAgentVersions)
		r.Get("/agents/{name}/versions/{version}", h.GetAgentVersion)

		r.Get("/skills", h.ListSkills)
		r.Get("/skills/{name}", h.GetSkill)
		r.Get("/skills/{name}/versions", h.ListSkillVersions)
		r.Get("/skills/{name}/versions/{version}", h.GetSkillVersion)

		r.Get("/prompts", h.ListPrompts)
		r.Get("/prompts/{name}", h.GetPrompt)
		r.Get("/prompts/{name}/versions", h.ListPromptVersions)
		r.Get("/prompts/{name}/versions/{version}", h.GetPromptVersion)
	}
	writes := func(r chi.Router) {
		r.Post("/servers", h.UpsertServer)
		r.Delete("/servers/{name}/versions/{version}", h.DeleteServerVersion)
		r.Post("/agents", h.UpsertAgent)
		r.Delete("/agents/{name}/versions/{version}", h.DeleteAgentVersion)
		r.Post("/skills", h.UpsertSkill)
		r.Delete("/skills/{name}/versions/{version}", h.DeleteSkillVersion)
		r.Post("/prompts", h.UpsertPrompt)
		r.Delete("/prompts/{name}/versions/{version}", h.DeletePromptVersion)
	}
	if readsPublic {
		r.Group(func(pub chi.Router) { reads(pub) })
	} else {
		reads(r)
	}
	writes(r)
}
