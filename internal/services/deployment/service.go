package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// BackendRegistrar decouples the deployment service from the MCP gateway
// manager, so wiring a successful deploy into a live gateway backend is
// done by the caller (router) without creating an import cycle.
type BackendRegistrar interface {
	// Register creates (or replaces) a gateway backend pointing at the
	// given HTTP endpoint, and returns its ID. Must be idempotent on slug.
	Register(ctx context.Context, slug, name, description, endpoint string) (uuid.UUID, error)
	// Unregister removes the backend by ID (best-effort; missing IDs are no-ops).
	Unregister(ctx context.Context, id uuid.UUID) error
}

// Service manages deployment lifecycles in the DB + delegates to adapters.
type Service struct {
	db       *gorm.DB
	logger   *zap.Logger
	adapter  Adapter
	registry BackendRegistrar // optional; nil means "don't auto-wire MCP gateway"
	ns       string           // default namespace if request omits one
}

// NewService constructs the deployment service.
func NewService(db *gorm.DB, logger *zap.Logger, adapter Adapter, registry BackendRegistrar, defaultNS string) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{db: db, logger: logger, adapter: adapter, registry: registry, ns: defaultNS}
}

// DeployFromServer takes a RegistryServer row and materializes a running
// workload. Caller has already performed permission checks.
//
// Resolution rules for "what runs":
//   - packages[*].registry_type == "oci" → ImageSpec
//   - packages[*].registry_type == "npm" → NPXSpec
//   - packages[*].registry_type == "pypi" → UVXSpec
//   - otherwise: error
//
// We pick the first matching package; users with multiple ecosystems are
// expected to pick one in the UI.
func (s *Service) DeployFromServer(ctx context.Context, server *models.RegistryServer, namespace string) (*models.Deployment, error) {
	if server == nil {
		return nil, errors.New("server is nil")
	}
	if namespace == "" {
		namespace = s.ns
	}
	req, err := buildRequestFromServer(server, namespace)
	if err != nil {
		return nil, err
	}

	// Find or create the Deployment row first so we have an ID and can
	// atomically track state across adapter calls.
	row, err := s.findOrCreateRow(ctx, server, req)
	if err != nil {
		return nil, err
	}
	s.updateStatus(ctx, row.ID, models.DeploymentStatusDeploying, "")

	res, err := s.adapter.Deploy(ctx, req)
	if err != nil {
		s.updateStatus(ctx, row.ID, models.DeploymentStatusFailed, err.Error())
		return nil, err
	}

	// Persist adapter state + endpoint.
	updates := map[string]any{
		"endpoint":         res.Endpoint,
		"adapter_state":    datatypes.JSON(res.AdapterState),
		"last_applied_at":  time.Now(),
		"status":           models.DeploymentStatusRunning,
		"status_reason":    "",
	}

	// Auto-register as a gateway backend, if a registrar is wired.
	if s.registry != nil {
		slug := gatewaySlug(req.WorkloadName)
		backendID, regErr := s.registry.Register(ctx, slug, server.Title, server.Description, res.Endpoint)
		if regErr != nil {
			s.logger.Warn("deployment: register gateway backend",
				zap.String("slug", slug), zap.Error(regErr))
		} else {
			updates["gateway_backend_id"] = backendID
		}
	}

	if err := s.db.WithContext(ctx).Model(&models.Deployment{}).
		Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("persist deployment: %w", err)
	}
	// Re-read for the response.
	if err := s.db.WithContext(ctx).First(row, "id = ?", row.ID).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// Undeploy tears down the workload and deletes the DB row.
func (s *Service) Undeploy(ctx context.Context, id uuid.UUID) error {
	var row models.Deployment
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return err
	}
	s.updateStatus(ctx, id, models.DeploymentStatusTerminating, "")
	if err := s.adapter.Undeploy(ctx, &row); err != nil {
		s.updateStatus(ctx, id, models.DeploymentStatusFailed, err.Error())
		return err
	}
	// Unregister the gateway backend first, then delete the row.
	if s.registry != nil && row.GatewayBackendID != nil {
		if err := s.registry.Unregister(ctx, *row.GatewayBackendID); err != nil {
			s.logger.Warn("deployment: unregister backend", zap.Error(err))
		}
	}
	return s.db.WithContext(ctx).Delete(&row).Error
}

// List returns all deployments (newest first).
func (s *Service) List(ctx context.Context) ([]models.Deployment, error) {
	var out []models.Deployment
	err := s.db.WithContext(ctx).Order("created_at desc").Find(&out).Error
	return out, err
}

// Get returns a single deployment by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.Deployment, error) {
	var row models.Deployment
	err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// RefreshStatus polls the adapter for live status and persists it.
func (s *Service) RefreshStatus(ctx context.Context, id uuid.UUID) (*models.Deployment, error) {
	row, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	rep, err := s.adapter.Status(ctx, row)
	if err != nil {
		return nil, err
	}
	row.Status = rep.Status
	row.StatusReason = rep.Reason
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// --- internals ----------------------------------------------------------

func (s *Service) updateStatus(ctx context.Context, id uuid.UUID, status models.DeploymentStatus, reason string) {
	if err := s.db.WithContext(ctx).Model(&models.Deployment{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "status_reason": reason}).Error; err != nil {
		s.logger.Warn("deployment: status update", zap.Error(err))
	}
}

func (s *Service) findOrCreateRow(ctx context.Context, server *models.RegistryServer, req *Request) (*models.Deployment, error) {
	var existing models.Deployment
	err := s.db.WithContext(ctx).
		Where("registry_server_id = ? AND namespace = ?", server.ID, req.Namespace).
		First(&existing).Error
	if err == nil {
		// Update cached fields on re-deploy.
		existing.ResourceName = server.Name
		existing.ResourceVersion = server.Version
		existing.WorkloadName = req.WorkloadName
		existing.Status = models.DeploymentStatusPending
		existing.StatusReason = ""
		if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	row := &models.Deployment{
		BaseModel:        models.BaseModel{ID: uuid.New()},
		RegistryServerID: server.ID,
		ResourceName:     server.Name,
		ResourceVersion:  server.Version,
		Platform:         s.adapter.Platform(),
		Namespace:        req.Namespace,
		WorkloadName:     req.WorkloadName,
		Status:           models.DeploymentStatusPending,
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// buildRequestFromServer translates the registry row + namespace into an
// adapter Request. Picks the first scannable/runnable package.
func buildRequestFromServer(server *models.RegistryServer, namespace string) (*Request, error) {
	req := &Request{
		Namespace:       namespace,
		WorkloadName:    workloadNameFor(server),
		DisplayName:     displayTitle(server),
		ResourceName:    server.Name,
		ResourceVersion: server.Version,
	}
	pkgs, err := decodePackages(server.Packages)
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		switch strings.ToLower(p.RegistryType) {
		case "oci", "docker":
			req.Image = &ImageSpec{Reference: fmt.Sprintf("%s:%s", p.Identifier, p.Version)}
			return req, nil
		case "npm":
			req.NPXPackage = &NPXSpec{Package: p.Identifier, Version: p.Version}
			return req, nil
		case "pypi":
			req.UVXPackage = &UVXSpec{Package: p.Identifier, Version: p.Version}
			return req, nil
		}
	}
	return nil, errors.New("no deployable package found on server (need npm, pypi, or oci)")
}

// serverPackage is the wire shape of one entry in RegistryServer.Packages.
type serverPackage struct {
	RegistryType string `json:"registry_type"`
	Identifier   string `json:"identifier"`
	Version      string `json:"version"`
}

func decodePackages(raw []byte) ([]serverPackage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []serverPackage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("packages: %w", err)
	}
	return out, nil
}

// workloadNameFor derives a DNS-1123 safe name from the server.
// "io.modelcontextprotocol/server-everything" + "0.6.2" → "server-everything-0-6-2"
// Collisions are avoided by including the package basename.
var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func workloadNameFor(server *models.RegistryServer) string {
	base := server.Name
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	s := strings.ToLower(base + "-" + server.Version)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}

func displayTitle(server *models.RegistryServer) string {
	if server.Title != "" {
		return server.Title
	}
	return server.Name
}

// gatewaySlug derives a stable slug for the MCPServer gateway backend row.
// Mirrors workloadNameFor so the two stay aligned and readable.
func gatewaySlug(workloadName string) string {
	return "reg-" + workloadName
}
