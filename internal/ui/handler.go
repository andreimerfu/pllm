package ui

import (
	"net/http"
	"path"
	"strings"

	"github.com/amerfu/pllm/internal/config"
	"go.uber.org/zap"
)

// Handler handles UI routes
type Handler struct {
	logger       *zap.Logger
	fileSystem   http.FileSystem
	enabled      bool
}

// NewHandler creates a new UI handler
func NewHandler(cfg *config.Config, logger *zap.Logger) (*Handler, error) {
	// Check if database is configured
	dbConfigured := cfg.Database.URL != ""
	
	// Only enable UI if database is configured
	if !dbConfigured {
		logger.Info("UI disabled - database not configured")
		return &Handler{
			logger:  logger,
			enabled: false,
		}, nil
	}

	// Get embedded filesystem
	fs, err := GetFileSystem()
	if err != nil {
		return nil, err
	}

	logger.Info("UI enabled - database configured")
	return &Handler{
		logger:     logger,
		fileSystem: fs,
		enabled:    true,
	}, nil
}

// IsEnabled returns whether the UI is enabled
func (h *Handler) IsEnabled() bool {
	return h.enabled
}

// ServeHTTP serves the UI files
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If UI is disabled, return 503
	if !h.enabled {
		http.Error(w, "UI requires database connection", http.StatusServiceUnavailable)
		return
	}

	// Clean the path
	uiPath := strings.TrimPrefix(r.URL.Path, "/ui")
	if uiPath == "" || uiPath == "/" {
		uiPath = "/index.html"
	}

	// Check if file exists
	file, err := h.fileSystem.Open(uiPath)
	if err != nil {
		// For any 404, serve index.html for client-side routing
		uiPath = "/index.html"
		file, err = h.fileSystem.Open(uiPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	defer file.Close()

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// If it's a directory, serve index.html
	if stat.IsDir() {
		uiPath = path.Join(uiPath, "index.html")
		file, err = h.fileSystem.Open(uiPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
	}

	// Set content type based on file extension
	ext := path.Ext(uiPath)
	switch ext {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	}

	// Serve the file
	http.FileServer(h.fileSystem).ServeHTTP(w, r)
}

