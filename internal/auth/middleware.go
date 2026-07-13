package auth

import (
	"net/http"
	"strings"
)

// Middleware validates Authorization: Bearer <apiKey> (or raw api key).
func Middleware(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" {
				next.ServeHTTP(w, r)
				return
			}

			header := r.Header.Get("Authorization")
			token := strings.TrimSpace(header)
			if strings.HasPrefix(strings.ToLower(token), "bearer ") {
				token = strings.TrimSpace(token[7:])
			}
			if token == "" || token != apiKey {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"name":"AUTHENTICATION_FAILURE","message":"Authentication failed due to invalid authentication credentials."}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
