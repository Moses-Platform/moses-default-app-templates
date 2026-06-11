package oidcauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

func TestSessionRoundTrip(t *testing.T) {
	s := &Session{
		Subject:  "user-1",
		Email:    "u@example.com",
		Name:     "User One",
		Username: "uone",
		Roles:    []string{"viewer", "editor"},
		Expiry:   time.Now().Add(time.Hour).Unix(),
		IssuedAt: time.Now().Unix(),
	}
	encoded, err := encodeSession(s, testSecret)
	if err != nil {
		t.Fatalf("encodeSession: %v", err)
	}
	decoded, err := decodeSession(encoded, testSecret)
	if err != nil {
		t.Fatalf("decodeSession: %v", err)
	}
	if decoded.Subject != s.Subject || decoded.Email != s.Email {
		t.Errorf("round-trip mismatch: %+v vs %+v", decoded, s)
	}
	if len(decoded.Roles) != 2 || decoded.Roles[1] != "editor" {
		t.Errorf("roles not preserved: %v", decoded.Roles)
	}
}

func TestDecodeSession_TamperedPayloadFails(t *testing.T) {
	s := &Session{Subject: "user-1", Expiry: time.Now().Add(time.Hour).Unix()}
	encoded, _ := encodeSession(s, testSecret)

	// Flip a byte in the payload segment.
	parts := strings.SplitN(encoded, ".", 2)
	tampered := parts[0][:len(parts[0])-1] + "A" + "." + parts[1]
	if _, err := decodeSession(tampered, testSecret); err == nil {
		t.Errorf("decodeSession accepted a tampered payload")
	}
}

func TestDecodeSession_WrongSecretFails(t *testing.T) {
	s := &Session{Subject: "user-1", Expiry: time.Now().Add(time.Hour).Unix()}
	encoded, _ := encodeSession(s, testSecret)
	if _, err := decodeSession(encoded, []byte("a-totally-different-secret-32by!")); err == nil {
		t.Errorf("decodeSession accepted a cookie signed with a different secret")
	}
}

func TestDecodeSession_Malformed(t *testing.T) {
	for _, bad := range []string{"", "nodot", "a.b.c.d", "!!!.???"} {
		if _, err := decodeSession(bad, testSecret); err == nil {
			t.Errorf("decodeSession(%q) accepted malformed input", bad)
		}
	}
}

func TestSessionValid(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		s    *Session
		want bool
	}{
		{"nil", nil, false},
		{"no subject", &Session{Expiry: now.Add(time.Hour).Unix()}, false},
		{"no expiry", &Session{Subject: "u"}, false},
		{"expired", &Session{Subject: "u", Expiry: now.Add(-time.Minute).Unix()}, false},
		{"valid", &Session{Subject: "u", Expiry: now.Add(time.Hour).Unix()}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.s.Valid(now); got != c.want {
				t.Errorf("Valid() = %v, want %v", got, c.want)
			}
		})
	}
}

// The session cookie MUST be HttpOnly, Secure (by default), SameSite=Lax,
// and Path=/. These attributes are the security contract from the ticket.
func TestSetSessionCookie_Attributes(t *testing.T) {
	cfg := Config{
		BasePath:     "/apps/t/s",
		CookieSecret: testSecret,
		SecureCookie: true,
	}
	s := &Session{Subject: "u", Expiry: time.Now().Add(time.Hour).Unix()}
	rec := httptest.NewRecorder()
	if err := setSessionCookie(rec, cfg, s); err != nil {
		t.Fatalf("setSessionCookie: %v", err)
	}
	c := readSetCookie(t, rec, SessionCookieName)

	if !c.HttpOnly {
		t.Errorf("session cookie must be HttpOnly")
	}
	if !c.Secure {
		t.Errorf("session cookie must be Secure by default")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie SameSite = %v, want Lax", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("session cookie Path = %q, want /", c.Path)
	}
}

func TestSetSessionCookie_InsecureEscapeHatch(t *testing.T) {
	cfg := Config{BasePath: "", CookieSecret: testSecret, SecureCookie: false}
	s := &Session{Subject: "u", Expiry: time.Now().Add(time.Hour).Unix()}
	rec := httptest.NewRecorder()
	_ = setSessionCookie(rec, cfg, s)
	c := readSetCookie(t, rec, SessionCookieName)
	if c.Secure {
		t.Errorf("Secure should be false when SecureCookie=false (local-dev hatch)")
	}
	if c.Path != "/" {
		t.Errorf("root deploy cookie Path = %q, want /", c.Path)
	}
}

func TestClearSessionCookie(t *testing.T) {
	cfg := Config{BasePath: "/apps/t/s", CookieSecret: testSecret, SecureCookie: true}
	rec := httptest.NewRecorder()
	clearSessionCookie(rec, cfg)
	c := readSetCookie(t, rec, SessionCookieName)
	if c.MaxAge >= 0 {
		t.Errorf("cleared cookie MaxAge = %d, want negative", c.MaxAge)
	}
	if c.Value != "" {
		t.Errorf("cleared cookie Value = %q, want empty", c.Value)
	}
}

func TestReadSessionCookie(t *testing.T) {
	cfg := Config{BasePath: "", CookieSecret: testSecret, SecureCookie: true}
	s := &Session{Subject: "u-read", Expiry: time.Now().Add(time.Hour).Unix()}
	rec := httptest.NewRecorder()
	_ = setSessionCookie(rec, cfg, s)
	set := readSetCookie(t, rec, SessionCookieName)

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: set.Value})

	got, err := readSessionCookie(r, cfg)
	if err != nil {
		t.Fatalf("readSessionCookie: %v", err)
	}
	if got.Subject != "u-read" {
		t.Errorf("Subject = %q, want u-read", got.Subject)
	}
}

func TestStateCookieRoundTrip(t *testing.T) {
	cfg := Config{BasePath: "/apps/t/s", CookieSecret: testSecret, SecureCookie: true}
	hs := &handshakeState{
		State:        "state-nonce",
		CodeVerifier: "verifier-xyz",
		ReturnTo:     "/apps/t/s/dashboard",
		Prompt:       "none",
	}
	rec := httptest.NewRecorder()
	if err := setStateCookie(rec, cfg, hs); err != nil {
		t.Fatalf("setStateCookie: %v", err)
	}
	set := readSetCookie(t, rec, stateCookieName)
	if !set.HttpOnly || !set.Secure || set.SameSite != http.SameSiteLaxMode {
		t.Errorf("state cookie attributes wrong: %+v", set)
	}

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: stateCookieName, Value: set.Value})
	got, err := readStateCookie(r, cfg)
	if err != nil {
		t.Fatalf("readStateCookie: %v", err)
	}
	if got.State != "state-nonce" || got.CodeVerifier != "verifier-xyz" || got.Prompt != "none" {
		t.Errorf("state round-trip mismatch: %+v", got)
	}
}

func TestStateCookie_TamperFails(t *testing.T) {
	cfg := Config{BasePath: "", CookieSecret: testSecret, SecureCookie: true}
	hs := &handshakeState{State: "s", CodeVerifier: "v", ReturnTo: "/"}
	rec := httptest.NewRecorder()
	_ = setStateCookie(rec, cfg, hs)
	set := readSetCookie(t, rec, stateCookieName)

	parts := strings.SplitN(set.Value, ".", 2)
	tampered := parts[0] + "." + strings.Repeat("A", len(parts[1]))
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: stateCookieName, Value: tampered})
	if _, err := readStateCookie(r, cfg); err == nil {
		t.Errorf("readStateCookie accepted a tampered tag")
	}
}

func TestRandomURLToken(t *testing.T) {
	a := randomURLToken(24)
	b := randomURLToken(24)
	if a == b {
		t.Errorf("randomURLToken returned identical tokens — not random")
	}
	if a == "" {
		t.Errorf("randomURLToken returned empty")
	}
}

func TestCookieNames_NamespacedByClient(t *testing.T) {
	a := Config{ClientID: "app-aaa"}
	b := Config{ClientID: "app-bbb"}
	if a.sessionCookieName() == b.sessionCookieName() {
		t.Fatalf("session cookie names must differ per app: %q", a.sessionCookieName())
	}
	if !strings.HasPrefix(a.sessionCookieName(), SessionCookieName) {
		t.Fatalf("session cookie name %q must keep the %q prefix", a.sessionCookieName(), SessionCookieName)
	}
	if a.stateCookieName() == b.stateCookieName() {
		t.Fatalf("state cookie names must differ per app")
	}
}

func TestCookiePath_AlwaysRoot(t *testing.T) {
	if got := cookiePath("/apps/t/s"); got != "/" {
		t.Fatalf("cookiePath = %q, want / (root so it is returned on every app path on the host)", got)
	}
}

func TestSessionRoundTrip_NamespacedCookieReadBack(t *testing.T) {
	cfg := Config{ClientID: "app-aaa", CookieSecret: []byte("0123456789abcdef0123456789abcdef")}
	w := httptest.NewRecorder()
	if err := setSessionCookie(w, cfg, &Session{Subject: "u1", Expiry: time.Now().Add(time.Hour).Unix()}); err != nil {
		t.Fatal(err)
	}
	res := w.Result()
	c := res.Cookies()
	if len(c) != 1 || c[0].Name != cfg.sessionCookieName() || c[0].Path != "/" {
		t.Fatalf("set cookie = %+v, want name=%q path=/", c, cfg.sessionCookieName())
	}
	r := httptest.NewRequest("GET", "/admin", nil)
	r.AddCookie(c[0])
	sess, err := readSessionCookie(r, cfg)
	if err != nil || sess.Subject != "u1" {
		t.Fatalf("read back = %+v, err=%v", sess, err)
	}
}

// readSetCookie pulls a named cookie out of a recorder's Set-Cookie
// headers and fails the test if it is absent.
func readSetCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	resp := rec.Result()
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("Set-Cookie %q not found", name)
	return nil
}
