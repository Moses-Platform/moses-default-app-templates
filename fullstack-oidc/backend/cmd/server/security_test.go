package main

import (
	"github.com/moses-platform/fullstack-oidc/internal/oidcauth"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFBeforeOIDC(t *testing.T) {
	cfg := oidcauth.Config{Issuer: "https://idp.invalid", ClientID: "app", ClientSecret: "secret", CookieSecret: []byte("01234567890123456789012345678901"), GatewayAuthSecret: "gateway", ProtectedPaths: []string{"/private"}}
	for _, tc := range []struct {
		name, method, path, site string
		marker                   bool
		want                     int
	}{
		{"cross-site public POST", "POST", "/health", "cross-site", false, 403},
		{"cross-site forged identity", "POST", "/private", "cross-site", true, 403},
		{"same-origin public POST", "POST", "/health", "same-origin", false, 204},
		{"same-origin gateway POST", "POST", "/private", "same-origin", true, 204},
		{"safe cross-site GET", "GET", "/health", "cross-site", false, 204},
		{"safe cross-site HEAD", "HEAD", "/health", "cross-site", false, 204},
		{"callback reaches state validation", "GET", "/auth/callback", "cross-site", false, 400},
		{"unsafe logout", "POST", "/auth/logout", "cross-site", false, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := buildHandler(oidcauth.New(cfg), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r.Header.Set("Sec-Fetch-Site", tc.site)
			if tc.marker {
				r.Header.Set("X-Moses-Gateway-Auth", "gateway")
				r.Header.Set("X-Moses-User-ID", "user")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d", w.Code, tc.want)
			}
		})
	}
}
