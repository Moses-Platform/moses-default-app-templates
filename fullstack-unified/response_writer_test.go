package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type deadlineWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

func (w *deadlineWriter) SetWriteDeadline(d time.Time) error { w.deadline = d; return nil }

type basicWriter struct{ http.ResponseWriter }

func TestResponseWriterCapabilities(t *testing.T) {
	underlying := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	wrapped := &responseWriter{ResponseWriter: underlying, statusCode: 200}
	unwrapper, ok := any(wrapped).(interface{ Unwrap() http.ResponseWriter })
	if !ok {
		t.Fatal("missing Unwrap")
	}
	if unwrapper.Unwrap() != underlying {
		t.Fatal("Unwrap returned a different writer")
	}
	deadline := time.Now()
	if err := http.NewResponseController(wrapped).SetWriteDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if !underlying.deadline.Equal(deadline) {
		t.Fatal("deadline did not reach writer")
	}
	// A writer without flush support must not panic or falsely report success.
	unsupported := &responseWriter{ResponseWriter: basicWriter{httptest.NewRecorder()}}
	if err := http.NewResponseController(unsupported).Flush(); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("unsupported flush = %v", err)
	}
	if f, ok := any(unsupported).(http.Flusher); ok {
		f.Flush()
	} else {
		t.Fatal("missing Flush")
	}
	unsupported.WriteHeader(http.StatusNotFound)
	if unsupported.statusCode != http.StatusNotFound {
		t.Fatalf("unsupported flush committed status %d", unsupported.statusCode)
	}
}

// ResponseRecorder treats 1xx as final; model net/http's informational headers
// separately so these tests can exercise logging without opening sockets.
type informationalRecorder struct {
	*httptest.ResponseRecorder
	informational []int
}

func (w *informationalRecorder) WriteHeader(code int) {
	if code >= 100 && code < 200 {
		w.informational = append(w.informational, code)
		return
	}
	w.ResponseRecorder.WriteHeader(code)
}

func TestLoggingCommittedStatus(t *testing.T) {
	for _, tc := range []struct {
		name          string
		handle        func(http.ResponseWriter)
		want          int
		informational int
	}{
		{"flush then late header", func(w http.ResponseWriter) { w.(http.Flusher).Flush(); w.WriteHeader(500) }, 200, 0},
		{"controller flush then late header", func(w http.ResponseWriter) { _ = http.NewResponseController(w).Flush(); w.WriteHeader(500) }, 200, 0},
		{"write then late header", func(w http.ResponseWriter) { _, _ = w.Write([]byte("ok")); w.WriteHeader(500) }, 200, 0},
		{"final header then write", func(w http.ResponseWriter) { w.WriteHeader(404); _, _ = w.Write([]byte("missing")) }, 404, 0},
		{"first final header wins", func(w http.ResponseWriter) { w.WriteHeader(404); w.WriteHeader(500) }, 404, 0},
		{"informational then final", func(w http.ResponseWriter) { w.WriteHeader(103); w.WriteHeader(204) }, 204, 1},
		{"informational then write", func(w http.ResponseWriter) { w.WriteHeader(103); _, _ = w.Write([]byte("ok")) }, 200, 1},
		{"informational then flush", func(w http.ResponseWriter) { w.WriteHeader(103); w.(http.Flusher).Flush() }, 200, 1},
		{"informational only", func(w http.ResponseWriter) { w.WriteHeader(103) }, 200, 1},
		{"no write", func(http.ResponseWriter) {}, 200, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			old := log.Writer()
			log.SetOutput(&buf)
			defer log.SetOutput(old)
			recorder := &informationalRecorder{ResponseRecorder: httptest.NewRecorder()}
			Logging(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				tc.handle(w)
			})).ServeHTTP(recorder, httptest.NewRequest("GET", "/", nil))
			if got := recorder.Result().StatusCode; got != tc.want {
				t.Fatalf("response status = %d, want %d", got, tc.want)
			}
			if len(recorder.informational) != tc.informational {
				t.Fatalf("informational headers = %v, want %d", recorder.informational, tc.informational)
			}
			if !strings.Contains(buf.String(), fmt.Sprintf(" %d ", tc.want)) {
				t.Fatalf("log = %q, want status %d", buf.String(), tc.want)
			}
		})
	}
}

// net/http commits 200 before returning a transport error from FlushError.
type failingFlushWriter struct {
	*httptest.ResponseRecorder
	err error
}

func (w *failingFlushWriter) FlushError() error {
	w.ResponseRecorder.Flush()
	return w.err
}

func TestResponseWriterFlushErrorStatus(t *testing.T) {
	flushErr := errors.New("flush transport failure")
	underlying := &failingFlushWriter{ResponseRecorder: httptest.NewRecorder(), err: flushErr}
	wrapped := &responseWriter{ResponseWriter: underlying, statusCode: http.StatusOK}
	if err := http.NewResponseController(wrapped).Flush(); !errors.Is(err, flushErr) {
		t.Fatalf("flush error = %v, want %v", err, flushErr)
	}
	wrapped.WriteHeader(http.StatusInternalServerError)
	if wrapped.statusCode != underlying.Result().StatusCode {
		t.Fatalf("logged status = %d, committed status = %d", wrapped.statusCode, underlying.Result().StatusCode)
	}
}
