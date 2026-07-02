package oidcauth

import (
	"context"
	"crypto"
	"crypto/rsa"
	_ "crypto/sha256" // register SHA-256 for crypto.Hash.New
	_ "crypto/sha512" // register SHA-384/512 for crypto.Hash.New
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// jwksDoc is the subset of an OIDC discovery JWKS document we parse.
type jwksDoc struct {
	Keys []jwkRSA `json:"keys"`
}

// jwkRSA is one RSA public key in JWKS form (RFC 7517).
type jwkRSA struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"` // modulus, base64url
	E   string `json:"e"` // exponent, base64url
}

// discoveryDoc is the subset of the OIDC `.well-known` document used to
// locate the JWKS, authorization, token, and end-session endpoints.
type discoveryDoc struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// jwkKey is a parsed JWKS entry: the RSA public key plus the JWK's own
// declared `alg` (empty when the JWK omits it). The alg is retained so
// verifyToken can cross-check it against the JWT header `alg`.
type jwkKey struct {
	pub *rsa.PublicKey
	alg string // JWK "alg" header, "" when absent
}

// keySet caches parsed RSA public keys keyed by `kid`, fetched from the
// INTERNAL issuer's JWKS endpoint. It refreshes on an unknown kid (key
// rotation) but no more often than minRefreshInterval.
type keySet struct {
	jwksURI    string
	httpClient *http.Client

	mu          sync.RWMutex
	keys        map[string]jwkKey
	lastRefresh time.Time
}

const minRefreshInterval = 30 * time.Second

func newKeySet(jwksURI string, hc *http.Client) *keySet {
	return &keySet{
		jwksURI:    jwksURI,
		httpClient: hc,
		keys:       map[string]jwkKey{},
	}
}

// keyForKID returns the cached key entry for kid, refreshing once from
// JWKS if the kid is unknown and the cache is older than the throttle.
func (ks *keySet) keyForKID(ctx context.Context, kid string) (jwkKey, error) {
	ks.mu.RLock()
	k, ok := ks.keys[kid]
	stale := time.Since(ks.lastRefresh) > minRefreshInterval
	ks.mu.RUnlock()
	if ok {
		return k, nil
	}
	if !stale && len(ks.keys) > 0 {
		return jwkKey{}, fmt.Errorf("oidcauth: no JWKS key for kid %q (refresh throttled)", kid)
	}
	if err := ks.refresh(ctx); err != nil {
		return jwkKey{}, err
	}
	ks.mu.RLock()
	k, ok = ks.keys[kid]
	ks.mu.RUnlock()
	if !ok {
		return jwkKey{}, fmt.Errorf("oidcauth: no JWKS key for kid %q after refresh", kid)
	}
	return k, nil
}

// refresh fetches and parses the JWKS document.
func (ks *keySet) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ks.jwksURI, nil)
	if err != nil {
		return fmt.Errorf("oidcauth: build JWKS request: %w", err)
	}
	resp, err := ks.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("oidcauth: fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("oidcauth: JWKS endpoint returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("oidcauth: read JWKS body: %w", err)
	}
	parsed, err := parseJWKS(body)
	if err != nil {
		return err
	}
	ks.mu.Lock()
	ks.keys = parsed
	ks.lastRefresh = time.Now()
	ks.mu.Unlock()
	return nil
}

// parseJWKS decodes a JWKS document into RSA public keys keyed by `kid`.
// Split out so it is unit-testable without a live endpoint.
//
// Empty-kid handling: a JWKS may legally contain a key with no `kid`,
// but two or more empty-kid keys would silently collapse onto the same
// "" map slot — only the last would survive. To avoid a silent,
// hard-to-diagnose key loss, the FIRST empty-kid key is kept and any
// further empty-kid keys are skipped with a warning to stderr.
func parseJWKS(body []byte) (map[string]jwkKey, error) {
	var doc jwksDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("oidcauth: parse JWKS: %w", err)
	}
	out := map[string]jwkKey{}
	seenEmptyKid := false
	for _, jwk := range doc.Keys {
		if jwk.Kty != "RSA" {
			continue
		}
		pub, err := jwk.toRSA()
		if err != nil {
			continue // skip malformed keys, keep the rest
		}
		if jwk.Kid == "" {
			if seenEmptyKid {
				fmt.Fprintln(os.Stderr, "oidcauth: warning: JWKS has multiple keys with an empty kid; keeping the first, skipping the rest")
				continue
			}
			seenEmptyKid = true
		}
		out[jwk.Kid] = jwkKey{pub: pub, alg: jwk.Alg}
	}
	if len(out) == 0 {
		return nil, errors.New("oidcauth: JWKS contained no usable RSA keys")
	}
	return out, nil
}

// toRSA converts a JWK to an *rsa.PublicKey.
func (j jwkRSA) toRSA() (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, fmt.Errorf("oidcauth: decode JWK modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, fmt.Errorf("oidcauth: decode JWK exponent: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	// Exponent is a big-endian integer, typically 3 bytes (65537).
	var eInt int
	switch len(eBytes) {
	case 0:
		return nil, errors.New("oidcauth: empty JWK exponent")
	default:
		padded := make([]byte, 8)
		copy(padded[8-len(eBytes):], eBytes)
		eInt = int(binary.BigEndian.Uint64(padded))
	}
	if eInt <= 0 {
		return nil, errors.New("oidcauth: non-positive JWK exponent")
	}
	return &rsa.PublicKey{N: n, E: eInt}, nil
}

// Claims is the validated subset of an OIDC ID token exposed to handlers.
type Claims struct {
	Subject string `json:"sub"`
	// MosesUserID is the STABLE Moses platform user UUID, emitted by the
	// tenant realm's moses_user_id protocol mapper (platform P3a). It is the
	// SAME value the header-trust path carries in X-Moses-User-ID, whereas
	// `sub` is a realm-local Keycloak UUID that differs per auth path. Prefer
	// this for durable identity/ACLs; falls back to `sub` when empty (tokens
	// minted before the mapper was deployed).
	MosesUserID string   `json:"moses_user_id"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Username    string   `json:"preferred_username"`
	Issuer      string   `json:"iss"`
	Audience    audSlice `json:"aud"`
	Expiry      int64    `json:"exp"`
	IssuedAt    int64    `json:"iat"`

	// Nonce echoes the `nonce` the RP sent on the authorization request
	// (OIDC Core 3.1.3.7 rule 11). Verified against the value stored in
	// the handshake state cookie — replay protection for the ID token.
	Nonce string `json:"nonce"`

	// ResourceAccess carries `resource_access.<client>.roles` — the
	// Keycloak client-role projection used for in-app authorization.
	ResourceAccess map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access"`

	// RealmAccess carries realm-level roles.
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`

	// Raw is the full decoded claim set for any field not modeled above.
	Raw map[string]any `json:"-"`
}

// RolesForClient returns resource_access.<client>.roles for the given
// client id (typically the app's own MOSES_OIDC_CLIENT_ID). Nil-safe.
func (c *Claims) RolesForClient(client string) []string {
	if c == nil {
		return nil
	}
	if ra, ok := c.ResourceAccess[client]; ok {
		return ra.Roles
	}
	return nil
}

// HasRole reports whether the token grants `role` for `client`.
func (c *Claims) HasRole(client, role string) bool {
	for _, r := range c.RolesForClient(client) {
		if r == role {
			return true
		}
	}
	return false
}

// audSlice tolerates `aud` being either a string or a []string (both
// are legal per RFC 7519).
type audSlice []string

func (a *audSlice) UnmarshalJSON(b []byte) error {
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		*a = []string{single}
		return nil
	}
	var multi []string
	if err := json.Unmarshal(b, &multi); err == nil {
		*a = multi
		return nil
	}
	return errors.New("oidcauth: `aud` is neither string nor []string")
}

func (a audSlice) contains(v string) bool {
	for _, s := range a {
		if s == v {
			return true
		}
	}
	return false
}

// verifyOptions controls token validation. Split out for testability.
type verifyOptions struct {
	expectedIssuer   string
	expectedAudience string
	// expectedNonce, when non-empty, requires the token's `nonce` claim
	// to be present and equal it (constant-time). Empty skips the check
	// (a flow that sent no nonce).
	expectedNonce string
	now           func() time.Time
	leeway        time.Duration
}

// verifyToken parses a compact JWS, verifies the RSA signature against
// the supplied key set, and validates the standard claims (iss, aud,
// exp). It returns the decoded Claims on success.
//
// Supported algorithms: RS256 / RS384 / RS512 (asymmetric, the only
// algs Keycloak issues by default). `alg: none` and HMAC algs are
// rejected outright — that closes the classic JWT alg-confusion hole.
func verifyToken(ctx context.Context, raw string, ks *keySet, opts verifyOptions) (*Claims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("oidcauth: token is not a compact JWS (need 3 parts)")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("oidcauth: decode JWT header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("oidcauth: parse JWT header: %w", err)
	}

	hashID, err := hashForAlg(header.Alg)
	if err != nil {
		return nil, err
	}

	key, err := ks.keyForKID(ctx, header.Kid)
	if err != nil {
		return nil, err
	}

	// CHAT-t5d1u.28.20 (N1): when the JWK declares its own `alg`, the
	// JWT header `alg` must match it. A signing key published as RS256
	// must not be used to verify a token that claims a different alg.
	// Cheap hardening — not exploitable today (RS-only key set), but it
	// closes a key-substitution avenue if a key set ever mixes algs.
	if key.alg != "" && key.alg != header.Alg {
		return nil, fmt.Errorf("oidcauth: JWT alg %q does not match JWK alg %q for kid %q", header.Alg, key.alg, header.Kid)
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("oidcauth: decode JWT signature: %w", err)
	}
	signingInput := parts[0] + "." + parts[1]
	h := hashID.New()
	h.Write([]byte(signingInput))
	digest := h.Sum(nil)
	if err := rsa.VerifyPKCS1v15(key.pub, hashID, digest, sig); err != nil {
		return nil, fmt.Errorf("oidcauth: JWT signature invalid: %w", err)
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("oidcauth: decode JWT payload: %w", err)
	}
	claims, err := parseClaims(payloadJSON)
	if err != nil {
		return nil, err
	}

	if err := validateClaims(claims, opts); err != nil {
		return nil, err
	}
	return claims, nil
}

// parseClaims decodes a JWT payload into Claims (plus the raw map).
func parseClaims(payloadJSON []byte) (*Claims, error) {
	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("oidcauth: parse JWT claims: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(payloadJSON, &raw); err != nil {
		return nil, fmt.Errorf("oidcauth: parse JWT claims (raw): %w", err)
	}
	claims.Raw = raw
	return &claims, nil
}

// validateClaims enforces iss / aud / exp. Split out for unit testing
// without any RSA signing.
func validateClaims(claims *Claims, opts verifyOptions) error {
	now := time.Now
	if opts.now != nil {
		now = opts.now
	}
	leeway := opts.leeway
	if leeway == 0 {
		leeway = 60 * time.Second
	}

	if opts.expectedIssuer != "" {
		// Constant-time compare so a mismatch does not leak length.
		want := strings.TrimRight(opts.expectedIssuer, "/")
		got := strings.TrimRight(claims.Issuer, "/")
		if subtle.ConstantTimeCompare([]byte(want), []byte(got)) != 1 {
			return fmt.Errorf("oidcauth: issuer mismatch: token %q != expected %q", claims.Issuer, opts.expectedIssuer)
		}
	}

	if opts.expectedAudience != "" && !claims.Audience.contains(opts.expectedAudience) {
		return fmt.Errorf("oidcauth: audience mismatch: token aud %v lacks %q", []string(claims.Audience), opts.expectedAudience)
	}

	// OIDC Core 3.1.3.7 rule 11: when a nonce was sent on the
	// authorization request, the ID token MUST carry it back verbatim. A
	// missing claim is as fatal as a mismatch — an IdP that drops the
	// nonce provides no replay binding.
	if opts.expectedNonce != "" {
		if claims.Nonce == "" {
			return errors.New("oidcauth: token has no `nonce` claim but one was sent on the authorization request")
		}
		if subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(opts.expectedNonce)) != 1 {
			return errors.New("oidcauth: token `nonce` does not match the authorization request nonce")
		}
	}

	if claims.Expiry == 0 {
		return errors.New("oidcauth: token has no `exp` claim")
	}
	if now().After(time.Unix(claims.Expiry, 0).Add(leeway)) {
		return errors.New("oidcauth: token expired")
	}
	if claims.IssuedAt != 0 && now().Add(leeway).Before(time.Unix(claims.IssuedAt, 0)) {
		return errors.New("oidcauth: token `iat` is in the future")
	}
	return nil
}

// hashForAlg maps a JWS `alg` header to a crypto.Hash. Only the RSA
// PKCS1v15 family is accepted — `none`, HS*, ES*, PS* are rejected.
// Rejecting `none` and the HMAC family closes the classic JWT
// alg-confusion vulnerability.
func hashForAlg(alg string) (crypto.Hash, error) {
	switch alg {
	case "RS256":
		return crypto.SHA256, nil
	case "RS384":
		return crypto.SHA384, nil
	case "RS512":
		return crypto.SHA512, nil
	case "", "none":
		return 0, errors.New("oidcauth: unsigned token rejected (alg=none)")
	default:
		return 0, fmt.Errorf("oidcauth: unsupported JWT alg %q (RS256/384/512 only)", alg)
	}
}
