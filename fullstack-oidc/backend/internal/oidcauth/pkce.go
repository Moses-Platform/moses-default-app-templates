package oidcauth

import (
	"crypto/sha256"
	"encoding/base64"
)

// PKCE (RFC 7636) protects the authorization-code flow against code
// interception. We use the `S256` challenge method exclusively — `plain`
// is never offered.

// newCodeVerifier returns a high-entropy PKCE code verifier. RFC 7636
// requires 43–128 chars from the unreserved URL set; base64url of 32
// random bytes yields 43 chars.
func newCodeVerifier() string {
	return randomURLToken(32)
}

// codeChallengeS256 derives the S256 code challenge from a verifier:
// base64url(SHA256(verifier)).
func codeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
