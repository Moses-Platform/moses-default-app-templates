package oidcauth

import (
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

// isHeaderTrusted reports whether the request carries the platform's
// trusted pod-to-pod header marker. The platform's authenticated proxy
// sets X-Moses-User-ID on the in-cluster hop; a browser request through
// the user-facing ingress does NOT carry it. Preserving this path keeps
// the OpenAPI workspace-tool surface working without an OIDC session.
//
// NOTE on trust: this header is only trustworthy because the app pod is
// not directly internet-reachable — all browser traffic arrives via the
// platform ingress, which strips client-supplied X-Moses-* headers. The
// same assumption underpins fullstack-showcase's existing
// MosesHeaders middleware. If an app is exposed on a raw custom
// hostname with no platform ingress in front, set protectedPaths to
// cover the API and rely on the session path instead.
func isHeaderTrusted(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("X-Moses-User-ID")) != ""
}

// classify decides how to handle a request. It is pure given the
// request, the config, and the (already-verified) session — no I/O — so
// it is exhaustively unit-testable.
//
// Order of precedence:
//  1. Public paths (incl. /health + spec + handshake routes) -> public.
//  2. Trusted X-Moses-* pod-to-pod header -> header-trust (dual mode).
//  3. Valid session cookie -> session.
//  4. Path is protected (or ProtectedPaths empty == protect-all) and
//     this is a browser request -> challenge.
//  5. Otherwise -> public (unprotected path, no creds needed).
func (m *Middleware) classify(r *http.Request, sess *Session, sessionValid bool) decision {
	appPath := trimBasePath(r.URL.Path, m.cfg.BasePath)

	if matchesPrefix(appPath, m.alwaysPublic()) || matchesPrefix(appPath, m.cfg.PublicPaths) {
		return decisionPublic
	}

	// Dual mode: trusted pod-to-pod header bypasses OIDC entirely so
	// MCP / workspace-tool calls keep working.
	if isHeaderTrusted(r) {
		return decisionHeaderTrust
	}

	if sessionValid && sess != nil {
		return decisionSession
	}

	if m.pathIsProtected(appPath) {
		return decisionChallenge
	}

	return decisionPublic
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
