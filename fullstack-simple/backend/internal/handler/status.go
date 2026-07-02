package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

// DEMO FILE — deleted by clean_out_template.sh. Status is the reference for
// the Moses-context header-echo pattern: the platform injects X-Moses-User-ID
// / X-Moses-Chart-ID / X-Moses-Request-ID on proxied requests, and echoing
// them back is a cheap way to verify the plumbing end-to-end. The pattern is
// documented in skills/usage.md § Moses request-context headers so it
// survives the demo's removal. Note the CHAT-w6gt rule below: tenant UUIDs
// stay OUT of response bodies.

var startTime = time.Now()

func Status(w http.ResponseWriter, r *http.Request) {
	// CHAT-w6gt: omit tenant_id from the JSON response (defense in
	// depth — the caller already knows their tenant context via the
	// header they sent in). user_id / chart_id / request_id remain so
	// the demo card still shows Moses-platform context.
	response := map[string]interface{}{
		"app":     "fullstack-simple",
		"version": "1.0.0",
		"uptime":  time.Since(startTime).String(),
		"moses": map[string]string{
			"user_id":    r.Header.Get("X-Moses-User-ID"),
			"chart_id":   r.Header.Get("X-Moses-Chart-ID"),
			"request_id": r.Header.Get("X-Moses-Request-ID"),
		},
		// CHAT-pbup: expose BOTH BASE_URL (deprecated alias) and
		// MOSES_BASE_PATH (canonical) so the frontend can verify either.
		"env": map[string]string{
			"port":            os.Getenv("PORT"),
			"base_url":        os.Getenv("BASE_URL"),
			"moses_base_path": os.Getenv("MOSES_BASE_PATH"),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
