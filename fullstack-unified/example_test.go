// WORKED EXAMPLE — the `things` vertical slice (routes + handlers).
//
// WHY THIS IS A _test.go FILE: it is a compiled example that must never ship.
// `go vet ./...` and `go test ./...` (both in CI) compile every _test.go file,
// so a defect here fails the build; `go build` excludes _test.go files, so
// nothing below ever reaches the binary. Deliberately declaration-only — there
// is no Test*/Benchmark*/Example* function here, so `go test` compiles it and
// runs nothing.
//
// HOW TO USE IT: copy the body of exampleRegisterRoutes into registerDemoRoutes
// (demo_routes.go) and the handlers into a real file, rename Thing/things to
// your real resource, and wire the rest of the slice — the "/things" path in
// api/openapi.json (worked spec example: the comment above the
// //go:embed api/openapi.json directive in main.go) and the browser half in
// static/app.js. Then delete this file.
//
// The tenant helpers these handlers call — enforceTenantMatch and
// config.SelfTenantID() — live in main.go.
//
// The three rules this example encodes, in order:
//  1. enforceTenantMatch FIRST in every write handler; return immediately on
//     a true result — the 403 body is already written.
//  2. Scope storage by config.SelfTenantID() (the deploy-pinned
//     MOSES_TENANT_ID), NEVER by the X-Moses-Tenant-ID request header.
//  3. Never echo a tenant UUID in a response body (CHAT-w6gt) — note the
//     Thing struct below has no tenant field.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/moses-platform/fullstack-unified/internal/config"
)

// exampleRegisterRoutes is what registerDemoRoutes looks like once you have
// routes.
//
// MOSES ROUTING contract (CHAT-8qiu0): ONE registration per route, under
// baseURL — never a second bare-path mount at "/api/v1/things" as well — so
// both the browser and the platform's workspace-tool proxy reach it. Mirror
// every route in api/openapi.json RELATIVE to servers[0].url ("/api/v1"), never
// /health: the served spec is what registers your MCP tools, and main_test.go
// locks spec↔mux consistency.
func exampleRegisterRoutes(mux *http.ServeMux, baseURL string) {
	mux.HandleFunc("GET "+baseURL+"/api/v1/things", handleListThings)
	mux.HandleFunc("POST "+baseURL+"/api/v1/things", handleCreateThing)
}

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

// newThingID mints a collision-free id. Every row needs its own — a
// hardcoded placeholder makes each rendered key={t.id} collide.
func newThingID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// handleListThings answers GET {baseURL}/api/v1/things (operationId listThings).
func handleListThings(w http.ResponseWriter, r *http.Request) {
	thingsMu.RLock()
	things := append([]Thing(nil), thingsByTenant[config.SelfTenantID()]...) // rule 2
	thingsMu.RUnlock()
	if things == nil {
		things = []Thing{}
	}
	w.Header().Set("Content-Type", "application/json")
	// The list response is an OBJECT ({"things": [...]}), never a bare array —
	// the OpenAPI schema and the static/app.js fetch (data.things) must agree
	// with this shape.
	_ = json.NewEncoder(w).Encode(map[string]any{"things": things})
}

// handleCreateThing answers POST {baseURL}/api/v1/things (operationId createThing).
func handleCreateThing(w http.ResponseWriter, r *http.Request) {
	if enforceTenantMatch(w, r) { // rule 1 — before any mutation
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
	thing := Thing{ID: newThingID(), Name: in.Name}
	thingsMu.Lock()
	thingsByTenant[self] = append(thingsByTenant[self], thing)
	thingsMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(thing) // rule 3 — no tenant id in the body
}
