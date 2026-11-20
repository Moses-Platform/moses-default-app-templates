package middleware

import (
	"context"
	"net/http"

	"github.com/moses-platform/backend-template/internal/config"
)

type contextKey string

const MosesContextKey contextKey = "moses_context"

// MosesContext holds context extracted from X-Moses-* headers.
// These headers are injected by Moses WorkspaceToolProxy when calling workspace tool APIs.
// These caller-supplied fields are audit context, never authentication.
//
// CHAT-pxeo.12: SelfTenantID is the deploy-pinned authoritative tenant
// id (from MOSES_TENANT_ID env via internal/config.SelfTenantID()).
// CallerTenantID is the audit/caller-context value read from the
// X-Moses-Tenant-ID header. Storage/lookup MUST use SelfTenantID.
// UntrustedUserID is caller-controlled audit context, NOT an authenticated identity.
type MosesContext struct {
	SelfTenantID    string // MOSES_TENANT_ID env value — authoritative for storage
	CallerTenantID  string // X-Moses-Tenant-ID header — caller audit context only
	UntrustedUserID string // Caller-supplied; never use for authentication or authorization.
	ChartID         string // X-Moses-Chart-ID: Project/workspace identifier
	ToolID          string // X-Moses-Tool-ID: Workspace tool deployment identifier
	RequestID       string // X-Moses-Request-ID: Unique request identifier for tracing
	MCPSource       string // X-Moses-MCP-Source: Source of MCP call (e.g., "claude-code", "moses-manager")
	APIKeyID        string // X-Moses-API-Key-ID: API key identifier (optional, for audit logging)
}

// MosesHeaders is a middleware that extracts Moses platform headers from incoming requests
// and adds untrusted caller context. This does not establish identity or RBAC.
//
// When Moses headers are absent (e.g., during local development), the context will contain
// empty strings for caller fields. SelfTenantID is always populated from env.
func MosesHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := &MosesContext{
			SelfTenantID:    config.SelfTenantID(),
			CallerTenantID:  r.Header.Get("X-Moses-Tenant-ID"),
			UntrustedUserID: r.Header.Get("X-Moses-User-ID"),
			ChartID:         r.Header.Get("X-Moses-Chart-ID"),
			ToolID:          r.Header.Get("X-Moses-Tool-ID"),
			RequestID:       r.Header.Get("X-Moses-Request-ID"),
			MCPSource:       r.Header.Get("X-Moses-MCP-Source"),
			APIKeyID:        r.Header.Get("X-Moses-API-Key-ID"),
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
