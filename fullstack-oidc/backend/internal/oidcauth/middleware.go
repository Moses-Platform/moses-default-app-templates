package oidcauth

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Auth handshake route suffixes (relative to BasePath). The middleware
// owns these and serves them itself.
const (
	routeLogin       = "/auth/login"        // start interactive login
	routeCallback    = "/auth/callback"     // OIDC redirect target
	routeLogout      = "/auth/logout"       // clear session + RP logout
	routeSilentCheck = "/auth/silent-check" // prompt=none probe target
)

// Identity is the authenticated principal exposed to handler code via
// the request context. It is populated from a validated session cookie,
// OR from the trusted X-Moses-* headers on a pod-to-pod call.
type Identity struct {
	// Authenticated is true when the request carries either a valid
	// session or trusted pod-to-pod headers.
	Authenticated bool

	// Source is "session", "moses-headers", or "interapp".
	Source string

	Subject  string
	Email    string
	Name     string
	Username string

	// Roles is the resource_access.<client>.roles snapshot from the
	// validated token (empty for the header-trust path — pod-to-pod
	// callers are authorized by the platform, not by app roles).
	Roles []string
}

// HasRole reports whether the identity carries `role`.
func (id Identity) HasRole(role string) bool {
	for _, r := range id.Roles {
		if r == role {
			return true
		}
	}
	return false
}

type ctxKey string

const identityCtxKey ctxKey = "oidcauth.identity"

// IdentityFrom extracts the authenticated Identity from a request
// context. The zero Identity (Authenticated=false) is returned when the
// middleware did not run or the route was public.
func IdentityFrom(ctx context.Context) Identity {
	if id, ok := ctx.Value(identityCtxKey).(Identity); ok {
		return id
	}
	return Identity{}
}

// ContextWithIdentity returns a copy of ctx carrying id. The middleware
// uses this internally; it is also exported so handler tests can build a
// request as if the middleware had already authenticated the caller —
// without standing up a live OIDC provider.
func ContextWithIdentity(ctx context.Context, id Identity) context.Context {
	return withIdentity(ctx, id)
}

// Middleware is the OIDC relying-party HTTP middleware. Build it with
// New, then wrap your mux with Middleware.Handler. The auth handshake
// routes (/auth/login, /auth/callback, /auth/logout, /auth/silent-check)
// are served by the middleware itself.
type Middleware struct {
	cfg      Config
	provider *provider
}

// New builds a Middleware from a Config. When cfg.Enabled() is false the
// middleware runs in pass-through mode: public routes and the
// header-trust path still work, but browser requests are NOT
// redirected (the app is reachable un-authenticated). main() should log
// cfg.Validate() so a misconfigured deploy is visible.
func New(cfg Config) *Middleware {
	return &Middleware{
		cfg:      cfg,
		provider: newProvider(cfg),
	}
}

// Config exposes the resolved configuration (read-only use by handlers,
// e.g. to learn the app's own client id for role checks).
func (m *Middleware) Config() Config { return m.cfg }

// Handler wraps next with OIDC enforcement. The handshake routes are
// intercepted; every other request is classified and either served,
// header-trust-populated, session-populated, or redirected to login.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appPath := trimBasePath(r.URL.Path, m.cfg.BasePath)

		// Handshake routes are served by the middleware itself.
		switch {
		case appPath == routeLogin:
			m.handleLogin(w, r)
			return
		case appPath == routeCallback:
			m.handleCallback(w, r)
			return
		case appPath == routeLogout:
			m.handleLogout(w, r)
			return
		case appPath == routeSilentCheck:
			m.handleSilentCheck(w, r)
			return
		}

		// Pass-through mode: OIDC not configured. Still honour the
		// header-trust path — but only when it is marker-gated and the
		// marker matches (CHAT-t5d1u.28.21 S3). Everything else is
		// served un-gated.
		if !m.cfg.Enabled() {
			id := Identity{}
			if m.isHeaderTrusted(r) {
				id = identityFromHeaders(r)
			}
			next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), id)))
			return
		}

		sess, sessErr := readSessionCookie(r, m.cfg)
		sessionValid := sessErr == nil && sess.Valid(time.Now())

		dec, interAppUserID := m.classify(r, sess, sessionValid)
		switch dec {
		case decisionPublic:
			next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), Identity{})))

		case decisionHeaderTrust:
			next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), identityFromHeaders(r))))

		case decisionInterApp:
			next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), identityFromInterApp(interAppUserID))))

		case decisionSession:
			// The session may outlive its ROLES snapshot (Moses app
			// tokens live ~120s; the session lives hours). Re-mint a
			// stale snapshot via the refresh-token grant before
			// serving, so authorization decisions never run on roles
			// older than ~the token TTL.
			if sess.rolesStale(time.Now()) {
				renewed, ok := m.refreshSessionRoles(w, r, sess)
				if !ok {
					m.challenge(w, r)
					return
				}
				sess = renewed
			}
			next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), identityFromSession(sess))))

		case decisionChallenge:
			m.challenge(w, r)
		}
	})
}

// refreshSessionRoles renews a session whose roles snapshot has gone
// stale by running the refresh-token grant and re-validating the new ID
// token. On success the renewed session is written back as a cookie and
// returned. On any failure the session cookie is cleared (false) — the
// caller challenges, and the SPA falls back to its silent-SSO probe.
//
// NOTE: this runs per-request with no single-flight. That is safe with
// Keycloak's default refresh-token settings (rotation OFF — concurrent
// grants against the same token all succeed). If an operator enables
// "Revoke Refresh Token" (rotation + reuse detection), concurrent
// requests crossing the stale boundary can invalidate each other and
// drop the session (recoverable via silent SSO, but visible). Keep
// rotation off for Moses app clients, or add single-flight here first.
func (m *Middleware) refreshSessionRoles(w http.ResponseWriter, r *http.Request, sess *Session) (*Session, bool) {
	if sess.RefreshToken == "" {
		clearSessionCookie(w, m.cfg)
		return nil, false
	}
	if err := m.provider.discover(r.Context()); err != nil {
		log.Printf("oidcauth: roles refresh skipped, discovery failed: %v", err)
		clearSessionCookie(w, m.cfg)
		return nil, false
	}
	tok, err := m.provider.refreshGrant(r.Context(), sess.RefreshToken)
	if err != nil {
		// Normal end-of-session (Keycloak SSO idled out / revoked) —
		// not an error condition worth more than a log line.
		log.Printf("oidcauth: refresh grant failed, ending session: %v", err)
		clearSessionCookie(w, m.cfg)
		return nil, false
	}
	// No expectedNonce: nonce binds the interactive authorization
	// request, and ID tokens minted by the refresh grant SHOULD NOT
	// carry one (OIDC Core 12.2) — requiring it would end every
	// session at the first roles refresh.
	claims, err := m.provider.verifyIDToken(r.Context(), tok.IDToken, "")
	if err != nil {
		log.Printf("oidcauth: refreshed id_token invalid, ending session: %v", err)
		clearSessionCookie(w, m.cfg)
		return nil, false
	}
	// Keycloak may rotate the refresh token; fall back to the old one
	// when the response omits it (rotation disabled — the KC default).
	nextRT := tok.RefreshToken
	if nextRT == "" {
		nextRT = sess.RefreshToken
	}
	renewed := sessionFromClaims(claims, m.cfg, tok.ExpiresIn, nextRT)
	if err := setSessionCookie(w, m.cfg, renewed); err != nil {
		log.Printf("oidcauth: renewed session cookie write failed: %v", err)
		clearSessionCookie(w, m.cfg)
		return nil, false
	}
	return renewed, true
}

// withIdentity stores an Identity in the request context.
func withIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityCtxKey, id)
}

// identityFromHeaders builds an Identity from the trusted X-Moses-*
// headers (pod-to-pod path). Roles come from X-Moses-Roles — agent role
// grants made in the Moses shield modal (Agents toggle) — and are
// trustworthy here because this function is only reached after the
// constant-time gateway-secret match in isHeaderTrusted.
func identityFromHeaders(r *http.Request) Identity {
	return Identity{
		Authenticated: true,
		Source:        "moses-headers",
		Subject:       strings.TrimSpace(r.Header.Get("X-Moses-User-ID")),
		Roles:         parseRolesHeader(r.Header.Get(HeaderMosesRoles)),
	}
}

// parseRolesHeader splits a comma-separated role list, trimming whitespace
// and dropping empties. Returns nil for a blank or all-empty header.
func parseRolesHeader(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// identityFromInterApp builds an Identity from a verified inter-app token
// (P2c). The Subject is the trusted moses_user_id the sibling asserted.
//
// ROLES ARE INTENTIONALLY EMPTY (nil) AND MUST STAY THAT WAY. An
// inter-app token carries NO authorization — the receiving app authorizes
// this user against its OWN ACLs. We never trust a sibling app (or an
// agent) to assert roles, even if a forward-compatible token were to
// carry a `roles` claim: verifyInterAppToken does not read one, and this
// constructor sets Roles to nil unconditionally. This is the core
// privilege-escalation guard for the shared-secret trust model.
func identityFromInterApp(mosesUserID string) Identity {
	return Identity{
		Authenticated: true,
		Source:        "interapp",
		Subject:       mosesUserID,
		Roles:         nil, // never carry roles across the inter-app hop
	}
}

// MintInterAppToken mints a short-lived inter-app trust token for a call
// to targetAppSlug on behalf of actingUserMosesID (the caller's OWN
// authenticated Identity.Subject — never a caller-supplied arbitrary id).
// It is a thin wrapper over Config.MintInterAppToken so handler code can
// mint from the *Middleware it already holds. Returns an error when the
// inter-app path is not configured.
func (m *Middleware) MintInterAppToken(targetAppSlug, actingUserMosesID string) (string, error) {
	return m.cfg.MintInterAppToken(targetAppSlug, actingUserMosesID)
}

// identityFromSession builds an Identity from a validated session.
func identityFromSession(s *Session) Identity {
	return Identity{
		Authenticated: true,
		Source:        "session",
		Subject:       s.Subject,
		Email:         s.Email,
		Name:          s.Name,
		Username:      s.Username,
		Roles:         s.Roles,
	}
}

// challenge responds to an unauthenticated request for a protected path.
// XHR/fetch requests get a 401 JSON body (so the SPA can react); a
// top-level browser navigation gets a 302 to the interactive login.
func (m *Middleware) challenge(w http.ResponseWriter, r *http.Request) {
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("WWW-Authenticate", `Bearer realm="moses-oidc"`)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthenticated","login":"` + m.route(routeLogin) + `"}`))
		return
	}
	// Top-level navigation -> interactive login, returning here after.
	http.Redirect(w, r, m.route(routeLogin)+"?return_to="+url.QueryEscape(r.URL.RequestURI()),
		http.StatusFound)
}

// handleLogin starts an interactive authorization-code + PKCE flow.
func (m *Middleware) handleLogin(w http.ResponseWriter, r *http.Request) {
	prompt := "" // interactive: a login page is acceptable
	m.startAuth(w, r, prompt, r.URL.Query().Get("return_to"))
}

// handleSilentCheck starts a SILENT (prompt=none) authorization-code
// flow. If the user already has a Moses SSO session Keycloak returns a
// code with no visible login page; otherwise it returns
// `error=login_required`, which the callback turns into a benign
// "not logged in" signal instead of an error page.
func (m *Middleware) handleSilentCheck(w http.ResponseWriter, r *http.Request) {
	m.startAuth(w, r, "none", r.URL.Query().Get("return_to"))
}

// isLocalRedirect reports whether returnTo is a safe same-origin relative
// target: a path beginning with a single "/" and carrying no scheme or
// authority. It rejects "", absolute URLs ("https://evil.com"), and the
// protocol-relative forms ("//evil.com", "/\evil.com") that browsers
// normalise to a foreign origin — all open-redirect vectors. The value
// is validated here once before being stored in the handshake cookie, so
// every later redirect that reuses hs.ReturnTo (callback success, silent
// failure) inherits the guarantee.
func isLocalRedirect(returnTo string) bool {
	if returnTo == "" || returnTo[0] != '/' {
		return false
	}
	// "//host" and the backslash variant "/\host" (which browsers treat
	// as "//host") both escape the current origin.
	if len(returnTo) > 1 && (returnTo[1] == '/' || returnTo[1] == '\\') {
		return false
	}
	// Defence in depth: anything that parses with a scheme or host is
	// not a bare local path.
	if u, err := url.Parse(returnTo); err != nil || u.Scheme != "" || u.Host != "" {
		return false
	}
	return true
}

// startAuth is the shared authorization-request kickoff for both the
// interactive and silent flows.
func (m *Middleware) startAuth(w http.ResponseWriter, r *http.Request, prompt, returnTo string) {
	if err := m.provider.discover(r.Context()); err != nil {
		log.Printf("oidcauth: discovery failed: %v", err)
		http.Error(w, "oidc provider unavailable", http.StatusBadGateway)
		return
	}

	state := randomURLToken(24)
	nonce := randomURLToken(24)
	verifier := newCodeVerifier()
	challenge := codeChallengeS256(verifier)

	if !isLocalRedirect(returnTo) {
		// Only same-origin relative return targets are honoured —
		// an absolute or protocol-relative URL here would be an
		// open-redirect vector.
		returnTo = m.cfg.BasePath + "/"
	}

	hs := &handshakeState{
		State:        state,
		CodeVerifier: verifier,
		ReturnTo:     returnTo,
		Prompt:       prompt,
		Nonce:        nonce,
	}
	if err := setStateCookie(w, m.cfg, hs); err != nil {
		http.Error(w, "oidc state error", http.StatusInternalServerError)
		return
	}

	redirectURI := m.absoluteURL(r, routeCallback)
	authURL := m.provider.authorizeRedirectURL(redirectURI, state, challenge, nonce, prompt)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleCallback completes the authorization-code exchange.
func (m *Middleware) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	hs, err := readStateCookie(r, m.cfg)
	if err != nil {
		http.Error(w, "missing or invalid auth state", http.StatusBadRequest)
		return
	}
	clearStateCookie(w, m.cfg)

	// CSRF: the `state` query param MUST equal the one we stored.
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(hs.State)) != 1 {
		http.Error(w, "auth state mismatch", http.StatusBadRequest)
		return
	}

	// Keycloak signals "no SSO session" for a prompt=none attempt via
	// an OAuth error. Treat it as a benign "not logged in" outcome.
	if oerr := q.Get("error"); oerr != "" {
		if hs.Prompt == "none" {
			// Silent attempt failed gracefully: bounce back to the
			// return target un-authenticated. The SPA helper reads
			// the ?silent_sso=failed marker.
			http.Redirect(w, r, appendQuery(hs.ReturnTo, "silent_sso", "failed"), http.StatusFound)
			return
		}
		http.Error(w, "login failed: "+oerr, http.StatusUnauthorized)
		return
	}

	code := q.Get("code")
	if code == "" {
		http.Error(w, "no authorization code", http.StatusBadRequest)
		return
	}

	if err := m.provider.discover(r.Context()); err != nil {
		http.Error(w, "oidc provider unavailable", http.StatusBadGateway)
		return
	}

	redirectURI := m.absoluteURL(r, routeCallback)
	tok, err := m.provider.exchangeCode(r.Context(), code, redirectURI, hs.CodeVerifier)
	if err != nil {
		log.Printf("oidcauth: code exchange failed: %v", err)
		http.Error(w, "token exchange failed", http.StatusBadGateway)
		return
	}

	// Verify signature + iss/aud/exp AND the OIDC nonce: the token's
	// `nonce` claim must echo the value startAuth stored in the HMAC
	// state cookie, binding this token to this handshake.
	claims, err := m.provider.verifyIDToken(r.Context(), tok.IDToken, hs.Nonce)
	if err != nil {
		log.Printf("oidcauth: id_token verification failed: %v", err)
		http.Error(w, "id token invalid", http.StatusUnauthorized)
		return
	}

	sess := sessionFromClaims(claims, m.cfg, tok.ExpiresIn, tok.RefreshToken)
	if err := setSessionCookie(w, m.cfg, sess); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	dest := hs.ReturnTo
	if hs.Prompt == "none" {
		dest = appendQuery(dest, "silent_sso", "ok")
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

// handleLogout clears the session cookie and performs a Keycloak
// RP-initiated logout, sending the user back to the app root.
//
// This is deliberately an UN-GATED GET (industry-common for RP-initiated
// logout links). The worst a forced/CSRF'd navigation here achieves is
// logging the victim OUT — an annoyance, not a privilege gain — and the
// Keycloak end-session endpoint independently prompts/validates via
// client_id + registered post_logout_redirect_uri.
func (m *Middleware) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w, m.cfg)

	postLogout := m.absoluteURL(r, "/")
	if err := m.provider.discover(r.Context()); err != nil {
		// Provider unreachable — the local cookie is already gone,
		// which is the security-critical half. Just go home.
		http.Redirect(w, r, postLogout, http.StatusFound)
		return
	}
	// We do not retain the id_token in the BFF session (the browser
	// never holds it), so id_token_hint is omitted; Keycloak still
	// honours post_logout_redirect_uri + client_id.
	http.Redirect(w, r, m.provider.endSessionRedirectURL(postLogout, ""), http.StatusFound)
}

// sessionFromClaims projects a validated ID token into a Session.
//
// The SESSION lifetime (Expiry) is cfg.sessionMaxAge() — deliberately
// DECOUPLED from the token lifetime. Moses provisions app clients with
// ~120s tokens (role-revocation latency, CHAT-t5d1u.28.6); binding the
// session to that made every page refresh a full re-login. The token
// lifetime instead bounds only the ROLES snapshot (RolesFreshUntil):
// once it lapses, the middleware re-mints the snapshot via the
// refresh-token grant, so role revocation still lands within ~the token
// TTL while the login itself survives. A failed refresh (Keycloak
// session idled out, revoked, or unreachable) ends the session. When
// the IdP issues no refresh token the snapshot cannot be renewed, so
// the session falls back to the pre-fix behaviour (dies with the token).
func sessionFromClaims(c *Claims, cfg Config, expiresIn int, refreshToken string) *Session {
	now := time.Now()
	exp := now.Add(cfg.sessionMaxAge())

	rolesFresh := exp
	if c.Expiry != 0 {
		tokExp := time.Unix(c.Expiry, 0)
		if tokExp.Before(rolesFresh) {
			rolesFresh = tokExp
		}
	}
	if expiresIn > 0 {
		tokExp := now.Add(time.Duration(expiresIn) * time.Second)
		if tokExp.Before(rolesFresh) {
			rolesFresh = tokExp
		}
	}
	if refreshToken == "" && rolesFresh.Before(exp) {
		exp = rolesFresh
	}
	return &Session{
		// Prefer the STABLE Moses platform user UUID (moses_user_id) so the
		// session subject matches the header-trust path's X-Moses-User-ID for
		// the same human. Fall back to the realm-local `sub` for tokens minted
		// before the platform deployed the moses_user_id mapper (P3a).
		Subject:         firstNonEmpty(c.MosesUserID, c.Subject),
		Email:           c.Email,
		Name:            c.Name,
		Username:        c.Username,
		Roles:           c.RolesForClient(cfg.ClientID),
		Expiry:          exp.Unix(),
		IssuedAt:        now.Unix(),
		RolesFreshUntil: rolesFresh.Unix(),
		RefreshToken:    refreshToken,
	}
}

// firstNonEmpty returns the first argument whose trimmed value is non-empty,
// or "" when all are empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// --- URL helpers -----------------------------------------------------

// route returns a handshake route as an app-absolute path (BasePath +
// suffix).
func (m *Middleware) route(suffix string) string {
	return strings.TrimRight(m.cfg.BasePath, "/") + suffix
}

// absoluteURL builds a fully-qualified URL for an app route, used for
// OIDC redirect_uri / post_logout_redirect_uri (those MUST be absolute).
// Scheme + host are derived from forwarded headers when present (the app
// sits behind the platform ingress / nginx).
func (m *Middleware) absoluteURL(r *http.Request, suffix string) string {
	// Multi-home: prefer the configured origin whose hostname matches the
	// host the browser actually used, so login completes on that same host
	// (the matched origin also supplies the correct scheme+port even when the
	// gateway strips :port from the Host header).
	if origin := m.matchPublicOrigin(r); origin != "" {
		return strings.TrimRight(origin, "/") + m.route(suffix)
	}
	if m.cfg.PublicURL != "" {
		return strings.TrimRight(m.cfg.PublicURL, "/") + m.route(suffix)
	}
	// Legacy fallback: derive from request (non-platform deployments).
	scheme := "https"
	if xf := r.Header.Get("X-Forwarded-Proto"); xf != "" {
		scheme = xf
	} else if r.TLS == nil && !m.cfg.SecureCookie {
		scheme = "http"
	}
	host := r.Host
	if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" {
		host = xfh
	}
	return scheme + "://" + host + m.route(suffix)
}

// matchPublicOrigin returns the configured public origin whose hostname equals
// the request host (X-Forwarded-Host, else r.Host), compared on hostname only
// (port-insensitive). "" when there is no allowlist or no match.
func (m *Middleware) matchPublicOrigin(r *http.Request) string {
	if len(m.cfg.PublicURLs) == 0 {
		return ""
	}
	reqHost := requestHostname(r)
	if reqHost == "" {
		return ""
	}
	for _, o := range m.cfg.PublicURLs {
		if originHostname(o) == reqHost {
			return o
		}
	}
	return ""
}

// requestHostname returns the lower-cased, port-stripped hostname the browser
// used: X-Forwarded-Host (gateway-injected) wins over r.Host.
func requestHostname(r *http.Request) string {
	h := r.Host
	if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" {
		h = xfh
	}
	return hostnameOnly(h)
}

// originHostname returns the lower-cased, port-stripped hostname of a
// scheme://host[:port] origin.
func originHostname(origin string) string {
	if i := strings.Index(origin, "://"); i >= 0 {
		origin = origin[i+3:]
	}
	if j := strings.IndexAny(origin, "/?#"); j >= 0 {
		origin = origin[:j]
	}
	return hostnameOnly(origin)
}

// hostnameOnly lower-cases and strips a trailing :port (and any leading
// comma-list artifact from a chained X-Forwarded-Host).
func hostnameOnly(h string) string {
	h = strings.TrimSpace(strings.ToLower(h))
	if i := strings.IndexByte(h, ','); i >= 0 { // first hop of a chained XFH
		h = strings.TrimSpace(h[:i])
	}
	if i := strings.LastIndexByte(h, ':'); i >= 0 && !strings.Contains(h[i:], "]") {
		h = h[:i]
	}
	return h
}

// wantsJSON reports whether the request looks like an XHR/fetch call
// (so the challenge returns 401 JSON rather than a 302 redirect).
func wantsJSON(r *http.Request) bool {
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		return true
	}
	if r.Header.Get("X-Requested-With") != "" {
		return true
	}
	// fetch() default Sec-Fetch-Mode is "cors"/"same-origin"; a
	// top-level navigation is "navigate".
	if m := r.Header.Get("Sec-Fetch-Mode"); m != "" && m != "navigate" {
		return true
	}
	return false
}

// appendQuery adds a single query parameter to a URL or path.
func appendQuery(u, key, val string) string {
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + key + "=" + url.QueryEscape(val)
}
