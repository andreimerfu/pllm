package a2a

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// ErrAgentNotFound is returned when no running AgentDeployment matches
// the requested slug.
var ErrAgentNotFound = errors.New("agent not found")

// Manager resolves an A2A slug to an AgentDeployment row. The slug is
// the AgentDeployment.WorkloadName (already unique + DNS-safe). Keeping
// this trivial for Phase 2; Phase 2.5 can add friendlier aliases or
// domain-based routing.
type Manager struct {
	db *gorm.DB
}

func NewManager(db *gorm.DB) *Manager {
	return &Manager{db: db}
}

// Resolve looks up the agent by slug. Returns ErrAgentNotFound if no row.
func (m *Manager) Resolve(ctx context.Context, slug string) (*models.AgentDeployment, error) {
	var row models.AgentDeployment
	err := m.db.WithContext(ctx).
		Where("workload_name = ?", slug).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("a2a manager: lookup: %w", err)
	}
	return &row, nil
}
