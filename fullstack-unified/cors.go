package main

import (
	"net/http"
	"os"
	"strings"
)

// corsMiddleware is deny-by-default Cross-Origin Resource Sharing.
//
// The app is same-origin by design (this single binary serves the frontend
// and the API under one origin), so NO CORS headers are emitted unless the
// operator explicitly opts in via CORS_ALLOWED_ORIGINS — a comma-separated
// list of exact origins, e.g.
//
//	CORS_ALLOWED_ORIGINS="https://portal.example.com,https://admin.example.com"
//
// Rules:
//   - Unset/empty env → pure pass-through (no CORS headers at all).
//   - A request Origin matching an allowlisted entry exactly → that origin
//     is echoed with Allow-Credentials, plus Vary: Origin for caches.
//   - A literal "*" entry → wildcard WITHOUT Allow-Credentials. The
//     wildcard+credentials combination is never emitted: it would let any
//     external site read authenticated responses (the pre-hardening bug —
//     the old middleware reflected arbitrary Origins with credentials).
//   - Non-allowlisted Origins get no CORS headers (the browser blocks the
//     cross-origin read).
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed, wildcard := originAllowed(origin, os.Getenv("CORS_ALLOWED_ORIGINS"))

		if allowed {
			if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				// NEVER combine wildcard with credentials.
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Add("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Moses-Tenant-ID, X-Moses-User-ID, X-Moses-Chart-ID, X-Moses-Request-ID")

			// Preflight for an allowed origin is answered here.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// originAllowed reports whether origin is covered by the comma-separated
// allowlist, and whether the match came from a literal "*" entry.
func originAllowed(origin, allowlist string) (allowed, wildcard bool) {
	if origin == "" || strings.TrimSpace(allowlist) == "" {
		return false, false
	}
	for _, entry := range strings.Split(allowlist, ",") {
		entry = strings.TrimSpace(entry)
		switch {
		case entry == "":
			continue
		case entry == "*":
			// Remember the wildcard but keep scanning: an exact match is
			// preferred so credentialed use still works when both are listed.
			wildcard = true
			allowed = true
		case entry == origin:
			return true, false
		}
	}
	return allowed, wildcard
}
