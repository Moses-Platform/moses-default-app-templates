package main

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

// Demo endpoints — concentrated here so clean_out_template.sh can swap in an
// empty stub. The plumbing routes (health, openapi) stay in main.go's
// registerAPIRoutes; the tenant-contract helpers this file uses
// (enforceTenantMatch / getMosesContext) live in main.go and survive the
// clean-out.

// registerDemoRoutes wires the template's demo API endpoints ONCE under
// MOSES_BASE_PATH (see the MOSES ROUTING comment on registerAPIRoutes).
// Every route added here must also appear in api/openapi.json (relative to
// servers[0].url = /api/v1) so it becomes an MCP tool.
func registerDemoRoutes(mux *http.ServeMux, baseURL string) {
	mux.HandleFunc(baseURL+"/api/v1/status", handleStatus)
}

var startTime = time.Now()

// handleStatus is the reference for the Moses-context header-echo pattern:
// the platform injects X-Moses-User-ID / X-Moses-Chart-ID /
// X-Moses-Request-ID on proxied requests and echoing them back verifies the
// plumbing end-to-end. CHAT-w6gt: tenant UUIDs stay OUT of the body — the
// mosesContext struct enforces that with json:"-" (test-locked).
func handleStatus(w http.ResponseWriter, r *http.Request) {
	if enforceTenantMatch(w, r) {
		return
	}
	mc := getMosesContext(r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"app":     "fullstack-unified",
		"version": "1.0.0",
		"uptime":  time.Since(startTime).Round(time.Second).String(),
		"moses":   mc,
		"env": map[string]string{
			"port":            os.Getenv("PORT"),
			"base_url":        os.Getenv("BASE_URL"),
			"moses_base_path": os.Getenv("MOSES_BASE_PATH"),
		},
	})
}
