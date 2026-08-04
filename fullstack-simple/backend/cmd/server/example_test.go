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
// rest of the slice — the handlers in internal/handler (see
// internal/handler/example_test.go) and the "/things" path in api/openapi.json
// (worked spec example: the comment above the //go:embed directive in
// api/api.go). Then delete this file.
package main

import "net/http"

// exampleRegisterRoutes is what registerDemoRoutes looks like once you have
// routes.
//
// MOSES ROUTING contract (CHAT-8qiu0): ONE registration per route, under
// basePath, so both the browser and the platform's workspace-tool proxy reach
// it — never a second bare-path mount at "/api/v1/things" as well. Mirror every
// route in api/openapi.json, RELATIVE to servers[0].url ("/api/v1") — the
// served spec is what registers your MCP tools, and main_test.go locks
// spec↔mux consistency.
//
// In your real code the handlers come from the handler package:
//
//	mux.HandleFunc("GET "+basePath+"/api/v1/things", handler.ListThings)
//	mux.HandleFunc("POST "+basePath+"/api/v1/things", handler.CreateThing)
//
// (adding the import "github.com/moses-platform/fullstack-simple/internal/handler"
// is part of that). Below they are local stand-ins only because a _test.go file
// in package main cannot reference another package's test-only declarations —
// the real handler bodies live in internal/handler/example_test.go.
func exampleRegisterRoutes(mux *http.ServeMux, basePath string) {
	mux.HandleFunc("GET "+basePath+"/api/v1/things", exampleListThings)
	mux.HandleFunc("POST "+basePath+"/api/v1/things", exampleCreateThing)
}

// Stand-ins for handler.ListThings / handler.CreateThing. See
// internal/handler/example_test.go for the real bodies (and the tenant rules
// they encode).
func exampleListThings(w http.ResponseWriter, r *http.Request)  { _, _ = w, r }
func exampleCreateThing(w http.ResponseWriter, r *http.Request) { _, _ = w, r }
