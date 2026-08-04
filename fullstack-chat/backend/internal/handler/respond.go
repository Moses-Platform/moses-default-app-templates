// Package handler implements the HTTP surface for the fullstack-chat
// template: the Moses completion-webhook receiver plus whatever handlers
// you add for your own API.
package handler

import (
	"encoding/json"
	"net/http"
)

// Shared JSON response helpers. Load-bearing plumbing: the webhook receiver
// writes through these — put your own handlers' responses through them too
// so error shapes stay consistent ({"error": ..., "code": ...}).

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": message,
		"code":  code,
	})
}
