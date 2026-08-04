// VENDORED COPY of shared/csrf-go/csrf.go.
//
// Canonical source: shared/csrf-go/csrf.go (paths here are from the templates
// monorepo root).
//
// To re-sync after upstream changes, run (from the templates monorepo root):
//   sed 's/^package csrf$/package middleware/' shared/csrf-go/csrf.go \
//     > fullstack-chat/backend/internal/middleware/csrf.go.body && ...
// (or hand-copy, changing only the package clause).
//
// ---------------------------------------------------------------------------
//
// Cross-Site Request Forgery guard using the browser Fetch Metadata
// (Sec-Fetch-Site) request header.
//
// Why a deployed app needs its OWN CSRF guard even behind the Moses platform
// edge: the platform access_token cookie is SameSite=None (deployed apps load
// in cross-site iframes — Tauri, embed mode), so the platform ForwardAuth gate
// does NOT block cross-site requests via cookie SameSite. An app that mutates
// state must therefore defend itself and must not assume the edge filtered
// CSRF. This is the application-standalone half of a defense-in-depth union;
// the platform edge enforces the same policy as an early best-effort layer,
// but each layer is a complete, independent control.
//
// Policy (canonical — identical at the app layer and the platform edge):
//
//   - Safe methods (GET, HEAD, OPTIONS) are always allowed. A cross-site GET
//     navigation — e.g. an embed-mode iframe document load — is safe and must
//     pass.
//   - Unsafe methods (POST, PUT, PATCH, DELETE) are allowed only when
//     Sec-Fetch-Site is "same-origin", "none", or absent. Absent covers
//     non-browser / server-to-server callers and pre-Fetch-Metadata browsers;
//     a CSRF attack requires a victim's modern browser, which always sends the
//     header to the origin it targets. "cross-site" and "same-site" unsafe
//     requests receive 403.
//
// Browsers set Sec-Fetch-Site themselves and forbid scripts from spoofing it,
// so this is a robust, dependency-free CSRF defense for a same-origin app.
package middleware

import "net/http"

// RejectCrossSiteCSRF wraps next and rejects cross-site / same-site
// state-changing (unsafe-method) requests with 403. Compose it like the other
// middleware in this package:
//
//	var h http.Handler = mux
//	h = middleware.RejectCrossSiteCSRF(h)
func RejectCrossSiteCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfIsUnsafeMethod(r.Method) && csrfIsCrossSite(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"cross_site_blocked","code":"E_CROSS_SITE"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func csrfIsUnsafeMethod(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func csrfIsCrossSite(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
		return false
	default: // "cross-site", "same-site"
		return true
	}
}
