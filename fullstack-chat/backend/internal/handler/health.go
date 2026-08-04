package handler

import (
	"encoding/json"
	"net/http"
)

// Health returns service health status
func Health(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":  "healthy",
		"service": "fullstack-chat",
		// Hardcoded on purpose: nothing consumes this field programmatically
		// (probes only check the 200). Bump or wire to a build-time ldflag if
		// your app starts caring about reporting its release version here.
		"version": "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
