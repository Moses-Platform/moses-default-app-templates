package oidcauth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// decision is the per-request gating outcome.
type decision int

const (
	// decisionPublic — serve without any auth (health, OpenAPI spec,
	// the auth handshake routes, anything in PublicPaths).
	decisionPublic decision = iota
	// decisionHeaderTrust — request carries trusted X-Moses-* headers
	// (pod-to-pod MCP / workspace-tool call); serve, populating the
	// auth context from the headers, NO OIDC redirect.
	decisionHeaderTrust
	// decisionSession — request has a valid OIDC session cookie;
	// serve, populating the auth context from the session.
	decisionSession
	// decisionChallenge — protected path, no session, browser
	// request; redirect to the OIDC login.
	decisionChallenge
	// decisionInterApp — request carries a valid X-Moses-Interapp-Token
	// (a sibling app calling on behalf of the current user, P2c); serve,
	// populating the auth context from the token's moses_user_id, with NO
	// roles and NO OIDC redirect.
	decisionInterApp
)

// trimBasePath removes the deploy sub-path prefix from a request path so
// matching is done in the app's own route space. A request that is not
// under BasePath is returned unchanged (defensive — the ingress should
// never deliver such a request, but matching must still be sane).
func trimBasePath(reqPath, basePath string) string {
	bp := strings.TrimRight(basePath, "/")
	if bp == "" {
		return ensureLeadingSlash(reqPath)
	}
	if reqPath == bp {
		return "/"
	}
	if strings.HasPrefix(reqPath, bp+"/") {
		return reqPath[len(bp):]
	}
	return ensureLeadingSlash(reqPath)
}

func ensureLeadingSlash(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

// matchesPrefix reports whether appPath is at or under any of the given
// prefixes. A prefix matches the exact path, or the path with a "/"
// boundary after the prefix (so "/api" matches "/api" and "/api/v1" but
// NOT "/apiv2").
func matchesPrefix(appPath string, prefixes []string) bool {
	for _, pfx := range prefixes {
		pfx = ensureLeadingSlash(pfx)
		if appPath == pfx {
			return true
		}
		if pfx == "/" {
			return true
		}
		trimmed := strings.TrimRight(pfx, "/")
		if appPath == trimmed || strings.HasPrefix(appPath, trimmed+"/") {
			return true
		}
	}
	return false
}

// alwaysPublic returns the implicitly-public path set: /health, the
// OpenAPI spec path, and the auth handshake routes. These are matched
// in the app's own route space (after trimBasePath).
func (m *Middleware) alwaysPublic() []string {
	return []string{
		"/health",
		m.cfg.SpecPath,
		"/api/openapi.json",
		"/api/spec",
		routeCallback,
		routeLogin,
		routeLogout,
		routeSilentCheck,
	}
}

// headerTrustEnabled reports whether the header-trust path is armed for
// this middleware. CHAT-t5d1u.28.21 (S3): the path is gated behind a
// shared-secret marker; when GatewayAuthSecret is unset the path is
// disabled entirely (fail-safe — requests fall through to OIDC).
func (m *Middleware) headerTrustEnabled() bool {
	return m.cfg.GatewayAuthSecret != ""
}

// isHeaderTrusted reports whether the request may use the platform's
// trusted pod-to-pod header path. CHAT-t5d1u.28.21 (S3): two conditions
// must BOTH hold:
//
//  1. The request carries a non-blank X-Moses-User-ID (the principal
//     the platform's authenticated proxy stamped on the in-cluster hop).
//  2. The request carries header HeaderGatewayAuth whose value equals
//     the configured GatewayAuthSecret (constant-time compare).
//
// Condition 2 is the security fix: relying only on condition 1 made the
// header-trust path depend entirely on the ingress stripping inbound
// X-Moses-* headers — a strip that is brittle on hardened clusters
// (the nginx snippet fails admission). The shared-secret marker means a
// forged X-Moses-* header without the marker is rejected even if it
// reaches the pod.
//
// When GatewayAuthSecret is unset the header-trust path is DISABLED:
// isHeaderTrusted always returns false and the request falls through to
// the OIDC session path (fail-safe).
func (m *Middleware) isHeaderTrusted(r *http.Request) bool {
	if !m.headerTrustEnabled() {
		return false
	}
	if strings.TrimSpace(r.Header.Get("X-Moses-User-ID")) == "" {
		return false
	}
	marker := r.Header.Get(HeaderGatewayAuth)
	return subtle.ConstantTimeCompare([]byte(marker), []byte(m.cfg.GatewayAuthSecret)) == 1
}

// classify decides how to handle a request. It is pure given the
// request, the config, and the (already-verified) session — no I/O — so
// it is exhaustively unit-testable. When the outcome is decisionInterApp
// the second return value is the trusted moses_user_id from the token.
//
// Order of precedence:
//  1. Public paths (incl. /health + spec + handshake routes) -> public.
//  2. Trusted X-Moses-* pod-to-pod header -> header-trust (dual mode).
//  3. Valid X-Moses-Interapp-Token from a sibling app (P2c) -> inter-app.
//  4. Valid session cookie -> session.
//  5. Path is protected (or ProtectedPaths empty == protect-all) and
//     this is a browser request -> challenge.
//  6. Otherwise -> public (unprotected path, no creds needed).
//
// WHY inter-app is checked AFTER header-trust but BEFORE session: the
// gateway header-trust path is the higher-trust platform-authenticated
// pod-to-pod hop (gated by the gateway shared secret), so it wins when
// both somehow appear on one request. The inter-app token is a sibling's
// assertion, so it is consulted next — ahead of the browser session,
// which it is meant to stand in for on the server-to-server hop. A
// BAD/expired/wrong-aud token is NOT a hard failure: verifyInterAppToken
// returns an error, classify ignores the token, and the request falls
// through to the normal session/protected/public path. So a forged
// inter-app token never breaks a legitimate browser session.
func (m *Middleware) classify(r *http.Request, sess *Session, sessionValid bool) (decision, string) {
	appPath := trimBasePath(r.URL.Path, m.cfg.BasePath)

	if matchesPrefix(appPath, m.alwaysPublic()) || matchesPrefix(appPath, m.cfg.PublicPaths) {
		return decisionPublic, ""
	}

	// Dual mode: a marker-gated, trusted pod-to-pod header bypasses OIDC
	// entirely so MCP / workspace-tool calls keep working. CHAT-t5d1u.28.21
	// (S3): isHeaderTrusted now also requires the X-Moses-Gateway-Auth
	// shared-secret marker, and is disabled when no secret is configured.
	if m.isHeaderTrusted(r) {
		return decisionHeaderTrust, ""
	}

	// Inter-app trust (P2c): a sibling app calling on behalf of the
	// current user. Gated independently of OIDC by interAppEnabled().
	// Verify is pure (no I/O). On ANY validation failure we ignore the
	// token and fall through — a bad token must not break a real session.
	if m.cfg.interAppEnabled() {
		if tok := interAppTokenFromRequest(r.Header.Get); tok != "" {
			if uid, err := m.cfg.verifyInterAppToken(tok); err == nil {
				return decisionInterApp, uid
			}
		}
	}

	if sessionValid && sess != nil {
		return decisionSession, ""
	}

	if m.pathIsProtected(appPath) {
		return decisionChallenge, ""
	}

	return decisionPublic, ""
}

// pathIsProtected reports whether appPath requires authentication. When
// ProtectedPaths is empty the policy is "protect everything that is not
// explicitly public" (a deny-by-default posture); when it is non-empty
// only the listed prefixes are gated.
func (m *Middleware) pathIsProtected(appPath string) bool {
	if len(m.cfg.ProtectedPaths) == 0 {
		return true
	}
	return matchesPrefix(appPath, m.cfg.ProtectedPaths)
}
