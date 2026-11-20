package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLimitRequestBody(t *testing.T) {
	for _, tc := range []struct {
		name, limit     string
		size            int
		unknown, reject bool
	}{
		{"at limit", "4", 4, false, false},
		{"known oversized", "4", 5, false, true},
		{"chunked oversized", "4", 5, true, true},
		{"default", "", 1<<20 + 1, true, true},
		{"invalid override", "bad", 1<<20 + 1, true, true},
		{"zero override", "0", 1<<20 + 1, true, true},
		{"negative override", "-1", 1<<20 + 1, true, true},
		{"override", "2097152", 1<<20 + 1, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MOSES_MAX_BODY_BYTES", tc.limit)
			h := LimitRequestBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := io.ReadAll(r.Body)
				var maxErr *http.MaxBytesError
				if errors.As(err, &maxErr) {
					w.WriteHeader(413)
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				w.WriteHeader(204)
			}))
			r := httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("x", tc.size)))
			if tc.unknown {
				r.ContentLength = -1
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			want := 204
			if tc.reject {
				want = 413
			}
			if w.Code != want {
				t.Fatalf("got %d want %d", w.Code, want)
			}
		})
	}
}
