package main

import "net/http"

// responseWriter captures the response status while preserving streaming and
// access to optional capabilities through http.ResponseController.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Unwrap() http.ResponseWriter { return rw.ResponseWriter }

// Flush supports handlers using http.Flusher. Unsupported writers are a no-op;
// callers needing an error should use ResponseController.Flush instead.
func (rw *responseWriter) Flush() { _ = rw.FlushError() }

// ResponseController prefers FlushError to Flush, retaining ErrNotSupported
// and any underlying flush error rather than incorrectly reporting success.
func (rw *responseWriter) FlushError() error {
	return http.NewResponseController(rw.ResponseWriter).Flush()
}
