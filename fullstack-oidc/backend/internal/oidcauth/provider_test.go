package oidcauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAuthorizeRedirectURL(t *testing.T) {
	p := &provider{
		cfg:          Config{ClientID: "app-client"},
		authorizeURL: "https://kc.example.com/realms/moses/protocol/openid-connect/auth",
	}
	got := p.authorizeRedirectURL(
		"https://app.example.com/apps/t/s/auth/callback",
		"state-123", "challenge-abc", "")

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("authorize URL not parseable: %v", err)
	}
	q := u.Query()
	checks := map[string]string{
		"response_type":         "code",
		"client_id":             "app-client",
		"redirect_uri":          "https://app.example.com/apps/t/s/auth/callback",
		"scope":                 "openid profile email",
		"state":                 "state-123",
		"code_challenge":        "challenge-abc",
		"code_challenge_method": "S256",
	}
	for k, want := range checks {
		if q.Get(k) != want {
			t.Errorf("authorize query %q = %q, want %q", k, q.Get(k), want)
		}
	}
	if q.Has("prompt") {
		t.Errorf("prompt should be absent for an interactive flow")
	}
}

func TestAuthorizeRedirectURL_SilentPromptNone(t *testing.T) {
	p := &provider{
		cfg:          Config{ClientID: "c"},
		authorizeURL: "https://kc/auth",
	}
	got := p.authorizeRedirectURL("https://app/cb", "s", "ch", "none")
	u, _ := url.Parse(got)
	if u.Query().Get("prompt") != "none" {
		t.Errorf("silent flow must carry prompt=none, got %q", u.Query().Get("prompt"))
	}
}

func TestEndSessionRedirectURL(t *testing.T) {
	p := &provider{
		cfg:           Config{ClientID: "app-client"},
		endSessionURL: "https://kc.example.com/realms/moses/protocol/openid-connect/logout",
	}
	got := p.endSessionRedirectURL("https://app.example.com/apps/t/s/", "")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("end-session URL not parseable: %v", err)
	}
	if u.Query().Get("post_logout_redirect_uri") != "https://app.example.com/apps/t/s/" {
		t.Errorf("post_logout_redirect_uri missing/wrong: %q", u.Query().Get("post_logout_redirect_uri"))
	}
	if u.Query().Get("client_id") != "app-client" {
		t.Errorf("client_id missing from end-session URL")
	}
}

func TestEndSessionRedirectURL_NoEndpointFallsBack(t *testing.T) {
	p := &provider{cfg: Config{ClientID: "c"}} // no endSessionURL
	got := p.endSessionRedirectURL("https://app/home", "")
	if got != "https://app/home" {
		t.Errorf("with no end-session endpoint, should fall back to post-logout target, got %q", got)
	}
}

// discover() against an in-process discovery server — exercises the
// external + internal split without a live Keycloak.
func TestDiscover(t *testing.T) {
	discoveryFor := func(issuer string) string {
		doc := discoveryDoc{
			Issuer:                issuer,
			AuthorizationEndpoint: issuer + "/auth",
			TokenEndpoint:         issuer + "/token",
			JWKSURI:               issuer + "/certs",
			EndSessionEndpoint:    issuer + "/logout",
		}
		b, _ := json.Marshal(doc)
		return string(b)
	}

	var extURL, intURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Differentiate the two issuers by path prefix.
		if strings.HasPrefix(r.URL.Path, "/ext") {
			_, _ = w.Write([]byte(discoveryFor(extURL)))
		} else {
			_, _ = w.Write([]byte(discoveryFor(intURL)))
		}
	}))
	defer srv.Close()
	extURL = srv.URL + "/ext"
	intURL = srv.URL + "/int"

	p := newProvider(Config{
		Issuer:         extURL,
		InternalIssuer: intURL,
		ClientID:       "c",
		ClientSecret:   "s",
	})
	if err := p.discover(context.Background()); err != nil {
		t.Fatalf("discover: %v", err)
	}
	if p.authorizeURL != extURL+"/auth" {
		t.Errorf("authorizeURL should come from the EXTERNAL issuer: %q", p.authorizeURL)
	}
	if p.endSessionURL != extURL+"/logout" {
		t.Errorf("endSessionURL should come from the EXTERNAL issuer: %q", p.endSessionURL)
	}
	if p.tokenURL != intURL+"/token" {
		t.Errorf("tokenURL should come from the INTERNAL issuer: %q", p.tokenURL)
	}
	if p.keys == nil || p.keys.jwksURI != intURL+"/certs" {
		t.Errorf("JWKS URI should come from the INTERNAL issuer")
	}

	// discover() is idempotent — a second call is a no-op.
	if err := p.discover(context.Background()); err != nil {
		t.Errorf("second discover() should be a no-op, got %v", err)
	}
}

func TestExchangeCode(t *testing.T) {
	var gotForm url.Values
	var gotAuthHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		gotAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at","id_token":"idt","expires_in":300,"token_type":"Bearer"}`))
	}))
	defer srv.Close()

	p := &provider{
		cfg:        Config{ClientID: "app-client", ClientSecret: "topsecret"},
		tokenURL:   srv.URL,
		httpClient: srv.Client(),
	}
	tok, err := p.exchangeCode(context.Background(), "the-code",
		"https://app/cb", "the-verifier")
	if err != nil {
		t.Fatalf("exchangeCode: %v", err)
	}
	if tok.IDToken != "idt" {
		t.Errorf("id_token = %q, want idt", tok.IDToken)
	}
	if gotForm.Get("grant_type") != "authorization_code" {
		t.Errorf("grant_type = %q", gotForm.Get("grant_type"))
	}
	if gotForm.Get("code") != "the-code" {
		t.Errorf("code = %q", gotForm.Get("code"))
	}
	if gotForm.Get("code_verifier") != "the-verifier" {
		t.Errorf("PKCE code_verifier not sent")
	}
	if gotAuthHeader == "" || !strings.HasPrefix(gotAuthHeader, "Basic ") {
		t.Errorf("confidential client must send HTTP Basic auth, got %q", gotAuthHeader)
	}
}

func TestExchangeCode_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"code expired"}`))
	}))
	defer srv.Close()
	p := &provider{
		cfg:        Config{ClientID: "c", ClientSecret: "s"},
		tokenURL:   srv.URL,
		httpClient: srv.Client(),
	}
	if _, err := p.exchangeCode(context.Background(), "c", "r", "v"); err == nil {
		t.Errorf("exchangeCode should error on an OAuth error response")
	}
}
