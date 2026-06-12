package oidcauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testGatewaySecret is the shared-secret marker used across middleware
// tests to exercise the CHAT-t5d1u.28.21 (S3) header-trust path.
const testGatewaySecret = "test-gateway-auth-secret"

func enabledCfg() Config {
	return Config{
		Issuer:            "https://kc.example.com/auth/realms/moses",
		InternalIssuer:    "http://keycloak.moses.svc:8080/auth/realms/moses",
		ClientID:          "app-client",
		ClientSecret:      "secret",
		BasePath:          "/apps/t/s",
		ProtectedPaths:    []string{"/api/private"},
		SpecPath:          "/api/openapi.json",
		CookieSecret:      testSecret,
		SecureCookie:      true,
		GatewayAuthSecret: testGatewaySecret,
	}
}

// next is a trivial handler that records the Identity it received.
func recordingHandler(captured *Identity) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*captured = IdentityFrom(r.Context())
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestHandler_PublicPathServedNoAuth(t *testing.T) {
	m := New(enabledCfg())
	var got Identity
	h := m.Handler(recordingHandler(&got))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/apps/t/s/health", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("public path status = %d, want 200", rec.Code)
	}
	if got.Authenticated {
		t.Errorf("public path should not produce an authenticated identity")
	}
}

func TestHandler_HeaderTrustBypassesOIDC(t *testing.T) {
	m := New(enabledCfg())
	var got Identity
	h := m.Handler(recordingHandler(&got))

	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.Header.Set("X-Moses-User-ID", "svc-user-9")
	r.Header.Set(HeaderGatewayAuth, testGatewaySecret)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("header-trust path status = %d, want 200 (no redirect)", rec.Code)
	}
	if !got.Authenticated || got.Source != "moses-headers" {
		t.Errorf("expected moses-headers identity, got %+v", got)
	}
	if got.Subject != "svc-user-9" {
		t.Errorf("Subject = %q, want svc-user-9", got.Subject)
	}
	if got.Roles != nil {
		t.Errorf("Roles = %v, want nil when no X-Moses-Roles header is sent", got.Roles)
	}
}

// TestHandler_HeaderTrustLiftsAgentRoles confirms the agent-role-grants
// contract: X-Moses-Roles (set by the platform's WorkspaceToolProxy under
// the gateway-auth marker) is lifted onto Identity.Roles so role-gated
// handlers can authorise explicitly-granted agent calls.
func TestHandler_HeaderTrustLiftsAgentRoles(t *testing.T) {
	m := New(enabledCfg())
	var got Identity
	h := m.Handler(recordingHandler(&got))

	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.Header.Set("X-Moses-User-ID", "svc-user-9")
	r.Header.Set(HeaderGatewayAuth, testGatewaySecret)
	r.Header.Set(HeaderMosesRoles, " oidc-admin, oidc-member ,, ")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("header-trust path status = %d, want 200", rec.Code)
	}
	want := []string{"oidc-admin", "oidc-member"}
	if len(got.Roles) != len(want) || got.Roles[0] != want[0] || got.Roles[1] != want[1] {
		t.Errorf("Roles = %v, want %v (trimmed, empties dropped)", got.Roles, want)
	}
	if !got.HasRole("oidc-admin") {
		t.Errorf("HasRole(oidc-admin) = false, want true")
	}
}

// TestHandler_RolesHeaderIgnoredWithoutMarker confirms X-Moses-Roles is
// worthless without the gateway-auth marker: the request never reaches the
// header-trust branch, so a forged roles header cannot mint authorisation.
func TestHandler_RolesHeaderIgnoredWithoutMarker(t *testing.T) {
	m := New(enabledCfg())
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("next handler must not run — forged roles without the marker must be rejected")
	}))

	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.Header.Set("X-Moses-User-ID", "forged-user")
	r.Header.Set(HeaderMosesRoles, "oidc-admin")
	r.Header.Set("Sec-Fetch-Mode", "navigate")
	// No X-Moses-Gateway-Auth header.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusFound {
		t.Errorf("forged X-Moses-Roles without marker status = %d, want 302 (challenge)", rec.Code)
	}
}

// TestHandler_HeaderTrustRejectedWithoutMarker confirms the CHAT-t5d1u.28.21
// (S3) gate: an X-Moses-* header without the X-Moses-Gateway-Auth marker
// is NOT trusted — a protected-path navigation is redirected to login.
func TestHandler_HeaderTrustRejectedWithoutMarker(t *testing.T) {
	m := New(enabledCfg())
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("next handler must not run — header-trust must be rejected without the marker")
	}))

	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.Header.Set("X-Moses-User-ID", "forged-user")
	r.Header.Set("Sec-Fetch-Mode", "navigate")
	// No X-Moses-Gateway-Auth header.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusFound {
		t.Errorf("forged X-Moses-* without marker status = %d, want 302 (challenge)", rec.Code)
	}
}

// TestHandler_HeaderTrustRejectedWithWrongMarker confirms a wrong marker
// value is rejected (constant-time compare miss).
func TestHandler_HeaderTrustRejectedWithWrongMarker(t *testing.T) {
	m := New(enabledCfg())
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("next handler must not run — a wrong marker must be rejected")
	}))

	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.Header.Set("X-Moses-User-ID", "forged-user")
	r.Header.Set(HeaderGatewayAuth, "wrong-secret-value")
	r.Header.Set("Sec-Fetch-Mode", "navigate")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusFound {
		t.Errorf("X-Moses-* with wrong marker status = %d, want 302 (challenge)", rec.Code)
	}
}

func TestHandler_ValidSessionServed(t *testing.T) {
	cfg := enabledCfg()
	m := New(cfg)
	var got Identity
	h := m.Handler(recordingHandler(&got))

	sess := &Session{
		Subject: "u-1", Email: "u@x.io", Roles: []string{"editor"},
		Expiry: time.Now().Add(time.Hour).Unix(),
	}
	cookieVal, _ := encodeSession(sess, cfg.CookieSecret)

	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.AddCookie(&http.Cookie{Name: cfg.sessionCookieName(), Value: cookieVal})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("valid-session status = %d, want 200", rec.Code)
	}
	if !got.Authenticated || got.Source != "session" {
		t.Errorf("expected session identity, got %+v", got)
	}
	if !got.HasRole("editor") {
		t.Errorf("roles not propagated to Identity: %v", got.Roles)
	}
}

func TestHandler_ProtectedNoSession_BrowserRedirects(t *testing.T) {
	m := New(enabledCfg())
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("next handler should not run for an unauth protected request")
	}))

	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.Header.Set("Sec-Fetch-Mode", "navigate") // top-level navigation
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusFound {
		t.Errorf("protected unauth navigation status = %d, want 302", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/apps/t/s/auth/login") {
		t.Errorf("redirect Location = %q, want it to point at /auth/login", loc)
	}
}

func TestHandler_ProtectedNoSession_XHRGets401JSON(t *testing.T) {
	m := New(enabledCfg())
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("next handler should not run")
	}))

	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("XHR unauth status = %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"error":"unauthenticated"`) {
		t.Errorf("401 body = %q, want unauthenticated JSON", rec.Body.String())
	}
}

func TestHandler_PassThroughModeWhenDisabled(t *testing.T) {
	// OIDC env not set -> Enabled() false -> pass-through.
	m := New(Config{BasePath: "/apps/t/s", CookieSecret: testSecret})
	var got Identity
	h := m.Handler(recordingHandler(&got))

	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("pass-through status = %d, want 200 (no redirect when OIDC disabled)", rec.Code)
	}
	if got.Authenticated {
		t.Errorf("pass-through with no headers should not be authenticated")
	}
}

// TestHandler_PassThroughStillHonoursHeaderTrust confirms that in
// pass-through mode (OIDC not configured) the marker-gated header-trust
// path still works — provided a GatewayAuthSecret is set and the
// X-Moses-Gateway-Auth marker matches (CHAT-t5d1u.28.21 S3).
func TestHandler_PassThroughStillHonoursHeaderTrust(t *testing.T) {
	m := New(Config{BasePath: "", CookieSecret: testSecret, GatewayAuthSecret: testGatewaySecret})
	var got Identity
	h := m.Handler(recordingHandler(&got))

	r := httptest.NewRequest("GET", "/api/private/data", nil)
	r.Header.Set("X-Moses-User-ID", "svc-7")
	r.Header.Set(HeaderGatewayAuth, testGatewaySecret)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if !got.Authenticated || got.Subject != "svc-7" {
		t.Errorf("pass-through mode should still honour marker-gated header-trust: %+v", got)
	}
}

// TestHandler_PassThroughHeaderTrustDisabledWithoutSecret confirms the
// fail-safe in pass-through mode: with no GatewayAuthSecret configured,
// an X-Moses-* header (even with a marker) is NOT trusted.
func TestHandler_PassThroughHeaderTrustDisabledWithoutSecret(t *testing.T) {
	m := New(Config{BasePath: "", CookieSecret: testSecret}) // no GatewayAuthSecret
	var got Identity
	h := m.Handler(recordingHandler(&got))

	r := httptest.NewRequest("GET", "/api/private/data", nil)
	r.Header.Set("X-Moses-User-ID", "svc-7")
	r.Header.Set(HeaderGatewayAuth, "any-marker")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if got.Authenticated {
		t.Errorf("pass-through with no GatewayAuthSecret must NOT honour header-trust: %+v", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("pass-through status = %d, want 200 (still served un-gated)", rec.Code)
	}
}

func TestHandler_LoginRouteRedirectsToDiscovery(t *testing.T) {
	// /auth/login triggers discovery; with a bogus issuer discovery
	// fails and the handler returns 502 — that proves the route is
	// owned by the middleware (not passed through).
	m := New(enabledCfg())
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("next handler must not run for /auth/login")
	}))
	r := httptest.NewRequest("GET", "/apps/t/s/auth/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusBadGateway {
		t.Errorf("login with unreachable issuer status = %d, want 502", rec.Code)
	}
}

func TestHandler_LogoutClearsCookieEvenIfProviderDown(t *testing.T) {
	m := New(enabledCfg())
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("next handler must not run for /auth/logout")
	}))
	r := httptest.NewRequest("GET", "/apps/t/s/auth/logout", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	// Cookie must be cleared regardless of provider reachability.
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == enabledCfg().sessionCookieName() && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Errorf("logout did not clear the session cookie")
	}
	if rec.Code != http.StatusFound {
		t.Errorf("logout status = %d, want 302", rec.Code)
	}
}

func TestHandler_CallbackRejectsMissingState(t *testing.T) {
	m := New(enabledCfg())
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	r := httptest.NewRequest("GET", "/apps/t/s/auth/callback?code=abc&state=xyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("callback without state cookie status = %d, want 400", rec.Code)
	}
}

func TestHandler_CallbackRejectsStateMismatch(t *testing.T) {
	cfg := enabledCfg()
	m := New(cfg)
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	// Build a valid state cookie carrying state "real-state".
	stateRec := httptest.NewRecorder()
	_ = setStateCookie(stateRec, cfg, &handshakeState{
		State: "real-state", CodeVerifier: "v", ReturnTo: "/apps/t/s/",
	})
	stateCookie := stateRec.Result().Cookies()[0]

	// Callback presents a DIFFERENT state in the query.
	r := httptest.NewRequest("GET", "/apps/t/s/auth/callback?code=abc&state=forged", nil)
	r.AddCookie(stateCookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("callback with state mismatch status = %d, want 400", rec.Code)
	}
}

func TestHandler_SilentCheckErrorRedirectsGracefully(t *testing.T) {
	cfg := enabledCfg()
	m := New(cfg)
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	// State cookie marked as a silent (prompt=none) attempt.
	stateRec := httptest.NewRecorder()
	_ = setStateCookie(stateRec, cfg, &handshakeState{
		State: "s1", CodeVerifier: "v", ReturnTo: "/apps/t/s/dashboard", Prompt: "none",
	})
	stateCookie := stateRec.Result().Cookies()[0]

	// Keycloak returns error=login_required for a failed silent SSO.
	r := httptest.NewRequest("GET", "/apps/t/s/auth/callback?error=login_required&state=s1", nil)
	r.AddCookie(stateCookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusFound {
		t.Errorf("failed silent SSO status = %d, want 302 (graceful bounce)", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "silent_sso=failed") {
		t.Errorf("silent-fail redirect Location = %q, want silent_sso=failed marker", loc)
	}
}

func TestIdentityFrom_ZeroValueWhenAbsent(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	id := IdentityFrom(r.Context())
	if id.Authenticated {
		t.Errorf("IdentityFrom on a bare context should be unauthenticated")
	}
}

func TestIdentity_HasRole(t *testing.T) {
	id := Identity{Roles: []string{"a", "b"}}
	if !id.HasRole("b") {
		t.Errorf("HasRole(b) = false")
	}
	if id.HasRole("c") {
		t.Errorf("HasRole(c) = true")
	}
}

func TestSessionFromClaims_ExpiryIsCapped(t *testing.T) {
	cfg := enabledCfg()
	// Token exp far in the future — session must be capped to <= 8h.
	c := &Claims{
		Subject: "u", Expiry: time.Now().Add(72 * time.Hour).Unix(),
	}
	s := sessionFromClaims(c, cfg, 0)
	maxExp := time.Now().Add(8*time.Hour + time.Minute)
	if time.Unix(s.Expiry, 0).After(maxExp) {
		t.Errorf("session expiry not capped: %v", time.Unix(s.Expiry, 0))
	}
}

func TestSessionFromClaims_TokenExpWins(t *testing.T) {
	cfg := enabledCfg()
	tokExp := time.Now().Add(10 * time.Minute)
	c := &Claims{Subject: "u", Expiry: tokExp.Unix()}
	s := sessionFromClaims(c, cfg, 0)
	if s.Expiry != tokExp.Unix() {
		t.Errorf("short token exp should win over the 8h cap")
	}
}

func TestSessionFromClaims_RolesProjected(t *testing.T) {
	cfg := enabledCfg() // ClientID app-client
	c := &Claims{
		Subject: "u", Expiry: time.Now().Add(time.Hour).Unix(),
		ResourceAccess: map[string]struct {
			Roles []string `json:"roles"`
		}{
			"app-client": {Roles: []string{"viewer"}},
			"other":      {Roles: []string{"ignored"}},
		},
	}
	s := sessionFromClaims(c, cfg, 0)
	if len(s.Roles) != 1 || s.Roles[0] != "viewer" {
		t.Errorf("session roles = %v, want [viewer] (only app-client's roles)", s.Roles)
	}
}

func TestWantsJSON(t *testing.T) {
	jsonReq := httptest.NewRequest("GET", "/", nil)
	jsonReq.Header.Set("Accept", "application/json")
	if !wantsJSON(jsonReq) {
		t.Errorf("Accept: application/json should be detected as JSON")
	}

	xhrReq := httptest.NewRequest("GET", "/", nil)
	xhrReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	if !wantsJSON(xhrReq) {
		t.Errorf("X-Requested-With should be detected as JSON")
	}

	navReq := httptest.NewRequest("GET", "/", nil)
	navReq.Header.Set("Sec-Fetch-Mode", "navigate")
	if wantsJSON(navReq) {
		t.Errorf("a top-level navigation should NOT be detected as JSON")
	}

	corsReq := httptest.NewRequest("GET", "/", nil)
	corsReq.Header.Set("Sec-Fetch-Mode", "cors")
	if !wantsJSON(corsReq) {
		t.Errorf("Sec-Fetch-Mode: cors should be detected as JSON")
	}
}

func TestRoute(t *testing.T) {
	m := New(Config{BasePath: "/apps/t/s"})
	if got := m.route(routeLogin); got != "/apps/t/s/auth/login" {
		t.Errorf("route = %q", got)
	}
	mRoot := New(Config{BasePath: ""})
	if got := mRoot.route(routeLogin); got != "/auth/login" {
		t.Errorf("root-deploy route = %q", got)
	}
}

func TestAppendQuery(t *testing.T) {
	if got := appendQuery("/path", "k", "v"); got != "/path?k=v" {
		t.Errorf("appendQuery = %q", got)
	}
	if got := appendQuery("/path?a=1", "k", "v"); got != "/path?a=1&k=v" {
		t.Errorf("appendQuery with existing query = %q", got)
	}
}

func TestIsLocalRedirect(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		// Safe same-origin relative targets — no authority component, so a
		// browser resolves them against the current origin.
		{"/", true},
		{"/apps/t/s/", true},
		{"/dashboard?tab=1", true},
		{"/path#frag", true},
		{"/%2f%2fevil.com", true}, // percent-encoded, NOT decoded before origin resolution
		{"/ /evil.com", true},     // space after slash — still a path on this origin
		{"/.evil.com", true},      // leading dot in a path segment
		{"/@evil.com", true},      // '@' in a path, no authority
		// Open-redirect vectors that must be rejected.
		{"", false},
		{"//evil.com", false},          // protocol-relative
		{"//evil.com/path", false},     // protocol-relative with path
		{`/\evil.com`, false},          // backslash variant browsers treat as //
		{"/\t/evil.com", false},        // control char — url.Parse rejects
		{"/\nhttps://evil.com", false}, // newline injection
		{"/\rhttp://evil.com", false},  // CR injection
		{"https://evil.com", false},    // absolute URL
		{"http://evil.com", false},     // absolute URL
		{"https:/evil.com", false},     // scheme with single slash
		{"javascript:alert(1)", false}, // scheme, no leading slash
		{"evil.com", false},            // bare host, no leading slash
		{"\\\\evil.com", false},        // does not start with /
	}
	for _, c := range cases {
		if got := isLocalRedirect(c.in); got != c.want {
			t.Errorf("isLocalRedirect(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestAbsoluteURL_PublicURLPreferred verifies that when Config.PublicURL
// is set (platform-injected MOSES_PUBLIC_URL), absoluteURL uses it as the
// scheme+host+port source — ignoring r.Host. This is the fix for the
// envoy port-stripping bug: r.Host = "localhost" (no port) on the
// in-cluster hop, but Keycloak has "http://localhost:9877/..." registered.
func TestAbsoluteURL_PublicURLPreferred(t *testing.T) {
	cfg := enabledCfg()
	cfg.PublicURL = "http://localhost:9877"
	m := New(cfg)

	// r.Host is the envoy-stripped value ("localhost", no port) — proves
	// PublicURL is the source and r.Host is ignored.
	r := httptest.NewRequest("GET", "/apps/t/s/auth/callback", nil)
	r.Host = "localhost"

	got := m.absoluteURL(r, routeCallback)
	want := "http://localhost:9877/apps/t/s/auth/callback"
	if got != want {
		t.Errorf("absoluteURL = %q, want %q", got, want)
	}
}

// TestAbsoluteURL_FallbackWhenPublicURLEmpty pins the legacy behaviour:
// when PublicURL is unset, absoluteURL derives scheme+host from the
// request as it always has. This preserves behaviour for non-platform
// deployments and for the existing test/proxy flows.
func TestAbsoluteURL_FallbackWhenPublicURLEmpty(t *testing.T) {
	cfg := enabledCfg()
	cfg.PublicURL = ""
	m := New(cfg)

	r := httptest.NewRequest("GET", "/apps/t/s/auth/callback", nil)
	r.Host = "app.example.com"
	r.Header.Set("X-Forwarded-Proto", "https")

	got := m.absoluteURL(r, routeCallback)
	want := "https://app.example.com/apps/t/s/auth/callback"
	if got != want {
		t.Errorf("absoluteURL fallback = %q, want %q", got, want)
	}
}

// TestAbsoluteURL_FallbackHonoursForwardedHost confirms the legacy
// X-Forwarded-Host override still wins over r.Host when PublicURL is
// empty (no behaviour change for ingress-fronted deploys).
func TestAbsoluteURL_FallbackHonoursForwardedHost(t *testing.T) {
	cfg := enabledCfg()
	cfg.PublicURL = ""
	m := New(cfg)

	r := httptest.NewRequest("GET", "/apps/t/s/auth/callback", nil)
	r.Host = "internal.svc.cluster.local"
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "public.example.com")

	got := m.absoluteURL(r, routeCallback)
	want := "https://public.example.com/apps/t/s/auth/callback"
	if got != want {
		t.Errorf("absoluteURL with X-Forwarded-Host = %q, want %q", got, want)
	}
}

// TestAbsoluteURL_PublicURLWithTrailingSlash verifies a trailing slash
// on MOSES_PUBLIC_URL is trimmed so the joined route does not produce a
// double slash (e.g. "http://localhost:9877//apps/...").
func TestAbsoluteURL_PublicURLWithTrailingSlash(t *testing.T) {
	cfg := enabledCfg()
	cfg.PublicURL = "http://localhost:9877/"
	m := New(cfg)

	r := httptest.NewRequest("GET", "/apps/t/s/auth/callback", nil)
	r.Host = "localhost"

	got := m.absoluteURL(r, routeCallback)
	want := "http://localhost:9877/apps/t/s/auth/callback"
	if got != want {
		t.Errorf("absoluteURL trailing-slash = %q, want %q", got, want)
	}
}

func TestAbsoluteURL_PublicURLsMatchesRequestHost(t *testing.T) {
	cfg := enabledCfg() // verified helper (middleware_test.go:15); absoluteURL ignores its OIDC fields
	cfg.BasePath = "/apps/t/s"
	cfg.PublicURL = "https://platform.example.com"
	cfg.PublicURLs = []string{"https://apex.example.com", "https://platform.example.com"}
	m := New(cfg)

	r := httptest.NewRequest("GET", "/apps/t/s/auth/callback", nil)
	r.Host = "apex.example.com" // browser came in on the apex
	if got, want := m.absoluteURL(r, routeCallback), "https://apex.example.com/apps/t/s/auth/callback"; got != want {
		t.Errorf("apex absoluteURL = %q, want %q", got, want)
	}

	r2 := httptest.NewRequest("GET", "/apps/t/s/auth/callback", nil)
	r2.Host = "platform.example.com"
	if got, want := m.absoluteURL(r2, routeCallback), "https://platform.example.com/apps/t/s/auth/callback"; got != want {
		t.Errorf("platform absoluteURL = %q, want %q", got, want)
	}
}

func TestAbsoluteURL_PublicURLsPortStrippedHostStillMatches(t *testing.T) {
	cfg := enabledCfg()
	cfg.BasePath = "/apps/t/s"
	cfg.PublicURLs = []string{"http://localhost:9877"}
	m := New(cfg)
	r := httptest.NewRequest("GET", "/apps/t/s/auth/callback", nil)
	r.Host = "localhost" // Envoy stripped :9877
	if got, want := m.absoluteURL(r, routeCallback), "http://localhost:9877/apps/t/s/auth/callback"; got != want {
		t.Errorf("absoluteURL = %q, want %q", got, want)
	}
}

func TestAbsoluteURL_PublicURLsNoMatchFallsBackToPublicURL(t *testing.T) {
	cfg := enabledCfg()
	cfg.BasePath = "/apps/t/s"
	cfg.PublicURL = "https://platform.example.com"
	cfg.PublicURLs = []string{"https://apex.example.com"}
	m := New(cfg)
	r := httptest.NewRequest("GET", "/apps/t/s/auth/callback", nil)
	r.Host = "unknown.example.com"
	if got, want := m.absoluteURL(r, routeCallback), "https://platform.example.com/apps/t/s/auth/callback"; got != want {
		t.Errorf("absoluteURL fallback = %q, want %q", got, want)
	}
}
