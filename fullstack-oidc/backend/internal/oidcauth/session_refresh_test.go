package oidcauth

// Tests for the session-TTL / token-TTL decoupling (relogin-on-refresh fix).
//
// Before this change the BFF session expiry was min(idToken.exp,
// expires_in, 8h) — and because Moses provisions app clients with
// access.token.lifespan=120s, every session died after 2 minutes and each
// page refresh restarted the OIDC handshake. The fix gives the session its
// own lifetime and keeps the ROLES snapshot fresh via the refresh-token
// grant, preserving the platform's ~120s role-revocation latency contract.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// --- session shape -----------------------------------------------------

func TestSessionFromClaims_SessionTTLDecoupledFromTokenTTL(t *testing.T) {
	now := time.Now()
	claims := &Claims{
		Subject: "kc-sub",
		Email:   "u@x.io",
		Expiry:  now.Add(120 * time.Second).Unix(), // KC app clients: 120s tokens
	}
	cfg := Config{ClientID: "app-client"}

	s := sessionFromClaims(claims, cfg, 120, "rt-1")

	// The SESSION must outlive the token: default max age is 8h.
	wantExpiry := now.Add(defaultSessionMaxAge)
	if got := time.Unix(s.Expiry, 0); got.Before(wantExpiry.Add(-time.Minute)) ||
		got.After(wantExpiry.Add(time.Minute)) {
		t.Errorf("session Expiry = %v, want ~%v (decoupled from the 120s token)", got, wantExpiry)
	}
	// The ROLES snapshot goes stale when the token would have expired.
	wantFresh := now.Add(120 * time.Second)
	if got := time.Unix(s.RolesFreshUntil, 0); got.Before(wantFresh.Add(-10*time.Second)) ||
		got.After(wantFresh.Add(10*time.Second)) {
		t.Errorf("RolesFreshUntil = %v, want ~%v (token lifetime)", got, wantFresh)
	}
	if s.RefreshToken != "rt-1" {
		t.Errorf("RefreshToken = %q, want rt-1", s.RefreshToken)
	}
}

func TestSessionFromClaims_SessionMaxAgeConfigurable(t *testing.T) {
	now := time.Now()
	claims := &Claims{Subject: "kc-sub", Expiry: now.Add(2 * time.Minute).Unix()}
	cfg := Config{ClientID: "app-client", SessionMaxAge: time.Hour}

	s := sessionFromClaims(claims, cfg, 120, "rt-1")

	want := now.Add(time.Hour)
	if got := time.Unix(s.Expiry, 0); got.Before(want.Add(-time.Minute)) ||
		got.After(want.Add(time.Minute)) {
		t.Errorf("session Expiry = %v, want ~%v (cfg.SessionMaxAge)", got, want)
	}
}

// --- cookie confidentiality --------------------------------------------

// The cookie now carries the OIDC refresh token, so the payload must be
// ENCRYPTED (AEAD), not merely HMAC-signed plaintext.
func TestEncodeSession_RefreshTokenNotRecoverableWithoutKey(t *testing.T) {
	const rt = "super-secret-refresh-token-value"
	s := &Session{
		Subject:      "u-1",
		RefreshToken: rt,
		Expiry:       time.Now().Add(time.Hour).Unix(),
	}
	encoded, err := encodeSession(s, testSecret)
	if err != nil {
		t.Fatalf("encodeSession: %v", err)
	}
	if strings.Contains(encoded, rt) {
		t.Fatalf("refresh token appears verbatim in the cookie value")
	}
	for _, part := range strings.Split(encoded, ".") {
		for _, dec := range []*base64.Encoding{base64.RawURLEncoding, base64.StdEncoding} {
			if raw, err := dec.DecodeString(part); err == nil &&
				strings.Contains(string(raw), rt) {
				t.Fatalf("refresh token recoverable by base64-decoding the cookie — payload is not encrypted")
			}
		}
	}
	// Round-trip still works with the right key.
	got, err := decodeSession(encoded, testSecret)
	if err != nil {
		t.Fatalf("decodeSession: %v", err)
	}
	if got.RefreshToken != rt {
		t.Errorf("RefreshToken round-trip = %q, want %q", got.RefreshToken, rt)
	}
}

// A legacy (pre-fix) HMAC-format cookie must be rejected cleanly — the
// user just re-authenticates once after the app upgrades.
func TestDecodeSession_LegacyHMACFormatRejected(t *testing.T) {
	payload, _ := json.Marshal(&Session{Subject: "u-1", Expiry: time.Now().Add(time.Hour).Unix()})
	mac := hmac.New(sha256.New, testSecret)
	mac.Write(payload)
	legacy := base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if _, err := decodeSession(legacy, testSecret); err == nil {
		t.Fatalf("legacy HMAC-format cookie should be rejected")
	}
}

// --- middleware: roles-staleness handling ------------------------------

// fakeKC is a minimal Keycloak stand-in: discovery + JWKS + token endpoint.
type fakeKC struct {
	srv      *httptest.Server
	signer   *testSigner
	issuer   string
	tokenHit int
	// tokenHandler serves POSTs to /token. Set by each test.
	tokenHandler func(t *testing.T, w http.ResponseWriter, form url.Values)
}

func newFakeKC(t *testing.T) *fakeKC {
	f := &fakeKC{signer: newTestSigner(t)}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := discoveryDoc{
			Issuer:                f.issuer,
			AuthorizationEndpoint: f.issuer + "/auth",
			TokenEndpoint:         f.issuer + "/token",
			JWKSURI:               f.issuer + "/certs",
			EndSessionEndpoint:    f.issuer + "/logout",
		}
		b, _ := json.Marshal(doc)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("/certs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(f.signer.jwksJSON())
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		f.tokenHit++
		_ = r.ParseForm()
		f.tokenHandler(t, w, r.PostForm)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	f.issuer = f.srv.URL
	return f
}

func (f *fakeKC) cfg() Config {
	c := enabledCfg()
	c.Issuer = f.issuer
	c.InternalIssuer = f.issuer
	return c
}

// mintIDToken signs an ID token for the fake KC's issuer/client.
func (f *fakeKC) mintIDToken(t *testing.T, roles []string, ttl time.Duration) string {
	now := time.Now()
	return f.signer.sign(t, map[string]any{
		"iss":                f.issuer,
		"aud":                "app-client",
		"sub":                "kc-sub-1",
		"moses_user_id":      "moses-u-1",
		"email":              "u@x.io",
		"preferred_username": "u",
		"exp":                now.Add(ttl).Unix(),
		"iat":                now.Unix(),
		"resource_access":    map[string]any{"app-client": map[string]any{"roles": roles}},
	})
}

// staleSession builds a session whose roles snapshot has gone stale but
// whose hard expiry is far in the future.
func staleSession(refreshToken string) *Session {
	now := time.Now()
	return &Session{
		Subject:         "moses-u-1",
		Email:           "u@x.io",
		Roles:           []string{"viewer"},
		Expiry:          now.Add(4 * time.Hour).Unix(),
		RolesFreshUntil: now.Add(-10 * time.Second).Unix(),
		IssuedAt:        now.Add(-time.Hour).Unix(),
		RefreshToken:    refreshToken,
	}
}

func TestHandler_StaleRolesRefreshedViaRefreshGrant(t *testing.T) {
	kc := newFakeKC(t)
	var gotForm url.Values
	kc.tokenHandler = func(t *testing.T, w http.ResponseWriter, form url.Values) {
		gotForm = form
		resp := map[string]any{
			"access_token":  "at-2",
			"id_token":      kc.mintIDToken(t, []string{"admin"}, 120*time.Second),
			"refresh_token": "rt-2",
			"expires_in":    120,
			"token_type":    "Bearer",
		}
		b, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}

	cfg := kc.cfg()
	m := New(cfg)
	var got Identity
	h := m.Handler(recordingHandler(&got))

	cookieVal, _ := encodeSession(staleSession("rt-1"), cfg.CookieSecret)
	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.AddCookie(&http.Cookie{Name: cfg.sessionCookieName(), Value: cookieVal})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("stale-roles request status = %d, want 200 (refreshed in place); body=%s",
			rec.Code, rec.Body.String())
	}
	if !got.Authenticated || got.Source != "session" {
		t.Errorf("identity = %+v, want authenticated session identity", got)
	}
	if !got.HasRole("admin") || got.HasRole("viewer") {
		t.Errorf("roles = %v, want refreshed snapshot [admin]", got.Roles)
	}
	if gotForm.Get("grant_type") != "refresh_token" || gotForm.Get("refresh_token") != "rt-1" {
		t.Errorf("token endpoint form = %v, want refresh_token grant with rt-1", gotForm)
	}

	// The renewed session must be re-set on the response with a fresh
	// roles bound and the rotated refresh token.
	ck := readSetCookie(t, rec, cfg.sessionCookieName())
	renewed, err := decodeSession(ck.Value, cfg.CookieSecret)
	if err != nil {
		t.Fatalf("decode renewed session: %v", err)
	}
	if renewed.RefreshToken != "rt-2" {
		t.Errorf("renewed RefreshToken = %q, want rt-2 (rotated)", renewed.RefreshToken)
	}
	if time.Unix(renewed.RolesFreshUntil, 0).Before(time.Now().Add(30 * time.Second)) {
		t.Errorf("renewed RolesFreshUntil not advanced: %v", renewed.RolesFreshUntil)
	}
}

func TestHandler_StaleRolesRefreshFailure_ChallengesAndClearsCookie(t *testing.T) {
	kc := newFakeKC(t)
	kc.tokenHandler = func(t *testing.T, w http.ResponseWriter, form url.Values) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"session idled out"}`))
	}

	cfg := kc.cfg()
	m := New(cfg)
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("next handler must not run when the roles refresh fails")
	}))

	cookieVal, _ := encodeSession(staleSession("rt-dead"), cfg.CookieSecret)
	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.Header.Set("Accept", "application/json")
	r.AddCookie(&http.Cookie{Name: cfg.sessionCookieName(), Value: cookieVal})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("failed-refresh XHR status = %d, want 401", rec.Code)
	}
	ck := readSetCookie(t, rec, cfg.sessionCookieName())
	if ck.MaxAge >= 0 && ck.Value != "" {
		t.Errorf("dead session cookie should be cleared, got MaxAge=%d Value=%q", ck.MaxAge, ck.Value)
	}
}

func TestHandler_StaleRolesNoRefreshToken_Challenges(t *testing.T) {
	// No refresh token (e.g. IdP without refresh tokens): stale roles
	// fall back to the legacy behaviour — the session ends.
	cfg := enabledCfg()
	m := New(cfg)
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("next handler must not run for a stale session with no refresh token")
	}))

	cookieVal, _ := encodeSession(staleSession(""), cfg.CookieSecret)
	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.Header.Set("Accept", "application/json")
	r.AddCookie(&http.Cookie{Name: cfg.sessionCookieName(), Value: cookieVal})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("stale-no-rt status = %d, want 401", rec.Code)
	}
}

func TestHandler_FreshRolesServedWithoutTokenEndpoint(t *testing.T) {
	kc := newFakeKC(t)
	kc.tokenHandler = func(t *testing.T, w http.ResponseWriter, form url.Values) {
		t.Errorf("token endpoint must not be called while the roles snapshot is fresh")
	}

	cfg := kc.cfg()
	m := New(cfg)
	var got Identity
	h := m.Handler(recordingHandler(&got))

	now := time.Now()
	sess := &Session{
		Subject:         "moses-u-1",
		Roles:           []string{"viewer"},
		Expiry:          now.Add(4 * time.Hour).Unix(),
		RolesFreshUntil: now.Add(60 * time.Second).Unix(),
		RefreshToken:    "rt-1",
	}
	cookieVal, _ := encodeSession(sess, cfg.CookieSecret)
	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.AddCookie(&http.Cookie{Name: cfg.sessionCookieName(), Value: cookieVal})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("fresh-roles status = %d, want 200", rec.Code)
	}
	if kc.tokenHit != 0 {
		t.Errorf("token endpoint hit %d times, want 0", kc.tokenHit)
	}
}

// FAIL-SAFE: a session without a roles bound (RolesFreshUntil==0) was
// not minted by the login/refresh path — it counts as stale, and with
// no refresh token it is challenged. The ~120s role-revocation contract
// must never silently degrade to the session lifetime.
func TestHandler_ZeroRolesFreshUntil_TreatedAsStale(t *testing.T) {
	cfg := enabledCfg()
	m := New(cfg)
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("next handler must not run for a session without a roles bound")
	}))

	sess := &Session{
		Subject: "u-1",
		Expiry:  time.Now().Add(time.Hour).Unix(),
		// RolesFreshUntil deliberately zero, no refresh token.
	}
	cookieVal, _ := encodeSession(sess, cfg.CookieSecret)
	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.Header.Set("Accept", "application/json")
	r.AddCookie(&http.Cookie{Name: cfg.sessionCookieName(), Value: cookieVal})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("zero-RolesFreshUntil status = %d, want 401 (fail-safe stale)", rec.Code)
	}
}

// A refresh response that omits refresh_token (Keycloak with rotation
// disabled — the default) must keep the ORIGINAL refresh token so the
// next stale window can renew again.
func TestHandler_RefreshResponseWithoutRotation_KeepsOldRefreshToken(t *testing.T) {
	kc := newFakeKC(t)
	kc.tokenHandler = func(t *testing.T, w http.ResponseWriter, form url.Values) {
		resp := map[string]any{
			"access_token": "at-2",
			"id_token":     kc.mintIDToken(t, []string{"viewer"}, 120*time.Second),
			// refresh_token deliberately omitted.
			"expires_in": 120,
			"token_type": "Bearer",
		}
		b, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}

	cfg := kc.cfg()
	m := New(cfg)
	var got Identity
	h := m.Handler(recordingHandler(&got))

	cookieVal, _ := encodeSession(staleSession("rt-keep"), cfg.CookieSecret)
	r := httptest.NewRequest("GET", "/apps/t/s/api/private/data", nil)
	r.AddCookie(&http.Cookie{Name: cfg.sessionCookieName(), Value: cookieVal})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	ck := readSetCookie(t, rec, cfg.sessionCookieName())
	renewed, err := decodeSession(ck.Value, cfg.CookieSecret)
	if err != nil {
		t.Fatalf("decode renewed session: %v", err)
	}
	if renewed.RefreshToken != "rt-keep" {
		t.Errorf("renewed RefreshToken = %q, want the original rt-keep", renewed.RefreshToken)
	}
}

// A v2-prefixed but corrupted/truncated ciphertext must fail cleanly.
func TestDecodeSession_TruncatedV2CiphertextRejected(t *testing.T) {
	s := &Session{Subject: "u-1", Expiry: time.Now().Add(time.Hour).Unix()}
	encoded, err := encodeSession(s, testSecret)
	if err != nil {
		t.Fatalf("encodeSession: %v", err)
	}
	for _, bad := range []string{
		encoded[:len(encoded)-8],       // truncated tail
		"v2." + encoded[3:len(encoded)-1] + "A", // flipped last char
		"v2.AAAA",                      // shorter than a GCM nonce
		"v2.",                          // empty ciphertext
	} {
		if _, err := decodeSession(bad, testSecret); err == nil {
			t.Errorf("decodeSession(%q...) should fail", bad[:min(16, len(bad))])
		}
	}
}

// --- config ------------------------------------------------------------

func TestConfigFromEnv_SessionMaxAge(t *testing.T) {
	t.Setenv(EnvIssuer, "https://kc/realms/t")
	t.Setenv(EnvClientID, "c")
	t.Setenv(EnvClientSecret, "s")

	cfg := ConfigFromEnv()
	if cfg.SessionMaxAge != defaultSessionMaxAge {
		t.Errorf("default SessionMaxAge = %v, want %v", cfg.SessionMaxAge, defaultSessionMaxAge)
	}

	t.Setenv(EnvSessionMaxAgeSeconds, "3600")
	cfg = ConfigFromEnv()
	if cfg.SessionMaxAge != time.Hour {
		t.Errorf("SessionMaxAge with env=3600 = %v, want 1h", cfg.SessionMaxAge)
	}

	t.Setenv(EnvSessionMaxAgeSeconds, "not-a-number")
	cfg = ConfigFromEnv()
	if cfg.SessionMaxAge != defaultSessionMaxAge {
		t.Errorf("SessionMaxAge with junk env = %v, want default %v", cfg.SessionMaxAge, defaultSessionMaxAge)
	}
}
