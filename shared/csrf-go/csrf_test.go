package csrf

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler is the wrapped target; a 200 means the guard let the request
// through.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRejectCrossSiteCSRF_Matrix(t *testing.T) {
	cases := []struct {
		method string
		site   string // Sec-Fetch-Site value; "-" means header absent
		want   int
	}{
		// Safe methods always pass, regardless of site (cross-site GET is an
		// embed-mode iframe navigation and must work).
		{http.MethodGet, "cross-site", http.StatusOK},
		{http.MethodGet, "same-site", http.StatusOK},
		{http.MethodHead, "cross-site", http.StatusOK},
		{http.MethodOptions, "cross-site", http.StatusOK},

		// Unsafe methods: allowed only for same-origin / none / absent.
		{http.MethodPost, "same-origin", http.StatusOK},
		{http.MethodPost, "none", http.StatusOK},
		{http.MethodPost, "-", http.StatusOK},
		{http.MethodPut, "same-origin", http.StatusOK},
		{http.MethodDelete, "-", http.StatusOK},

		// Unsafe methods: blocked for cross-site / same-site.
		{http.MethodPost, "cross-site", http.StatusForbidden},
		{http.MethodPost, "same-site", http.StatusForbidden},
		{http.MethodPut, "cross-site", http.StatusForbidden},
		{http.MethodPatch, "cross-site", http.StatusForbidden},
		{http.MethodDelete, "cross-site", http.StatusForbidden},
	}

	h := RejectCrossSiteCSRF(okHandler())
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
