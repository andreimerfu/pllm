package middleware

import (
	"net/http"
)

func RateLimit() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: Implement rate limiting based on user/API key
			// For now, just pass through
			next.ServeHTTP(w, r)
		})
	}
}