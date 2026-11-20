package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
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
}
