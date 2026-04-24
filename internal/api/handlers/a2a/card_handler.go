package a2a

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
	a2asvc "github.com/amerfu/pllm/internal/services/a2a"
)

// CardHandler serves /v1/a2a/{slug}/.well-known/agent.json.
type CardHandler struct {
	logger    *zap.Logger
	manager   *a2asvc.Manager
	db        *gorm.DB
	publicURL string // base URL clients see pllm at; e.g. "https://pllm.example.com"
}

func NewCardHandler(logger *zap.Logger, manager *a2asvc.Manager, db *gorm.DB, publicURL string) *CardHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CardHandler{logger: logger, manager: manager, db: db, publicURL: publicURL}
}

// Card returns the A2A agent card derived from the RegistryAgent (for
// display fields) and RegistryRef rows (for skills).
func (h *CardHandler) Card(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	agent, err := h.manager.Resolve(r.Context(), slug)
	if err != nil {
		if errors.Is(err, a2asvc.ErrAgentNotFound) {
			writeJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var reg models.RegistryAgent
	if err := h.db.WithContext(r.Context()).
		First(&reg, "id = ?", agent.RegistryAgentID).Error; err != nil {
		h.logger.Warn("card: registry agent missing", zap.String("slug", slug))
	}

	var refs []models.RegistryRef
	_ = h.db.WithContext(r.Context()).
		Where("owner_kind = ? AND owner_id = ? AND target_kind = ?",
			models.RegistryKindAgent, agent.RegistryAgentID, models.RegistryKindSkill).
		Find(&refs).Error

	skills := make([]a2asvc.AgentCardSkill, 0, len(refs))
	for _, ref := range refs {
		skills = append(skills, a2asvc.AgentCardSkill{
			Name: lastSegmentOrName(ref.TargetName),
		})
	}

	card := a2asvc.AgentCard{
		Name:        agent.ResourceName,
		Description: reg.Description,
		URL:         strings.TrimRight(h.publicURL, "/") + "/v1/a2a/" + slug,
		Version:     agent.ResourceVersion,
		Capabilities: a2asvc.AgentCapabilities{
			Streaming:         false,
			PushNotifications: false,
		},
		Skills: skills,
	}
	writeJSON(w, http.StatusOK, card)
}

func lastSegmentOrName(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
