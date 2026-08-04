package main

// Register your real API endpoints here — buildMux (main.go) calls
// registerDemoRoutes(mux, basePath) exactly once. MOSES ROUTING contract
// (CHAT-8qiu0): API routes are registered ONCE under basePath so both the
// browser and the platform's workspace-tool proxy reach them.
//
// Keep api/openapi.json in sync — every operationId becomes an MCP tool
// (workspace_<toolKey>_<operationId>), with paths RELATIVE to
// servers[0].url ("/api/v1") and NO /health entry.

import "net/http"

// registerDemoRoutes is the single route-registration hook. Rename freely
// (e.g. registerRoutes) together with the call site in buildMux.
//
// WORKED EXAMPLE: example_test.go (this package) — real, CI-compiled code, not
// a comment. Its handler half is internal/handler/example_test.go; the spec
// half is the comment above the //go:embed directive in api/api.go.
func registerDemoRoutes(mux *http.ServeMux, basePath string) {
	_, _ = mux, basePath // remove when you register your first route
}

// logDemoEndpoints logs the API surface at startup. Update as you add routes
// (see exampleLogDemoEndpoints in example_test.go).
func logDemoEndpoints(displayURL, basePath string) {
	_, _ = displayURL, basePath // remove when you log your real endpoints
}
