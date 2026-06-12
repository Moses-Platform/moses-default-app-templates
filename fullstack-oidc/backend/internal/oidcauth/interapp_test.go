package oidcauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

// interAppTestSecret is a fixed per-tenant signing key for the tests.
const interAppTestSecret = "tenant-shared-hs256-secret-0123456789"

// newInterAppConfig builds a Config with the inter-app path armed for the
// given app slug + tenant. OIDC is deliberately LEFT DISABLED (no Issuer/
// ClientID/ClientSecret) to prove interAppEnabled() is independent of
// Enabled().
func newInterAppConfig(appSlug, tenantID string) Config {
	return Config{
		BasePath:       "/apps/t/" + appSlug,
		SpecPath:       "/api/openapi.json",
		ProtectedPaths: []string{"/api/private"},
		CookieSecret:   []byte("0123456789abcdef0123456789abcdef"),
		InterAppSecret: interAppTestSecret,
		AppSlug:        appSlug,
		TenantID:       tenantID,
	}
}

// signRaw signs an arbitrary claims map with the given secret, producing a
// compact HS256 JWS. Used to craft tampered / off-contract tokens the mint
// helper would never emit.
func signRaw(t *testing.T, claims map[string]any, secret string) string {
	t.Helper()
	hdr, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	clm, _ := json.Marshal(claims)
	input := base64.RawURLEncoding.EncodeToString(hdr) + "." +
		base64.RawURLEncoding.EncodeToString(clm)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(input))
	return input + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func nowUnix() int64 { return time.Now().Unix() }

// TestInterAppMintVerifyRoundTrip: caller "docs" mints a token for callee
// "sheets"; "sheets" verifies it and recovers the moses_user_id.
func TestInterAppMintVerifyRoundTrip(t *testing.T) {
	caller := newInterAppConfig("docs", "tenant-abc")
	callee := newInterAppConfig("sheets", "tenant-abc")

	tok, err := caller.MintInterAppToken("sheets", "moses-user-42")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	uid, err := callee.verifyInterAppToken(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if uid != "moses-user-42" {
		t.Errorf("uid = %q, want moses-user-42", uid)
	}
}

// TestInterAppClassifyDecision: a valid inbound token on a protected path
// yields decisionInterApp with Identity{Subject=moses_user_id,
// Source=interapp, Roles empty} — and OIDC is disabled, proving the path
// is independent of Enabled().
func TestInterAppClassifyDecision(t *testing.T) {
	caller := newInterAppConfig("docs", "tenant-abc")
	callee := newInterAppConfig("sheets", "tenant-abc")
	if callee.Enabled() {
		t.Fatal("precondition: OIDC must be DISABLED to prove independence")
	}
	m := New(callee)

	tok, err := caller.MintInterAppToken("sheets", "moses-user-77")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	r := httptest.NewRequest("GET", "/apps/t/sheets/api/private/x", nil)
	r.Header.Set(HeaderInterAppToken, tok)

	dec, uid := m.classify(r, nil, false)
	if dec != decisionInterApp {
		t.Fatalf("decision = %d, want decisionInterApp", dec)
	}
	id := identityFromInterApp(uid)
	if !id.Authenticated || id.Subject != "moses-user-77" || id.Source != "interapp" {
		t.Errorf("identity = %+v, want authenticated subject=moses-user-77 source=interapp", id)
	}
	if len(id.Roles) != 0 {
		t.Errorf("Roles = %v, want empty (roles NEVER carried)", id.Roles)
	}
}

// TestInterAppWrongAudienceFallsThrough: a token addressed to a DIFFERENT
// app is not trusted — classify ignores it and falls through to the
// protected-path challenge (no hard 401).
func TestInterAppWrongAudienceFallsThrough(t *testing.T) {
	caller := newInterAppConfig("docs", "tenant-abc")
	callee := newInterAppConfig("sheets", "tenant-abc")
	m := New(callee)

	// Minted for "calendar", not for this app ("sheets").
	tok, err := caller.MintInterAppToken("calendar", "moses-user-1")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if _, err := callee.verifyInterAppToken(tok); err == nil {
		t.Fatal("verify: wrong aud should NOT be trusted")
	}

	r := httptest.NewRequest("GET", "/apps/t/sheets/api/private/x", nil)
	r.Header.Set(HeaderInterAppToken, tok)
	dec, _ := m.classify(r, nil, false)
	if dec != decisionChallenge {
		t.Errorf("decision = %d, want decisionChallenge (fall through)", dec)
	}
}

// TestInterAppExpiredFallsThrough: an expired token is not trusted.
func TestInterAppExpiredFallsThrough(t *testing.T) {
	callee := newInterAppConfig("sheets", "tenant-abc")
	past := nowUnix() - 120
	tok := signRaw(t, map[string]any{
		"iss": "docs", "aud": "sheets", "moses_user_id": "u1",
		"tenant_id": "tenant-abc", "iat": past, "exp": past + 60, // expired ~60s ago
	}, interAppTestSecret)

	if _, err := callee.verifyInterAppToken(tok); err == nil {
		t.Fatal("verify: expired token should NOT be trusted")
	}

	m := New(callee)
	r := httptest.NewRequest("GET", "/apps/t/sheets/api/private/x", nil)
	r.Header.Set(HeaderInterAppToken, tok)
	if dec, _ := m.classify(r, nil, false); dec != decisionChallenge {
		t.Errorf("decision = %d, want decisionChallenge (fall through)", dec)
	}
}

// TestInterAppWrongTenantFallsThrough: a token whose tenant_id differs
// from this app's own tenant is not trusted (even with a valid signature
// from the same secret value — defence in depth).
func TestInterAppWrongTenantFallsThrough(t *testing.T) {
	callee := newInterAppConfig("sheets", "tenant-abc")
	now := nowUnix()
	tok := signRaw(t, map[string]any{
		"iss": "docs", "aud": "sheets", "moses_user_id": "u1",
		"tenant_id": "tenant-OTHER", "iat": now, "exp": now + 60,
	}, interAppTestSecret)

	if _, err := callee.verifyInterAppToken(tok); err == nil {
		t.Fatal("verify: wrong tenant should NOT be trusted")
	}
}

// TestInterAppTamperedSignatureFallsThrough: flipping a byte in the
// payload invalidates the HMAC.
func TestInterAppTamperedSignatureFallsThrough(t *testing.T) {
	caller := newInterAppConfig("docs", "tenant-abc")
	callee := newInterAppConfig("sheets", "tenant-abc")
	tok, err := caller.MintInterAppToken("sheets", "moses-user-1")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	// Corrupt the last char of the signature segment.
	tampered := tok[:len(tok)-1]
	if tok[len(tok)-1] == 'A' {
		tampered += "B"
	} else {
		tampered += "A"
	}
	if _, err := callee.verifyInterAppToken(tampered); err == nil {
		t.Fatal("verify: tampered signature should NOT be trusted")
	}

	// Also a token signed with a DIFFERENT secret must fail.
	wrongSig := signRaw(t, map[string]any{
		"iss": "docs", "aud": "sheets", "moses_user_id": "u1",
		"tenant_id": "tenant-abc", "iat": nowUnix(), "exp": nowUnix() + 60,
	}, "a-totally-different-tenant-secret")
	if _, err := callee.verifyInterAppToken(wrongSig); err == nil {
		t.Fatal("verify: wrong-secret signature should NOT be trusted")
	}
}

// TestInterAppDisabledWhenSecretUnset: with no MOSES_INTERAPP_SECRET the
// path is disabled — interAppEnabled() false, verify errors, and a token
// is ignored (falls through to challenge).
func TestInterAppDisabledWhenSecretUnset(t *testing.T) {
	noSecret := Config{
		BasePath:       "/apps/t/sheets",
		SpecPath:       "/api/openapi.json",
		ProtectedPaths: []string{"/api/private"},
		CookieSecret:   []byte("0123456789abcdef0123456789abcdef"),
		AppSlug:        "sheets",
		TenantID:       "tenant-abc",
		// InterAppSecret deliberately unset.
	}
	if noSecret.interAppEnabled() {
		t.Fatal("interAppEnabled() must be false when secret unset")
	}

	// A token (signed with the real secret) must still be ignored.
	caller := newInterAppConfig("docs", "tenant-abc")
	tok, _ := caller.MintInterAppToken("sheets", "u1")

	if _, err := noSecret.verifyInterAppToken(tok); err == nil {
		t.Fatal("verify must error when path disabled")
	}

	m := New(noSecret)
	r := httptest.NewRequest("GET", "/apps/t/sheets/api/private/x", nil)
	r.Header.Set(HeaderInterAppToken, tok)
	if dec, _ := m.classify(r, nil, false); dec != decisionChallenge {
		t.Errorf("decision = %d, want decisionChallenge (path disabled)", dec)
	}

	// Mint must also refuse when secret unset.
	if _, err := noSecret.MintInterAppToken("sheets", "u1"); err == nil {
		t.Fatal("mint must error when secret unset")
	}
}

// TestInterAppRolesNeverCarried is the core privilege-escalation guard: a
// token carrying a `roles` claim must NOT populate Identity.Roles. The
// verifier does not read roles and identityFromInterApp sets them nil.
func TestInterAppRolesNeverCarried(t *testing.T) {
	callee := newInterAppConfig("sheets", "tenant-abc")
	now := nowUnix()
	// A malicious/forward-compat token that smuggles a roles claim.
	tok := signRaw(t, map[string]any{
		"iss": "docs", "aud": "sheets", "moses_user_id": "u1",
		"tenant_id": "tenant-abc", "iat": now, "exp": now + 60,
		"roles":          []string{"admin", "superuser"},
		"resource_access": map[string]any{"sheets": map[string]any{"roles": []string{"admin"}}},
	}, interAppTestSecret)

	uid, err := callee.verifyInterAppToken(tok)
	if err != nil {
		t.Fatalf("verify (valid token with extra roles claim): %v", err)
	}
	if uid != "u1" {
		t.Errorf("uid = %q, want u1", uid)
	}

	id := identityFromInterApp(uid)
	if len(id.Roles) != 0 {
		t.Errorf("Roles = %v, want EMPTY — roles must NEVER be carried across the inter-app hop", id.Roles)
	}
	if id.HasRole("admin") {
		t.Error("HasRole(admin) = true, want false — inter-app token must not grant roles")
	}
}

// TestInterAppNonHS256Rejected guards against alg-confusion: a token whose
// header claims alg=none (with an empty signature) must be rejected even
// though "header.payload." technically parses into three segments.
func TestInterAppNonHS256Rejected(t *testing.T) {
	callee := newInterAppConfig("sheets", "tenant-abc")
	now := nowUnix()
	hdr, _ := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	clm, _ := json.Marshal(map[string]any{
		"iss": "docs", "aud": "sheets", "moses_user_id": "u1",
		"tenant_id": "tenant-abc", "iat": now, "exp": now + 60,
	})
	// alg=none with an empty signature — the classic bypass attempt.
	tok := base64.RawURLEncoding.EncodeToString(hdr) + "." +
		base64.RawURLEncoding.EncodeToString(clm) + "."
	if _, err := callee.verifyInterAppToken(tok); err == nil {
		t.Fatal("verify: alg=none must be rejected")
	}
}

// TestInterAppLifetimeCapped rejects a token whose exp sits further than
// the contract's 60s (+skew) past iat — a caller cannot mint a long-lived
// token by inflating exp.
func TestInterAppLifetimeCapped(t *testing.T) {
	callee := newInterAppConfig("sheets", "tenant-abc")
	now := nowUnix()
	tok := signRaw(t, map[string]any{
		"iss": "docs", "aud": "sheets", "moses_user_id": "u1",
		"tenant_id": "tenant-abc", "iat": now, "exp": now + 3600, // 1h — over contract
	}, interAppTestSecret)
	if _, err := callee.verifyInterAppToken(tok); err == nil {
		t.Fatal("verify: over-contract lifetime must be rejected")
	}
}

// TestInterAppDoesNotBreakSession: when no inter-app token is present, a
// valid session still wins (no regression to the OIDC session path).
func TestInterAppDoesNotBreakSession(t *testing.T) {
	cfg := newInterAppConfig("sheets", "tenant-abc")
	// Arm OIDC too so decisionSession is reachable.
	cfg.Issuer, cfg.ClientID, cfg.ClientSecret = "i", "c", "s"
	m := New(cfg)

	sess := &Session{Subject: "session-user", Expiry: time.Now().Add(time.Hour).Unix()}
	r := httptest.NewRequest("GET", "/apps/t/sheets/api/private/x", nil)
	if dec, _ := m.classify(r, sess, true); dec != decisionSession {
		t.Errorf("decision = %d, want decisionSession (no token present)", dec)
	}
}

// TestInterAppMintRequiresActingUser: mint refuses a blank acting user id
// (cannot forge an anonymous identity).
func TestInterAppMintRequiresActingUser(t *testing.T) {
	caller := newInterAppConfig("docs", "tenant-abc")
	if _, err := caller.MintInterAppToken("sheets", "   "); err == nil {
		t.Fatal("mint must refuse a blank acting user id")
	}
}
