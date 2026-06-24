package webhooks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/webhooks/adapters"
)

// ErrNotFound / ErrConflict mirror the pattern used by other services.
var (
	ErrNotFound = errors.New("webhook: not found")
	ErrConflict = errors.New("webhook: conflict")
)

// Service wraps CRUD for Webhook rows plus convenience helpers.
// The Dispatcher is a separate struct so tests can wire one without the other.
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService constructs a CRUD service.
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{db: db, logger: logger}
}

// UpsertInput is what the admin handler hands to Create/Update.
type UpsertInput struct {
	Name            string
	URL             string
	Secret          string   // empty on create → generated; on update → leave existing
	Events          []string // empty = subscribe to all
	Enabled         *bool    // pointer to distinguish "leave alone" from "set false"
	TeamID          *uuid.UUID
	CreatedByUserID *uuid.UUID

	// DestinationType picks the adapter that formats the event payload.
	// Empty defaults to "generic" so unspecified callers behave like the
	// pre-adapter dispatcher.
	DestinationType models.WebhookDestinationType
	// Config carries vendor-specific credentials and options. Validated
	// against the chosen adapter's schema before persisting.
	Config map[string]any
}

// List returns all webhooks, newest-first.
func (s *Service) List(ctx context.Context) ([]models.Webhook, error) {
	var rows []models.Webhook
	if err := s.db.WithContext(ctx).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Get loads a single webhook by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
	var row models.Webhook
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// Create inserts a new webhook. If Secret is empty we generate a
// 32-byte hex secret; operators are responsible for writing it down —
// there's no view-once-then-mask flow in v1.
func (s *Service) Create(ctx context.Context, in *UpsertInput) (*models.Webhook, error) {
	if err := validate(in, true); err != nil {
		return nil, err
	}
	secret := in.Secret
	if secret == "" {
		secret = randomHex(32)
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	dest := in.DestinationType
	if dest == "" {
		dest = models.WebhookDestGeneric
	}
	row := &models.Webhook{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		Name:            in.Name,
		URL:             in.URL,
		DestinationType: dest,
		Secret:          secret,
		Events:          pq.StringArray(in.Events),
		Enabled:         enabled,
		TeamID:          in.TeamID,
		CreatedByUserID: in.CreatedByUserID,
	}
	if err := row.SetConfig(in.Config); err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// Update mutates name, URL, events, enabled, team, destination, and
// config. Secret is immutable via this path (delete + re-create to
// rotate). Config is replaced wholesale on update — partial merges
// would force the UI to round-trip masked values, which leaks them.
// Empty Config preserves the existing value so a no-credential edit
// (e.g. renaming a Splunk webhook) doesn't wipe its HEC token.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in *UpsertInput) (*models.Webhook, error) {
	if err := validate(in, false); err != nil {
		return nil, err
	}
	var row models.Webhook
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	row.Name = in.Name
	row.URL = in.URL
	row.Events = pq.StringArray(in.Events)
	row.TeamID = in.TeamID
	if in.Enabled != nil {
		row.Enabled = *in.Enabled
	}
	if in.DestinationType != "" {
		row.DestinationType = in.DestinationType
	}
	if len(in.Config) > 0 {
		// Pull existing config so we can merge: any unspecified key
		// keeps its prior value, any specified key overwrites. This
		// lets the UI ship just the changed fields instead of asking
		// the user to retype every credential on every edit.
		merged := row.ConfigMap()
		for k, v := range in.Config {
			// A "***" sentinel from the redacted ConfigSummary means
			// "no change" — drop it so we keep the persisted value.
			if s, ok := v.(string); ok && s == "***" {
				continue
			}
			merged[k] = v
		}
		if err := row.SetConfig(merged); err != nil {
			return nil, fmt.Errorf("encode config: %w", err)
		}
	}
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// Delete removes a webhook row. No soft-delete — hooks are operational
// config, not historical evidence.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	res := s.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Webhook{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// --- internal ------------------------------------------------------------

func validate(in *UpsertInput, isCreate bool) error {
	if in == nil {
		return errors.New("input is nil")
	}
	if in.Name == "" {
		return errors.New("name is required")
	}
	if in.URL == "" {
		return errors.New("url is required")
	}
	// Minimal URL sanity — don't try to do full parsing; trust operators,
	// let bad URLs surface as delivery errors.
	if !(startsWith(in.URL, "http://") || startsWith(in.URL, "https://")) {
		return errors.New("url must start with http:// or https://")
	}
	if in.DestinationType != "" {
		// Defer config validation to the adapter so each destination owns
		// its required-fields list. On create we always check; on update
		// we only check when the operator actually supplied config (since
		// the merge logic preserves existing credentials otherwise).
		needsCheck := isCreate || len(in.Config) > 0
		if needsCheck {
			if err := adapters.For(in.DestinationType).ValidateConfig(in.Config); err != nil {
				return err
			}
		}
	}
	return nil
}

func startsWith(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// fall back to a deterministic but still unique value; this only
		// happens on a broken crypto source, which would be noisy elsewhere
		return fmt.Sprintf("insecure-%s", uuid.New().String())
	}
	return hex.EncodeToString(b)
}
