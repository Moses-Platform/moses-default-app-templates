package oidcauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestNewCodeVerifier(t *testing.T) {
	a := newCodeVerifier()
	b := newCodeVerifier()
	if a == b {
		t.Errorf("code verifiers must be unique per call")
	}
	// RFC 7636: 43–128 chars. base64url(32 bytes) == 43 chars.
	if len(a) < 43 || len(a) > 128 {
		t.Errorf("code verifier length %d outside RFC 7636 range", len(a))
	}
}

func TestCodeChallengeS256(t *testing.T) {
	verifier := "fixed-verifier-for-determinism"
	got := codeChallengeS256(verifier)

	sum := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if got != want {
		t.Errorf("codeChallengeS256 = %q, want %q", got, want)
	}
	// Deterministic for the same verifier.
	if codeChallengeS256(verifier) != got {
		t.Errorf("codeChallengeS256 is not deterministic")
	}
	// Different verifier -> different challenge.
	if codeChallengeS256("other") == got {
		t.Errorf("different verifiers produced the same challenge")
	}
}
