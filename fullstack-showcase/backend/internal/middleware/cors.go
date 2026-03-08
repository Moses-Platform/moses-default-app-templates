package middleware

import (
	"net/http"
	"os"
)

// CORS middleware handles Cross-Origin Resource Sharing
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow frontend origin (in K8s: service DNS, in dev: localhost)
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		// In production, restrict to known origins
		env := os.Getenv("NODE_ENV")
		if env == "production" {
			// Allow K8s service mesh origins
			if origin != "*" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		} else {
			// Development: allow all
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Moses-Tenant-ID, X-Moses-User-ID, X-Moses-Chart-ID, X-Moses-Request-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
