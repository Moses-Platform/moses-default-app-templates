package handler

import (
	"encoding/json"
	"net/http"
)

// Health returns service health status. Always public — never gated by
// the OIDCAuth middleware (it is in the implicit public-path set).
func Health(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":  "healthy",
		"service": "fullstack-oidc",
		"version": "1.0.0",
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
