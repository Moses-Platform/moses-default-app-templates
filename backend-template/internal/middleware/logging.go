package middleware

import (
	"log"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Logging is a middleware that logs HTTP requests with method, path, status, and duration.
// It also includes the Moses Request ID if present for request tracing.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default status
		}

		// Get Moses context for request ID
		mosesCtx := GetMosesContext(r)
		requestID := mosesCtx.RequestID
		if requestID == "" {
			requestID = "-"
		}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Log request details
		duration := time.Since(start)
		log.Printf(
			"[%s] %s %s %d %v %s",
			requestID,
			r.Method,
			r.URL.Path,
			wrapped.statusCode,
			duration,
			mosesCtx.TenantID,
		)
	})
}
