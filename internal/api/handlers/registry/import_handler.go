package registry

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/services/registry/importer"
)

// ImportHandler triggers imports from external package registries.
// Writes are gated by the admin middleware; no further ACL needed since
// imports are system-wide and admins are the only callers.
type ImportHandler struct {
	logger   *zap.Logger
	importer *importer.Service
}

// NewImportHandler constructs an import handler.
func NewImportHandler(logger *zap.Logger, imp *importer.Service) *ImportHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ImportHandler{logger: logger, importer: imp}
}

// Trigger is POST /api/admin/registry/import — body: {"query":"mcp","limit":50}.
// Returns the per-source report.
func (h *ImportHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	if h.importer == nil {
		writeErr(w, http.StatusServiceUnavailable, "importer not configured")
		return
	}
	var body struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	// Allow empty body — defaults apply.
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	// Also accept query params for convenience.
	if body.Query == "" {
		body.Query = r.URL.Query().Get("query")
	}
	if body.Limit == 0 {
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
			body.Limit = n
		}
	}
	reports, err := h.importer.Import(r.Context(), body.Query, body.Limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reports": reports})
}
