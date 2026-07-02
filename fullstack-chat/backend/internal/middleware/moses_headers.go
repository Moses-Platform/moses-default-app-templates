package middleware

import (
	"context"
	"net/http"
)

// MosesContext holds Moses-specific request context
type MosesContext struct {
	TenantID    string
	UserID      string
	ChartID     string
	ToolID      string
	RequestID   string
	MCPSource   string
	APIKeyID    string
}

type contextKey string

const mosesContextKey contextKey = "moses"

// MosesHeaders middleware extracts Moses headers from the request
func MosesHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mosesCtx := MosesContext{
			TenantID:   r.Header.Get("X-Moses-Tenant-ID"),
			UserID:     r.Header.Get("X-Moses-User-ID"),
			ChartID:    r.Header.Get("X-Moses-Chart-ID"),
			ToolID:     r.Header.Get("X-Moses-Tool-ID"),
			RequestID:  r.Header.Get("X-Moses-Request-ID"),
			MCPSource:  r.Header.Get("X-Moses-MCP-Source"),
			APIKeyID:   r.Header.Get("X-Moses-API-Key-ID"),
		}

		ctx := context.WithValue(r.Context(), mosesContextKey, mosesCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetMosesContext retrieves Moses context from request context. This is the
// standard header-extraction surface shared across templates — handlers may
// consume it (e.g. for audit logging or caller-context checks) even though
// no handler in the demo does yet.
func GetMosesContext(ctx context.Context) MosesContext {
	if mosesCtx, ok := ctx.Value(mosesContextKey).(MosesContext); ok {
		return mosesCtx
	}
	return MosesContext{}
}
