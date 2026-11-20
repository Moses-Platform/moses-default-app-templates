package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingEscapesRequestValues(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)
	r := httptest.NewRequest("GET", "/", nil)
	r.URL.Path = "/evil\nFORGED\r\t\x1b"
	r.RemoteAddr = "host\nFORGED"
	r.Header.Set("X-Moses-Request-ID", "id\nFORGED")
	r.Header.Set("X-Moses-Tenant-ID", "tenant\nFORGED")
	h := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))

	h.ServeHTTP(httptest.NewRecorder(), r)
	if strings.Count(buf.String(), "\n") != 1 || strings.ContainsAny(buf.String(), "\r\t\x1b") {
		t.Fatalf("unescaped log: %q", buf.String())
	}
	if !strings.Contains(buf.String(), `\nFORGED`) {
		t.Fatalf("missing escaped value: %q", buf.String())
	}
}
