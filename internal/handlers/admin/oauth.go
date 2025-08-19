package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/amerfu/pllm/internal/models"
	"github.com/amerfu/pllm/internal/services/team"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// OAuthHandler handles OAuth authentication callbacks
type OAuthHandler struct {
	logger       *zap.Logger
	db           *gorm.DB
	dexURL       string
	clientID     string
	clientSecret string
	teamService  *team.TeamService
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(logger *zap.Logger, db *gorm.DB, dexURL, clientID, clientSecret string) *OAuthHandler {
	return &OAuthHandler{
		logger:       logger,
		db:           db,
		dexURL:       dexURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		teamService:  team.NewTeamService(db),
	}
}

// TokenExchange handles the OAuth token exchange
func (h *OAuthHandler) TokenExchange(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for this endpoint
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req struct {
		Code         string `json:"code"`
		RedirectURI  string `json:"redirect_uri"`
		CodeVerifier string `json:"code_verifier,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request", zap.Error(err))
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Exchange the authorization code for tokens
	tokenURL := fmt.Sprintf("%s/token", strings.TrimSuffix(h.dexURL, "/"))
	
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", req.Code)
	data.Set("redirect_uri", req.RedirectURI)
	data.Set("client_id", h.clientID)
	data.Set("client_secret", h.clientSecret)
	if req.CodeVerifier != "" {
		data.Set("code_verifier", req.CodeVerifier)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(tokenURL, data)
	if err != nil {
		h.logger.Error("Failed to exchange token", zap.Error(err))
		http.Error(w, "Token exchange failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read the response body once
	var responseBody map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		h.logger.Error("Failed to decode response", zap.Error(err))
		http.Error(w, "Failed to decode token response", http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != http.StatusOK {
		h.logger.Error("Token exchange failed", 
			zap.Int("status", resp.StatusCode),
			zap.Any("response", responseBody),
			zap.String("code", req.Code),
			zap.String("redirect_uri", req.RedirectURI))
		
		// Return the error from Dex to the frontend
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(responseBody)
		return
	}

	// Return the tokens to the frontend
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responseBody)
}

// UserInfo fetches user information from Dex
func (h *OAuthHandler) UserInfo(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for this endpoint
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Get the access token from the Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing authorization header", http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
		return
	}

	// Fetch user info from Dex
	userInfoURL := fmt.Sprintf("%s/userinfo", strings.TrimSuffix(h.dexURL, "/"))
	
	req, err := http.NewRequestWithContext(context.Background(), "GET", userInfoURL, nil)
	if err != nil {
		h.logger.Error("Failed to create userinfo request", zap.Error(err))
		http.Error(w, "Failed to fetch user info", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		h.logger.Error("Failed to fetch user info", zap.Error(err))
		http.Error(w, "Failed to fetch user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.logger.Error("User info request failed", zap.Int("status", resp.StatusCode))
		http.Error(w, "Failed to fetch user info", http.StatusInternalServerError)
		return
	}

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		h.logger.Error("Failed to decode user info", zap.Error(err))
		http.Error(w, "Failed to process user info", http.StatusInternalServerError)
		return
	}

	// Auto-provision user if they don't exist
	if h.db != nil {
		if err := h.autoProvisionUser(userInfo); err != nil {
			h.logger.Error("Failed to auto-provision user", zap.Error(err))
			// Don't fail the request, just log the error
		}
	}

	// Return the user info to the frontend
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userInfo)
}

// autoProvisionUser creates or updates a user based on Dex user info
func (h *OAuthHandler) autoProvisionUser(userInfo map[string]interface{}) error {
	// Extract user information from Dex response
	sub, _ := userInfo["sub"].(string)
	email, _ := userInfo["email"].(string)
	name, _ := userInfo["name"].(string)
	preferredUsername, _ := userInfo["preferred_username"].(string)
	emailVerified, _ := userInfo["email_verified"].(bool)
	
	// Extract connector_id to determine the provider
	connectorID, _ := userInfo["connector_id"].(string)
	provider := h.getProviderFromConnectorID(connectorID)
	
	// Extract groups if present
	var groups []string
	if groupsInterface, ok := userInfo["groups"].([]interface{}); ok {
		for _, g := range groupsInterface {
			if groupStr, ok := g.(string); ok {
				groups = append(groups, groupStr)
			}
		}
	}
	
	// Generate username if not provided
	username := preferredUsername
	if username == "" {
		username = strings.Split(email, "@")[0]
	}
	
	// Split name into first and last
	firstName := ""
	lastName := ""
	if name != "" {
		parts := strings.SplitN(name, " ", 2)
		firstName = parts[0]
		if len(parts) > 1 {
			lastName = parts[1]
		}
	}
	
	// Extract avatar URL if available
	avatarURL, _ := userInfo["picture"].(string)
	
	// Check if user exists
	var user models.User
	err := h.db.Where("dex_id = ?", sub).First(&user).Error
	
	now := time.Now()
	
	if err == gorm.ErrRecordNotFound {
		// Create new user
		user = models.User{
			DexID:            sub,
			Email:            email,
			Username:         username,
			FirstName:        firstName,
			LastName:         lastName,
			EmailVerified:    emailVerified,
			IsActive:         true,
			Role:             models.RoleUser, // Default role
			ExternalID:       sub,
			ExternalProvider: provider,
			ExternalGroups:   groups,
			ProvisionedAt:    &now,
			AvatarURL:        avatarURL,
			LastLoginAt:      &now,
			// Set default budget
			MaxBudget:      100.0, // Default $100 monthly budget
			BudgetDuration: models.BudgetPeriodMonthly,
			BudgetResetAt:  time.Now().AddDate(0, 1, 0),
			// Set default rate limits
			TPM:              1000,
			RPM:              60,
			MaxParallelCalls: 5,
		}
		
		if err := h.db.Create(&user).Error; err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
		
		// Add user to default team
		_, err := h.teamService.AddUserToDefaultTeam(context.Background(), user.ID, models.TeamRoleMember)
		if err != nil {
			h.logger.Warn("Failed to add new user to default team", 
				zap.String("dex_id", sub),
				zap.String("user_id", user.ID.String()),
				zap.Error(err))
			// Don't fail the user creation, just log the warning
		} else {
			h.logger.Info("Added new user to default team",
				zap.String("dex_id", sub),
				zap.String("user_id", user.ID.String()))
		}
		
		h.logger.Info("Auto-provisioned new user from Dex",
			zap.String("dex_id", sub),
			zap.String("email", email),
			zap.String("provider", provider))
	} else if err == nil {
		// Update existing user
		user.LastLoginAt = &now
		user.ExternalProvider = provider
		user.ExternalGroups = groups
		user.EmailVerified = emailVerified
		if avatarURL != "" {
			user.AvatarURL = avatarURL
		}
		if firstName != "" {
			user.FirstName = firstName
		}
		if lastName != "" {
			user.LastName = lastName
		}
		
		if err := h.db.Save(&user).Error; err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}
		
		h.logger.Info("Updated existing user from Dex",
			zap.String("dex_id", sub),
			zap.String("email", email),
			zap.String("provider", provider))
	} else {
		return fmt.Errorf("database error: %w", err)
	}
	
	return nil
}

// getProviderFromConnectorID determines the provider from the Dex connector ID
func (h *OAuthHandler) getProviderFromConnectorID(connectorID string) string {
	switch connectorID {
	case "github":
		return "github"
	case "google":
		return "google"
	case "microsoft", "entra":
		return "microsoft"
	case "gitlab":
		return "gitlab"
	case "ldap":
		return "ldap"
	case "local":
		return "local"
	default:
		// Try to extract from connector ID if it contains the provider name
		connectorID = strings.ToLower(connectorID)
		if strings.Contains(connectorID, "github") {
			return "github"
		} else if strings.Contains(connectorID, "google") {
			return "google"
		} else if strings.Contains(connectorID, "microsoft") || strings.Contains(connectorID, "entra") {
			return "microsoft"
		} else if strings.Contains(connectorID, "gitlab") {
			return "gitlab"
		}
		return "oauth"
	}
}