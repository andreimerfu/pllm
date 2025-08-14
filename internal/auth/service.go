package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
	
	"github.com/amerfu/pllm/internal/models"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserInactive      = errors.New("user is inactive")
	ErrInvalidToken      = errors.New("invalid token")
	ErrTokenExpired      = errors.New("token expired")
	ErrInvalidAPIKey     = errors.New("invalid API key")
	ErrInvalidKey        = errors.New("invalid key")
	ErrMasterKeyRequired = errors.New("master key required")
)

type AuthService struct {
	db           *gorm.DB
	dexProvider  *DexAuthProvider
	jwtSecret    []byte
	jwtIssuer    string
	tokenExpiry  time.Duration
	masterKeyService *MasterKeyService
}

type AuthConfig struct {
	DB               *gorm.DB
	DexConfig        *DexConfig
	JWTSecret        string
	JWTIssuer        string
	TokenExpiry      time.Duration
	MasterKeyService *MasterKeyService
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
		db:               config.DB,
		dexProvider:      dexProvider,
		jwtSecret:        []byte(config.JWTSecret),
		jwtIssuer:        config.JWTIssuer,
		tokenExpiry:      config.TokenExpiry,
		masterKeyService: config.MasterKeyService,
	}, nil
}


// LoginWithDex handles Dex OAuth authentication and auto-provision users
func (s *AuthService) LoginWithDex(ctx context.Context, code string) (*LoginResponse, error) {
	if s.dexProvider == nil {
		return nil, errors.New("Dex not configured")
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
			
			// Mark as provisioned from Dex
			user.MarkAsProvisioned("dex", claims.Subject, claims.Groups)
			
			if err := s.db.Create(&user).Error; err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
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
		if err := s.db.Save(&user).Error; err != nil {
			// Log error but don't fail login
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

	// Update usage statistics
	now := time.Now()
	dbKey.LastUsedAt = &now
	dbKey.UsageCount++
	s.db.Save(&dbKey)

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
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims, nil
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

func (s *AuthService) generateRefreshToken(user *models.User) (string, error) {
	claims := &jwt.RegisteredClaims{
		Issuer:    s.jwtIssuer,
		Subject:   user.DexID, // Use Dex ID as subject
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
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