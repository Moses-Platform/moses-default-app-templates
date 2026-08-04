package handler

import (
	"net/http"
	"os"
	"strings"

	"github.com/moses-platform/fullstack-chat/internal/config"
)

// Tenant cross-check gate (CHAT-pxeo.12). Load-bearing plumbing: every
// write handler you add should call enforceTenantMatch before mutating
// state. Storage/lookup keys ALWAYS come from config.SelfTenantID()
// (deploy-pinned env), never from the header.

// envStrictTenantCheck gates the 403 cross-check on write handlers
// (CHAT-pxeo.12). Default true; flip to "false" / "0" to disable in a
// hot-fix scenario.
const envStrictTenantCheck = "MOSES_STRICT_TENANT_CHECK"

// strictTenantCheckEnabled reports whether the cross-check is on. Default
// true so accidentally-misconfigured workloads fail loudly.
func strictTenantCheckEnabled() bool {
	v := strings.TrimSpace(os.Getenv(envStrictTenantCheck))
	if v == "" {
		return true
	}
	switch strings.ToLower(v) {
	case "0", "false", "no", "off":
		return false
	}
	return true
}

// callerTenantHeader returns the X-Moses-Tenant-ID header value from the
// request. It is caller context (workspace-tool / audit) — NOT the
// authoritative tenant for storage. Storage keys come from
// config.SelfTenantID().
func callerTenantHeader(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Moses-Tenant-ID"))
}

// enforceTenantMatch returns true and writes a 403 response when the
// caller-supplied header tenant disagrees with our deploy-pinned self
// tenant. Caller MUST stop processing on a true return. Body intentionally
// omits the actual UUIDs so we never leak tenant ids cross-context.
func enforceTenantMatch(w http.ResponseWriter, r *http.Request) bool {
	if !strictTenantCheckEnabled() {
		return false
	}
	caller := callerTenantHeader(r)
	if caller == "" {
		return false // no caller context to cross-check
	}
	if caller == config.SelfTenantID() {
		return false
	}
	writeError(w, http.StatusForbidden, "E_TENANT_MISMATCH", "tenant_mismatch")
	return true
}

// WORKED EXAMPLE — your first tenant-scoped handler pair (ThingsHandler.List /
// .Create) lives in example_test.go in this package. It is REAL code, not a
// comment: `go vet ./...` / `go test ./...` compile it, while `go build`
// excludes _test.go files so it can never ship. The route half is
// cmd/server/example_test.go; the spec half is the comment above the //go:embed
// directive in api/api.go.
