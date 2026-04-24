// Package importer discovers MCP server packages on public package
// registries and upserts them into the pllm registry as RegistryServer
// rows. Phase 2 covers npm + PyPI only.
package importer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/registry/service"
	"gorm.io/datatypes"
)

// Source is a pluggable package-registry backend.
type Source interface {
	Name() string
	Search(ctx context.Context, query string, limit int) ([]Package, error)
}

// Package is the normalized shape each source emits.
// `RegistryType` maps to the OSV/scanner `registry_type` vocabulary.
type Package struct {
	Name         string
	Title        string
	Description  string
	Version      string
	WebsiteURL   string
	Repository   string
	Identifier   string // canonical package id (same as Name for npm/PyPI)
	RegistryType string // "npm" | "pypi"
}

// Service runs imports. Accepts multiple Sources; each is queried
// concurrently when Import is called.
type Service struct {
	servers *service.ServerService
	sources []Source
	logger  *zap.Logger
}

// NewService constructs an importer bound to the registry's ServerService.
// If `sources` is empty, the default npm + PyPI pair is wired up.
func NewService(servers *service.ServerService, logger *zap.Logger, sources ...Source) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	if len(sources) == 0 {
		sources = []Source{NewNPMSource(nil), NewPyPISource(nil)}
	}
	return &Service{servers: servers, sources: sources, logger: logger}
}

// Report is what Import returns — one entry per source.
type Report struct {
	Source    string   `json:"source"`
	Found     int      `json:"found"`
	Imported  int      `json:"imported"`
	Skipped   int      `json:"skipped"`
	Errors    []string `json:"errors,omitempty"`
}

// Import searches each configured Source for `query` (typically "mcp"),
// normalizes the hits, and upserts them as RegistryServer rows.
// Existing rows at the same (name, version) are overwritten — the
// service layer preserves the ID.
func (s *Service) Import(ctx context.Context, query string, limit int) ([]Report, error) {
	if s.servers == nil {
		return nil, errors.New("importer: no server service configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	reports := make([]Report, 0, len(s.sources))
	for _, src := range s.sources {
		rep := Report{Source: src.Name()}
		hits, err := src.Search(ctx, query, limit)
		if err != nil {
			rep.Errors = append(rep.Errors, err.Error())
			reports = append(reports, rep)
			continue
		}
		rep.Found = len(hits)
		for _, p := range hits {
			if err := s.upsertOne(ctx, p); err != nil {
				rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %s", p.Name, err.Error()))
				rep.Skipped++
				continue
			}
			rep.Imported++
		}
		reports = append(reports, rep)
	}
	return reports, nil
}

// upsertOne translates a normalized Package into a RegistryServer and
// publishes it. Names are normalized with a `<registry>.import/<ident>`
// prefix so they're easy to distinguish from hand-published entries.
func (s *Service) upsertOne(ctx context.Context, p Package) error {
	if p.Name == "" || p.Version == "" {
		return errors.New("missing name or version")
	}
	packages, _ := json.Marshal([]map[string]string{{
		"registry_type": p.RegistryType,
		"identifier":    p.Identifier,
		"version":       p.Version,
	}})
	row := &models.RegistryServer{
		Name:        canonicalName(p),
		Version:     p.Version,
		Title:       p.Title,
		Description: p.Description,
		WebsiteURL:  p.WebsiteURL,
		Packages:    datatypes.JSON(packages),
		PublishedAt: time.Now(),
	}
	if p.Repository != "" {
		repo, _ := json.Marshal(map[string]string{"type": "git", "url": p.Repository})
		row.Repository = datatypes.JSON(repo)
	}
	_, err := s.servers.Upsert(ctx, row)
	return err
}

// canonicalName yields a stable, namespaced name for an imported package.
// Example: (npm, "@modelcontextprotocol/server-filesystem")
//
//	-> "npm.import/@modelcontextprotocol/server-filesystem"
func canonicalName(p Package) string {
	return fmt.Sprintf("%s.import/%s", p.RegistryType, strings.TrimPrefix(p.Identifier, "/"))
}

// --- helpers shared by source implementations ---

// newHTTPClient returns a sensible HTTP client for calling public package
// registries. Short timeouts — if npm is slow, we'd rather fail and retry
// than hold the admin request open.
func newHTTPClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &http.Client{Timeout: timeout}
}
