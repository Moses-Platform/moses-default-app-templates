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

// TestIsHeaderTrusted exercises the marker-gated header-trust predicate
// (CHAT-t5d1u.28.21 S3). Header-trust is honoured ONLY when both the
// X-Moses-User-ID header AND a matching X-Moses-Gateway-Auth marker are
// present, and only when the middleware has a GatewayAuthSecret set.
func TestIsHeaderTrusted(t *testing.T) {
	const marker = "shared-gateway-secret-123"
	mGated := New(Config{GatewayAuthSecret: marker})
	mDisabled := New(Config{}) // no GatewayAuthSecret -> path disabled

	// Valid: user header + matching marker, on a gated middleware.
	good := httptest.NewRequest("GET", "/api/v1/x", nil)
	good.Header.Set("X-Moses-User-ID", "user-123")
	good.Header.Set(HeaderGatewayAuth, marker)
	if !mGated.isHeaderTrusted(good) {
		t.Errorf("isHeaderTrusted should be true with a valid user header + matching marker")
	}

	// User header present but NO marker -> rejected.
	noMarker := httptest.NewRequest("GET", "/api/v1/x", nil)
	noMarker.Header.Set("X-Moses-User-ID", "user-123")
	if mGated.isHeaderTrusted(noMarker) {
		t.Errorf("isHeaderTrusted should be false when the gateway marker is absent")
	}

	// User header present but WRONG marker -> rejected.
	wrongMarker := httptest.NewRequest("GET", "/api/v1/x", nil)
	wrongMarker.Header.Set("X-Moses-User-ID", "user-123")
	wrongMarker.Header.Set(HeaderGatewayAuth, "not-the-secret")
	if mGated.isHeaderTrusted(wrongMarker) {
		t.Errorf("isHeaderTrusted should be false when the gateway marker mismatches")
	}

	// Marker present but NO user header -> rejected.
	noUser := httptest.NewRequest("GET", "/api/v1/x", nil)
	noUser.Header.Set(HeaderGatewayAuth, marker)
	if mGated.isHeaderTrusted(noUser) {
		t.Errorf("isHeaderTrusted should be false without X-Moses-User-ID")
	}

	// Blank user header (even with a valid marker) -> rejected.
	blankUser := httptest.NewRequest("GET", "/api/v1/x", nil)
	blankUser.Header.Set("X-Moses-User-ID", "   ")
	blankUser.Header.Set(HeaderGatewayAuth, marker)
	if mGated.isHeaderTrusted(blankUser) {
		t.Errorf("isHeaderTrusted should be false for a blank user header")
	}

	// GatewayAuthSecret unset -> header-trust path DISABLED entirely,
	// even with both headers present (fail-safe).
	bothHeaders := httptest.NewRequest("GET", "/api/v1/x", nil)
	bothHeaders.Header.Set("X-Moses-User-ID", "user-123")
	bothHeaders.Header.Set(HeaderGatewayAuth, marker)
	if mDisabled.isHeaderTrusted(bothHeaders) {
		t.Errorf("isHeaderTrusted should be false when no GatewayAuthSecret is configured")
	}
	if mDisabled.headerTrustEnabled() {
		t.Errorf("headerTrustEnabled should be false when no GatewayAuthSecret is configured")
	}
	if !mGated.headerTrustEnabled() {
		t.Errorf("headerTrustEnabled should be true when a GatewayAuthSecret is configured")
	}
}

// classify decision matrix — the dual-mode core, fully testable with no
// live Keycloak.
func TestClassify(t *testing.T) {
	const gatewayMarker = "classify-gateway-secret"
	baseCfg := Config{
		BasePath:       "/apps/t/s",
		ProtectedPaths: []string{"/api/private", "/admin"},
		PublicPaths:    []string{"/api/v1/info"},
		SpecPath:       "/api/openapi.json",
		Issuer:         "i", ClientID: "c", ClientSecret: "s",
		CookieSecret:      []byte("0123456789abcdef0123456789abcdef"),
		SecureCookie:      true,
		GatewayAuthSecret: gatewayMarker,
	}
	m := New(baseCfg)

	validSession := &Session{Subject: "u1", Expiry: time.Now().Add(time.Hour).Unix()}

	type tc struct {
		name         string
		path         string
		headerUser   string
		marker       string // X-Moses-Gateway-Auth value; "" => header absent
		sess         *Session
		sessionValid bool
		want         decision
	}
	cases := []tc{
		{"health is public", "/apps/t/s/health", "", "", nil, false, decisionPublic},
		{"openapi spec is public", "/apps/t/s/api/openapi.json", "", "", nil, false, decisionPublic},
		{"callback route is public", "/apps/t/s/auth/callback", "", "", nil, false, decisionPublic},
		{"login route is public", "/apps/t/s/auth/login", "", "", nil, false, decisionPublic},
		{"explicit public path", "/apps/t/s/api/v1/info", "", "", nil, false, decisionPublic},
		{"pod-to-pod header + valid marker bypasses OIDC on protected path", "/apps/t/s/api/private/x", "svc-user", gatewayMarker, nil, false, decisionHeaderTrust},
		{"header trust + marker wins even with no session on protected path", "/apps/t/s/admin", "svc-user", gatewayMarker, nil, false, decisionHeaderTrust},
		{"X-Moses-User-ID without marker -> NOT header-trust -> challenge", "/apps/t/s/api/private/x", "svc-user", "", nil, false, decisionChallenge},
		{"X-Moses-User-ID with WRONG marker -> NOT header-trust -> challenge", "/apps/t/s/api/private/x", "svc-user", "wrong-secret", nil, false, decisionChallenge},
		{"valid session on protected path", "/apps/t/s/api/private/x", "", "", validSession, true, decisionSession},
		{"no session, protected path, browser -> challenge", "/apps/t/s/api/private/x", "", "", nil, false, decisionChallenge},
		{"no session, /admin protected -> challenge", "/apps/t/s/admin/users", "", "", nil, false, decisionChallenge},
		{"unprotected path, no creds -> public", "/apps/t/s/some/page", "", "", nil, false, decisionPublic},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", c.path, nil)
			if c.headerUser != "" {
				r.Header.Set("X-Moses-User-ID", c.headerUser)
			}
			if c.marker != "" {
				r.Header.Set(HeaderGatewayAuth, c.marker)
			}
			got, _ := m.classify(r, c.sess, c.sessionValid)
			if got != c.want {
				t.Errorf("classify(%q) = %d, want %d", c.path, got, c.want)
			}
		})
	}
}

// TestClassify_HeaderTrustDisabledWithoutSecret confirms the fail-safe:
// when no GatewayAuthSecret is configured, an X-Moses-* header (even
// with a marker) does NOT win header-trust — the request falls through
// to the OIDC challenge on a protected path.
func TestClassify_HeaderTrustDisabledWithoutSecret(t *testing.T) {
	m := New(Config{
		BasePath:       "/apps/t/s",
		ProtectedPaths: []string{"/api/private"},
		SpecPath:       "/api/openapi.json",
		Issuer:         "i", ClientID: "c", ClientSecret: "s",
		CookieSecret: []byte("0123456789abcdef0123456789abcdef"),
		// GatewayAuthSecret deliberately unset.
	})
	r := httptest.NewRequest("GET", "/apps/t/s/api/private/x", nil)
	r.Header.Set("X-Moses-User-ID", "svc-user")
	r.Header.Set(HeaderGatewayAuth, "any-marker")
	if got, _ := m.classify(r, nil, false); got != decisionChallenge {
		t.Errorf("header-trust disabled: classify = %d, want decisionChallenge", got)
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
	if got, _ := m.classify(r, nil, false); got != decisionChallenge {
		t.Errorf("deny-by-default: classify = %d, want decisionChallenge", got)
	}

	rPublic := httptest.NewRequest("GET", "/api/v1/info", nil)
	if got, _ := m.classify(rPublic, nil, false); got != decisionPublic {
		t.Errorf("explicit public path: classify = %d, want decisionPublic", got)
	}

	rHealth := httptest.NewRequest("GET", "/health", nil)
	if got, _ := m.classify(rHealth, nil, false); got != decisionPublic {
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
