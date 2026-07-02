package handler

import (
	"net/http"
	"os"
	"strings"

	"github.com/moses-platform/backend-template/internal/middleware"
)

// Tenant-check plumbing — NOT demo code. These helpers survive
// ./clean_out_template.sh and back every handler you add.
//
// CHAT-pxeo.12: storage tenant_id is deploy-pinned (env). The
// X-Moses-Tenant-ID header is caller-context only and drives the 403
// cross-check on writes.

// strictTenantCheckEnabled gates the 403 cross-check. Default true.
func strictTenantCheckEnabled() bool {
	v := strings.TrimSpace(os.Getenv("MOSES_STRICT_TENANT_CHECK"))
	if v == "" {
		return true
	}
	switch strings.ToLower(v) {
	case "0", "false", "no", "off":
		return false
	}
	return true
}

// enforceTenantMatch returns true when it has written a 403 response.
// Body intentionally omits the actual UUIDs.
func enforceTenantMatch(w http.ResponseWriter, mc *middleware.MosesContext) bool {
	if !strictTenantCheckEnabled() {
		return false
	}
	caller := strings.TrimSpace(mc.CallerTenantID)
	if caller == "" || caller == mc.SelfTenantID {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}`))
	return true
}
