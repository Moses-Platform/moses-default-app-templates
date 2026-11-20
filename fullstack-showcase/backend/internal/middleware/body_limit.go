package middleware

import (
	"net/http"
	"os"
	"strconv"
)

// LimitRequestBody caps reads at 1 MiB by default. MOSES_MAX_BODY_BYTES may
// override the cap with a positive byte count; invalid values retain the default.
// Handlers must handle *http.MaxBytesError (413) before committing a response
// or side effects. Unknown-length bodies are limited while read, without buffering.
// More restrictive endpoint limits (e.g. webhooks) still apply.
func LimitRequestBody(next http.Handler) http.Handler {
	const defaultLimit int64 = 1 << 20
	limit, err := strconv.ParseInt(os.Getenv("MOSES_MAX_BODY_BYTES"), 10, 64)
	if err != nil || limit <= 0 {
		limit = defaultLimit
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > limit {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}
