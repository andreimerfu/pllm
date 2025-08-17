package key

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	// Key prefixes for different types
	KeyPrefixAPI     = "sk-api"
	KeyPrefixVirtual = "sk-vrt"
	KeyPrefixMaster  = "sk-mst"
	
	// Key lengths (before encoding)
	KeyLengthBytes = 32
)

type KeyGenerator struct{}

func NewKeyGenerator() *KeyGenerator {
	return &KeyGenerator{}
}

// GenerateAPIKey creates a new API key
func (kg *KeyGenerator) GenerateAPIKey() (string, string, error) {
	return kg.generateKey(KeyPrefixAPI)
}

// GenerateVirtualKey creates a new virtual key
func (kg *KeyGenerator) GenerateVirtualKey() (string, string, error) {
	return kg.generateKey(KeyPrefixVirtual)
}

// GenerateMasterKey creates a new master key
func (kg *KeyGenerator) GenerateMasterKey() (string, string, error) {
	return kg.generateKey(KeyPrefixMaster)
}

// generateKey creates a secure key with the given prefix
// Returns: (plaintext_key, hashed_key, error)
func (kg *KeyGenerator) generateKey(prefix string) (string, string, error) {
	// Generate random bytes
	keyBytes := make([]byte, KeyLengthBytes)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate random key: %w", err)
	}
	
	// Encode to base64
	keyData := base64.URLEncoding.EncodeToString(keyBytes)
	
	// Remove padding for cleaner keys
	keyData = strings.TrimRight(keyData, "=")
	
	// Create full key with prefix
	fullKey := fmt.Sprintf("%s-%s", prefix, keyData)
	
	// Hash the key for storage
	hashedKey := kg.HashKey(fullKey)
	
	return fullKey, hashedKey, nil
}

// HashKey creates a SHA-256 hash of the key for secure storage
func (kg *KeyGenerator) HashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// ValidateKeyFormat checks if a key has the correct format
func (kg *KeyGenerator) ValidateKeyFormat(key string) error {
	parts := strings.Split(key, "-")
	if len(parts) < 2 {
		return fmt.Errorf("invalid key format: missing prefix")
	}
	
	prefix := strings.Join(parts[:2], "-")
	switch prefix {
	case KeyPrefixAPI, KeyPrefixVirtual, KeyPrefixMaster:
		// Valid prefix
	default:
		return fmt.Errorf("invalid key prefix: %s", prefix)
	}
	
	if len(parts) < 3 {
		return fmt.Errorf("invalid key format: missing key data")
	}
	
	keyData := parts[2]
	if len(keyData) < 32 {
		return fmt.Errorf("invalid key format: key too short")
	}
	
	return nil
}

// GenerateKeyID creates a new UUID for key identification
func (kg *KeyGenerator) GenerateKeyID() uuid.UUID {
	return uuid.New()
}

// ExtractKeyPrefix returns the prefix from a key
func (kg *KeyGenerator) ExtractKeyPrefix(key string) string {
	parts := strings.Split(key, "-")
	if len(parts) >= 2 {
		return strings.Join(parts[:2], "-")
	}
	return ""
}

// IsAPIKey checks if the key is an API key
func (kg *KeyGenerator) IsAPIKey(key string) bool {
	return strings.HasPrefix(key, KeyPrefixAPI)
}

// IsVirtualKey checks if the key is a virtual key
func (kg *KeyGenerator) IsVirtualKey(key string) bool {
	return strings.HasPrefix(key, KeyPrefixVirtual)
}

// IsMasterKey checks if the key is a master key
func (kg *KeyGenerator) IsMasterKey(key string) bool {
	return strings.HasPrefix(key, KeyPrefixMaster)
}