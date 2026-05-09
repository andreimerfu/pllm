package otel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/amerfu/pllm/internal/services/discovery"
	"go.uber.org/zap"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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
	var req discovery.ExportTraceServiceRequest
	switch {
	case strings.Contains(ct, "application/json"):
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Warn("otel ingest decode failed (json)", zap.Error(err))
			http.Error(w, "bad otlp json", http.StatusBadRequest)
			return
		}
	case strings.Contains(ct, "application/x-protobuf"):
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body failed", http.StatusBadRequest)
			return
		}
		var pb coltracepb.ExportTraceServiceRequest
		if err := proto.Unmarshal(body, &pb); err != nil {
			h.logger.Warn("otel ingest decode failed (protobuf)", zap.Error(err))
			http.Error(w, "bad otlp protobuf", http.StatusBadRequest)
			return
		}
		// Convert proto -> JSON -> our internal struct. The
		// double-encoding is cheap relative to the rest of the request
		// path and avoids hand-mapping every proto field.
		j, err := protojson.Marshal(&pb)
		if err != nil {
			h.logger.Warn("otel proto->json marshal failed", zap.Error(err))
			http.Error(w, "otlp transcode failed", http.StatusInternalServerError)
			return
		}
		if err := json.Unmarshal(j, &req); err != nil {
			h.logger.Warn("otel json decode failed after transcode", zap.Error(err))
			http.Error(w, "otlp transcode failed", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "Content-Type must be application/json or application/x-protobuf", http.StatusUnsupportedMediaType)
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
