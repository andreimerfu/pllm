// Package service implements the registry business logic: list, get,
// upsert (with versioning + is_latest bookkeeping), and delete for each
// artifact kind. Handlers call these — no DB access in handlers.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// ErrNotFound is returned by Get when the requested name/version is absent.
var ErrNotFound = errors.New("registry: not found")

// ErrConflict is returned when a unique constraint would be violated.
var ErrConflict = errors.New("registry: conflict")

// ListFilter narrows a list query. All fields optional.
type ListFilter struct {
	Search       string     // substring match on name or description
	UpdatedSince *time.Time // only rows updated at/after this time
	LatestOnly   bool       // return only is_latest=true rows
	Status       models.RegistryStatus
	Limit        int // clamped in ListAll
	Offset       int
}

// ListResult wraps a list query's rows and pagination cursor.
type ListResult[T any] struct {
	Items      []T
	Total      int64
	NextOffset int
}

// nameValidRE checks that names resemble the dotted / slash-separated
// convention used by most registries: "io.github.org/project", "org/name",
// "@scope/pkg", or a bare identifier. Keep the rule lenient — we normalize
// casing and allow the common punctuation used by npm, PyPI, Go, OCI.
func validName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name is required")
	}
	for _, ch := range name {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '.' || ch == '-' || ch == '_' || ch == '/' || ch == '@':
		default:
			return fmt.Errorf("name contains invalid character %q", ch)
		}
	}
	return nil
}

func validVersion(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("version is required")
	}
	return nil
}

// --- Per-kind services ----------------------------------------------------

// ServerService manages RegistryServer rows.
type ServerService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// AgentService manages RegistryAgent rows + refs.
type AgentService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// SkillService manages RegistrySkill rows.
type SkillService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// PromptService manages RegistryPrompt rows.
type PromptService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewServerService constructs a ServerService.
func NewServerService(db *gorm.DB, logger *zap.Logger) *ServerService {
	return &ServerService{db: db, logger: nopLogger(logger)}
}

// NewAgentService constructs an AgentService.
func NewAgentService(db *gorm.DB, logger *zap.Logger) *AgentService {
	return &AgentService{db: db, logger: nopLogger(logger)}
}

// NewSkillService constructs a SkillService.
func NewSkillService(db *gorm.DB, logger *zap.Logger) *SkillService {
	return &SkillService{db: db, logger: nopLogger(logger)}
}

// NewPromptService constructs a PromptService.
func NewPromptService(db *gorm.DB, logger *zap.Logger) *PromptService {
	return &PromptService{db: db, logger: nopLogger(logger)}
}

func nopLogger(l *zap.Logger) *zap.Logger {
	if l == nil {
		return zap.NewNop()
	}
	return l
}

// --- helpers shared across kinds ------------------------------------------

// applyFilter layers a ListFilter onto a base query for a given table.
// The caller supplies the column name prefix and the SQL fields available.
// We only do substring search on name/description; more exotic search
// (pgvector, full-text) plugs in later.
func applyFilter(q *gorm.DB, f ListFilter) *gorm.DB {
	if f.Search != "" {
		pattern := "%" + strings.ToLower(f.Search) + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", pattern, pattern)
	}
	if f.UpdatedSince != nil {
		q = q.Where("updated_at >= ?", *f.UpdatedSince)
	}
	if f.LatestOnly {
		q = q.Where("is_latest = ?", true)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	} else {
		// By default hide soft-deleted rows.
		q = q.Where("status <> ?", models.RegistryStatusDeleted)
	}
	return q
}

func clampLimit(n int) int {
	if n <= 0 {
		return 30
	}
	if n > 200 {
		return 200
	}
	return n
}

// setLatestAtomically flips is_latest so only `currentID` is true within
// the rows that share the given name. Runs inside the caller's tx.
// The caller is responsible for refreshing any in-memory struct they hold
// — DB is authoritative but the Go value is out of date after this call.
func setLatestAtomically(tx *gorm.DB, table string, name string, currentID uuid.UUID) error {
	if err := tx.Table(table).
		Where("name = ? AND id <> ?", name, currentID).
		Update("is_latest", false).Error; err != nil {
		return err
	}
	return tx.Table(table).
		Where("id = ?", currentID).
		Update("is_latest", true).Error
}

// ensureBaseFields stamps PublishedAt/Status defaults before insert.
func ensureBaseFields(status *models.RegistryStatus, publishedAt *time.Time) {
	if *status == "" {
		*status = models.RegistryStatusActive
	}
	if publishedAt.IsZero() {
		*publishedAt = time.Now()
	}
}

// ctxWrap is a tiny helper to keep every method concise.
func ctxWrap(ctx context.Context, db *gorm.DB) *gorm.DB {
	if ctx == nil {
		return db
	}
	return db.WithContext(ctx)
}
