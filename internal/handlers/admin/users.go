package admin

import (
	"net/http"
	"go.uber.org/zap"
)

type UserHandler struct {
	baseHandler
}

func NewUserHandler(logger *zap.Logger) *UserHandler {
	return &UserHandler{
		baseHandler: baseHandler{logger: logger},
	}
}

func (h *UserHandler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Admin login")
}

func (h *UserHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Refresh token")
}

func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "List users")
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Create user")
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Get user")
}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Update user")
}

func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Delete user")
}

func (h *UserHandler) ActivateUser(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Activate user")
}

func (h *UserHandler) DeactivateUser(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Deactivate user")
}

func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Reset password")
}

func (h *UserHandler) GetUserAPIKeys(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Get user API keys")
}

func (h *UserHandler) GetUserUsage(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Get user usage")
}

func (h *UserHandler) ListAllAPIKeys(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "List all API keys")
}

func (h *UserHandler) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Get API key")
}

func (h *UserHandler) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Update API key")
}

func (h *UserHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Delete API key")
}

func (h *UserHandler) ActivateAPIKey(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Activate API key")
}

func (h *UserHandler) DeactivateAPIKey(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Deactivate API key")
}