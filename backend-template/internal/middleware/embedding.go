// Package middleware — CHAT-pbup Bug 3: Moses-aware embedding headers.
//
// This middleware emits Content-Security-Policy: frame-ancestors and
// (when policy is "denied") X-Frame-Options: DENY based on the
// MOSES_EMBEDDING_FRAMING / MOSES_EMBEDDING_ALLOWED_ANCESTORS /
// MOSES_EMBEDDING_REPORT_URI env vars resolved at process startup.
//
// API templates default to "denied" framing — they're never legitimately
// iframed — so a deploy with no env vars set still emits a strict policy.
// The platform's deploy pipeline overrides this default for hybrid apps
// that include the backend template behind a frontend (see
// fullstack-unified's main.go for the matching reference middleware).
package middleware

import (
	"net/http"
	"os"
)

type embeddingPolicy struct {
	cspFrameAncestors string
	xFrameOptions     string // "" -> omit header
	reportURI         string
}

// resolvedEmbeddingPolicy is computed once at startup from env vars set by
// the platform Helm values (CHAT-pbup):
//
//	MOSES_EMBEDDING_FRAMING            "moses-only" | "public" | "denied" (default: "denied" for the api template)
//	MOSES_EMBEDDING_ALLOWED_ANCESTORS  space-separated CSP source list
//	MOSES_EMBEDDING_REPORT_URI         optional report-uri
var resolvedEmbeddingPolicy = func() embeddingPolicy {
	framing := os.Getenv("MOSES_EMBEDDING_FRAMING")
	if framing == "" {
		// API templates are never iframed — default to denied.
		framing = "denied"
	}
	allowed := os.Getenv("MOSES_EMBEDDING_ALLOWED_ANCESTORS")
	report := os.Getenv("MOSES_EMBEDDING_REPORT_URI")

	switch framing {
	case "public":
		return embeddingPolicy{cspFrameAncestors: "*", reportURI: report}
	case "denied":
		return embeddingPolicy{cspFrameAncestors: "'none'", xFrameOptions: "DENY", reportURI: report}
	default: // moses-only
		if allowed == "" {
			allowed = "'self'"
		}
		return embeddingPolicy{cspFrameAncestors: allowed, reportURI: report}
	}
}()

// WithEmbeddingHeaders attaches Content-Security-Policy: frame-ancestors
// (and X-Frame-Options for the strict "denied" case) to every response.
//
// Unlike browser-rendered HTML responses, API/JSON responses are never
// framed — but emitting the headers is harmless and gives security
// scanners a consistent answer per origin. The fullstack-unified template
// only attaches these on HTML routes; for the api-only backend-template
// we attach unconditionally.
func WithEmbeddingHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		policy := "frame-ancestors " + resolvedEmbeddingPolicy.cspFrameAncestors
		if resolvedEmbeddingPolicy.reportURI != "" {
			policy += "; report-uri " + resolvedEmbeddingPolicy.reportURI
		}
		w.Header().Set("Content-Security-Policy", policy)
		if resolvedEmbeddingPolicy.xFrameOptions != "" {
			w.Header().Set("X-Frame-Options", resolvedEmbeddingPolicy.xFrameOptions)
		}
		next.ServeHTTP(w, r)
	})
}
