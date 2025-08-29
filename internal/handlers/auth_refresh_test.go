package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/handlers/admin"
	"go.uber.org/zap"
)

// Create a basic zap logger for testing
func newTestLogger(t *testing.T) *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	logger, _ := config.Build()
	return logger
}

// TestRefreshTokenOAuth2Integration tests the OAuth2 refresh token endpoints
// This test verifies that the session loss issue is fixed by testing the actual
// refresh token flow implemented in the admin OAuth handler
func TestRefreshTokenOAuth2Integration(t *testing.T) {
	// Create a mock OAuth2 server that simulates Dex behavior
	mockOAuth := createMockOAuth2Server(t)
	defer mockOAuth.Close()

	t.Log("Testing OAuth2 refresh token integration with mock Dex server")
	t.Logf("Mock server URL: %s", mockOAuth.URL)

	t.Run("Admin OAuth token endpoint with refresh_token grant", func(t *testing.T) {
		// Test the /api/admin/auth/token endpoint which is the OAuth2-compliant endpoint
		// This is the endpoint that handles grant_type=refresh_token

		// Create OAuth handler pointing to our mock server
		logger := newTestLogger(t)
		oauthHandler := admin.NewOAuthHandler(logger, nil, mockOAuth.URL, "test-client-id", "test-client-secret")

		// Test refresh token request
		initialRefreshToken := "valid_refresh_token_12345"
		requestBody := map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": initialRefreshToken,
		}
		reqJSON, err := json.Marshal(requestBody)
		require.NoError(t, err)

		// Create HTTP request
		req := httptest.NewRequest("POST", "/api/admin/auth/token", bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Call the handler
		oauthHandler.TokenExchange(w, req)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code, "Response body: %s", w.Body.String())

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify we got the expected OAuth2 response fields
		assert.NotEmpty(t, response["access_token"], "access_token should be present")
		assert.NotEmpty(t, response["refresh_token"], "refresh_token should be present")
		assert.NotEmpty(t, response["id_token"], "id_token should be present")
		assert.Equal(t, "Bearer", response["token_type"])
		assert.NotNil(t, response["expires_in"])

		// Verify the refresh token has changed (this proves token rotation works)
		newRefreshToken := response["refresh_token"].(string)
		assert.NotEqual(t, initialRefreshToken, newRefreshToken)
		assert.Contains(t, newRefreshToken, "refreshed")

		t.Logf("✓ Successfully refreshed token: %s -> %s", initialRefreshToken, newRefreshToken)
		t.Logf("✓ Received access_token: %s", response["access_token"])
		t.Logf("✓ Received id_token: %s", response["id_token"])
	})

	t.Run("Invalid refresh token returns error", func(t *testing.T) {
		logger := newTestLogger(t)
		oauthHandler := admin.NewOAuthHandler(logger, nil, mockOAuth.URL, "test-client-id", "test-client-secret")

		requestBody := map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": "invalid_token_xyz",
		}
		reqJSON, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/admin/auth/token", bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		oauthHandler.TokenExchange(w, req)

		// Should return an error status
		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errorResp map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &errorResp)
		require.NoError(t, err)

		// Should contain OAuth2 error format
		assert.Contains(t, errorResp, "error")
		assert.Equal(t, "invalid_grant", errorResp["error"])
		t.Logf("✓ Correctly rejected invalid refresh token with error: %v", errorResp)
	})

	t.Run("Wrong grant_type returns error", func(t *testing.T) {
		logger := newTestLogger(t)
		oauthHandler := admin.NewOAuthHandler(logger, nil, mockOAuth.URL, "test-client-id", "test-client-secret")

		requestBody := map[string]string{
			"grant_type": "authorization_code", // Wrong grant type
			"code":       "some_code",
		}
		reqJSON, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/admin/auth/token", bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		oauthHandler.TokenExchange(w, req)

		// Should return an error for invalid code (since we're not testing auth code flow)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		t.Logf("✓ Correctly handled authorization_code grant type")
	})
}

// TestRefreshTokenSequentialOperations tests multiple refresh operations in sequence
func TestRefreshTokenSequentialOperations(t *testing.T) {
	mockServer := createMockOAuth2Server(t)
	defer mockServer.Close()

	logger := newTestLogger(t)
	oauthHandler := admin.NewOAuthHandler(logger, nil, mockServer.URL, "test-client-id", "test-client-secret")

	// Start with an initial refresh token
	currentRefreshToken := "sequential_refresh_token_99999"

	t.Log("Testing sequential refresh token operations to prove token rotation")

	// Perform 3 sequential refresh operations to prove the flow works
	for i := 0; i < 3; i++ {
		t.Logf("\n--- Refresh operation %d ---", i+1)
		t.Logf("Using refresh token: %s", currentRefreshToken)

		// Create refresh request
		requestBody := map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": currentRefreshToken,
		}
		reqJSON, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/admin/auth/token", bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Perform refresh
		oauthHandler.TokenExchange(w, req)
		require.Equal(t, http.StatusOK, w.Code, "Refresh %d failed: %s", i+1, w.Body.String())

		// Parse response
		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify we got new tokens
		newAccessToken := response["access_token"].(string)
		newRefreshToken := response["refresh_token"].(string)
		idToken := response["id_token"].(string)

		assert.NotEmpty(t, newAccessToken)
		assert.NotEmpty(t, newRefreshToken)
		assert.NotEmpty(t, idToken)
		assert.NotEqual(t, currentRefreshToken, newRefreshToken, "Refresh token should change")

		t.Logf("✓ Received new access token: %s", newAccessToken[:20]+"...")
		t.Logf("✓ Received new refresh token: %s", newRefreshToken)
		t.Logf("✓ Received new ID token: %s", idToken[:30]+"...")

		// CRITICAL TEST: Try to use the old refresh token (should fail)
		t.Log("Testing that old refresh token is invalidated...")
		oldTokenReq := map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": currentRefreshToken,
		}
		oldReqJSON, err := json.Marshal(oldTokenReq)
		require.NoError(t, err)

		oldReq := httptest.NewRequest("POST", "/api/admin/auth/token", bytes.NewBuffer(oldReqJSON))
		oldReq.Header.Set("Content-Type", "application/json")
		oldW := httptest.NewRecorder()

		oauthHandler.TokenExchange(oldW, oldReq)
		assert.Equal(t, http.StatusBadRequest, oldW.Code, "Old token should be invalid")
		t.Logf("✓ Old refresh token correctly invalidated")

		// Use new refresh token for next iteration
		currentRefreshToken = newRefreshToken
	}

	t.Log("\n=== REFRESH TOKEN INTEGRATION TEST RESULTS ===")
	t.Log("✓ Successfully completed 3 sequential refresh operations")
	t.Log("✓ Each refresh invalidated the previous token (security)")
	t.Log("✓ OAuth2 refresh token flow is working correctly")
	t.Log("✓ Session loss issue should be resolved")
	t.Log("================================================")
}

// createMockOAuth2Server creates a mock OAuth2/OIDC server that behaves like Dex
func createMockOAuth2Server(t *testing.T) *httptest.Server {
	validTokens := map[string]bool{
		"valid_refresh_token_12345":    true,
		"sequential_refresh_token_99999": true,
		"another_valid_token":           true,
	}

	mux := http.NewServeMux()

	// OAuth2 token endpoint - this is what the admin handler calls
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Mock server received %s request to /token", r.Method)

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseForm()
		if err != nil {
			t.Logf("Failed to parse form: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		grantType := r.Form.Get("grant_type")
		refreshToken := r.Form.Get("refresh_token")
		clientID := r.Form.Get("client_id")
		clientSecret := r.Form.Get("client_secret")

		t.Logf("Grant type: %s, Refresh token: %s, Client ID: %s", grantType, refreshToken, clientID)

		// Validate grant type
		if grantType != "refresh_token" {
			t.Logf("Invalid grant type: %s", grantType)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "unsupported_grant_type",
				"error_description": fmt.Sprintf("Grant type '%s' not supported", grantType),
			})
			return
		}

		// Validate client credentials (basic validation)
		if clientID == "" || clientSecret == "" {
			t.Logf("Missing client credentials")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "invalid_client",
				"error_description": "Client credentials required",
			})
			return
		}

		// Validate refresh token
		if !validTokens[refreshToken] {
			t.Logf("Invalid refresh token: %s", refreshToken)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "invalid_grant",
				"error_description": "Invalid or expired refresh token",
			})
			return
		}

		// Mark old token as invalid and create new one (token rotation)
		delete(validTokens, refreshToken)
		newRefreshToken := refreshToken + "_refreshed_" + fmt.Sprintf("%d", time.Now().Unix())
		validTokens[newRefreshToken] = true

		t.Logf("Issuing new tokens, old refresh token invalidated")

		// Return new tokens in OAuth2 format
		w.Header().Set("Content-Type", "application/json")
		tokenResponse := map[string]interface{}{
			"access_token":  fmt.Sprintf("access_%d_%s", time.Now().Unix(), clientID),
			"refresh_token": newRefreshToken,
			"id_token":      "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0LXVzZXIiLCJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJpYXQiOjE3MjUwMDA4MDB9.mock_signature",
			"token_type":    "Bearer",
			"expires_in":    3600,
		}

		err = json.NewEncoder(w).Encode(tokenResponse)
		if err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	server := httptest.NewServer(mux)
	t.Logf("Created mock OAuth2 server at: %s", server.URL)

	return server
}
