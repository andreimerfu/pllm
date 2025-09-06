package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/amerfu/pllm/internal/services"
	"github.com/amerfu/pllm/internal/services/models"
	"github.com/amerfu/pllm/internal/services/providers"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type FilesHandler struct {
	logger         *zap.Logger
	modelManager   *models.ModelManager
	metricsEmitter *services.MetricEventEmitter
}

func NewFilesHandler(logger *zap.Logger, modelManager *models.ModelManager) *FilesHandler {
	return &FilesHandler{
		logger:       logger,
		modelManager: modelManager,
	}
}

func NewFilesHandlerWithMetrics(logger *zap.Logger, modelManager *models.ModelManager, metricsEmitter *services.MetricEventEmitter) *FilesHandler {
	return &FilesHandler{
		logger:         logger,
		modelManager:   modelManager,
		metricsEmitter: metricsEmitter,
	}
}

// UploadFile handles file uploads for chat attachments
// @Summary Upload file
// @Description Upload a file for use in chat messages
// @Tags Files
// @Accept multipart/form-data
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param file formData file true "File to upload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} providers.ErrorResponse
// @Failure 413 {object} providers.ErrorResponse
// @Failure 500 {object} providers.ErrorResponse
// @Router /files [post]
func (h *FilesHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (limit to 32MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to parse form: "+err.Error())
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "No file provided or invalid file")
		return
	}
	defer func() { _ = file.Close() }()

	// Validate file size (max 10MB)
	if fileHeader.Size > 10<<20 {
		h.sendError(w, http.StatusRequestEntityTooLarge, "File too large (max 10MB)")
		return
	}

	// Validate file type (images only for now)
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		// Try to detect from extension
		contentType = http.DetectContentType(make([]byte, 512))
	}
	
	if !strings.HasPrefix(contentType, "image/") {
		h.sendError(w, http.StatusBadRequest, "Only image files are supported")
		return
	}

	// Generate unique filename
	fileID := fmt.Sprintf("%d_%s", time.Now().Unix(), fileHeader.Filename)
	uploadDir := "./uploads"
	
	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		h.logger.Error("Failed to create upload directory", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to create upload directory")
		return
	}

	// Save file to disk
	filepath := fmt.Sprintf("%s/%s", uploadDir, fileID)
	dst, err := os.Create(filepath)
	if err != nil {
		h.logger.Error("Failed to create file", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to save file")
		return
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, file); err != nil {
		h.logger.Error("Failed to write file", zap.Error(err))
		h.sendError(w, http.StatusInternalServerError, "Failed to save file")
		return
	}

	// Return file info
	response := map[string]interface{}{
		"id":       fileID,
		"filename": fileHeader.Filename,
		"size":     fileHeader.Size,
		"type":     contentType,
		"url":      fmt.Sprintf("/files/%s", fileID),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode upload response", zap.Error(err))
	}
}

// ListFiles lists uploaded files
// @Summary List files
// @Description Returns a list of files that belong to the user's organization
// @Tags Files
// @Accept json
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} providers.ErrorResponse
// @Router /files [get]
func (h *FilesHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "List files not yet implemented")
}

// GetFile serves uploaded files
// @Summary Get file
// @Description Serve an uploaded file
// @Tags Files
// @Param fileID path string true "File ID"
// @Success 200 {file} binary
// @Failure 404 {object} providers.ErrorResponse
// @Router /files/{fileID} [get]
func (h *FilesHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	// Extract file ID from URL parameter
	fileID := chi.URLParam(r, "fileID")
	if fileID == "" {
		h.sendError(w, http.StatusBadRequest, "File ID required")
		return
	}

	// Basic security: prevent directory traversal
	if strings.Contains(fileID, "..") || strings.Contains(fileID, "/") {
		h.sendError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	filepath := fmt.Sprintf("./uploads/%s", fileID)
	
	// Check if file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		h.sendError(w, http.StatusNotFound, "File not found")
		return
	}

	// Serve the file
	http.ServeFile(w, r, filepath)
}

// DeleteFile deletes an uploaded file
// @Summary Delete file
// @Description Delete a file
// @Tags Files
// @Accept json
// @Produce json
// @Param X-API-Key header string false "API Key for authentication"
// @Param Authorization header string false "Bearer token for authentication"
// @Param file_id path string true "File ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} providers.ErrorResponse
// @Failure 401 {object} providers.ErrorResponse
// @Failure 404 {object} providers.ErrorResponse
// @Router /files/{file_id} [delete]
func (h *FilesHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	h.sendError(w, http.StatusNotImplemented, "Delete file not yet implemented")
}

func (h *FilesHandler) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(providers.ErrorResponse{
		Error: providers.APIError{
			Message: message,
			Type:    "invalid_request_error",
		},
	}); err != nil {
		h.logger.Error("Failed to encode files error response", zap.Error(err))
	}
}