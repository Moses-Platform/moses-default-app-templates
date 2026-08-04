// WORKED EXAMPLE — your first tenant-scoped handler pair.
//
// WHY THIS IS A _test.go FILE: it is a compiled example that must never ship.
// `go vet ./...` and `go test ./...` (both in CI) compile every _test.go file,
// so a defect here fails the build; `go build` excludes _test.go files, so
// nothing below ever reaches the binary. Deliberately declaration-only — there
// is no Test*/Benchmark*/Example* function here, so `go test` compiles it and
// runs nothing.
//
// HOW TO USE IT: copy the declarations into a real file (e.g.
// internal/handler/things.go), rename Thing/things to your real resource, and
// wire the rest of the slice — the routes in cmd/server/demo_routes.go (see
// cmd/server/example_test.go) and the "/things" path in api/openapi.json
// (worked spec example: the comment above the //go:embed directive in
// api/api.go). Then delete this file.
//
// The three rules this example encodes, in order:
//  1. enforceTenantMatch FIRST in every write handler; return immediately on
//     a true result — the 403 body is already written.
//  2. Scope storage by config.SelfTenantID() (the deploy-pinned
//     MOSES_TENANT_ID env), NEVER by mc.CallerTenantID / the
//     X-Moses-Tenant-ID header. Read it from config directly rather than from
//     mc.SelfTenantID: GetMosesContext returns a zero-value context when the
//     middleware did not run (a direct mux mount, a unit test, a future
//     refactor), and that silently scopes every row to the empty tenant —
//     exactly the legacy class MigrateTenant exists to clean up. mc stays in
//     the write path only for the enforceTenantMatch cross-check.
//  3. Never echo a tenant UUID in a response body (CHAT-w6gt) — note the
//     Thing struct below has no tenant field.
package handler

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/moses-platform/backend-template/internal/config"
	"github.com/moses-platform/backend-template/internal/middleware"
)

// Replace with your real storage. Keyed by the deploy-pinned tenant id.
// A package-level map is NOT safe for concurrent use: two simultaneous
// POSTs to an unguarded map are a "concurrent map writes" fatal error
// that kills the pod. Real storage (a database) brings its own
// concurrency control; an in-memory placeholder owes you this mutex.
var (
	thingsMu       sync.RWMutex
	thingsByTenant = map[string][]Thing{}
)

// Thing is the example resource.
type Thing struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// No TenantID field on purpose (CHAT-w6gt).
}

// ListThings answers GET {basePath}/api/v1/things (operationId listThings).
func ListThings(w http.ResponseWriter, r *http.Request) {
	thingsMu.RLock()
	things := append([]Thing(nil), thingsByTenant[config.SelfTenantID()]...) // rule 2
	thingsMu.RUnlock()
	if things == nil {
		things = []Thing{}
	}
	w.Header().Set("Content-Type", "application/json")
	// The list response is an OBJECT ({"things": [...]}), never a bare array —
	// the OpenAPI schema and the frontend's fetchAPI<{ things: Thing[] }> must
	// agree with this shape.
	_ = json.NewEncoder(w).Encode(map[string]any{"things": things})
}

// CreateThing answers POST {basePath}/api/v1/things (operationId createThing).
func CreateThing(w http.ResponseWriter, r *http.Request) {
	mc := middleware.GetMosesContext(r)
	if enforceTenantMatch(w, mc) { // rule 1 — before any mutation
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	self := config.SelfTenantID() // rule 2
	thing := Thing{ID: uuid.NewString(), Name: in.Name}
	thingsMu.Lock()
	thingsByTenant[self] = append(thingsByTenant[self], thing)
	thingsMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(thing) // rule 3 — no tenant id in the body
}
