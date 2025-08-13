package auth

import (
	"context"
	"errors"
	"fmt"
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
)

type AuthService struct {
	db           *gorm.DB
	dexProvider  *DexAuthProvider
	jwtSecret    []byte
	jwtIssuer    string
	tokenExpiry  time.Duration
}

type AuthConfig struct {
	DB          *gorm.DB
	DexConfig   *DexConfig
	JWTSecret   string
	JWTIssuer   string
	TokenExpiry time.Duration
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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
		db:          config.DB,
		dexProvider: dexProvider,
		jwtSecret:   []byte(config.JWTSecret),
		jwtIssuer:   config.JWTIssuer,
		tokenExpiry: config.TokenExpiry,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	var user models.User
	err := s.db.Preload("Teams").Where("email = ?", req.Email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	if !user.ComparePassword(req.Password) {
		return nil, ErrInvalidCredentials
	}

	now := time.Now()
	user.LastLoginAt = &now
	s.db.Save(&user)

	token, err := s.generateJWT(&user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateRefreshToken(&user)
	if err != nil {
		return nil, err
	}

	// Get team names from Teams relationship
	groups := make([]string, 0)
	if len(user.Teams) > 0 {
		for _, tm := range user.Teams {
			// Note: tm is TeamMember, we might need to load the actual Team data
			groups = append(groups, string(tm.Role)) // Convert TeamRole to string
		}
	}

	return &LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.tokenExpiry),
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

func (s *AuthService) LoginWithSSO(ctx context.Context, code string) (*LoginResponse, error) {
	if s.dexProvider == nil {
		return nil, errors.New("SSO not configured")
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

	var user models.User
	err = s.db.Preload("Teams").Where("email = ?", claims.Email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			username := claims.PreferredUsername
			if username == "" {
				username = claims.Email
			}
			
			user = models.User{
				Email:         claims.Email,
				Username:      username,
				FirstName:     claims.Name,
				EmailVerified: claims.EmailVerified,
				IsActive:      true,
				Role:          models.RoleUser,
				Password:      "SSO_USER",
			}
			
			if err := s.db.Create(&user).Error; err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}
			
			// Note: Team synchronization could be added here if needed
			// For now, teams are managed separately
		} else {
			return nil, err
		}
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	now := time.Now()
	user.LastLoginAt = &now
	s.db.Save(&user)

	jwtToken, err := s.generateJWT(&user)
	if err != nil {
		return nil, err
	}

	// Get team names from Teams relationship
	groups := make([]string, 0)
	if len(user.Teams) > 0 {
		for _, tm := range user.Teams {
			// Note: tm is TeamMember, we might need to load the actual Team data
			groups = append(groups, string(tm.Role)) // Convert TeamRole to string
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

func (s *AuthService) ValidateAPIKey(ctx context.Context, key string) (*models.APIKey, error) {
	var apiKey models.APIKey
	err := s.db.Preload("User").Where("key = ? AND is_active = ?", key, true).First(&apiKey).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidAPIKey
		}
		return nil, err
	}

	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, ErrInvalidAPIKey
	}

	now := time.Now()
	apiKey.LastUsedAt = &now
	apiKey.TotalRequests++
	s.db.Save(&apiKey)

	return &apiKey, nil
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
	if s.dexProvider != nil {
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

		var user models.User
		err = s.db.Preload("Teams").Where("email = ?", claims.Email).First(&user).Error
		if err != nil {
			return nil, err
		}

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

	return nil, errors.New("refresh not supported without SSO")
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
			// Note: tm is TeamMember, we might need to load the actual Team data
			groups = append(groups, string(tm.Role)) // Convert TeamRole to string
		}
	}

	claims := &TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.jwtIssuer,
			Subject:   user.ID.String(),
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
		Subject:   user.ID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// Team synchronization is handled separately through the Team service
// Teams are managed explicitly rather than auto-created from SSO claims