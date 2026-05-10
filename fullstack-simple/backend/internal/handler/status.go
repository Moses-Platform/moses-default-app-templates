package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

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
