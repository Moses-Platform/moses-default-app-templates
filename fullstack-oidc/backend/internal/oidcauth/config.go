// Package oidcauth is a self-contained, stdlib-only OIDC relying-party
// (BFF — Backend For Frontend) middleware for Moses-deployed apps.
//
// VENDORED, NOT IMPORTED. This package is delivered by copying the whole
// `oidcauth/` directory into a template's `backend/internal/oidcauth/`.
// It has no third-party Go dependencies — RSA/JWT/JWKS verification is
// built on `crypto/rsa`, `crypto/sha256`, and `encoding/json` from the
// standard library, exactly like `shared/mosesproxy-go`. A template that
// vendors this package ships as a self-contained repo with no `replace`
// directive and no dependency on the templates monorepo.
//
// # What it does (BFF / authorization-code + PKCE)
//
// A Moses-deployed app becomes an OIDC relying party. The platform
// (ticket CHAT-t5d1u.28.5) injects five env vars into the app pod:
//
//	MOSES_OIDC_ISSUER           external issuer URL — browser redirects + iss claim
//	MOSES_OIDC_INTERNAL_ISSUER  in-cluster issuer URL — JWKS fetch (server-to-server)
//	MOSES_OIDC_CLIENT_ID        confidential client id
//	MOSES_OIDC_CLIENT_SECRET    confidential client secret
//	MOSES_OIDC_AUDIENCE         expected `aud` claim (often == client id)
//
// The middleware runs the authorization-code flow with PKCE, exchanges
// the code at the token endpoint using the confidential client secret,
// validates the ID token's signature against JWKS, and establishes a
// server-side-validated, HttpOnly session cookie. The browser never
// sees a token — that is the BFF pattern.
//
// # Dual mode
//
// The package preserves the existing `X-Moses-*` trusted-header path for
// pod-to-pod MCP / workspace-tool calls: a request carrying
// `X-Moses-User-ID` (set by the platform's authenticated proxy on the
// in-cluster hop) is treated as already-authenticated and bypasses the
// OIDC redirect, so the OpenAPI tool surface keeps working. Browser
// requests with no session are redirected to the OIDC login.
//
// `/health` and the OpenAPI spec path are always public.
package oidcauth

import (
	"fmt"
	"os"
	"strings"
)

// Env var names the platform injects (CHAT-t5d1u.28.5).
const (
	EnvIssuer         = "MOSES_OIDC_ISSUER"
	EnvInternalIssuer = "MOSES_OIDC_INTERNAL_ISSUER"
	EnvClientID       = "MOSES_OIDC_CLIENT_ID"
	EnvClientSecret   = "MOSES_OIDC_CLIENT_SECRET"
	EnvAudience       = "MOSES_OIDC_AUDIENCE"

	// EnvProtectedPaths / EnvPublicPaths let the app's `access.oidc`
	// block flow in as env vars (space- or comma-separated path
	// prefixes). Both are optional; defaults are sensible.
	EnvProtectedPaths = "MOSES_OIDC_PROTECTED_PATHS"
	EnvPublicPaths    = "MOSES_OIDC_PUBLIC_PATHS"

	// EnvBasePath is the deploy sub-path (e.g. /apps/<tenant>/<slug>).
	// Callback/logout routes and the session cookie Path are derived
	// from it. Shared with the rest of the template (MOSES_BASE_PATH).
	EnvBasePath = "MOSES_BASE_PATH"

	// EnvCookieSecret keys the cookie's HMAC integrity tag. The
	// platform should inject a per-deploy random value; absent one,
	// the middleware derives a process-local fallback (sessions then
	// do not survive a pod restart — acceptable, fail-safe).
	EnvCookieSecret = "MOSES_OIDC_COOKIE_SECRET"

	// EnvInsecureCookie, when "1"/"true", drops the cookie Secure
	// attribute. Local-dev / plain-HTTP escape hatch ONLY; never set
	// it on a real deploy. Default is Secure.
	EnvInsecureCookie = "MOSES_OIDC_INSECURE_COOKIE"
)

// Config is the fully-resolved middleware configuration. Build it from
// the environment with ConfigFromEnv, or assemble one directly in tests.
type Config struct {
	// Issuer is the EXTERNAL issuer URL. Used for browser-facing
	// redirects (authorize / end-session endpoints) and for `iss`
	// claim validation — the token's `iss` is minted by the external
	// issuer even though JWKS is fetched in-cluster.
	Issuer string

	// InternalIssuer is the IN-CLUSTER issuer URL. Used for the
	// server-to-server JWKS fetch and the token-endpoint code
	// exchange. Falls back to Issuer when unset.
	InternalIssuer string

	ClientID     string
	ClientSecret string

	// Audience is the expected `aud` claim. When empty, audience
	// validation falls back to requiring ClientID in `aud`.
	Audience string

	// BasePath is the deploy sub-path, no trailing slash ("" for root
	// deploys). Callback/logout routes and the cookie Path derive
	// from it.
	BasePath string

	// ProtectedPaths is the set of path prefixes (relative to
	// BasePath) that require an authenticated session. A request
	// whose path matches any prefix is gated. Empty ProtectedPaths
	// means "protect everything not explicitly public".
	ProtectedPaths []string

	// PublicPaths is the set of path prefixes that are always
	// reachable without a session. /health and the OpenAPI spec path
	// are added implicitly — callers need not list them.
	PublicPaths []string

	// CookieSecret keys the session cookie's HMAC tag.
	CookieSecret []byte

	// SecureCookie controls the cookie Secure attribute. Default
	// true; flipped off only by EnvInsecureCookie for local dev.
	SecureCookie bool

	// SpecPath is the OpenAPI spec path treated as always-public
	// (default "/api/openapi.json").
	SpecPath string
}

// ConfigFromEnv builds a Config from the platform-injected environment.
// It never returns an error for missing OIDC vars — instead the
// resulting Config reports !Config.Enabled(), and the middleware then
// runs in pass-through mode (header-trust path still works, browser
// requests are NOT redirected). This mirrors mosesproxy-go's
// graceful-degradation contract: a misconfigured deploy must not hard
// 500 every request; it degrades visibly.
func ConfigFromEnv() Config {
	cfg := Config{
		Issuer:         strings.TrimRight(strings.TrimSpace(os.Getenv(EnvIssuer)), "/"),
		InternalIssuer: strings.TrimRight(strings.TrimSpace(os.Getenv(EnvInternalIssuer)), "/"),
		ClientID:       strings.TrimSpace(os.Getenv(EnvClientID)),
		ClientSecret:   strings.TrimSpace(os.Getenv(EnvClientSecret)),
		Audience:       strings.TrimSpace(os.Getenv(EnvAudience)),
		BasePath:       strings.TrimRight(strings.TrimSpace(os.Getenv(EnvBasePath)), "/"),
		ProtectedPaths: splitPaths(os.Getenv(EnvProtectedPaths)),
		PublicPaths:    splitPaths(os.Getenv(EnvPublicPaths)),
		SpecPath:       "/api/openapi.json",
		SecureCookie:   !isTruthy(os.Getenv(EnvInsecureCookie)),
	}

	if cfg.InternalIssuer == "" {
		cfg.InternalIssuer = cfg.Issuer
	}

	if secret := strings.TrimSpace(os.Getenv(EnvCookieSecret)); secret != "" {
		cfg.CookieSecret = []byte(secret)
	} else {
		// Process-local fallback. Sessions do not survive a pod
		// restart, which is fail-safe (re-login), not fail-open.
		cfg.CookieSecret = randomBytes(32)
	}

	return cfg
}

// Enabled reports whether OIDC enforcement can run. It requires the
// issuer, client id, and client secret. When false the middleware
// degrades to pass-through mode.
func (c Config) Enabled() bool {
	return c.Issuer != "" && c.ClientID != "" && c.ClientSecret != ""
}

// Validate returns a non-nil error describing why the config cannot
// enforce OIDC. It is advisory — main() can log it and continue (the
// app still serves public routes + the header-trust path). It is NOT
// called implicitly by ConfigFromEnv.
func (c Config) Validate() error {
	var missing []string
	if c.Issuer == "" {
		missing = append(missing, EnvIssuer)
	}
	if c.ClientID == "" {
		missing = append(missing, EnvClientID)
	}
	if c.ClientSecret == "" {
		missing = append(missing, EnvClientSecret)
	}
	if len(missing) > 0 {
		return fmt.Errorf("oidcauth: OIDC disabled — missing env: %s", strings.Join(missing, ", "))
	}
	return nil
}

// expectedAudience returns the `aud` value the verifier must find. When
// Audience is set it is authoritative; otherwise the client id is used
// (the common Keycloak default where the client is its own audience).
func (c Config) expectedAudience() string {
	if c.Audience != "" {
		return c.Audience
	}
	return c.ClientID
}

// splitPaths parses a space- or comma-separated list of path prefixes
// into a normalized, leading-slash slice. Empty/blank input -> nil.
func splitPaths(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	var out []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if !strings.HasPrefix(f, "/") {
			f = "/" + f
		}
		out = append(out, f)
	}
	return out
}

// isTruthy reports whether s is an affirmative env-var value.
func isTruthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
