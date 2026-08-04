package main

import "net/http"

// registerDemoRoutes is the single route-registration hook — add your real
// API routes here (feel free to rename the file/function —
// registerAPIRoutes in main.go is the single call site).
//
// Rules (see the MOSES ROUTING comment on registerAPIRoutes):
//   - Register API routes ONCE, under baseURL: mux.HandleFunc(baseURL+"/api/v1/...", h)
//   - Mirror every route in api/openapi.json, RELATIVE to servers[0].url
//     ("/api/v1") — the served spec is what registers your MCP tools, and
//     main_test.go locks spec↔mux consistency.
//   - Call enforceTenantMatch (main.go) first in every write handler;
//     tenant-scoped state is keyed by config.SelfTenantID() (env), never by
//     the X-Moses-Tenant-ID header. Never put tenant UUIDs in response
//     bodies (CHAT-w6gt).
//
// WORKED EXAMPLE: example_test.go (module root) — real, CI-compiled code, not a
// comment. It carries both halves (routes + handlers). Its spec half is the
// comment above the //go:embed api/openapi.json directive in main.go; its
// browser half is the fetch example in static/app.js.
func registerDemoRoutes(mux *http.ServeMux, baseURL string) {
}
