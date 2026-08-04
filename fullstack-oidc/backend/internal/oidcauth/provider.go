package oidcauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// provider holds the resolved OIDC endpoints and the JWKS key set. The
// EXTERNAL issuer drives browser-facing endpoints (authorize, end
// session); the INTERNAL issuer drives server-to-server endpoints (JWKS,
// token). Keycloak returns absolute URLs in discovery that embed the
// issuer host, so we discover against each and use the right one per
// hop. When external discovery is unreachable from inside the pod (Bug A,
// CHAT-t5d1u.28.23) the browser-facing URLs are synthesised from the
// external Issuer + Keycloak's well-known paths.
type provider struct {
	cfg Config

	// external endpoints — browser sees these.
	authorizeURL  string
	endSessionURL string

	// internal endpoints — server-to-server only.
	tokenURL string
	keys     *keySet

	httpClient *http.Client

	mu    sync.Mutex
	ready bool
}

// newProvider builds an un-discovered provider. Call discover() before
// use; the middleware does this lazily on first request.
func newProvider(cfg Config) *provider {
	return &provider{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// wellKnown is the standard OIDC discovery path suffix.
const wellKnown = "/.well-known/openid-configuration"

// Keycloak's standard relative endpoint paths under an issuer realm URL.
// Synthesised when external discovery is unreachable (e.g. an in-cluster
// pod cannot dial the external ingress host); Keycloak ships these paths
// at stable, well-known locations.
const (
	keycloakAuthorizePath  = "/protocol/openid-connect/auth"
	keycloakEndSessionPath = "/protocol/openid-connect/logout"
)

// discover resolves both the external and internal endpoint sets. It is
// idempotent and safe to call repeatedly — only the first successful
// call does work.
//
// CHAT-t5d1u.28.23 (Bug A): internal discovery is REQUIRED (server-side
// JWKS + token exchange cannot work without it). External discovery is
// BEST-EFFORT — when the pod cannot reach the external issuer host (a
// common case behind in-cluster ingress) the browser-facing
// authorize/end-session URLs are synthesised from p.cfg.Issuer plus
// Keycloak's standard relative paths. The external Issuer remains the
// source of truth for browser URLs (the browser cannot resolve in-cluster
// DNS) and for token `iss` validation (Keycloak self-declares the
// external issuer in tokens it mints for the browser).
func (p *provider) discover(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		return nil
	}

	// Internal discovery is required: JWKS + token endpoints MUST be
	// reachable for the BFF to function. This is the call that runs from
	// inside the pod, so it MUST target InternalIssuer (Bug A fix).
	intDoc, err := p.fetchDiscovery(ctx, p.cfg.InternalIssuer)
	if err != nil {
		return fmt.Errorf("oidcauth: internal discovery (%s): %w", p.cfg.InternalIssuer, err)
	}
	// Keycloak fills token_endpoint/jwks_uri in the discovery doc with its
	// FRONTEND host (from KC_HOSTNAME_URL) even when the doc is fetched from
	// the internal service URL. Rewrite both onto the in-cluster issuer's
	// scheme+host so the server-to-server token exchange + JWKS fetch are
	// reachable from inside the pod (Bug B, CHAT-t5d1u.28.x).
	p.tokenURL = rewriteToInternalHost(intDoc.TokenEndpoint, p.cfg.InternalIssuer)
	p.keys = newKeySet(rewriteToInternalHost(intDoc.JWKSURI, p.cfg.InternalIssuer), p.httpClient)

	// External discovery is best-effort: when it succeeds it gives us
	// the canonical browser-facing endpoints; when it fails (in-cluster
	// pod cannot reach the external ingress) we synthesise them from
	// p.cfg.Issuer + Keycloak's standard paths. Either way the browser
	// sees an EXTERNAL host — never the in-cluster one.
	if extDoc, err := p.fetchDiscovery(ctx, p.cfg.Issuer); err == nil {
		p.authorizeURL = extDoc.AuthorizationEndpoint
		p.endSessionURL = extDoc.EndSessionEndpoint
	} else {
		issuer := strings.TrimRight(p.cfg.Issuer, "/")
		log.Printf("oidcauth: external discovery unreachable (%v), synthesising browser URLs from Issuer=%s via standard Keycloak paths", err, issuer)
		p.authorizeURL = issuer + keycloakAuthorizePath
		p.endSessionURL = issuer + keycloakEndSessionPath
	}

	p.ready = true
	return nil
}

// fetchDiscovery GETs and parses an OIDC discovery document.
func (p *provider) fetchDiscovery(ctx context.Context, issuer string) (*discoveryDoc, error) {
	u := strings.TrimRight(issuer, "/") + wellKnown
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var doc discoveryDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse discovery: %w", err)
	}
	if doc.AuthorizationEndpoint == "" || doc.TokenEndpoint == "" || doc.JWKSURI == "" {
		return nil, fmt.Errorf("discovery missing required endpoints")
	}
	return &doc, nil
}

// rewriteToInternalHost forces a Keycloak-advertised endpoint (which carries
// the BROWSER/frontend host from KC_HOSTNAME_URL) onto the in-cluster issuer's
// scheme+host so server-to-server calls (token exchange, JWKS) are reachable
// from inside the pod. Paths (incl. custom realm paths) are preserved.
func rewriteToInternalHost(discovered, internalIssuer string) string {
	d, err := url.Parse(discovered)
	if err != nil || d.Host == "" {
		return discovered
	}
	i, err := url.Parse(internalIssuer)
	if err != nil || i.Host == "" {
		return discovered
	}
	d.Scheme = i.Scheme
	d.Host = i.Host
	return d.String()
}

// authorizeRedirectURL builds the browser redirect to the OIDC
// authorization endpoint for an authorization-code + PKCE flow.
//
// The `nonce` is echoed by the IdP into the ID token's `nonce` claim,
// where the callback verifies it against the value stored in the HMAC
// state cookie — binding the issued token to this handshake.
//
// When prompt == "none" the request is a SILENT SSO attempt: Keycloak
// returns immediately with a code if the user already has an SSO
// session, or with `error=login_required` if not — no login page is
// shown. The middleware uses that for the in-iframe case.
func (p *provider) authorizeRedirectURL(redirectURI, state, codeChallenge, nonce, prompt string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", p.cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", "openid profile email")
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	if nonce != "" {
		q.Set("nonce", nonce)
	}
	if prompt != "" {
		q.Set("prompt", prompt)
	}
	sep := "?"
	if strings.Contains(p.authorizeURL, "?") {
		sep = "&"
	}
	return p.authorizeURL + sep + q.Encode()
}

// endSessionRedirectURL builds the Keycloak RP-initiated logout URL. The
// post-logout redirect sends the user back to the app root.
func (p *provider) endSessionRedirectURL(postLogoutRedirect, idTokenHint string) string {
	if p.endSessionURL == "" {
		// Provider has no end-session endpoint — fall back to just
		// the post-logout target (cookie was already cleared locally).
		return postLogoutRedirect
	}
	q := url.Values{}
	if postLogoutRedirect != "" {
		q.Set("post_logout_redirect_uri", postLogoutRedirect)
	}
	q.Set("client_id", p.cfg.ClientID)
	if idTokenHint != "" {
		q.Set("id_token_hint", idTokenHint)
	}
	sep := "?"
	if strings.Contains(p.endSessionURL, "?") {
		sep = "&"
	}
	return p.endSessionURL + sep + q.Encode()
}

// tokenResponse is the subset of the token-endpoint JSON we consume.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// exchangeCode runs the authorization-code grant against the INTERNAL
// token endpoint. It is a confidential-client exchange: the client
// secret is sent (HTTP Basic) alongside the PKCE verifier.
func (p *provider) exchangeCode(ctx context.Context, code, redirectURI, codeVerifier string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", codeVerifier)
	form.Set("client_id", p.cfg.ClientID)
	return p.postTokenForm(ctx, form)
}

// refreshGrant runs the refresh-token grant against the INTERNAL token
// endpoint. The middleware uses it to renew a session's roles snapshot
// once the short-lived token bound (Session.RolesFreshUntil) lapses —
// a failed grant (Keycloak SSO session idled out or revoked) ends the
// BFF session.
func (p *provider) refreshGrant(ctx context.Context, refreshToken string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", p.cfg.ClientID)
	return p.postTokenForm(ctx, form)
}

// postTokenForm POSTs a grant to the token endpoint with confidential-
// client authentication and decodes the response.
func (p *provider) postTokenForm(ctx context.Context, form url.Values) (*tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	// Confidential client: HTTP Basic with client_id:client_secret.
	req.SetBasicAuth(p.cfg.ClientID, p.cfg.ClientSecret)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidcauth: token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("oidcauth: parse token response (HTTP %d): %w", resp.StatusCode, err)
	}
	if tr.Error != "" {
		return nil, fmt.Errorf("oidcauth: token endpoint error %q: %s", tr.Error, tr.ErrorDesc)
	}
	if resp.StatusCode != http.StatusOK || tr.IDToken == "" {
		return nil, fmt.Errorf("oidcauth: token exchange failed (HTTP %d, no id_token)", resp.StatusCode)
	}
	return &tr, nil
}

// verifyIDToken validates an ID token's signature + claims against the
// internally-fetched JWKS, using the EXTERNAL issuer for the `iss`
// check (the token is minted by the external issuer). expectedNonce is
// the nonce sent on the authorization request (from the HMAC state
// cookie); when non-empty the token's `nonce` claim MUST be present and
// equal it.
func (p *provider) verifyIDToken(ctx context.Context, idToken, expectedNonce string) (*Claims, error) {
	if p.keys == nil {
		return nil, fmt.Errorf("oidcauth: provider not discovered")
	}
	return verifyToken(ctx, idToken, p.keys, verifyOptions{
		expectedIssuer:   p.cfg.Issuer,
		expectedAudience: p.cfg.expectedAudience(),
		expectedNonce:    expectedNonce,
	})
}
