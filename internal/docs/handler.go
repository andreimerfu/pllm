package docs

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/amerfu/pllm/internal/config"
	"go.uber.org/zap"
)

//go:embed dist/*
var docsFS embed.FS

// Handler handles documentation routes
type Handler struct {
	logger     *zap.Logger
	fileSystem http.FileSystem
	enabled    bool
}

// NewHandler creates a new documentation handler
func NewHandler(cfg *config.Config, logger *zap.Logger) (*Handler, error) {
	// Get the dist subdirectory from embedded filesystem
	distFS, err := fs.Sub(docsFS, "dist")
	if err != nil {
		// If dist doesn't exist yet (not built), disable docs
		logger.Info("Documentation not built yet - run 'make docs-build' to build")
		return &Handler{
			logger:  logger,
			enabled: false,
		}, nil
	}

	logger.Info("Documentation handler initialized")
	return &Handler{
		logger:     logger,
		fileSystem: http.FS(distFS),
		enabled:    true,
	}, nil
}

// IsEnabled returns whether the documentation is enabled
func (h *Handler) IsEnabled() bool {
	return h.enabled
}

// ServeHTTP serves the documentation files
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If docs are disabled, return 503
	if !h.enabled {
		http.Error(w, "Documentation not built. Run 'make docs-build' to build.", http.StatusServiceUnavailable)
		return
	}

	// Clean the path
	docPath := strings.TrimPrefix(r.URL.Path, "/docs")
	if docPath == "" || docPath == "/" {
		docPath = "/index.html"
	}

	// Check if file exists
	file, err := h.fileSystem.Open(docPath)
	if err != nil {
		// For any 404, serve index.html for client-side routing
		docPath = "/index.html"
		file, err = h.fileSystem.Open(docPath)
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
		docPath = path.Join(docPath, "index.html")
		file, err = h.fileSystem.Open(docPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
	}

	// Set content type based on file extension
	ext := path.Ext(docPath)
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