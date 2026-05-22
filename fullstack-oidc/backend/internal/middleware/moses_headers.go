package middleware

import (
	"context"
	"net/http"

	"github.com/moses-platform/fullstack-oidc/internal/config"
)

// MosesContext holds Moses-specific request context extracted from the
// X-Moses-* headers. SelfTenantID is the deploy-pinned authoritative
// tenant id; CallerTenantID is the audit/caller-context value from the
// header. Storage/lookup MUST use SelfTenantID.
type MosesContext struct {
	SelfTenantID   string
	CallerTenantID string
	UserID         string
	ChartID        string
	RequestID      string
}

type contextKey string

const mosesContextKey contextKey = "moses"

// MosesHeaders middleware extracts Moses headers and populates
// MosesContext for downstream handlers.
func MosesHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mosesCtx := MosesContext{
			SelfTenantID:   config.SelfTenantID(),
			CallerTenantID: r.Header.Get("X-Moses-Tenant-ID"),
			UserID:         r.Header.Get("X-Moses-User-ID"),
			ChartID:        r.Header.Get("X-Moses-Chart-ID"),
			RequestID:      r.Header.Get("X-Moses-Request-ID"),
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
