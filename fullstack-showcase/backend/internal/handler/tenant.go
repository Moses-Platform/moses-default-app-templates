package handler

import (
	"net/http"
	"os"
	"strings"

	"github.com/moses-platform/fullstack-showcase/internal/middleware"
)

// Load-bearing tenant-enforcement helpers — platform plumbing, not app
// code. Reuse them from YOUR handlers when you add tenant-scoped routes.
//
// CHAT-pxeo.12: storage tenant_id is deploy-pinned (env). Header is
// caller-context only and drives the 403 cross-check on writes.

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

// enforceTenantMatch returns true and writes 403 when the caller-supplied
// header tenant disagrees with the deploy-pinned self tenant. Body
// intentionally omits the actual UUIDs so we never leak tenant ids.
func enforceTenantMatch(w http.ResponseWriter, mc middleware.MosesContext) bool {
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

// WORKED EXAMPLE — your first tenant-scoped handler pair (ThingsHandler.List /
// .Create) lives in example_test.go in this package. It is REAL code, not a
// comment: `go vet ./...` / `go test ./...` compile it, while `go build`
// excludes _test.go files so it can never ship. The route half is
// cmd/server/example_test.go; the spec half is the comment above the //go:embed
// directive in api/api.go.
