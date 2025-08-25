package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// JWTService handles JWT token validation from Dex
type JWTService struct {
	logger       *zap.Logger
	issuer       string
	clientID     string
	jwksURL      string
	publicKeys   map[string]*rsa.PublicKey
	lastKeyFetch time.Time
}

// JWTConfig configuration for JWT service
type JWTConfig struct {
	Logger   *zap.Logger
	Issuer   string // Dex issuer URL
	ClientID string // OAuth2 client ID
}

// Claims represents the JWT claims from Dex
type Claims struct {
	jwt.RegisteredClaims
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Name          string   `json:"name"`
	Groups        []string `json:"groups"`
	Username      string   `json:"preferred_username"`
	// Custom claims we'll add for pLLM
	UserID uuid.UUID `json:"user_id,omitempty"`
	TeamID uuid.UUID `json:"team_id,omitempty"`
	Role   string    `json:"role,omitempty"`
}

// NewJWTService creates a new JWT validation service
func NewJWTService(config *JWTConfig) *JWTService {
	return &JWTService{
		logger:     config.Logger,
		issuer:     config.Issuer,
		clientID:   config.ClientID,
		jwksURL:    fmt.Sprintf("%s/keys", strings.TrimSuffix(config.Issuer, "/")),
		publicKeys: make(map[string]*rsa.PublicKey),
	}
}

// ValidateToken validates a JWT token from Dex
func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	// Parse token without validation first to get the header
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Check signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the key ID from the token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}

		// Get or fetch the public key
		publicKey, err := s.getPublicKey(kid)
		if err != nil {
			return nil, fmt.Errorf("failed to get public key: %w", err)
		}

		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract and validate claims
	claims := &Claims{}
	if mapClaims, ok := token.Claims.(jwt.MapClaims); ok {
		// Convert map claims to our Claims struct
		claimsJSON, _ := json.Marshal(mapClaims)
		if err := json.Unmarshal(claimsJSON, claims); err != nil {
			return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
		}
	} else {
		return nil, fmt.Errorf("invalid claims type")
	}

	// Validate standard claims
	if err := s.validateClaims(claims); err != nil {
		return nil, err
	}

	return claims, nil
}

// validateClaims validates the standard JWT claims
func (s *JWTService) validateClaims(claims *Claims) error {
	// Check issuer
	if claims.Issuer != s.issuer {
		return fmt.Errorf("invalid issuer: expected %s, got %s", s.issuer, claims.Issuer)
	}

	// Check audience
	audienceFound := false
	for _, aud := range claims.Audience {
		if aud == s.clientID {
			audienceFound = true
			break
		}
	}
	if !audienceFound {
		return fmt.Errorf("invalid audience")
	}

	// Check expiration
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return fmt.Errorf("token has expired")
	}

	// Check not before
	if claims.NotBefore != nil && time.Now().Before(claims.NotBefore.Time) {
		return fmt.Errorf("token not yet valid")
	}

	return nil
}

// getPublicKey fetches the public key for the given key ID
func (s *JWTService) getPublicKey(kid string) (*rsa.PublicKey, error) {
	// Check if we have the key cached and it's not too old
	if key, ok := s.publicKeys[kid]; ok && time.Since(s.lastKeyFetch) < time.Hour {
		return key, nil
	}

	// Fetch new keys from JWKS endpoint
	if err := s.fetchPublicKeys(); err != nil {
		return nil, fmt.Errorf("failed to fetch public keys: %w", err)
	}

	// Check if we have the key now
	if key, ok := s.publicKeys[kid]; ok {
		return key, nil
	}

	return nil, fmt.Errorf("public key not found for kid: %s", kid)
}

// fetchPublicKeys fetches public keys from the Dex JWKS endpoint
func (s *JWTService) fetchPublicKeys() error {
	resp, err := http.Get(s.jwksURL)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Parse RSA public keys
	for _, key := range jwks.Keys {
		if key.Kty != "RSA" || key.Use != "sig" {
			continue
		}

		publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(fmt.Sprintf(
			"-----BEGIN PUBLIC KEY-----\n%s\n-----END PUBLIC KEY-----",
			key.N,
		)))
		if err != nil {
			s.logger.Warn("Failed to parse RSA public key",
				zap.String("kid", key.Kid),
				zap.Error(err))
			continue
		}

		s.publicKeys[key.Kid] = publicKey
	}

	s.lastKeyFetch = time.Now()
	return nil
}

// GetUserFromClaims extracts user information from JWT claims
func (s *JWTService) GetUserFromClaims(ctx context.Context, claims *Claims) (uuid.UUID, string, []string, error) {
	// If UserID is already in claims (from custom claim), use it
	if claims.UserID != uuid.Nil {
		return claims.UserID, claims.Role, claims.Groups, nil
	}

	// Otherwise, generate a deterministic UUID from the subject
	userID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(claims.Subject))

	// Determine role from groups
	role := "user"
	for _, group := range claims.Groups {
		if group == "admin" {
			role = "admin"
			break
		} else if group == "developers" && role != "admin" {
			role = "developer"
		}
	}

	return userID, role, claims.Groups, nil
}
