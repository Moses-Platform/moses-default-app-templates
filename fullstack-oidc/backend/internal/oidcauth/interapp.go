package oidcauth

// Inter-app trust (P2c) — the office-suite sibling-call path.
//
// # What it is
//
// Apps in the same Moses tenant sometimes need to call one another's
// backend on behalf of the *current user* — the office suite (docs ↔
// sheets ↔ calendar, etc.) is the motivating case. A calling app's
// backend mints a short-lived HS256 token asserting "user U (a Moses
// platform UUID), in tenant T, is acting through me" and sets it as the
// X-Moses-Interapp-Token header when it calls a sibling app via that
// sibling's MOSES_APP_LINK_<NAME> URL. The sibling verifies the token
// and trusts the asserted user identity — but NOT any roles.
//
// # Trust model (RISK-ACCEPTED)
//
// All interop apps in a tenant share ONE signing secret,
// MOSES_INTERAPP_SECRET, injected per-tenant by the platform. HS256 is a
// symmetric MAC: any holder of the secret can both mint and verify.
//
//	RESIDUAL RISK (accepted by the office-suite epic): because the secret
//	is shared by every first-party app in the tenant, ANY first-party
//	tenant app can mint a token impersonating ANY user in that tenant.
//	There is no per-app key and no asymmetric signature, so the receiving
//	app cannot cryptographically distinguish *which* sibling minted the
//	token, nor prove the asserted user actually authenticated to the
//	caller. The mitigation is twofold and deliberate:
//	  1. The secret is per-TENANT, so the blast radius is one tenant's
//	     own first-party apps — never cross-tenant.
//	  2. ROLES ARE NEVER CARRIED across this hop (see below). The sibling
//	     authorizes the asserted user against its OWN ACLs; it never
//	     trusts the caller to assert authorization. So the worst a rogue
//	     sibling can do is act AS a user the receiving app would already
//	     have let that user do — it cannot escalate privilege.
//
// This is acceptable for first-party office apps that already run inside
// one tenant's trust boundary. It is NOT a general-purpose delegation
// protocol and must not be extended to third-party / marketplace apps
// without an asymmetric, per-app-keyed redesign.
//
// # Token contract (the platform provisions the secret to match)
//
//	Header:  X-Moses-Interapp-Token
//	Alg:     HS256 (HMAC-SHA256), compact JWS: base64url(header).base64url(payload).base64url(sig)
//	Claims:  { iss: <caller MOSES_APP_SLUG>, aud: <callee logical slug>,
//	           moses_user_id: <acting user's Moses UUID>,
//	           tenant_id: <MOSES_TENANT_ID>, iat, exp }
//	exp:     <= iat + 60s (short-lived; a leaked token expires fast)
//
// Implemented stdlib-only — the same crypto/hmac + crypto/sha256 +
// encoding/base64 + encoding/json the session cookie already uses. No
// JWT dependency is added.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	// EnvInterAppSecret is the per-tenant HS256 signing key shared by all
	// of the tenant's interop apps. Injected by the platform. When unset
	// the inter-app trust path is DISABLED entirely (interAppEnabled()
	// false) — fail-safe.
	EnvInterAppSecret = "MOSES_INTERAPP_SECRET"

	// EnvAppSlug is the app's own logical slug. It is the `iss` this app
	// stamps when minting, and the `aud` this app requires when verifying
	// an inbound token (a token addressed to a different app is rejected).
	EnvAppSlug = "MOSES_APP_SLUG"

	// EnvTenantID is the app's own tenant UUID. A verified token's
	// tenant_id MUST equal it — defence in depth against a token minted
	// with a different tenant's secret somehow reaching this pod.
	EnvTenantID = "MOSES_TENANT_ID"
)

// HeaderInterAppToken carries the compact HS256 inter-app token. A calling
// app sets it when invoking a sibling via MOSES_APP_LINK_<NAME>.
const HeaderInterAppToken = "X-Moses-Interapp-Token"

// interAppClockSkew is the tolerance applied to exp validation, covering
// small clock drift between sibling pods. Kept tiny — the tokens live at
// most 60s, so a large skew would defeat the short-lived design.
const interAppClockSkew = 3 * time.Second

// interAppMaxLifetime bounds how far in the future a token's exp may sit
// relative to its iat. A token that claims a longer lifetime than the
// contract allows is rejected (a caller cannot mint a long-lived token by
// inflating exp). 60s per the contract, plus the skew tolerance.
const interAppMaxLifetime = 60*time.Second + interAppClockSkew

// interAppClaims is the JWT payload for the inter-app trust token. Field
// names are the wire claim names the platform provisions against.
type interAppClaims struct {
	Issuer      string `json:"iss"`           // caller's MOSES_APP_SLUG
	Audience    string `json:"aud"`           // callee's logical slug
	MosesUserID string `json:"moses_user_id"` // acting user's Moses UUID
	TenantID    string `json:"tenant_id"`     // caller's MOSES_TENANT_ID
	IssuedAt    int64  `json:"iat"`           // unix seconds
	Expiry      int64  `json:"exp"`           // unix seconds, <= iat+60s
}

// interAppHeader is the fixed JWS header. We only ever emit/accept HS256.
type interAppHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// interAppEnabled reports whether the inter-app trust path is armed. It is
// gated SOLELY on the shared secret being present — INDEPENDENT of OIDC
// Enabled(): an interop app may be header-trust-/interapp-only and never
// run the full OIDC relying-party flow. When the secret is unset both the
// verify and mint paths are disabled (fail-safe).
func (c Config) interAppEnabled() bool {
	return c.InterAppSecret != "" && c.AppSlug != "" && c.TenantID != ""
}

// MintInterAppToken builds and HS256-signs an inter-app trust token for a
// call to targetAppSlug, asserting that actingUserMosesID is the user on
// whose behalf the call is made.
//
// The acting user's Moses UUID is passed EXPLICITLY (rather than read from
// ambient state) so the caller is forced to source it from its OWN
// authenticated Identity (Identity.Subject after a session/header-trust
// decision) — never from an unauthenticated, caller-supplied value. The
// receiving sibling trusts this identity; minting it from an unverified
// id would let a browser forge an arbitrary user.
//
// Returns an error when the inter-app path is not configured (secret /
// app-slug / tenant unset) or when actingUserMosesID is blank.
func (c Config) MintInterAppToken(targetAppSlug, actingUserMosesID string) (string, error) {
	if c.InterAppSecret == "" {
		return "", errors.New("oidcauth: inter-app secret unset (" + EnvInterAppSecret + "); cannot mint")
	}
	if strings.TrimSpace(c.AppSlug) == "" {
		return "", errors.New("oidcauth: own app slug unset (" + EnvAppSlug + "); cannot mint")
	}
	if strings.TrimSpace(c.TenantID) == "" {
		return "", errors.New("oidcauth: own tenant id unset (" + EnvTenantID + "); cannot mint")
	}
	if strings.TrimSpace(targetAppSlug) == "" {
		return "", errors.New("oidcauth: target app slug required to mint inter-app token")
	}
	if strings.TrimSpace(actingUserMosesID) == "" {
		// Refuse to mint an anonymous/forged identity. The caller must
		// pass its own authenticated user.
		return "", errors.New("oidcauth: acting user moses id required to mint inter-app token")
	}

	now := time.Now()
	claims := interAppClaims{
		Issuer:      c.AppSlug,
		Audience:    strings.TrimSpace(targetAppSlug),
		MosesUserID: strings.TrimSpace(actingUserMosesID),
		TenantID:    c.TenantID,
		IssuedAt:    now.Unix(),
		Expiry:      now.Add(60 * time.Second).Unix(),
	}
	return signInterAppToken(claims, []byte(c.InterAppSecret))
}

// signInterAppToken serializes the fixed HS256 header + claims and appends
// the base64url HMAC-SHA256 over "header.payload", yielding a compact JWS.
func signInterAppToken(claims interAppClaims, secret []byte) (string, error) {
	hdrJSON, err := json.Marshal(interAppHeader{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", err
	}
	clmJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(hdrJSON) + "." +
		base64.RawURLEncoding.EncodeToString(clmJSON)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig, nil
}

// verifyInterAppToken parses, signature-checks, and validates an inbound
// inter-app token against THIS app's own slug + tenant. On success it
// returns the trusted moses_user_id. ANY failure returns a non-nil error
// and the caller MUST NOT trust the request — it falls through to the
// normal session/header/challenge path (a bad token never 401s outright).
//
// Validations (all must pass):
//   - exactly three base64url segments, JWS header alg == "HS256";
//   - HMAC-SHA256 over "header.payload" matches the signature in
//     CONSTANT TIME (hmac.Equal);
//   - exp present and not expired (with a small skew), and exp not further
//     than the contract's max lifetime past iat (no long-lived tokens);
//   - aud == this app's own AppSlug;
//   - tenant_id == this app's own TenantID;
//   - moses_user_id non-empty.
//
// Roles are intentionally NOT part of the contract and are never read.
func (c Config) verifyInterAppToken(token string) (mosesUserID string, err error) {
	if !c.interAppEnabled() {
		return "", errors.New("oidcauth: inter-app path disabled")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("oidcauth: empty inter-app token")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("oidcauth: inter-app token not a compact JWS")
	}
	headerSeg, payloadSeg, sigSeg := parts[0], parts[1], parts[2]

	// 1) Verify the signature BEFORE trusting any claim bytes. Recompute
	//    HMAC over the exact "header.payload" signing input and compare in
	//    constant time.
	gotSig, err := base64.RawURLEncoding.DecodeString(sigSeg)
	if err != nil {
		return "", errors.New("oidcauth: inter-app token signature not base64url")
	}
	mac := hmac.New(sha256.New, []byte(c.InterAppSecret))
	mac.Write([]byte(headerSeg + "." + payloadSeg))
	wantSig := mac.Sum(nil)
	if !hmac.Equal(gotSig, wantSig) {
		return "", errors.New("oidcauth: inter-app token signature invalid")
	}

	// 2) Confirm the declared algorithm is HS256 — reject "none" and any
	//    asymmetric alg an attacker might substitute (alg-confusion guard).
	hdrJSON, err := base64.RawURLEncoding.DecodeString(headerSeg)
	if err != nil {
		return "", errors.New("oidcauth: inter-app token header not base64url")
	}
	var hdr interAppHeader
	if err := json.Unmarshal(hdrJSON, &hdr); err != nil {
		return "", errors.New("oidcauth: inter-app token header not JSON")
	}
	if hdr.Alg != "HS256" {
		return "", errors.New("oidcauth: inter-app token alg not HS256")
	}

	// 3) Decode + validate the claims.
	clmJSON, err := base64.RawURLEncoding.DecodeString(payloadSeg)
	if err != nil {
		return "", errors.New("oidcauth: inter-app token payload not base64url")
	}
	var claims interAppClaims
	if err := json.Unmarshal(clmJSON, &claims); err != nil {
		return "", errors.New("oidcauth: inter-app token payload not JSON")
	}

	now := time.Now()
	if claims.Expiry == 0 {
		return "", errors.New("oidcauth: inter-app token missing exp")
	}
	exp := time.Unix(claims.Expiry, 0)
	if now.After(exp.Add(interAppClockSkew)) {
		return "", errors.New("oidcauth: inter-app token expired")
	}
	// Reject a token whose claimed lifetime exceeds the contract — a
	// caller cannot mint a long-lived token by inflating exp past iat+60s.
	if claims.IssuedAt != 0 {
		iat := time.Unix(claims.IssuedAt, 0)
		if exp.Sub(iat) > interAppMaxLifetime {
			return "", errors.New("oidcauth: inter-app token lifetime exceeds contract")
		}
	}
	if !strings.EqualFold(strings.TrimSpace(claims.Audience), c.AppSlug) {
		return "", errors.New("oidcauth: inter-app token aud is not this app")
	}
	if strings.TrimSpace(claims.TenantID) != c.TenantID {
		return "", errors.New("oidcauth: inter-app token tenant mismatch")
	}
	if strings.TrimSpace(claims.MosesUserID) == "" {
		return "", errors.New("oidcauth: inter-app token missing moses_user_id")
	}

	return strings.TrimSpace(claims.MosesUserID), nil
}

// interAppTokenFromRequest extracts a non-blank inter-app token from the
// request header, or "" when absent.
func interAppTokenFromRequest(get func(string) string) string {
	return strings.TrimSpace(get(HeaderInterAppToken))
}
