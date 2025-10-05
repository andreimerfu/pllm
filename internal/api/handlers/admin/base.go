package admin

import (
	"encoding/json"
	"log"
	"net/http"

	"go.uber.org/zap"
)

type baseHandler struct {
	logger *zap.Logger
}

func (h *baseHandler) sendResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

func (h *baseHandler) sendError(w http.ResponseWriter, status int, message string) {
	h.sendResponse(w, status, map[string]string{
		"error": message,
	})
}

func (h *baseHandler) notImplemented(w http.ResponseWriter, endpoint string) {
	h.sendError(w, http.StatusNotImplemented, endpoint+" not yet implemented")
}

func (h *baseHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
	}
}
