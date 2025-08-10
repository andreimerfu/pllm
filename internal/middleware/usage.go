package middleware

import (
	"net/http"
)

func UsageTracking() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: Implement usage tracking
			// For now, just pass through
			next.ServeHTTP(w, r)
		})
	}
}