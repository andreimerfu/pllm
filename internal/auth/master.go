package auth

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/models"
)

// MasterKeyService handles master key operations
type MasterKeyService struct {
	db          *gorm.DB
	masterKey   string
	jwtSecret   []byte
	jwtIssuer   string
	tokenExpiry time.Duration
}

type MasterKeyConfig struct {
	DB          *gorm.DB
	MasterKey   string
	JWTSecret   []byte
	JWTIssuer   string
	TokenExpiry time.Duration
}

// MasterKeyContext represents a master key authentication context
type MasterKeyContext struct {
	KeyType     models.KeyType `json:"key_type"`
	IsActive    bool           `json:"is_active"`
	Scopes      []string       `json:"scopes"`
	ValidatedAt time.Time      `json:"validated_at"`
}

func NewMasterKeyService(config *MasterKeyConfig) *MasterKeyService {
	// Set defaults if not provided
	if config.TokenExpiry == 0 {
		config.TokenExpiry = 24 * time.Hour
	}
	if config.JWTIssuer == "" {
		config.JWTIssuer = "pllm"
	}
	if len(config.JWTSecret) == 0 {
		config.JWTSecret = []byte("default-secret-change-in-production")
	}

	return &MasterKeyService{
		db:          config.DB,
		masterKey:   config.MasterKey,
		jwtSecret:   config.JWTSecret,
		jwtIssuer:   config.JWTIssuer,
		tokenExpiry: config.TokenExpiry,
	}
}

// ValidateMasterKey validates the master key and returns a context
func (m *MasterKeyService) ValidateMasterKey(ctx context.Context, key string) (*MasterKeyContext, error) {
	if m.masterKey == "" || key != m.masterKey {
		return nil, ErrMasterKeyRequired
	}

	// Create audit entry for master key usage
	auditEntry := &models.Audit{
		EventType:    models.AuditEventAuth,
		EventAction:  "master_key_auth",
		EventResult:  models.AuditResultSuccess,
		AuthMethod:   "master_key",
		AuthProvider: "internal",
		Message:      "Master key authentication successful",
		Timestamp:    time.Now(),
	}

	// Save audit entry
	if err := m.db.Create(auditEntry).Error; err != nil {
		// Log error but don't fail authentication
	}

	return &MasterKeyContext{
		KeyType:     models.KeyTypeMaster,
		IsActive:    true,
		Scopes:      []string{"*"}, // Full access
		ValidatedAt: time.Now(),
	}, nil
}

// GenerateAdminToken generates a JWT token for master key authentication
func (m *MasterKeyService) GenerateAdminToken(masterCtx *MasterKeyContext) (string, error) {
	// Create JWT claims for master key admin
	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.jwtIssuer,
			Subject:   "master-key",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.tokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		UserID:   uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Special master key user ID
		Email:    "admin@master-key",
		Username: "master-admin",
		Role:     string(models.RoleAdmin),
		Groups:   []string{"admin", "master"},
	}

	// Create and sign the token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.jwtSecret)
}

// IsConfigured returns whether master key is configured
func (m *MasterKeyService) IsConfigured() bool {
	return m.masterKey != ""
}

// HasMasterKeyAccess checks if the current context has master key access
func HasMasterKeyAccess(ctx context.Context) bool {
	if masterCtx, ok := ctx.Value("master_key_context").(*MasterKeyContext); ok {
		return masterCtx.IsActive && masterCtx.KeyType == models.KeyTypeMaster
	}
	return false
}

// CreateBootstrapAdmin creates a bootstrap admin user using master key
func (m *MasterKeyService) CreateBootstrapAdmin(ctx context.Context, req *models.UserProvisionRequest) (*models.User, error) {
	// Verify this is called with master key
	if !HasMasterKeyAccess(ctx) {
		return nil, ErrMasterKeyRequired
	}

	// Check if admin already exists
	var existingUser models.User
	err := m.db.Where("email = ? OR role = ?", req.Email, models.RoleAdmin).First(&existingUser).Error
	if err == nil {
		return &existingUser, nil // Admin already exists
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Create bootstrap admin with special Dex ID
	admin := &models.User{
		Email:         req.Email,
		Username:      req.Username,
		DexID:         "bootstrap-admin-" + uuid.New().String(), // Special Dex ID for bootstrap
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		Role:          models.RoleAdmin,
		IsActive:      true,
		EmailVerified: true,
	}

	// Mark as provisioned from master key
	admin.MarkAsProvisioned("master_key", admin.DexID, []string{"admin"})

	if err := m.db.Create(admin).Error; err != nil {
		return nil, err
	}

	// Create audit entry
	auditEntry := &models.Audit{
		EventType:    models.AuditEventUserCreate,
		EventAction:  "bootstrap_admin_create",
		EventResult:  models.AuditResultSuccess,
		UserID:       &admin.ID,
		AuthMethod:   "master_key",
		AuthProvider: "internal",
		ResourceType: "user",
		ResourceID:   &admin.ID,
		Message:      "Bootstrap admin user created via master key",
		Timestamp:    time.Now(),
	}

	m.db.Create(auditEntry)

	return admin, nil
}
