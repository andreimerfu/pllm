package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/models"
)

// Forward declarations to avoid circular imports
type TeamService interface {
	AddUserToDefaultTeam(ctx context.Context, userID uuid.UUID, role models.TeamRole) (*models.TeamMember, error)
}

type KeyService interface {
	CreateDefaultKeyForUser(ctx context.Context, userID uuid.UUID, teamID uuid.UUID) (*models.Key, error)
}

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserInactive       = errors.New("user is inactive")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidAPIKey      = errors.New("invalid API key")
	ErrInvalidKey         = errors.New("invalid key")
	ErrMasterKeyRequired  = errors.New("master key required")
)

type AuthService struct {
	db                *gorm.DB
	dexProvider       *DexAuthProvider
	jwtSecret         []byte
	jwtIssuer         string
	tokenExpiry       time.Duration
	masterKeyService  *MasterKeyService
	teamService       TeamService
	keyService        KeyService
	permissionService *PermissionService
}

type AuthConfig struct {
	DB               *gorm.DB
	DexConfig        *DexConfig
	JWTSecret        string
	JWTIssuer        string
	TokenExpiry      time.Duration
	MasterKeyService *MasterKeyService
	TeamService      TeamService
	KeyService       KeyService
}

type LoginResponse struct {
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         UserInfo  `json:"user"`
}

type UserInfo struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Role      string    `json:"role"`
	Groups    []string  `json:"groups"`
}

type TokenClaims struct {
	jwt.RegisteredClaims
	UserID   uuid.UUID `json:"user_id"`
	Email    string    `json:"email"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	Groups   []string  `json:"groups"`
}

func NewAuthService(config *AuthConfig) (*AuthService, error) {
	var dexProvider *DexAuthProvider
	var err error

	if config.DexConfig != nil {
		dexProvider, err = NewDexAuthProvider(config.DexConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Dex provider: %w", err)
		}
	}

	if config.TokenExpiry == 0 {
		config.TokenExpiry = 24 * time.Hour
	}

	return &AuthService{
		db:                config.DB,
		dexProvider:       dexProvider,
		jwtSecret:         []byte(config.JWTSecret),
		jwtIssuer:         config.JWTIssuer,
		tokenExpiry:       config.TokenExpiry,
		masterKeyService:  config.MasterKeyService,
		teamService:       config.TeamService,
		keyService:        config.KeyService,
		permissionService: NewPermissionService(),
	}, nil
}

// LoginWithDex handles Dex OAuth authentication and auto-provision users
func (s *AuthService) LoginWithDex(ctx context.Context, code string) (*LoginResponse, error) {
	if s.dexProvider == nil {
		return nil, errors.New("dex not configured")
	}

	token, err := s.dexProvider.ExchangeCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("no id_token in response")
	}

	claims, err := s.dexProvider.VerifyIDToken(ctx, rawIDToken)
	if err != nil {
		return nil, err
	}

	// Use Dex subject ID to find or create user
	var user models.User
	err = s.db.Preload("Teams").Where("dex_id = ?", claims.Subject).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Auto-provision user from Dex claims
			username := claims.PreferredUsername
			if username == "" {
				username = claims.Email
			}

			// Determine role from groups
			role := models.RoleUser
			for _, group := range claims.Groups {
				switch strings.ToLower(group) {
				case "admin", "administrators":
					role = models.RoleAdmin
				case "manager", "managers":
					role = models.RoleManager
				}
			}

			user = models.User{
				Email:         claims.Email,
				Username:      username,
				DexID:         claims.Subject, // Use Dex subject as unique identifier
				FirstName:     claims.Name,
				EmailVerified: claims.EmailVerified,
				IsActive:      true,
				Role:          role,
			}

			// Extract the actual OAuth provider from Dex claims
			actualProvider := extractOAuthProvider(claims)
			
			// Debug log to help with troubleshooting
			fmt.Printf("DEBUG: Dex claims - Subject: %s, Email: %s, ConnectorID: %s, ExtractedProvider: %s\n", 
				claims.Subject, claims.Email, claims.ConnectorID, actualProvider)
			
			// Mark as provisioned from the actual provider
			user.MarkAsProvisioned(actualProvider, claims.Subject, claims.Groups)

			if err := s.db.Create(&user).Error; err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}

			// Auto-assign user to default team if team service is available
			fmt.Printf("DEBUG: TeamService available: %t\n", s.teamService != nil)
			if s.teamService != nil {
				// Map user role to team role
				teamRole := models.TeamRoleMember
				if user.Role == models.RoleAdmin {
					teamRole = models.TeamRoleAdmin
				}

				if teamMember, err := s.teamService.AddUserToDefaultTeam(ctx, user.ID, teamRole); err != nil {
					// Log error but don't fail user creation
					// User can be manually assigned to teams later
					fmt.Printf("ERROR: Failed to assign user %s to default team: %v\n", user.Email, err)
				} else {
					fmt.Printf("SUCCESS: User %s added to default team %s\n", user.Email, teamMember.TeamID)
					if s.keyService != nil && teamMember != nil {
						// Create default API key for the user
						if _, err := s.keyService.CreateDefaultKeyForUser(ctx, user.ID, teamMember.TeamID); err != nil {
							// Log error but don't fail user creation
							fmt.Printf("ERROR: Failed to create default key for user %s: %v\n", user.Email, err)
						} else {
							fmt.Printf("SUCCESS: Created default key for user %s\n", user.Email)
						}
					}
				}
			}

			// Create audit entry for user provisioning
			auditEntry := &models.Audit{
				EventType:    models.AuditEventUserProvision,
				EventAction:  "dex_provision",
				EventResult:  models.AuditResultSuccess,
				UserID:       &user.ID,
				AuthMethod:   "dex",
				AuthProvider: "dex",
				ResourceType: "user",
				ResourceID:   &user.ID,
				Message:      "User auto-provisioned from Dex",
				Timestamp:    time.Now(),
			}
			s.db.Create(auditEntry)

		} else {
			return nil, err
		}
	} else {
		// Update existing user with latest info from Dex
		user.FirstName = claims.Name
		user.EmailVerified = claims.EmailVerified
		user.UpdateExternalGroups(claims.Groups)
		
		// Update provider info if it has changed or was missing
		actualProvider := extractOAuthProvider(claims)
		
		// Debug log to help with troubleshooting (existing users)
		fmt.Printf("DEBUG: Existing user - Subject: %s, Email: %s, Current Provider: %s, ExtractedProvider: %s\n", 
			claims.Subject, claims.Email, user.ExternalProvider, actualProvider)
		
		if user.ExternalProvider != actualProvider {
			user.ExternalProvider = actualProvider
		}
		
		if err := s.db.Save(&user).Error; err != nil {
			// Log error but don't fail login
			log.Printf("Failed to update user from Dex claims: %v", err)
		}
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	s.db.Save(&user)

	// Create audit entry for login
	auditEntry := &models.Audit{
		EventType:    models.AuditEventLogin,
		EventAction:  "dex_login",
		EventResult:  models.AuditResultSuccess,
		UserID:       &user.ID,
		AuthMethod:   "dex",
		AuthProvider: "dex",
		Message:      "User logged in via Dex",
		Timestamp:    time.Now(),
	}
	s.db.Create(auditEntry)

	jwtToken, err := s.generateJWT(&user)
	if err != nil {
		return nil, err
	}

	// Get team names from Teams relationship
	groups := make([]string, 0)
	if len(user.Teams) > 0 {
		for _, tm := range user.Teams {
			groups = append(groups, string(tm.Role))
		}
	}

	return &LoginResponse{
		Token:        jwtToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.Expiry,
		User: UserInfo{
			ID:        user.ID,
			Email:     user.Email,
			Username:  user.Username,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Role:      string(user.Role),
			Groups:    groups,
		},
	}, nil
}

// ValidateKey validates any type of key (API, Virtual, Master)
func (s *AuthService) ValidateKey(ctx context.Context, key string) (*models.Key, error) {
	// Check master key first
	if s.masterKeyService != nil && s.masterKeyService.IsConfigured() {
		if masterCtx, err := s.masterKeyService.ValidateMasterKey(ctx, key); err == nil {
			return &models.Key{
				Name:     "Master Key",
				Type:     models.KeyTypeMaster,
				IsActive: true,
				Scopes:   masterCtx.Scopes,
			}, nil
		}
	}

	// Hash the key for lookup
	keyHash := models.HashKey(key)

	var dbKey models.Key
	err := s.db.Preload("User").Preload("Team").Where("key_hash = ? AND is_active = ?", keyHash, true).First(&dbKey).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidAPIKey
		}
		return nil, err
	}

	if !dbKey.IsValid() {
		return nil, ErrInvalidAPIKey
	}

	// Update usage statistics atomically to avoid race conditions
	now := time.Now()
	dbKey.LastUsedAt = &now
	// Use atomic increment for usage_count to handle concurrent access
	s.db.Model(&dbKey).Updates(map[string]interface{}{
		"last_used_at": now,
		"usage_count": gorm.Expr("usage_count + 1"),
	})

	// Create audit entry for key usage
	auditEntry := &models.Audit{
		EventType:    models.AuditEventKeyUsage,
		EventAction:  "key_validation",
		EventResult:  models.AuditResultSuccess,
		UserID:       dbKey.UserID,
		TeamID:       dbKey.TeamID,
		KeyID:        &dbKey.ID,
		AuthMethod:   "api_key",
		ResourceType: "key",
		ResourceID:   &dbKey.ID,
		Message:      "API key used successfully",
		Timestamp:    time.Now(),
	}
	s.db.Create(auditEntry)

	return &dbKey, nil
}

// ValidateAPIKey for backward compatibility
func (s *AuthService) ValidateAPIKey(ctx context.Context, key string) (*models.Key, error) {
	return s.ValidateKey(ctx, key)
}

// ValidateMasterKey validates master key specifically
func (s *AuthService) ValidateMasterKey(ctx context.Context, key string) (*MasterKeyContext, error) {
	if s.masterKeyService == nil {
		return nil, ErrMasterKeyRequired
	}
	return s.masterKeyService.ValidateMasterKey(ctx, key)
}

func (s *AuthService) ValidateToken(tokenString string) (*TokenClaims, error) {
	// Try Dex token validation first if Dex is configured
	if s.dexProvider != nil {
		ctx := context.Background()
		authClaims, err := s.dexProvider.VerifyIDToken(ctx, tokenString)
		if err == nil {
			// Successfully validated as Dex token
			// Find user by Dex subject ID to get user details
			user, err := s.GetUserByDexID(ctx, authClaims.Subject)
			if err != nil {
				return nil, fmt.Errorf("user not found for dex id %s: %w", authClaims.Subject, err)
			}

			// Get team names from Teams relationship for groups
			groups := make([]string, 0)
			if len(user.Teams) > 0 {
				for _, tm := range user.Teams {
					groups = append(groups, string(tm.Role))
				}
			}

			// Convert Dex claims to TokenClaims
			tokenClaims := &TokenClaims{
				RegisteredClaims: authClaims.RegisteredClaims,
				UserID:           user.ID,
				Email:            authClaims.Email,
				Username:         user.Username,
				Role:             string(user.Role),
				Groups:           groups,
			}

			return tokenClaims, nil
		}
		// Log Dex validation failure for debugging
		fmt.Printf("Debug: Dex token validation failed: %v\n", err)
	}

	// Only try HMAC validation if this looks like an internal token (shorter, different format)
	// Dex tokens are much longer and use RS256, so they'll always fail HMAC validation
	if len(tokenString) < 500 {
		// Fall back to HMAC validation for internal JWT tokens
		token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return s.jwtSecret, nil
		})

		if err == nil {
			if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
				return claims, nil
			}
		}
	}

	return nil, ErrInvalidToken
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*LoginResponse, error) {
	if s.dexProvider == nil {
		return nil, errors.New("refresh not supported without Dex")
	}

	token, err := s.dexProvider.RefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("no id_token in refresh response")
	}

	claims, err := s.dexProvider.VerifyIDToken(ctx, rawIDToken)
	if err != nil {
		return nil, err
	}

	// Find user by Dex subject ID
	var user models.User
	err = s.db.Preload("Teams").Where("dex_id = ?", claims.Subject).First(&user).Error
	if err != nil {
		return nil, ErrUserNotFound
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// Update user info from refreshed claims
	user.UpdateExternalGroups(claims.Groups)
	s.db.Save(&user)

	jwtToken, err := s.generateJWT(&user)
	if err != nil {
		return nil, err
	}

	// Get team names from Teams relationship
	groups := make([]string, 0)
	if len(user.Teams) > 0 {
		for _, tm := range user.Teams {
			groups = append(groups, string(tm.Role))
		}
	}

	return &LoginResponse{
		Token:        jwtToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.Expiry,
		User: UserInfo{
			ID:        user.ID,
			Email:     user.Email,
			Username:  user.Username,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Role:      string(user.Role),
			Groups:    groups,
		},
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	// TODO: Implement token blacklist
	return nil
}

func (s *AuthService) generateJWT(user *models.User) (string, error) {
	// Get team names from Teams relationship
	groups := make([]string, 0)
	if len(user.Teams) > 0 {
		for _, tm := range user.Teams {
			groups = append(groups, string(tm.Role))
		}
	}

	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.jwtIssuer,
			Subject:   user.DexID, // Use Dex ID as subject
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		UserID:   user.ID,
		Email:    user.Email,
		Username: user.Username,
		Role:     string(user.Role),
		Groups:   groups,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// GetUserByDexID retrieves a user by their Dex subject ID
func (s *AuthService) GetUserByDexID(ctx context.Context, dexID string) (*models.User, error) {
	var user models.User
	err := s.db.Preload("Teams").Where("dex_id = ?", dexID).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// ProvisionUser creates a new user from Dex authentication
func (s *AuthService) ProvisionUser(ctx context.Context, req *models.UserProvisionRequest) (*models.User, error) {
	// Check if user already exists
	var existingUser models.User
	err := s.db.Where("dex_id = ? OR email = ?", req.DexID, req.Email).First(&existingUser).Error
	if err == nil {
		return &existingUser, nil // User already exists
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	user := &models.User{
		Email:         req.Email,
		Username:      req.Username,
		DexID:         req.DexID,
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		Role:          req.Role,
		IsActive:      true,
		EmailVerified: true,
	}

	user.MarkAsProvisioned(req.ExternalProvider, req.DexID, req.ExternalGroups)

	if err := s.db.Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

// RecordUsage records LLM usage for tracking and budget calculations
func (s *AuthService) RecordUsage(ctx context.Context, usage *models.Usage) error {
	return s.db.WithContext(ctx).Create(usage).Error
}

// CheckBudgetCached checks if an entity has budget available (generic implementation)
func (s *AuthService) CheckBudgetCached(ctx context.Context, entityType, entityID string, estimatedCost float64) (bool, error) {
	// This is a simplified implementation - in production you'd have proper budget tracking
	// For now, always allow requests (prefer availability over strict enforcement)
	return true, nil
}

// GetUserPermissions gets all permissions for a user including role and team permissions
func (s *AuthService) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var user models.User
	err := s.db.Preload("Teams").First(&user, "id = ?", userID).Error
	if err != nil {
		return nil, err
	}

	permissions := s.permissionService.GetUserPermissions(&user)

	// Convert to strings
	var permStrings []string
	for _, p := range permissions {
		permStrings = append(permStrings, string(p))
	}

	return permStrings, nil
}

// GetUserActiveKey retrieves the first active API key for a user
func (s *AuthService) GetUserActiveKey(ctx context.Context, userID uuid.UUID) (*models.Key, error) {
	var key models.Key
	err := s.db.WithContext(ctx).Where("user_id = ? AND is_active = ?", userID, true).First(&key).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no active key found for user %s", userID.String())
		}
		return nil, err
	}
	return &key, nil
}

// GetKeyUser retrieves the user associated with an API key
func (s *AuthService) GetKeyUser(ctx context.Context, keyID uuid.UUID) (*models.User, error) {
	var key models.Key
	err := s.db.WithContext(ctx).Preload("User").Where("id = ?", keyID).First(&key).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("key not found: %s", keyID.String())
		}
		return nil, err
	}

	if key.UserID == nil {
		return nil, fmt.Errorf("key %s has no associated user", keyID.String())
	}

	var user models.User
	err = s.db.WithContext(ctx).Where("id = ?", *key.UserID).First(&user).Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// extractOAuthProvider determines the actual OAuth provider from Dex claims
func extractOAuthProvider(claims *AuthClaims) string {
	// If connector_id is available, use it directly
	if claims.ConnectorID != "" {
		return claims.ConnectorID
	}
	
	// Dex includes connector information in the subject ID
	// The format is typically: CiQwMzHexample (base64) or connectorID-userID
	subject := claims.Subject
	
	// Method 1: Check if subject contains connector prefix patterns
	if strings.Contains(subject, "github") || strings.HasPrefix(subject, "github-") {
		return "github"
	}
	if strings.Contains(subject, "google") || strings.HasPrefix(subject, "google-") {
		return "google"
	}
	if strings.Contains(subject, "microsoft") || strings.HasPrefix(subject, "microsoft-") {
		return "microsoft"
	}
	if strings.Contains(subject, "gitlab") || strings.HasPrefix(subject, "gitlab-") {
		return "gitlab"
	}
	
	// Method 2: Analyze the issuer and check for local vs external patterns
	email := claims.Email
	
	// Check if it's a local/static password user from Dex config
	if strings.Contains(email, "@pllm.local") {
		return "local"
	}
	
	// Method 3: Use email domain heuristics as fallback
	if email != "" {
		domain := strings.Split(email, "@")
		if len(domain) == 2 {
			switch strings.ToLower(domain[1]) {
			case "gmail.com", "googlemail.com":
				return "google"
			case "outlook.com", "hotmail.com", "live.com", "msn.com":
				return "microsoft"
			}
		}
	}
	
	// Method 4: Check connector data if available
	if claims.ConnectorData != nil {
		if connectorType, exists := claims.ConnectorData["connector_type"]; exists {
			if connectorStr, ok := connectorType.(string); ok {
				return connectorStr
			}
		}
		if connectorID, exists := claims.ConnectorData["connector_id"]; exists {
			if connectorStr, ok := connectorID.(string); ok {
				return connectorStr
			}
		}
	}
	
	// Default fallback - this shouldn't happen with proper Dex setup
	return "dex"
}
