// Package lifecycle exposes the lifecycle daemon over the admin API.
// Two endpoints: GET /status returns the cached last run, POST /run
// triggers a synchronous run and returns its summary.
package lifecycle

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	lifesvc "github.com/amerfu/pllm/internal/services/lifecycle"
)

// Handler is the HTTP edge for the lifecycle service. nil service is
// allowed at construction time (returns 503 from each handler) so the
// router can mount the routes regardless of feature gating.
type Handler struct {
	logger *zap.Logger
	svc    *lifesvc.Service
}

// NewHandler returns a Handler. Pass a nil service to disable.
func NewHandler(logger *zap.Logger, svc *lifesvc.Service) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, svc: svc}
}

// Status returns the cached summary of the most recent run.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		writeErr(w, http.StatusServiceUnavailable, "lifecycle disabled")
		return
	}
	writeJSON(w, http.StatusOK, h.svc.LastSummary())
}

// RunNow forces an out-of-band run and returns the resulting summary.
// Synchronous: callers should not invoke this on the request path of
// anything user-facing.
func (h *Handler) RunNow(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		writeErr(w, http.StatusServiceUnavailable, "lifecycle disabled")
		return
	}
	summary, err := h.svc.RunOnce(r.Context())
	if err != nil {
		h.logger.Warn("lifecycle run failed", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr emits a JSON error body matching the convention of the
// other admin handlers (use_cases, webhooks).
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
