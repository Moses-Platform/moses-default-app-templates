package main

import "net/http"

// registerDemoRoutes is the single route-registration hook — add your real
// API routes here (feel free to rename the file/function — buildMux is the
// single call site).
//
// Rules (see the MOSES ROUTING comment on buildMux):
//   - Register API routes ONCE, under basePath: mux.HandleFunc(basePath+"/api/v1/...", h)
//   - Mirror every route in api/openapi.json, RELATIVE to servers[0].url
//     ("/api/v1") — the served spec is what registers your MCP tools, and
//     main_test.go locks spec↔mux consistency.
//   - Tenant-scoped state is keyed by config.SelfTenantID() (env), never by
//     the X-Moses-Tenant-ID header; call handler-level enforceTenantMatch
//     (internal/handler/tenant.go) first in every write handler.
//
// WORKED EXAMPLE: example_test.go (this package) — real, CI-compiled code, not
// a comment. Its handler half is internal/handler/example_test.go; the spec
// half is the comment above the //go:embed directive in api/api.go.
func registerDemoRoutes(mux *http.ServeMux, basePath string) {
}
