package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/mcp/gateway"
)

// MCPBridge implements BackendRegistrar by writing to the MCPServer table
// and telling the live gateway Manager to add/remove backends.
// Persistence + live registration in one object so auto-wired backends
// survive restarts.
type MCPBridge struct {
	db      *gorm.DB
	manager *gateway.Manager
	logger  *zap.Logger
}

// NewMCPBridge constructs a bridge. If manager is nil, only DB state is
// touched and live routing kicks in on next manager.Load().
func NewMCPBridge(db *gorm.DB, manager *gateway.Manager, logger *zap.Logger) *MCPBridge {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MCPBridge{db: db, manager: manager, logger: logger}
}

// Register implements BackendRegistrar. Idempotent on slug: if a row
// exists, update its endpoint; otherwise create it. Returns the row ID.
func (b *MCPBridge) Register(ctx context.Context, slug, name, description, endpoint string) (uuid.UUID, error) {
	if b.db == nil {
		return uuid.Nil, errors.New("mcp bridge: no DB")
	}
	if slug == "" || endpoint == "" {
		return uuid.Nil, fmt.Errorf("mcp bridge: slug and endpoint required")
	}
	meta, _ := json.Marshal(map[string]any{
		"managed_by":   "pllm-deployment",
		"auto_created": true,
	})

	var row models.MCPServer
	err := b.db.WithContext(ctx).Where("slug = ?", slug).First(&row).Error
	switch {
	case err == nil:
		row.Name = nonEmpty(name, row.Name)
		row.Description = nonEmpty(description, row.Description)
		row.Transport = models.MCPTransportHTTP
		row.Endpoint = endpoint
		row.Enabled = true
		row.Metadata = datatypes.JSON(meta)
		if err := b.db.WithContext(ctx).Save(&row).Error; err != nil {
			return uuid.Nil, err
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		row = models.MCPServer{
			BaseModel:   models.BaseModel{ID: uuid.New()},
			Name:        nonEmpty(name, slug),
			Slug:        slug,
			Description: description,
			Enabled:     true,
			Transport:   models.MCPTransportHTTP,
			Endpoint:    endpoint,
			Metadata:    datatypes.JSON(meta),
		}
		if err := b.db.WithContext(ctx).Create(&row).Error; err != nil {
			return uuid.Nil, err
		}
	default:
		return uuid.Nil, err
	}

	// Live-register so routes are immediately available — no restart.
	if b.manager != nil {
		// Remove any previous live backend under the same ID, then add fresh.
		b.manager.RemoveBackend(row.ID)
		info, ierr := gateway.RowToInfo(&row)
		if ierr != nil {
			return row.ID, fmt.Errorf("mcp bridge: row→info: %w", ierr)
		}
		if _, aerr := b.manager.AddBackend(ctx, info); aerr != nil {
			b.logger.Warn("mcp bridge: live register", zap.Error(aerr))
		}
	}
	return row.ID, nil
}

// Unregister implements BackendRegistrar. Removes the DB row and pulls
// the backend out of the live manager.
func (b *MCPBridge) Unregister(ctx context.Context, id uuid.UUID) error {
	if b.manager != nil {
		b.manager.RemoveBackend(id)
	}
	// Tool cache rows reference this ID via FK.
	if err := b.db.WithContext(ctx).
		Where("mcp_server_id = ?", id).
		Delete(&models.MCPServerTool{}).Error; err != nil {
		return err
	}
	if err := b.db.WithContext(ctx).Delete(&models.MCPServer{}, "id = ?", id).Error; err != nil {
		return err
	}
	return nil
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
