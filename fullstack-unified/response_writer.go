package main

import (
	"errors"
	"net/http"
)

// responseWriter captures the response status while preserving streaming and
// access to optional capabilities through http.ResponseController.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	committed  bool
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.ResponseWriter.WriteHeader(code)
	if code < 100 || code >= 200 {
		rw.recordStatus(code)
	}
}

func (rw *responseWriter) recordStatus(code int) {
	if !rw.committed {
		rw.statusCode = code
		rw.committed = true
	}
}

func (rw *responseWriter) Write(p []byte) (int, error) {
	rw.recordStatus(http.StatusOK)
	return rw.ResponseWriter.Write(p)
}

func (rw *responseWriter) Unwrap() http.ResponseWriter { return rw.ResponseWriter }

// Flush supports handlers using http.Flusher. Unsupported writers are a no-op;
// callers needing an error should use ResponseController.Flush instead.
func (rw *responseWriter) Flush() { _ = rw.FlushError() }

// ResponseController prefers FlushError to Flush, retaining ErrNotSupported
// and any underlying flush error rather than incorrectly reporting success.
func (rw *responseWriter) FlushError() error {
	err := http.NewResponseController(rw.ResponseWriter).Flush()
	// A supported flush commits the headers even if flushing the body fails.
	// Unsupported flushes remain a no-op, so a later header can still be final.
	if !errors.Is(err, http.ErrNotSupported) {
		rw.recordStatus(http.StatusOK)
	}
	return err
}
