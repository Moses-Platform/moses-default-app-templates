package main

import "net/http"

// registerDemoRoutes used to wire the template's demo endpoint
// (/api/v1/status); the demo was removed by clean_out_template.sh. Add your
// real API routes here (feel free to rename the file/function —
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
func registerDemoRoutes(mux *http.ServeMux, baseURL string) {
}
