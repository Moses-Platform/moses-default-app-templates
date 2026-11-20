package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Observe each flush before the handler returns, without requiring a listening
// socket. Both the legacy Flusher and ResponseController paths must stream.
func testStreaming(t *testing.T, wrap func(http.Handler) http.Handler) {
	t.Helper()
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://stream.example")
	for _, controller := range []bool{false, true} {
		t.Run(map[bool]string{false: "Flusher", true: "ResponseController"}[controller], func(t *testing.T) {
			recorder := httptest.NewRecorder()
			h := wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				for _, chunk := range []string{"data: first\n\n", "data: second\n\n"} {
					recorder.Flushed = false
					if _, err := w.Write([]byte(chunk)); err != nil {
						t.Fatal(err)
					}
					if controller {
						if err := http.NewResponseController(w).Flush(); err != nil {
							t.Fatalf("controller flush: %v", err)
						}
					} else {
						f, ok := w.(http.Flusher)
						if !ok {
							t.Fatal("middleware hides http.Flusher")
						}
						f.Flush()
					}
					if !recorder.Flushed {
						t.Fatal("flush did not reach underlying writer before handler returned")
					}
				}
			}))
			req := httptest.NewRequest("GET", "/health", nil)
			req.Header.Set("Origin", "https://stream.example")
			h.ServeHTTP(recorder, req)
			if recorder.Code != 200 || recorder.Body.String() != "data: first\n\ndata: second\n\n" {
				t.Fatalf("stream response: %d %q", recorder.Code, recorder.Body.String())
			}
		})
	}
}
