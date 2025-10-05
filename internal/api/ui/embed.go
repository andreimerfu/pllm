package ui

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

// Embed the built UI files
// The dist folder must exist at compile time for embedding to work
//
//go:embed all:dist
var distFS embed.FS

// GetFileSystem returns the embedded filesystem for the UI
func GetFileSystem() (http.FileSystem, error) {
	// Check if dist folder exists in embedded FS
	entries, err := distFS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded filesystem: %w", err)
	}

	hasDistFolder := false
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == "dist" {
			hasDistFolder = true
			break
		}
	}

	if !hasDistFolder {
		return nil, fmt.Errorf("dist folder not found in embedded filesystem - UI was not built before compilation")
	}

	fsys, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, fmt.Errorf("failed to create sub-filesystem for dist: %w", err)
	}
	return http.FS(fsys), nil
}
