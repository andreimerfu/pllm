// Package federation exposes the federation surface as HTTP routes:
//
//   - /v1/federation/{health,servers,agents,skills,prompts} — read-only
//     "export" endpoints that other pllm instances poll. Bearer-token
//     authenticated against the receiver's configured FederationExportToken.
//   - /api/admin/federation/peers — admin CRUD for outbound peer config.
//   - /api/admin/federation/peers/{id}/sync — manual sync trigger.
//   - /api/admin/federation/peers/{id}/status — last-run summary.
//
// Receivers configure their export token via cfg.Federation.ExportToken.
// If empty, the export endpoints are still mounted but reject everything
// — operators must set a token before federation works.
package federation

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
	federationsvc "github.com/amerfu/pllm/internal/services/federation"
)

// Handler exposes both the export and admin federation surfaces.
type Handler struct {
	logger      *zap.Logger
	svc         *federationsvc.Service
	hooks       federationsvc.WebhookEmitter
	db          *gorm.DB
	exportToken string
}

// NewHandler builds a Handler. exportToken may be empty — in that case
// export endpoints will 503 until configured.
func NewHandler(logger *zap.Logger, svc *federationsvc.Service, hooks federationsvc.WebhookEmitter, db *gorm.DB, exportToken string) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, svc: svc, hooks: hooks, db: db, exportToken: exportToken}
}

// --- export surface -------------------------------------------------------

// MountExport wires the read-only export endpoints onto r.
func (h *Handler) MountExport(r chi.Router) {
	r.Get("/health", h.exportHealth)
	r.Get("/servers", h.exportServers)
	r.Get("/agents", h.exportAgents)
	r.Get("/skills", h.exportSkills)
	r.Get("/prompts", h.exportPrompts)
}

// MountAdmin wires the admin CRUD + control endpoints onto r.
func (h *Handler) MountAdmin(r chi.Router) {
	r.Get("/peers", h.listPeers)
	r.Post("/peers", h.createPeer)
	r.Get("/peers/{id}", h.getPeer)
	r.Put("/peers/{id}", h.updatePeer)
	r.Delete("/peers/{id}", h.deletePeer)
	r.Post("/peers/{id}/sync", h.syncPeer)
	r.Get("/peers/{id}/status", h.peerStatus)
}

// authExport gates an export endpoint on the configured token. We use
// constant-time compare so timing attacks can't enumerate the token.
// Empty configured token = always 503 (federation receiver disabled).
func (h *Handler) authExport(w http.ResponseWriter, r *http.Request) bool {
	if h.exportToken == "" {
		writeErr(w, http.StatusServiceUnavailable, "federation export is not configured")
		return false
	}
	hdr := r.Header.Get("Authorization")
	if !strings.HasPrefix(hdr, "Bearer ") {
		writeErr(w, http.StatusUnauthorized, "missing bearer token")
		return false
	}
	if subtle.ConstantTimeCompare([]byte(hdr[7:]), []byte(h.exportToken)) != 1 {
		writeErr(w, http.StatusUnauthorized, "invalid federation token")
		return false
	}
	return true
}

func (h *Handler) exportHealth(w http.ResponseWriter, r *http.Request) {
	if !h.authExport(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "pllm-federation",
	})
}

// pageQuery returns the limit/offset from the request, with reasonable bounds.
func pageQuery(r *http.Request) (limit, offset int) {
	limit, offset = 200, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}

// exportServers returns active latest-version servers that originated
// locally — federated rows aren't re-exported, so federation can't form
// loops across more than one hop.
func (h *Handler) exportServers(w http.ResponseWriter, r *http.Request) {
	if !h.authExport(w, r) {
		return
	}
	limit, offset := pageQuery(r)
	var rows []models.RegistryServer
	var total int64
	q := h.db.WithContext(r.Context()).Model(&models.RegistryServer{}).
		Where("source_peer_id = ? AND is_latest = TRUE AND status = ?", "", models.RegistryStatusActive)
	if err := q.Count(&total).Error; err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := q.Order("name asc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	next := offset + len(rows)
	if int64(next) >= total {
		next = 0
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": rows, "total": total, "next": next})
}

// exportAgents — same pattern.
func (h *Handler) exportAgents(w http.ResponseWriter, r *http.Request) {
	if !h.authExport(w, r) {
		return
	}
	limit, offset := pageQuery(r)
	var rows []models.RegistryAgent
	var total int64
	q := h.db.WithContext(r.Context()).Model(&models.RegistryAgent{}).
		Where("source_peer_id = ? AND is_latest = TRUE AND status = ?", "", models.RegistryStatusActive)
	if err := q.Count(&total).Error; err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := q.Order("name asc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	next := offset + len(rows)
	if int64(next) >= total {
		next = 0
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": rows, "total": total, "next": next})
}

func (h *Handler) exportSkills(w http.ResponseWriter, r *http.Request) {
	if !h.authExport(w, r) {
		return
	}
	limit, offset := pageQuery(r)
	var rows []models.RegistrySkill
	var total int64
	q := h.db.WithContext(r.Context()).Model(&models.RegistrySkill{}).
		Where("source_peer_id = ? AND is_latest = TRUE AND status = ?", "", models.RegistryStatusActive)
	if err := q.Count(&total).Error; err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := q.Order("name asc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	next := offset + len(rows)
	if int64(next) >= total {
		next = 0
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": rows, "total": total, "next": next})
}

func (h *Handler) exportPrompts(w http.ResponseWriter, r *http.Request) {
	if !h.authExport(w, r) {
		return
	}
	limit, offset := pageQuery(r)
	var rows []models.RegistryPrompt
	var total int64
	q := h.db.WithContext(r.Context()).Model(&models.RegistryPrompt{}).
		Where("source_peer_id = ? AND is_latest = TRUE AND status = ?", "", models.RegistryStatusActive)
	if err := q.Count(&total).Error; err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := q.Order("name asc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	next := offset + len(rows)
	if int64(next) >= total {
		next = 0
	}
	writeJSON(w, http.StatusOK, map[string]any{"prompts": rows, "total": total, "next": next})
}

// --- admin surface --------------------------------------------------------

type peerBody struct {
	PeerID              string                    `json:"peer_id"`
	Name                string                    `json:"name"`
	Endpoint            string                    `json:"endpoint"`
	FederationToken     string                    `json:"federation_token,omitempty"`
	ExpectedClientID    string                    `json:"expected_client_id,omitempty"`
	SyncMode            models.FederationSyncMode `json:"sync_mode"`
	WhitelistServers    []string                  `json:"whitelist_servers,omitempty"`
	WhitelistAgents     []string                  `json:"whitelist_agents,omitempty"`
	WhitelistSkills     []string                  `json:"whitelist_skills,omitempty"`
	WhitelistPrompts    []string                  `json:"whitelist_prompts,omitempty"`
	TagFilters          []string                  `json:"tag_filters,omitempty"`
	SyncIntervalMinutes int                       `json:"sync_interval_minutes,omitempty"`
	Enabled             *bool                     `json:"enabled,omitempty"`
}

func (h *Handler) listPeers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Mask the federation token before sending — operators rotate via
	// PUT, never read back.
	for i := range rows {
		if rows[i].FederationToken != "" {
			rows[i].FederationToken = "***"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"peers": rows})
}

func (h *Handler) getPeer(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.svc.Get(r.Context(), id)
	if err != nil {
		mapErr(w, err)
		return
	}
	if row.FederationToken != "" {
		row.FederationToken = "***"
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handler) createPeer(w http.ResponseWriter, r *http.Request) {
	var body peerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	row, err := h.svc.Create(r.Context(), federationsvc.CreateInput{
		PeerID:              body.PeerID,
		Name:                body.Name,
		Endpoint:            body.Endpoint,
		FederationToken:     body.FederationToken,
		ExpectedClientID:    body.ExpectedClientID,
		SyncMode:            body.SyncMode,
		WhitelistServers:    body.WhitelistServers,
		WhitelistAgents:     body.WhitelistAgents,
		WhitelistSkills:     body.WhitelistSkills,
		WhitelistPrompts:    body.WhitelistPrompts,
		TagFilters:          body.TagFilters,
		SyncIntervalMinutes: body.SyncIntervalMinutes,
		Enabled:             enabled,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	row.FederationToken = "***"
	writeJSON(w, http.StatusCreated, row)
}

func (h *Handler) updatePeer(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var body peerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in := federationsvc.UpdateInput{}
	if body.Name != "" {
		in.Name = &body.Name
	}
	if body.Endpoint != "" {
		in.Endpoint = &body.Endpoint
	}
	// Token: empty means "leave unchanged"; "***" sentinel from a read
	// round-trip also means "leave unchanged"; anything else replaces.
	if body.FederationToken != "" && body.FederationToken != "***" {
		t := body.FederationToken
		in.FederationToken = &t
	}
	if body.ExpectedClientID != "" {
		in.ExpectedClientID = &body.ExpectedClientID
	}
	if body.SyncMode != "" {
		in.SyncMode = &body.SyncMode
	}
	if body.WhitelistServers != nil {
		in.WhitelistServers = &body.WhitelistServers
	}
	if body.WhitelistAgents != nil {
		in.WhitelistAgents = &body.WhitelistAgents
	}
	if body.WhitelistSkills != nil {
		in.WhitelistSkills = &body.WhitelistSkills
	}
	if body.WhitelistPrompts != nil {
		in.WhitelistPrompts = &body.WhitelistPrompts
	}
	if body.TagFilters != nil {
		in.TagFilters = &body.TagFilters
	}
	if body.SyncIntervalMinutes != 0 {
		in.SyncIntervalMinutes = &body.SyncIntervalMinutes
	}
	if body.Enabled != nil {
		in.Enabled = body.Enabled
	}
	row, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		mapErr(w, err)
		return
	}
	if row.FederationToken != "" {
		row.FederationToken = "***"
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handler) deletePeer(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		mapErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// syncPeer runs SyncOne synchronously and returns the result. Used by
// the admin "Sync now" button — the runner still handles scheduled
// pulls in the background.
func (h *Handler) syncPeer(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	res, err := h.svc.SyncOne(r.Context(), id, h.hooks)
	if err != nil {
		// Even on failure we return the result envelope so the UI shows
		// counts that did land before the failing kind.
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":  err.Error(),
			"result": res,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": res})
}

// peerStatus returns the cached last-run state stored on the peer row.
func (h *Handler) peerStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.svc.Get(r.Context(), id)
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"peer_id":              row.PeerID,
		"enabled":              row.Enabled,
		"last_sync_at":         row.LastSyncAt,
		"last_sync_status":     row.LastSyncStatus,
		"last_sync_summary":    row.LastSyncSummary,
		"last_sync_error":      row.LastSyncError,
		"last_sync_generation": row.LastSyncGeneration,
	})
}

// --- helpers --------------------------------------------------------------

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func mapErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, federationsvc.ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	default:
		writeErr(w, http.StatusBadRequest, err.Error())
	}
}
