package admin

import (
	"net/http"

	"go.uber.org/zap"
)

type GroupHandler struct {
	baseHandler
}

func NewGroupHandler(logger *zap.Logger) *GroupHandler {
	return &GroupHandler{
		baseHandler: baseHandler{logger: logger},
	}
}

func (h *GroupHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "List groups")
}

func (h *GroupHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Create group")
}

func (h *GroupHandler) GetGroup(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Get group")
}

func (h *GroupHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Update group")
}

func (h *GroupHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Delete group")
}

func (h *GroupHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Get members")
}

func (h *GroupHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Add member")
}

func (h *GroupHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Remove member")
}

func (h *GroupHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Update member role")
}

func (h *GroupHandler) GetGroupUsage(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Get group usage")
}

func (h *GroupHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Update settings")
}
