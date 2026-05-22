package oidcauth

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestTrimBasePath(t *testing.T) {
	cases := []struct {
		reqPath, basePath, want string
	}{
		{"/api/v1/x", "", "/api/v1/x"},
		{"/apps/t/s/api/v1/x", "/apps/t/s", "/api/v1/x"},
		{"/apps/t/s/api/v1/x", "/apps/t/s/", "/api/v1/x"},
		{"/apps/t/s", "/apps/t/s", "/"},
		{"/apps/t/s/", "/apps/t/s", "/"},
		{"/health", "/apps/t/s", "/health"}, // not under base -> unchanged
		{"", "", "/"},
		{"api/v1", "", "/api/v1"}, // leading slash added
	}
	for _, c := range cases {
		got := trimBasePath(c.reqPath, c.basePath)
		if got != c.want {
			t.Errorf("trimBasePath(%q,%q) = %q, want %q", c.reqPath, c.basePath, got, c.want)
		}
	}
}

func TestMatchesPrefix(t *testing.T) {
	prefixes := []string{"/api", "/admin/"}
	cases := []struct {
		path string
		want bool
	}{
		{"/api", true},
		{"/api/v1/x", true},
		{"/apiv2", false}, // boundary respected — not a prefix match
		{"/admin", true},
		{"/admin/users", true},
		{"/public", false},
		{"/", false},
	}
	for _, c := range cases {
		if got := matchesPrefix(c.path, prefixes); got != c.want {
			t.Errorf("matchesPrefix(%q) = %v, want %v", c.path, got, c.want)
		}
	}
	// A "/" prefix matches everything.
	if !matchesPrefix("/anything", []string{"/"}) {
		t.Errorf(`matchesPrefix with "/" prefix should match all`)
	}
}

func TestIsHeaderTrusted(t *testing.T) {
	withHeader := httptest.NewRequest("GET", "/api/v1/x", nil)
	withHeader.Header.Set("X-Moses-User-ID", "user-123")
	if !isHeaderTrusted(withHeader) {
		t.Errorf("isHeaderTrusted should be true when X-Moses-User-ID is set")
	}

	noHeader := httptest.NewRequest("GET", "/api/v1/x", nil)
	if isHeaderTrusted(noHeader) {
		t.Errorf("isHeaderTrusted should be false without X-Moses-User-ID")
	}

	blankHeader := httptest.NewRequest("GET", "/api/v1/x", nil)
	blankHeader.Header.Set("X-Moses-User-ID", "   ")
	if isHeaderTrusted(blankHeader) {
		t.Errorf("isHeaderTrusted should be false for a blank header")
	}
}

// classify decision matrix — the dual-mode core, fully testable with no
// live Keycloak.
func TestClassify(t *testing.T) {
	baseCfg := Config{
		BasePath:       "/apps/t/s",
		ProtectedPaths: []string{"/api/private", "/admin"},
		PublicPaths:    []string{"/api/v1/info"},
		SpecPath:       "/api/openapi.json",
		Issuer:         "i", ClientID: "c", ClientSecret: "s",
		CookieSecret: []byte("0123456789abcdef0123456789abcdef"),
		SecureCookie: true,
	}
	m := New(baseCfg)

	validSession := &Session{Subject: "u1", Expiry: time.Now().Add(time.Hour).Unix()}

	type tc struct {
		name         string
		path         string
		headerUser   string
		sess         *Session
		sessionValid bool
		want         decision
	}
	cases := []tc{
		{"health is public", "/apps/t/s/health", "", nil, false, decisionPublic},
		{"openapi spec is public", "/apps/t/s/api/openapi.json", "", nil, false, decisionPublic},
		{"callback route is public", "/apps/t/s/auth/callback", "", nil, false, decisionPublic},
		{"login route is public", "/apps/t/s/auth/login", "", nil, false, decisionPublic},
		{"explicit public path", "/apps/t/s/api/v1/info", "", nil, false, decisionPublic},
		{"pod-to-pod header bypasses OIDC on protected path", "/apps/t/s/api/private/x", "svc-user", nil, false, decisionHeaderTrust},
		{"header trust wins even with no session on protected path", "/apps/t/s/admin", "svc-user", nil, false, decisionHeaderTrust},
		{"valid session on protected path", "/apps/t/s/api/private/x", "", validSession, true, decisionSession},
		{"no session, protected path, browser -> challenge", "/apps/t/s/api/private/x", "", nil, false, decisionChallenge},
		{"no session, /admin protected -> challenge", "/apps/t/s/admin/users", "", nil, false, decisionChallenge},
		{"unprotected path, no creds -> public", "/apps/t/s/some/page", "", nil, false, decisionPublic},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", c.path, nil)
			if c.headerUser != "" {
				r.Header.Set("X-Moses-User-ID", c.headerUser)
			}
			got := m.classify(r, c.sess, c.sessionValid)
			if got != c.want {
				t.Errorf("classify(%q) = %d, want %d", c.path, got, c.want)
			}
		})
	}
}

// With ProtectedPaths empty the policy is deny-by-default: everything
// not explicitly public is protected.
func TestClassify_DenyByDefault(t *testing.T) {
	cfg := Config{
		BasePath:     "",
		PublicPaths:  []string{"/api/v1/info"},
		SpecPath:     "/api/openapi.json",
		Issuer:       "i", ClientID: "c", ClientSecret: "s",
		CookieSecret: []byte("0123456789abcdef0123456789abcdef"),
	}
	m := New(cfg)

	r := httptest.NewRequest("GET", "/some/random/page", nil)
	if got := m.classify(r, nil, false); got != decisionChallenge {
		t.Errorf("deny-by-default: classify = %d, want decisionChallenge", got)
	}

	rPublic := httptest.NewRequest("GET", "/api/v1/info", nil)
	if got := m.classify(rPublic, nil, false); got != decisionPublic {
		t.Errorf("explicit public path: classify = %d, want decisionPublic", got)
	}

	rHealth := httptest.NewRequest("GET", "/health", nil)
	if got := m.classify(rHealth, nil, false); got != decisionPublic {
		t.Errorf("/health: classify = %d, want decisionPublic", got)
	}
}

func TestPathIsProtected(t *testing.T) {
	// Empty ProtectedPaths -> everything protected.
	mAll := New(Config{})
	if !mAll.pathIsProtected("/anything") {
		t.Errorf("empty ProtectedPaths should protect everything")
	}
	// Non-empty -> only listed prefixes.
	mSome := New(Config{ProtectedPaths: []string{"/api/private"}})
	if !mSome.pathIsProtected("/api/private/x") {
		t.Errorf("/api/private/x should be protected")
	}
	if mSome.pathIsProtected("/api/public") {
		t.Errorf("/api/public should NOT be protected")
	}
}
