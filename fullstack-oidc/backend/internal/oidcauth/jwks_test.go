package oidcauth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- test JWT signing helpers ----------------------------------------

// testSigner builds and signs JWTs with a generated RSA key, and serves
// the matching JWKS — everything offline, no live Keycloak.
type testSigner struct {
	key *rsa.PrivateKey
	kid string
}

func newTestSigner(t *testing.T) *testSigner {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	return &testSigner{key: key, kid: "test-kid-1"}
}

// jwksJSON returns a JWKS document for this signer's public key.
func (s *testSigner) jwksJSON() []byte {
	n := base64.RawURLEncoding.EncodeToString(s.key.PublicKey.N.Bytes())
	eBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(eBytes, uint64(s.key.PublicKey.E))
	// trim leading zero bytes
	i := 0
	for i < len(eBytes)-1 && eBytes[i] == 0 {
		i++
	}
	e := base64.RawURLEncoding.EncodeToString(eBytes[i:])
	doc := jwksDoc{Keys: []jwkRSA{{
		Kty: "RSA", Kid: s.kid, Use: "sig", Alg: "RS256", N: n, E: e,
	}}}
	b, _ := json.Marshal(doc)
	return b
}

// sign builds a compact RS256 JWT from the given claim map.
func (s *testSigner) sign(t *testing.T, claims map[string]any) string {
	return s.signWithAlg(t, "RS256", claims)
}

func (s *testSigner) signWithAlg(t *testing.T, alg string, claims map[string]any) string {
	t.Helper()
	header := map[string]any{"alg": alg, "typ": "JWT", "kid": s.kid}
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	seg := base64.RawURLEncoding.EncodeToString(hb) + "." +
		base64.RawURLEncoding.EncodeToString(cb)
	digest := sha256.Sum256([]byte(seg))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return seg + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// keySetFor builds a keySet backed by an in-process JWKS test server.
func keySetFor(t *testing.T, s *testSigner) *keySet {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(s.jwksJSON())
	}))
	t.Cleanup(srv.Close)
	return newKeySet(srv.URL, srv.Client())
}

// --- parseJWKS / toRSA -----------------------------------------------

func TestParseJWKS(t *testing.T) {
	s := newTestSigner(t)
	keys, err := parseJWKS(s.jwksJSON())
	if err != nil {
		t.Fatalf("parseJWKS: %v", err)
	}
	k, ok := keys[s.kid]
	if !ok {
		t.Fatalf("kid %q not in parsed key set", s.kid)
	}
	if k.pub.N.Cmp(s.key.PublicKey.N) != 0 || k.pub.E != s.key.PublicKey.E {
		t.Errorf("parsed RSA key does not match the source key")
	}
	if k.alg != "RS256" {
		t.Errorf("parsed JWK alg = %q, want %q", k.alg, "RS256")
	}
}

// TestParseJWKS_EmptyKidDuplicates verifies that two RSA keys with an
// empty `kid` do not silently collapse — the first is kept, the rest
// skipped (CHAT-t5d1u.28.20 N1).
func TestParseJWKS_EmptyKidDuplicates(t *testing.T) {
	a := newTestSigner(t)
	b := newTestSigner(t)
	jwkOf := func(s *testSigner) jwkRSA {
		n := base64.RawURLEncoding.EncodeToString(s.key.PublicKey.N.Bytes())
		eBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(eBytes, uint64(s.key.PublicKey.E))
		i := 0
		for i < len(eBytes)-1 && eBytes[i] == 0 {
			i++
		}
		e := base64.RawURLEncoding.EncodeToString(eBytes[i:])
		return jwkRSA{Kty: "RSA", Kid: "", Use: "sig", Alg: "RS256", N: n, E: e}
	}
	doc := jwksDoc{Keys: []jwkRSA{jwkOf(a), jwkOf(b)}}
	body, _ := json.Marshal(doc)

	keys, err := parseJWKS(body)
	if err != nil {
		t.Fatalf("parseJWKS: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key (empty-kid dedup), got %d", len(keys))
	}
	// The FIRST empty-kid key (signer a) must be the survivor.
	k := keys[""]
	if k.pub.N.Cmp(a.key.PublicKey.N) != 0 {
		t.Errorf("empty-kid survivor is not the first key")
	}
}

func TestParseJWKS_NoUsableKeys(t *testing.T) {
	doc := []byte(`{"keys":[{"kty":"EC","kid":"ec1"}]}`)
	if _, err := parseJWKS(doc); err == nil {
		t.Errorf("parseJWKS accepted a doc with no RSA keys")
	}
}

func TestParseJWKS_Garbage(t *testing.T) {
	if _, err := parseJWKS([]byte("not json")); err == nil {
		t.Errorf("parseJWKS accepted non-JSON input")
	}
}

// --- verifyToken: happy path -----------------------------------------

func TestVerifyToken_ValidRS256(t *testing.T) {
	s := newTestSigner(t)
	ks := keySetFor(t, s)

	now := time.Now()
	tok := s.sign(t, map[string]any{
		"sub": "user-1",
		"iss": "https://kc.example.com/realms/moses",
		"aud": "app-client",
		"exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(),
		"resource_access": map[string]any{
			"app-client": map[string]any{"roles": []string{"viewer", "admin"}},
		},
	})

	claims, err := verifyToken(context.Background(), tok, ks, verifyOptions{
		expectedIssuer:   "https://kc.example.com/realms/moses",
		expectedAudience: "app-client",
	})
	if err != nil {
		t.Fatalf("verifyToken: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Errorf("Subject = %q", claims.Subject)
	}
	if !claims.HasRole("app-client", "admin") {
		t.Errorf("expected resource_access role 'admin' to be exposed")
	}
	if claims.HasRole("app-client", "nope") {
		t.Errorf("HasRole returned true for a role not present")
	}
}

// --- verifyToken: rejection cases ------------------------------------

func TestVerifyToken_RejectsAlgNone(t *testing.T) {
	// An unsigned token: alg=none, empty signature.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"attacker"}`))
	tok := header + "." + payload + "."
	s := newTestSigner(t)
	ks := keySetFor(t, s)
	if _, err := verifyToken(context.Background(), tok, ks, verifyOptions{}); err == nil {
		t.Errorf("verifyToken accepted an alg=none token")
	}
}

func TestVerifyToken_RejectsTamperedSignature(t *testing.T) {
	s := newTestSigner(t)
	ks := keySetFor(t, s)
	tok := s.sign(t, map[string]any{
		"sub": "u", "exp": time.Now().Add(time.Hour).Unix(),
	})
	// Corrupt the signature segment.
	parts := strings.Split(tok, ".")
	tampered := parts[0] + "." + parts[1] + "." + strings.Repeat("A", len(parts[2]))
	if _, err := verifyToken(context.Background(), tampered, ks, verifyOptions{}); err == nil {
		t.Errorf("verifyToken accepted a tampered signature")
	}
}

func TestVerifyToken_RejectsWrongKey(t *testing.T) {
	signer := newTestSigner(t)
	other := newTestSigner(t) // JWKS serves a DIFFERENT key
	other.kid = signer.kid    // same kid, different key material
	ks := keySetFor(t, other)
	tok := signer.sign(t, map[string]any{
		"sub": "u", "exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := verifyToken(context.Background(), tok, ks, verifyOptions{}); err == nil {
		t.Errorf("verifyToken accepted a token signed by a non-JWKS key")
	}
}

// TestVerifyToken_RejectsAlgJWKMismatch verifies the N1 hardening: when
// the JWK declares an `alg`, a JWT whose header `alg` differs is
// rejected before signature verification (CHAT-t5d1u.28.20 N1).
func TestVerifyToken_RejectsAlgJWKMismatch(t *testing.T) {
	s := newTestSigner(t) // jwksJSON declares alg=RS256
	ks := keySetFor(t, s)
	// Header claims RS384 (a valid, accepted alg) but the JWK is RS256.
	// signWithAlg only sets the header — the body is RS256-signed.
	tok := s.signWithAlg(t, "RS384", map[string]any{
		"sub": "u", "exp": time.Now().Add(time.Hour).Unix(),
	})
	_, err := verifyToken(context.Background(), tok, ks, verifyOptions{})
	if err == nil {
		t.Fatalf("verifyToken accepted a token whose header alg != JWK alg")
	}
	if !strings.Contains(err.Error(), "does not match JWK alg") {
		t.Errorf("expected JWK-alg-mismatch error, got: %v", err)
	}
}

// TestVerifyToken_AlgMatchesJWK confirms a token whose header alg equals
// the JWK alg still verifies (regression guard for the N1 cross-check).
func TestVerifyToken_AlgMatchesJWK(t *testing.T) {
	s := newTestSigner(t) // jwksJSON declares alg=RS256
	ks := keySetFor(t, s)
	now := time.Now()
	tok := s.sign(t, map[string]any{ // sign() uses RS256
		"sub": "u",
		"iss": "https://iss",
		"aud": "app-client",
		"exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(),
	})
	if _, err := verifyToken(context.Background(), tok, ks, verifyOptions{
		expectedIssuer: "https://iss", expectedAudience: "app-client",
	}); err != nil {
		t.Fatalf("verifyToken rejected a token whose header alg matches the JWK alg: %v", err)
	}
}

func TestVerifyToken_NotThreeParts(t *testing.T) {
	s := newTestSigner(t)
	ks := keySetFor(t, s)
	if _, err := verifyToken(context.Background(), "a.b", ks, verifyOptions{}); err == nil {
		t.Errorf("verifyToken accepted a 2-part token")
	}
}

// --- validateClaims (no signing needed) ------------------------------

func TestValidateClaims(t *testing.T) {
	fixed := time.Unix(1_700_000_000, 0)
	nowFn := func() time.Time { return fixed }

	base := func() *Claims {
		return &Claims{
			Issuer:   "https://iss",
			Audience: audSlice{"app-client"},
			Expiry:   fixed.Add(time.Hour).Unix(),
			IssuedAt: fixed.Add(-time.Minute).Unix(),
		}
	}

	t.Run("valid", func(t *testing.T) {
		err := validateClaims(base(), verifyOptions{
			expectedIssuer: "https://iss", expectedAudience: "app-client", now: nowFn,
		})
		if err != nil {
			t.Errorf("validateClaims = %v, want nil", err)
		}
	})

	t.Run("issuer mismatch", func(t *testing.T) {
		err := validateClaims(base(), verifyOptions{
			expectedIssuer: "https://evil", now: nowFn,
		})
		if err == nil {
			t.Errorf("expected issuer-mismatch error")
		}
	})

	t.Run("issuer trailing slash tolerated", func(t *testing.T) {
		c := base()
		c.Issuer = "https://iss/"
		err := validateClaims(c, verifyOptions{expectedIssuer: "https://iss", now: nowFn})
		if err != nil {
			t.Errorf("trailing-slash issuer should match: %v", err)
		}
	})

	t.Run("audience mismatch", func(t *testing.T) {
		err := validateClaims(base(), verifyOptions{
			expectedAudience: "other-client", now: nowFn,
		})
		if err == nil {
			t.Errorf("expected audience-mismatch error")
		}
	})

	t.Run("expired", func(t *testing.T) {
		c := base()
		c.Expiry = fixed.Add(-time.Hour).Unix()
		if err := validateClaims(c, verifyOptions{now: nowFn}); err == nil {
			t.Errorf("expected expired-token error")
		}
	})

	t.Run("no exp", func(t *testing.T) {
		c := base()
		c.Expiry = 0
		if err := validateClaims(c, verifyOptions{now: nowFn}); err == nil {
			t.Errorf("expected missing-exp error")
		}
	})

	t.Run("iat in future beyond leeway", func(t *testing.T) {
		c := base()
		c.IssuedAt = fixed.Add(time.Hour).Unix()
		if err := validateClaims(c, verifyOptions{now: nowFn}); err == nil {
			t.Errorf("expected future-iat error")
		}
	})

	t.Run("exp within leeway is tolerated", func(t *testing.T) {
		c := base()
		c.Expiry = fixed.Add(-30 * time.Second).Unix() // expired 30s ago, < 60s leeway
		if err := validateClaims(c, verifyOptions{now: nowFn}); err != nil {
			t.Errorf("exp within leeway should pass: %v", err)
		}
	})
}

func TestAudSlice_UnmarshalBothShapes(t *testing.T) {
	var a audSlice
	if err := json.Unmarshal([]byte(`"single"`), &a); err != nil {
		t.Fatalf("string aud: %v", err)
	}
	if len(a) != 1 || a[0] != "single" {
		t.Errorf("string aud parsed wrong: %v", a)
	}
	var b audSlice
	if err := json.Unmarshal([]byte(`["x","y"]`), &b); err != nil {
		t.Fatalf("array aud: %v", err)
	}
	if len(b) != 2 || !b.contains("y") {
		t.Errorf("array aud parsed wrong: %v", b)
	}
}

func TestHashForAlg(t *testing.T) {
	for _, alg := range []string{"RS256", "RS384", "RS512"} {
		if _, err := hashForAlg(alg); err != nil {
			t.Errorf("hashForAlg(%q) = %v, want nil", alg, err)
		}
	}
	for _, alg := range []string{"none", "", "HS256", "ES256", "PS256"} {
		if _, err := hashForAlg(alg); err == nil {
			t.Errorf("hashForAlg(%q) accepted a forbidden alg", alg)
		}
	}
}

func TestRolesForClient_NilSafe(t *testing.T) {
	var c *Claims
	if c.RolesForClient("x") != nil {
		t.Errorf("RolesForClient on nil Claims should be nil")
	}
	if c.HasRole("x", "y") {
		t.Errorf("HasRole on nil Claims should be false")
	}
}
