package middleware

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	APIKeyIDKey contextKey = "api_key_id"
	GroupIDKey  contextKey = "group_id"
)

func Authenticate(jwtSecret string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: Implement JWT authentication
			// For now, just pass through
			next.ServeHTTP(w, r)
		})
	}
}

func APIKeyAuth() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				// Check Authorization header
				auth := r.Header.Get("Authorization")
				if strings.HasPrefix(auth, "Bearer ") {
					apiKey = strings.TrimPrefix(auth, "Bearer ")
				}
			}
			
			if apiKey == "" {
				http.Error(w, "API key required", http.StatusUnauthorized)
				return
			}
			
			// TODO: Validate API key from database
			// For now, just pass through
			next.ServeHTTP(w, r)
		})
	}
}

func AdminAuth(jwtSecret string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: Implement admin authentication
			// For now, just pass through
			next.ServeHTTP(w, r)
		})
	}
}

func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

func GetAPIKeyID(ctx context.Context) string {
	if apiKeyID, ok := ctx.Value(APIKeyIDKey).(string); ok {
		return apiKeyID
	}
	return ""
}

func GetGroupID(ctx context.Context) string {
	if groupID, ok := ctx.Value(GroupIDKey).(string); ok {
		return groupID
	}
	return ""
}