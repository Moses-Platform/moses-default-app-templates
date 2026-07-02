// Package handler implements the HTTP surface for the fullstack-chat
// template: the Moses completion-webhook receiver plus the demo entries
// feed (removed by clean_out_template.sh).
package handler

import (
	"encoding/json"
	"net/http"
)

// Shared JSON response helpers. Load-bearing plumbing: the webhook receiver
// and every demo handler write through these, and they survive
// clean_out_template.sh — put your own handlers' responses through them too
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
