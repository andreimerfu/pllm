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

	// Check if it's a static asset (has file extension) or a route
	hasExtension := strings.Contains(path.Base(uiPath), ".")
	
	// If it's not a static asset, serve index.html for client-side routing
	if !hasExtension && uiPath != "/index.html" {
		uiPath = "/index.html"
	}

	// Try to open the file
	file, err := h.fileSystem.Open(uiPath)
	if err != nil {
		// If file not found and it was a static asset request, return 404
		if hasExtension {
			http.NotFound(w, r)
			return
		}
		// Otherwise serve index.html
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
		uiPath = "/index.html"
		file.Close()
		file, err = h.fileSystem.Open(uiPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		
		stat, err = file.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	// Set content type based on file extension
	ext := path.Ext(uiPath)
	switch ext {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Disable caching for HTML to ensure updates are reflected
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
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

	// Serve the file using http.ServeContent for proper handling
	http.ServeContent(w, r, uiPath, stat.ModTime(), file.(http.File))
}

