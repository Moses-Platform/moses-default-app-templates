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
// The package preserves the `X-Moses-*` trusted-header path for
// pod-to-pod MCP / workspace-tool calls: a request carrying
// `X-Moses-User-ID` (set by the platform's authenticated proxy on the
// in-cluster hop) is treated as already-authenticated and bypasses the
// OIDC redirect, so the OpenAPI tool surface keeps working. Browser
// requests with no session are redirected to the OIDC login.
//
// CHAT-t5d1u.28.21 (S3): the header-trust path is gated behind a
// shared-secret MARKER. The trusted-header decision is honoured ONLY
// when the request also carries header `X-Moses-Gateway-Auth` whose
// value equals the env var `MOSES_GATEWAY_AUTH_SECRET` (constant-time
// compare). When that env var is unset/empty the header-trust path is
// DISABLED entirely and the request falls through to the OIDC session
// path — a fail-safe default. This removes the previous reliance on the
// ingress stripping inbound `X-Moses-*` headers, which is brittle on
// hardened clusters.
//
// `/health` and the OpenAPI spec path are always public.
package oidcauth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env var names the platform injects (CHAT-t5d1u.28.5).
const (
	EnvIssuer         = "MOSES_OIDC_ISSUER"
	EnvInternalIssuer = "MOSES_OIDC_INTERNAL_ISSUER"
	EnvClientID       = "MOSES_OIDC_CLIENT_ID"
	EnvClientSecret   = "MOSES_OIDC_CLIENT_SECRET"
	EnvAudience       = "MOSES_OIDC_AUDIENCE"
	// EnvCookieNamespace, when set by the platform, pins the session/state
	// cookie NAMES to a stable per-(chart,track) token instead of the default
	// sha256(ClientID). The platform mints a fresh ClientID for every dev-track
	// redeploy, so the default made the cookie name churn each deploy; since app
	// cookies are Path=/ on the shared browser origin and never evicted, they
	// accumulated until the Cookie header overflowed the edge (nginx 400,
	// "Request Header Or Cookie Too Large"). A stable namespace makes each
	// redeploy overwrite one cookie. Empty ⇒ legacy sha256(ClientID) fallback.
	EnvCookieNamespace = "MOSES_OIDC_COOKIE_NAMESPACE"

	// EnvProtectedPaths / EnvPublicPaths let the app's `access.oidc`
	// block flow in as env vars (space- or comma-separated path
	// prefixes). Both are optional; defaults are sensible.
	EnvProtectedPaths = "MOSES_OIDC_PROTECTED_PATHS"
	EnvPublicPaths    = "MOSES_OIDC_PUBLIC_PATHS"

	// EnvBasePath is the deploy sub-path (e.g. /apps/<tenant>/<slug>).
	// Callback/logout routes and the session cookie Path are derived
	// from it. Shared with the rest of the template (MOSES_BASE_PATH).
	EnvBasePath = "MOSES_BASE_PATH"

	// EnvCookieSecret keys the session cookie's AES-256-GCM encryption
	// (and the state cookie's HMAC tag). The platform should inject a
	// per-deploy random value; absent one, the middleware derives a
	// process-local fallback (sessions then do not survive a pod
	// restart — acceptable, fail-safe).
	EnvCookieSecret = "MOSES_OIDC_COOKIE_SECRET"

	// EnvInsecureCookie, when "1"/"true", drops the cookie Secure
	// attribute. Local-dev / plain-HTTP escape hatch ONLY; never set
	// it on a real deploy. Default is Secure.
	EnvInsecureCookie = "MOSES_OIDC_INSECURE_COOKIE"

	// EnvGatewayAuthSecret (CHAT-t5d1u.28.21 S3) is the shared-secret
	// marker that gates the X-Moses-* header-trust path. The platform's
	// authenticated proxy (WorkspaceToolProxy) sends header
	// HeaderGatewayAuth carrying this exact value on the in-cluster hop;
	// the middleware honours the X-Moses-* header-trust decision ONLY
	// when that header is present and matches (constant-time). When this
	// env var is unset/empty the header-trust path is DISABLED entirely
	// (fail-safe — requests fall through to the OIDC session path).
	EnvGatewayAuthSecret = "MOSES_GATEWAY_AUTH_SECRET"

	// EnvPublicURL is the external scheme+host+port the browser uses to
	// reach the app (e.g. "http://localhost:9877" on desktop,
	// "https://moses.example.com" in prod). The platform injects it from
	// spec.domain so the middleware can build OIDC redirect_uri /
	// post_logout_redirect_uri values that match what is registered with
	// the IdP. Envoy Gateway strips :port from the Host header (Gateway
	// API hostname matching is port-agnostic by spec), so deriving the
	// host from r.Host loses the port and produces a redirect_uri the IdP
	// rejects ("Invalid parameter: redirect_uri"). When unset the
	// middleware falls back to the legacy r.Host-based derivation —
	// preserves behaviour for non-platform deployments and existing tests.
	EnvPublicURL = "MOSES_PUBLIC_URL"

	// EnvPublicURLs is the comma/space-separated set of every external origin
	// (scheme://host[:port], no path) the app is reachable at — platform
	// subpath, apex custom hostname, remote/tunnel host. absoluteURL selects
	// the entry whose hostname matches the incoming request so login completes
	// on whichever host the browser used. When unset, absoluteURL falls back to
	// the single EnvPublicURL (legacy / standalone deploys).
	EnvPublicURLs = "MOSES_PUBLIC_URLS"

	// EnvSessionMaxAgeSeconds caps the BFF session lifetime in seconds
	// (default 8h). The session deliberately OUTLIVES the short (~120s)
	// Moses app tokens — the token lifetime bounds only the ROLES
	// snapshot, which the middleware renews via the refresh-token
	// grant. Each successful renewal re-stamps the session, so this
	// value behaves as an INACTIVITY bound (the window slides while
	// the user stays active). Keycloak's SSO-session max (10h default)
	// is the absolute server-side ceiling: once the KC session ends,
	// the refresh grant fails and the session dies regardless of this
	// value.
	EnvSessionMaxAgeSeconds = "MOSES_OIDC_SESSION_MAX_AGE_SECONDS"
)

// defaultSessionMaxAge is the BFF session lifetime when
// EnvSessionMaxAgeSeconds is unset or unparsable.
const defaultSessionMaxAge = 8 * time.Hour

// HeaderGatewayAuth (CHAT-t5d1u.28.21 S3) is the request header whose
// value the platform's in-cluster proxy sets to the shared
// MOSES_GATEWAY_AUTH_SECRET. It is the marker that authorises the
// X-Moses-* header-trust path.
const HeaderGatewayAuth = "X-Moses-Gateway-Auth"

// HeaderMosesRoles is the request header carrying comma-separated app role
// names granted to AGENT calls by the tenant admin (Moses shield modal →
// Agents toggle). It is honoured ONLY on the header-trust path — i.e. only
// when the request also passed the HeaderGatewayAuth constant-time check —
// so an external caller cannot mint roles by setting the header.
const HeaderMosesRoles = "X-Moses-Roles"

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

	// CookieNamespace, when non-empty, is the stable per-(chart,track) token the
	// session/state cookie NAMES are derived from (see EnvCookieNamespace).
	// Empty ⇒ cookieSuffix() falls back to sha256(ClientID).
	CookieNamespace string

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

	// CookieSecret keys the session cookie's AES-256-GCM encryption
	// and the state cookie's HMAC tag.
	CookieSecret []byte

	// SecureCookie controls the cookie Secure attribute. Default
	// true; flipped off only by EnvInsecureCookie for local dev.
	SecureCookie bool

	// SessionMaxAge is the BFF session's hard lifetime (see
	// EnvSessionMaxAgeSeconds). Zero means defaultSessionMaxAge.
	SessionMaxAge time.Duration

	// SpecPath is the OpenAPI spec path treated as always-public
	// (default "/api/openapi.json").
	SpecPath string

	// GatewayAuthSecret (CHAT-t5d1u.28.21 S3) is the shared-secret
	// marker that authorises the X-Moses-* header-trust path. The
	// middleware honours a request's X-Moses-* trusted headers ONLY when
	// the request also carries header HeaderGatewayAuth whose value
	// equals this secret (compared in constant time). When empty the
	// header-trust path is disabled entirely — a fail-safe default that
	// falls through to the OIDC session path.
	GatewayAuthSecret string

	// PublicURL is the external scheme+host+port the browser uses to
	// reach the app (e.g. "http://localhost:9877" on desktop). When set,
	// absoluteURL prefers it over the r.Host-based derivation — envoy
	// Gateway strips :port from Host, so r.Host alone produces an OIDC
	// redirect_uri the IdP rejects. When empty (non-platform deployments)
	// the middleware falls back to the legacy header/Host derivation.
	// Trailing slash is trimmed at use site.
	PublicURL string

	// PublicURLs is the allowlist of reachable external origins. Per request,
	// absoluteURL picks the entry whose hostname matches X-Forwarded-Host/r.Host
	// (the matched origin supplies the correct scheme+port). Empty -> fall back
	// to PublicURL then r.Host.
	PublicURLs []string

	// InterAppSecret (P2c) is the per-tenant HS256 key shared by every
	// interop app in the tenant. It signs/verifies the inter-app trust
	// token (see interapp.go). Empty -> the inter-app path is disabled
	// (interAppEnabled() false), INDEPENDENT of OIDC Enabled().
	InterAppSecret string

	// AppSlug is this app's own logical slug (MOSES_APP_SLUG). It is the
	// `iss` stamped when minting and the required `aud` when verifying an
	// inbound inter-app token. Shared with the inter-app path only.
	AppSlug string

	// TenantID is this app's own tenant UUID (MOSES_TENANT_ID). A verified
	// inter-app token's tenant_id must equal it.
	TenantID string
}

// cookieSuffix is a short, stable per-app token derived from the client id. It
// namespaces the cookie names so co-tenant apps sharing the platform host (each
// under its own /apps/<tenant>/<slug>/ subpath but all setting Path=/) do not
// collide. Empty client id -> no suffix (standalone/dev).
func (c Config) cookieSuffix() string {
	// Prefer the platform-supplied stable per-(chart,track) namespace so the
	// cookie NAME does not churn when the platform mints a fresh ClientID on
	// every dev-track redeploy (see EnvCookieNamespace). Hashing keeps the suffix
	// a fixed 8 hex chars regardless of source. Falls back to the ClientID hash
	// for older images / installs that do not inject the namespace — those stay
	// byte-for-byte identical to the previous behaviour.
	src := c.CookieNamespace
	if src == "" {
		src = c.ClientID
	}
	if src == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(src))
	return hex.EncodeToString(sum[:4]) // 8 hex chars
}

func (c Config) sessionCookieName() string {
	if s := c.cookieSuffix(); s != "" {
		return SessionCookieName + "_" + s
	}
	return SessionCookieName
}

func (c Config) stateCookieName() string {
	if s := c.cookieSuffix(); s != "" {
		return stateCookieName + "_" + s
	}
	return stateCookieName
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
		Issuer:            strings.TrimRight(strings.TrimSpace(os.Getenv(EnvIssuer)), "/"),
		InternalIssuer:    strings.TrimRight(strings.TrimSpace(os.Getenv(EnvInternalIssuer)), "/"),
		ClientID:          strings.TrimSpace(os.Getenv(EnvClientID)),
		ClientSecret:      strings.TrimSpace(os.Getenv(EnvClientSecret)),
		CookieNamespace:   strings.TrimSpace(os.Getenv(EnvCookieNamespace)),
		Audience:          strings.TrimSpace(os.Getenv(EnvAudience)),
		BasePath:          strings.TrimRight(strings.TrimSpace(os.Getenv(EnvBasePath)), "/"),
		ProtectedPaths:    splitPaths(os.Getenv(EnvProtectedPaths)),
		PublicPaths:       splitPaths(os.Getenv(EnvPublicPaths)),
		SpecPath:          "/api/openapi.json",
		SecureCookie:      !isTruthy(os.Getenv(EnvInsecureCookie)),
		GatewayAuthSecret: strings.TrimSpace(os.Getenv(EnvGatewayAuthSecret)),
		PublicURL:         strings.TrimSpace(os.Getenv(EnvPublicURL)),
		PublicURLs:        splitOrigins(os.Getenv(EnvPublicURLs)),
		InterAppSecret:    strings.TrimSpace(os.Getenv(EnvInterAppSecret)),
		AppSlug:           strings.TrimSpace(os.Getenv(EnvAppSlug)),
		TenantID:          strings.TrimSpace(os.Getenv(EnvTenantID)),
		SessionMaxAge:     sessionMaxAgeFromEnv(),
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

// splitOrigins parses a comma/space-separated list of scheme://host[:port]
// origins into a trimmed slice. Empty/blank input -> nil. Trailing slashes are
// stripped so the value joins cleanly with a route path.
func splitOrigins(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	var out []string
	for _, f := range fields {
		if f = strings.TrimRight(strings.TrimSpace(f), "/"); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// sessionMaxAge returns the effective session lifetime.
func (c Config) sessionMaxAge() time.Duration {
	if c.SessionMaxAge > 0 {
		return c.SessionMaxAge
	}
	return defaultSessionMaxAge
}

// sessionMaxAgeFromEnv parses EnvSessionMaxAgeSeconds, falling back to
// the default on an unset, unparsable, or non-positive value.
func sessionMaxAgeFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv(EnvSessionMaxAgeSeconds))
	if raw == "" {
		return defaultSessionMaxAge
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs <= 0 {
		return defaultSessionMaxAge
	}
	return time.Duration(secs) * time.Second
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
