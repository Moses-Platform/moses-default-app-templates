// WORKED EXAMPLE — the route half of the `things` vertical slice.
//
// WHY THIS IS A _test.go FILE: it is a compiled example that must never ship.
// `go vet ./...` and `go test ./...` (both in CI) compile every _test.go file,
// so a defect here fails the build; `go build` excludes _test.go files, so
// nothing below ever reaches the binary. Deliberately declaration-only — no
// Test*/Benchmark*/Example* function, so `go test` compiles it and runs
// nothing.
//
// HOW TO USE IT: copy the body of exampleRegisterRoutes into registerDemoRoutes
// (demo_routes.go), rename Thing/things to your real resource, and wire the
// rest of the slice — the table in internal/database/migrate_demo.go, the
// handlers in internal/handler (see internal/handler/example_test.go) and the
// "/things" path in api/openapi.json (worked spec example: the comment above
// the //go:embed directive in api/api.go). Then delete this file.
package main

import (
	"database/sql"
	"net/http"
)

// exampleRegisterRoutes is what registerDemoRoutes looks like once you have
// routes.
//
// Rules (enforced by the cmd/server tests):
//   - MOSES ROUTING contract (CHAT-8qiu0): register API routes ONCE under
//     basePath — never a second root mount — so both the browser and the
//     platform's workspace-tool proxy reach them.
//   - Mirror every route in api/openapi.json with paths RELATIVE to the
//     servers[0].url "/api/v1" base (never /api/-rooted, never /health).
//   - Scope storage by the deploy-pinned tenant (config.SelfTenantID()) and
//     reuse the enforceTenantMatch 403 cross-check from
//     internal/handler/tenant.go.
//
// In your real code the handler comes from the handler package:
//
//	things := handler.NewThingsHandler(db)
//	mux.HandleFunc("GET "+basePath+"/api/v1/things", things.List)
//	mux.HandleFunc("POST "+basePath+"/api/v1/things", things.Create)
//
// (adding the import "github.com/moses-platform/fullstack-showcase/internal/handler"
// is part of that). Below it is a local stand-in only because a _test.go file
// in package main cannot reference another package's test-only declarations —
// the real handler bodies live in internal/handler/example_test.go.
func exampleRegisterRoutes(mux *http.ServeMux, basePath string, db *sql.DB) {
	things := exampleNewThingsHandler(db)
	mux.HandleFunc("GET "+basePath+"/api/v1/things", things.List)
	mux.HandleFunc("POST "+basePath+"/api/v1/things", things.Create)
}

// Stand-in for handler.ThingsHandler. See internal/handler/example_test.go for
// the real bodies (and the tenant rules they encode).
type exampleThingsHandler struct{ db *sql.DB }

func exampleNewThingsHandler(db *sql.DB) *exampleThingsHandler {
	return &exampleThingsHandler{db: db}
}

func (h *exampleThingsHandler) List(w http.ResponseWriter, r *http.Request)   { _, _ = w, r }
func (h *exampleThingsHandler) Create(w http.ResponseWriter, r *http.Request) { _, _ = w, r }
