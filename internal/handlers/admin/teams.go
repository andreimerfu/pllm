package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/middleware"
	"github.com/amerfu/pllm/internal/services/audit"
	"github.com/amerfu/pllm/internal/services/budget"
	"github.com/amerfu/pllm/internal/services/team"
)

type TeamHandler struct {
	baseHandler
	teamService   *team.TeamService
	auditLogger   *audit.Logger
	budgetTracker *budget.Tracker
}

func NewTeamHandler(logger *zap.Logger, teamService *team.TeamService, db *gorm.DB) *TeamHandler {
	return &TeamHandler{
		baseHandler:   baseHandler{logger: logger},
		teamService:   teamService,
		auditLogger:   audit.NewLogger(db),
		budgetTracker: budget.NewTracker(db),
	}
}

// CreateTeam creates a new team
func (h *TeamHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	var req team.CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get user ID from context
	userID, ok := middleware.GetUserID(r.Context())
	if !ok && !middleware.IsMasterKey(r.Context()) {
		h.sendError(w, http.StatusUnauthorized, "User authentication required")
		return
	}

	// If master key is used, generate a system user ID
	if middleware.IsMasterKey(r.Context()) {
		userID = uuid.New() // In production, use a dedicated system user
	}

	createdTeam, err := h.teamService.CreateTeam(r.Context(), &req, userID)
	if err != nil {
		if err == team.ErrTeamNameExists {
			h.sendError(w, http.StatusConflict, "Team name already exists")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusCreated, createdTeam)
}

// GetTeam gets a team by ID
func (h *TeamHandler) GetTeam(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	foundTeam, err := h.teamService.GetTeam(r.Context(), teamID)
	if err != nil {
		if err == team.ErrTeamNotFound {
			h.sendError(w, http.StatusNotFound, "Team not found")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, foundTeam)
}

// UpdateTeam updates team settings
func (h *TeamHandler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	// Check if user can manage team
	if !middleware.IsMasterKey(r.Context()) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			h.sendError(w, http.StatusUnauthorized, "User authentication required")
			return
		}

		canManage, err := h.teamService.CanManageTeam(r.Context(), teamID, userID)
		if err != nil || !canManage {
			h.sendError(w, http.StatusForbidden, "Insufficient permissions")
			return
		}
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	updatedTeam, err := h.teamService.UpdateTeam(r.Context(), teamID, updates)
	if err != nil {
		if err == team.ErrTeamNotFound {
			h.sendError(w, http.StatusNotFound, "Team not found")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, updatedTeam)
}

// DeleteTeam deletes a team
func (h *TeamHandler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	// Check if user can manage team
	if !middleware.IsMasterKey(r.Context()) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			h.sendError(w, http.StatusUnauthorized, "User authentication required")
			return
		}

		role, err := h.teamService.GetMemberRole(r.Context(), teamID, userID)
		if err != nil || role != "owner" {
			h.sendError(w, http.StatusForbidden, "Only team owner can delete team")
			return
		}
	}

	if err := h.teamService.DeleteTeam(r.Context(), teamID); err != nil {
		if err == team.ErrTeamNotFound {
			h.sendError(w, http.StatusNotFound, "Team not found")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]string{"message": "Team deleted successfully"})
}

// ListTeams lists teams
func (h *TeamHandler) ListTeams(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	var userID *uuid.UUID
	isMasterKey := middleware.IsMasterKey(r.Context())

	// Check if this is a JWT token for the master key user
	if uid, ok := middleware.GetUserID(r.Context()); ok {
		// Special master key user ID should be treated as master key access
		if uid.String() == "00000000-0000-0000-0000-000000000001" {
			isMasterKey = true
		} else {
			userID = &uid
		}
	}

	h.logger.Debug("ListTeams request",
		zap.Bool("is_master_key", isMasterKey),
		zap.Any("user_id", userID),
		zap.Int("limit", limit),
		zap.Int("offset", offset),
	)

	teams, total, err := h.teamService.ListTeams(r.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("Failed to list teams", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.logger.Debug("ListTeams response",
		zap.Int64("total", total),
		zap.Int("teams_count", len(teams)),
	)

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"teams": teams,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// AddMember adds a member to a team
func (h *TeamHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	// Check if user can manage team
	if !middleware.IsMasterKey(r.Context()) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			h.sendError(w, http.StatusUnauthorized, "User authentication required")
			return
		}

		canManage, err := h.teamService.CanManageTeam(r.Context(), teamID, userID)
		if err != nil || !canManage {
			h.sendError(w, http.StatusForbidden, "Insufficient permissions")
			return
		}
	}

	var req team.AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	member, err := h.teamService.AddMember(r.Context(), teamID, &req)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusCreated, member)
}

// UpdateMember updates a team member's settings
func (h *TeamHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	memberID, err := uuid.Parse(chi.URLParam(r, "memberID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid member ID")
		return
	}

	// Check if user can manage team
	if !middleware.IsMasterKey(r.Context()) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			h.sendError(w, http.StatusUnauthorized, "User authentication required")
			return
		}

		canManage, err := h.teamService.CanManageTeam(r.Context(), teamID, userID)
		if err != nil || !canManage {
			h.sendError(w, http.StatusForbidden, "Insufficient permissions")
			return
		}
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	member, err := h.teamService.UpdateMember(r.Context(), teamID, memberID, updates)
	if err != nil {
		if err == team.ErrUserNotInTeam {
			h.sendError(w, http.StatusNotFound, "Member not found")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, member)
}

// RemoveMember removes a member from a team
func (h *TeamHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	memberID, err := uuid.Parse(chi.URLParam(r, "memberID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid member ID")
		return
	}

	// Check if user can manage team
	if !middleware.IsMasterKey(r.Context()) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok {
			h.sendError(w, http.StatusUnauthorized, "User authentication required")
			return
		}

		canManage, err := h.teamService.CanManageTeam(r.Context(), teamID, userID)
		if err != nil || !canManage {
			h.sendError(w, http.StatusForbidden, "Insufficient permissions")
			return
		}
	}

	if err := h.teamService.RemoveMember(r.Context(), teamID, memberID); err != nil {
		if err == team.ErrUserNotInTeam {
			h.sendError(w, http.StatusNotFound, "Member not found")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]string{"message": "Member removed successfully"})
}

// GetTeamStats gets team statistics
func (h *TeamHandler) GetTeamStats(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	stats, err := h.teamService.GetTeamStats(r.Context(), teamID)
	if err != nil {
		if err == team.ErrTeamNotFound {
			h.sendError(w, http.StatusNotFound, "Team not found")
			return
		}
		h.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.sendJSON(w, http.StatusOK, stats)
}
