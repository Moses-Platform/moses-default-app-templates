package middleware

import (
	"context"
	"net/http"
)

type contextKey string

const MosesContextKey contextKey = "moses_context"

// MosesContext holds context extracted from X-Moses-* headers.
// These headers are injected by Moses WorkspaceToolProxy when calling workspace tool APIs.
// They provide tenant isolation, user identity, and request tracking.
type MosesContext struct {
	TenantID  string // X-Moses-Tenant-ID: Multi-tenant isolation identifier
	UserID    string // X-Moses-User-ID: User who initiated the MCP tool call
	ChartID   string // X-Moses-Chart-ID: Project/workspace identifier
	ToolID    string // X-Moses-Tool-ID: Workspace tool deployment identifier
	RequestID string // X-Moses-Request-ID: Unique request identifier for tracing
	MCPSource string // X-Moses-MCP-Source: Source of MCP call (e.g., "claude-code", "moses-manager")
	APIKeyID  string // X-Moses-API-Key-ID: API key identifier (optional, for audit logging)
}

// MosesHeaders is a middleware that extracts Moses platform headers from incoming requests
// and adds them to the request context. This enables tenant-aware request handling and
// integration with the Moses platform's multi-tenancy and RBAC systems.
//
// When Moses headers are absent (e.g., during local development), the context will contain
// empty strings, and applications should implement appropriate fallback behavior.
func MosesHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := &MosesContext{
			TenantID:  r.Header.Get("X-Moses-Tenant-ID"),
			UserID:    r.Header.Get("X-Moses-User-ID"),
			ChartID:   r.Header.Get("X-Moses-Chart-ID"),
			ToolID:    r.Header.Get("X-Moses-Tool-ID"),
			RequestID: r.Header.Get("X-Moses-Request-ID"),
			MCPSource: r.Header.Get("X-Moses-MCP-Source"),
			APIKeyID:  r.Header.Get("X-Moses-API-Key-ID"),
		}

		// Add Moses context to request context
		newCtx := context.WithValue(r.Context(), MosesContextKey, ctx)
		next.ServeHTTP(w, r.WithContext(newCtx))
	})
}

// GetMosesContext retrieves the MosesContext from a request context.
// Returns an empty MosesContext if headers were not present.
func GetMosesContext(r *http.Request) *MosesContext {
	if ctx, ok := r.Context().Value(MosesContextKey).(*MosesContext); ok {
		return ctx
	}
	// Return empty context if headers not present (e.g., local development)
	return &MosesContext{}
}
