package oidcauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// SessionCookieName is the app-host-scoped session cookie. It is
// HttpOnly (no JS access — BFF: the browser never touches the token)
// and, by default, Secure.
const SessionCookieName = "moses_app_session"

// stateCookieName is a short-lived cookie that carries the OIDC `state`
// nonce and the PKCE verifier across the authorize redirect. It is
// cleared by the callback handler.
const stateCookieName = "moses_app_oidc_state"

// SameSite=Lax is the chosen value for the session cookie.
//
// WHY Lax (documented per the ticket):
//
//   - The Moses Manager iframe embeds the app at a SAME-SITE sub-path
//     (`/apps/<tenant>/<slug>/` on the very same host as Manager). A
//     same-site subframe is, for cookie purposes, same-site — Lax
//     cookies ARE sent on its requests, including its top-level
//     document load and its same-origin `fetch`/XHR. So Lax works for
//     the in-iframe case here.
//   - On a standalone custom hostname the app is the top-level
//     document; Lax cookies are always sent same-origin. Works.
//   - SameSite=None would be required only if the app were embedded
//     cross-site (a different registrable domain than the embedder),
//     and None additionally MANDATES Secure — which breaks plain-HTTP
//     local dev. Moses embeds same-site, so None buys nothing and
//     costs CSRF surface + a hard Secure requirement.
//   - SameSite=Strict would drop the cookie on the cross-site
//     top-level navigation BACK from Keycloak to `/auth/callback`
//     after login, silently breaking the auth-code round-trip. The
//     state cookie below works around that for the handshake, but Lax
//     keeps the steady-state session cookie present on the post-login
//     redirect too.
//
// Net: Lax is the correct, browser-default-aligned choice for a
// same-site-embedded BFF. The callback handshake's cross-site return
// is covered because the `state` cookie is also Lax and IS sent on the
// top-level GET navigation from Keycloak back to /auth/callback.
const sessionSameSite = http.SameSiteLaxMode

// Session is the server-side-validated session payload. It is the
// minimal projection of the validated ID token the app needs, plus the
// OIDC refresh token used to re-mint the roles snapshot. Stored
// AES-256-GCM encrypted inside the cookie value; the browser can
// neither read nor forge it.
type Session struct {
	Subject  string   `json:"sub"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"` // resource_access.<client>.roles snapshot
	Expiry   int64    `json:"exp"`   // unix seconds — session hard expiry
	IssuedAt int64    `json:"iat"`

	// RolesFreshUntil (unix seconds) bounds the ROLES snapshot's
	// validity — it tracks the ID/access token lifetime (~120s on Moses
	// app clients, the platform's role-revocation latency contract).
	// The middleware re-mints the snapshot via the refresh-token grant
	// when it goes stale; the SESSION (Expiry) lives much longer. Zero
	// means "no bound" (roles stay as-is until Expiry).
	RolesFreshUntil int64 `json:"rfu,omitempty"`

	// RefreshToken is the confidential-client OIDC refresh token used
	// to renew the roles snapshot. It is useless without the client
	// secret (Keycloak refresh grants require client authentication),
	// and the cookie payload is encrypted at rest in the browser.
	RefreshToken string `json:"rt,omitempty"`
}

// Valid reports whether the session is structurally usable and unexpired.
func (s *Session) Valid(now time.Time) bool {
	if s == nil || s.Subject == "" || s.Expiry == 0 {
		return false
	}
	return now.Before(time.Unix(s.Expiry, 0))
}

// rolesStale reports whether the roles snapshot has outlived its bound.
// FAIL-SAFE: a zero RolesFreshUntil counts as stale — sessionFromClaims
// always stamps the bound, so a session without one was not minted by
// the login/refresh path and must not carry roles until Expiry. (The
// ~120s role-revocation contract depends on this default staying
// stale-on-zero.)
func (s *Session) rolesStale(now time.Time) bool {
	return s.RolesFreshUntil == 0 || !now.Before(time.Unix(s.RolesFreshUntil, 0))
}

// cookiePath returns the Path attribute for the app's cookies. It is ALWAYS
// "/" so the session is returned on every path the app occupies on a host —
// root on a custom (apex) hostname, the /apps/<tenant>/<slug>/ subpath on the
// platform host. Cross-app isolation on a shared host is provided by the
// per-app cookie NAME (Config.sessionCookieName / stateCookieName), not by Path.
func cookiePath(string) string { return "/" }

// sessionCookieVersion prefixes the encrypted format so a future codec
// change (and today's legacy HMAC cookies) can be told apart cheaply.
const sessionCookieVersion = "v2"

// sessionAEAD builds the AES-256-GCM sealer/opener for the session
// cookie. The 32-byte key is derived from Config.CookieSecret so
// operators keep exactly one secret to manage.
func sessionAEAD(secret []byte) (cipher.AEAD, error) {
	key := sha256.Sum256(secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// encodeSession serializes + AES-256-GCM encrypts a Session into a
// cookie value:
//
//	"v2." base64url(nonce || ciphertext)
//
// GCM is an AEAD: the value is both confidential (it now carries the
// OIDC refresh token) and tamper-proof — the browser can neither read
// secrets nor forge a different subject. Pre-v2 (HMAC-signed plaintext)
// cookies fail decodeSession and simply trigger one re-login.
func encodeSession(s *Session, secret []byte) (string, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	aead, err := sessionAEAD(secret)
	if err != nil {
		return "", err
	}
	nonce := randomBytes(aead.NonceSize())
	sealed := aead.Seal(nonce, nonce, payload, nil)
	return sessionCookieVersion + "." + base64.RawURLEncoding.EncodeToString(sealed), nil
}

// decodeSession authenticates + decrypts the Session. A tampered,
// wrong-key, or legacy-format value fails the AEAD open.
func decodeSession(value string, secret []byte) (*Session, error) {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 || parts[0] != sessionCookieVersion {
		return nil, errors.New("oidcauth: unrecognised session cookie format")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("oidcauth: session cookie not base64url")
	}
	aead, err := sessionAEAD(secret)
	if err != nil {
		return nil, err
	}
	if len(sealed) < aead.NonceSize() {
		return nil, errors.New("oidcauth: session cookie too short")
	}
	payload, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], nil)
	if err != nil {
		return nil, errors.New("oidcauth: session cookie authentication failed")
	}
	var s Session
	if err := json.Unmarshal(payload, &s); err != nil {
		return nil, errors.New("oidcauth: session cookie payload not JSON")
	}
	return &s, nil
}

// setSessionCookie writes the signed session cookie. HttpOnly, Path=/,
// SameSite=Lax (see sessionSameSite), Secure unless explicitly disabled for
// local dev. Cookie name is per-app (Config.sessionCookieName) so co-tenant
// apps on the same host do not collide when all share Path=/.
func setSessionCookie(w http.ResponseWriter, cfg Config, s *Session) error {
	value, err := encodeSession(s, cfg.CookieSecret)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.sessionCookieName(),
		Value:    value,
		Path:     cookiePath(cfg.BasePath),
		HttpOnly: true,
		Secure:   cfg.SecureCookie,
		SameSite: sessionSameSite,
		Expires:  time.Unix(s.Expiry, 0),
		MaxAge:   int(time.Until(time.Unix(s.Expiry, 0)).Seconds()),
	})
	return nil
}

// clearSessionCookie expires the session cookie (used by /auth/logout).
func clearSessionCookie(w http.ResponseWriter, cfg Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.sessionCookieName(),
		Value:    "",
		Path:     cookiePath(cfg.BasePath),
		HttpOnly: true,
		Secure:   cfg.SecureCookie,
		SameSite: sessionSameSite,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

// readSessionCookie extracts and verifies the session from a request.
func readSessionCookie(r *http.Request, cfg Config) (*Session, error) {
	c, err := r.Cookie(cfg.sessionCookieName())
	if err != nil {
		return nil, err
	}
	return decodeSession(c.Value, cfg.CookieSecret)
}

// --- OIDC handshake state cookie -------------------------------------

// handshakeState bundles the data the callback handler needs to finish
// the auth-code exchange: the CSRF `state` value, the OIDC `nonce`, the
// PKCE code verifier, and where to send the user afterwards.
type handshakeState struct {
	State        string `json:"state"`
	CodeVerifier string `json:"cv"`
	ReturnTo     string `json:"rt"`
	Prompt       string `json:"p"` // "none" for the silent-SSO attempt

	// Nonce is the OIDC nonce sent on the authorization request and
	// verified against the ID token's `nonce` claim at the callback. It
	// binds the issued token to THIS handshake (replay protection),
	// complementing `state` (which binds the callback to this browser)
	// and PKCE (which binds the code to this client).
	Nonce string `json:"nc"`
}

// setStateCookie writes the short-lived (10 min) handshake cookie. It is
// also Lax so it survives the top-level redirect back from Keycloak.
func setStateCookie(w http.ResponseWriter, cfg Config, hs *handshakeState) error {
	payload, err := json.Marshal(hs)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, cfg.CookieSecret)
	mac.Write(payload)
	signed := base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.stateCookieName(),
		Value:    signed,
		Path:     cookiePath(cfg.BasePath),
		HttpOnly: true,
		Secure:   cfg.SecureCookie,
		SameSite: sessionSameSite,
		MaxAge:   600,
	})
	return nil
}

// readStateCookie verifies and decodes the handshake cookie.
func readStateCookie(r *http.Request, cfg Config) (*handshakeState, error) {
	c, err := r.Cookie(cfg.stateCookieName())
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(c.Value, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("oidcauth: malformed state cookie")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("oidcauth: state cookie payload not base64url")
	}
	gotTag, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("oidcauth: state cookie tag not base64url")
	}
	mac := hmac.New(sha256.New, cfg.CookieSecret)
	mac.Write(payload)
	if subtle.ConstantTimeCompare(gotTag, mac.Sum(nil)) != 1 {
		return nil, errors.New("oidcauth: state cookie signature invalid")
	}
	var hs handshakeState
	if err := json.Unmarshal(payload, &hs); err != nil {
		return nil, errors.New("oidcauth: state cookie payload not JSON")
	}
	return &hs, nil
}

// clearStateCookie expires the handshake cookie after the callback.
func clearStateCookie(w http.ResponseWriter, cfg Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.stateCookieName(),
		Value:    "",
		Path:     cookiePath(cfg.BasePath),
		HttpOnly: true,
		Secure:   cfg.SecureCookie,
		SameSite: sessionSameSite,
		MaxAge:   -1,
	})
}

// randomBytes returns n cryptographically-random bytes.
func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is catastrophic; surface it loudly
		// rather than returning predictable zeros.
		panic("oidcauth: crypto/rand failed: " + err.Error())
	}
	return b
}

// randomURLToken returns a URL-safe random token of byte-length n.
func randomURLToken(n int) string {
	return base64.RawURLEncoding.EncodeToString(randomBytes(n))
}
