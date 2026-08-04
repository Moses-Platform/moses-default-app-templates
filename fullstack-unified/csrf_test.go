package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRejectCrossSiteCSRF_Matrix(t *testing.T) {
	cases := []struct {
		method string
		site   string // Sec-Fetch-Site value; "-" means header absent
		want   int
	}{
		// Safe methods always pass (cross-site GET = embed-mode iframe nav).
		{http.MethodGet, "cross-site", http.StatusOK},
		{http.MethodHead, "cross-site", http.StatusOK},
		{http.MethodOptions, "cross-site", http.StatusOK},
		// Unsafe methods allowed for same-origin / none / absent.
		{http.MethodPost, "same-origin", http.StatusOK},
		{http.MethodPost, "none", http.StatusOK},
		{http.MethodPost, "-", http.StatusOK},
		{http.MethodDelete, "-", http.StatusOK},
		// Unsafe methods blocked for cross-site / same-site.
		{http.MethodPost, "cross-site", http.StatusForbidden},
		{http.MethodPost, "same-site", http.StatusForbidden},
		{http.MethodPut, "cross-site", http.StatusForbidden},
		{http.MethodPatch, "cross-site", http.StatusForbidden},
		{http.MethodDelete, "cross-site", http.StatusForbidden},
	}

	h := RejectCrossSiteCSRF(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for _, c := range cases {
		req := httptest.NewRequest(c.method, "/api/v1/items", nil)
		if c.site != "-" {
			req.Header.Set("Sec-Fetch-Site", c.site)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s Sec-Fetch-Site=%q: got %d, want %d", c.method, c.site, rec.Code, c.want)
		}
	}
}
