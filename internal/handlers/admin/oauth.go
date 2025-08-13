package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

// OAuthHandler handles OAuth authentication callbacks
type OAuthHandler struct {
	logger       *zap.Logger
	dexURL       string
	clientID     string
	clientSecret string
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(logger *zap.Logger, dexURL, clientID, clientSecret string) *OAuthHandler {
	return &OAuthHandler{
		logger:       logger,
		dexURL:       dexURL,
		clientID:     clientID,
		clientSecret: clientSecret,
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

	// Return the user info to the frontend
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userInfo)
}