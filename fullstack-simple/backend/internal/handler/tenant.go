package handler

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/moses-platform/fullstack-simple/internal/config"
)

// Tenant-contract helpers (CHAT-pxeo.12). These are Moses platform plumbing,
// NOT demo code — every handler that writes or reads tenant-scoped state
// must use them. Self-identification (storage/lookup keys) comes from the
// deploy-pinned MOSES_TENANT_ID env via config.SelfTenantID(); the
// X-Moses-Tenant-ID request header is caller-context only (audit + the 403
// cross-check below), never a storage key.

// strictTenantCheckEnabled gates the 403 cross-check. Default true.
// Opt out with MOSES_STRICT_TENANT_CHECK=false.
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

// enforceTenantMatch returns true when it has written a 403 response. Caller
// MUST stop processing on a true return. Body intentionally omits UUIDs.
func enforceTenantMatch(w http.ResponseWriter, r *http.Request) bool {
	if !strictTenantCheckEnabled() {
		return false
	}
	caller := strings.TrimSpace(r.Header.Get("X-Moses-Tenant-ID"))
	if caller == "" || caller == config.SelfTenantID() {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}`))
	return true
}

// WORKED EXAMPLE — your first tenant-scoped handler pair (ListThings /
// CreateThing) lives in example_test.go in this package. It is REAL code, not a
// comment: `go vet ./...` / `go test ./...` compile it, while `go build`
// excludes _test.go files so it can never ship. The route half is
// cmd/server/example_test.go; the spec half is the comment above the //go:embed
// directive in api/api.go.

// generateUUID creates a random UUID v4 without external dependencies.
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
