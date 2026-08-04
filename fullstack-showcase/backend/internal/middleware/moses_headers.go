package middleware

import (
	"context"
	"net/http"

	"github.com/moses-platform/fullstack-showcase/internal/config"
)

// MosesContext holds Moses-specific request context.
//
// CHAT-pxeo.12: SelfTenantID is the deploy-pinned authoritative tenant
// id (from MOSES_TENANT_ID env via internal/config.SelfTenantID()).
// CallerTenantID is the audit/caller-context value read from the
// X-Moses-Tenant-ID request header. Storage/lookup MUST use
// SelfTenantID. Use CallerTenantID only for logs and the strict-tenant
// 403 cross-check.
type MosesContext struct {
	SelfTenantID   string
	CallerTenantID string
	UserID         string
	ChartID        string
	ToolID         string
	RequestID      string
	MCPSource      string
	APIKeyID       string
}

type contextKey string

const mosesContextKey contextKey = "moses"

// MosesHeaders middleware extracts Moses headers from the request and
// populates MosesContext for downstream handlers.
func MosesHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mosesCtx := MosesContext{
			SelfTenantID:   config.SelfTenantID(),
			CallerTenantID: r.Header.Get("X-Moses-Tenant-ID"),
			UserID:         r.Header.Get("X-Moses-User-ID"),
			ChartID:        r.Header.Get("X-Moses-Chart-ID"),
			ToolID:         r.Header.Get("X-Moses-Tool-ID"),
			RequestID:      r.Header.Get("X-Moses-Request-ID"),
			MCPSource:      r.Header.Get("X-Moses-MCP-Source"),
			APIKeyID:       r.Header.Get("X-Moses-API-Key-ID"),
		}

		ctx := context.WithValue(r.Context(), mosesContextKey, mosesCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetMosesContext retrieves Moses context from request context.
func GetMosesContext(ctx context.Context) MosesContext {
	if mosesCtx, ok := ctx.Value(mosesContextKey).(MosesContext); ok {
		return mosesCtx
	}
	return MosesContext{}
}
