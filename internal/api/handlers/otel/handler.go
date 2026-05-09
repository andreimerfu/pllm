package otel

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/amerfu/pllm/internal/services/discovery"
	"go.uber.org/zap"
)

// Ingester is the surface this handler needs from the discovery service.
type Ingester interface {
	Ingest(ctx context.Context, req discovery.ExportTraceServiceRequest) error
}

type Handler struct {
	logger *zap.Logger
	svc    Ingester
}

func New(logger *zap.Logger, svc Ingester) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{logger: logger, svc: svc}
}

// Traces is the OTLP/HTTP traces endpoint.
//
//	POST /v1/traces
//	Content-Type: application/json
//	Body: ExportTraceServiceRequest (OTLP/HTTP JSON)
//
// Response: 200 with empty {} on success (matches OTLP convention).
func (h *Handler) Traces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ct := r.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		http.Error(w, "only application/json OTLP is accepted; set OTEL_EXPORTER_OTLP_PROTOCOL=http/json", http.StatusUnsupportedMediaType)
		return
	}

	var req discovery.ExportTraceServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("otel ingest decode failed", zap.Error(err))
		http.Error(w, "bad otlp body", http.StatusBadRequest)
		return
	}

	// Best-effort — we don't surface ingest errors to the client (otel
	// exporters retry the entire batch and amplify partial failures).
	if err := h.svc.Ingest(r.Context(), req); err != nil {
		h.logger.Warn("otel ingest failed", zap.Error(err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"partialSuccess":{}}`))
}
