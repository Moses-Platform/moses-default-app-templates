package main

// Demo routes were removed by ./clean_out_template.sh.
//
// Register your real API endpoints here — buildMux (main.go) calls
// registerDemoRoutes(mux, basePath) exactly once. MOSES ROUTING contract
// (CHAT-8qiu0): API routes are registered ONCE under basePath so both the
// browser and the platform's workspace-tool proxy reach them:
//
//	mux.HandleFunc("GET "+basePath+"/api/v1/things", handler.ListThings)
//
// Keep api/openapi.json in sync — every operationId becomes an MCP tool
// (workspace_<toolKey>_<operationId>), with paths RELATIVE to
// servers[0].url ("/api/v1") and NO /health entry.

import "net/http"

// registerDemoRoutes is the single route-registration hook. Rename freely
// (e.g. registerRoutes) together with the call site in buildMux.
func registerDemoRoutes(mux *http.ServeMux, basePath string) {
	_, _ = mux, basePath // remove when you register your first route
}

// logDemoEndpoints logs the API surface at startup. Update as you add routes.
func logDemoEndpoints(displayURL, basePath string) {
	_, _ = displayURL, basePath // remove when you log your real endpoints
}
