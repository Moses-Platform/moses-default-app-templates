package oidcauth

import (
	"reflect"
	"testing"
)

func TestSplitPaths(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"blank", "   ", nil},
		{"single", "/api", []string{"/api"}},
		{"adds leading slash", "api", []string{"/api"}},
		{"comma separated", "/api,/admin", []string{"/api", "/admin"}},
		{"space separated", "/api /admin", []string{"/api", "/admin"}},
		{"mixed separators", "/api, /admin\n/secret", []string{"/api", "/admin", "/secret"}},
		{"trims blanks", " /api ,, /admin ", []string{"/api", "/admin"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := splitPaths(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("splitPaths(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestIsTruthy(t *testing.T) {
	truthy := []string{"1", "true", "TRUE", "yes", "on", " true "}
	for _, s := range truthy {
		if !isTruthy(s) {
			t.Errorf("isTruthy(%q) = false, want true", s)
		}
	}
	falsy := []string{"", "0", "false", "no", "off", "nope"}
	for _, s := range falsy {
		if isTruthy(s) {
			t.Errorf("isTruthy(%q) = true, want false", s)
		}
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv(EnvIssuer, "https://kc.example.com/realms/moses/")
	t.Setenv(EnvInternalIssuer, "http://keycloak.moses.svc:8080/realms/moses")
	t.Setenv(EnvClientID, "app-client")
	t.Setenv(EnvClientSecret, "s3cr3t")
	t.Setenv(EnvAudience, "app-audience")
	t.Setenv(EnvBasePath, "/apps/tenant/slug/")
	t.Setenv(EnvProtectedPaths, "/api/private,/admin")
	t.Setenv(EnvPublicPaths, "/api/v1/info")
	t.Setenv(EnvCookieSecret, "deadbeefdeadbeefdeadbeefdeadbeef")

	cfg := ConfigFromEnv()

	if cfg.Issuer != "https://kc.example.com/realms/moses" {
		t.Errorf("Issuer trailing slash not trimmed: %q", cfg.Issuer)
	}
	if cfg.InternalIssuer != "http://keycloak.moses.svc:8080/realms/moses" {
		t.Errorf("InternalIssuer = %q", cfg.InternalIssuer)
	}
	if cfg.BasePath != "/apps/tenant/slug" {
		t.Errorf("BasePath trailing slash not trimmed: %q", cfg.BasePath)
	}
	if !reflect.DeepEqual(cfg.ProtectedPaths, []string{"/api/private", "/admin"}) {
		t.Errorf("ProtectedPaths = %v", cfg.ProtectedPaths)
	}
	if !reflect.DeepEqual(cfg.PublicPaths, []string{"/api/v1/info"}) {
		t.Errorf("PublicPaths = %v", cfg.PublicPaths)
	}
	if string(cfg.CookieSecret) != "deadbeefdeadbeefdeadbeefdeadbeef" {
		t.Errorf("CookieSecret not read from env")
	}
	if !cfg.SecureCookie {
		t.Errorf("SecureCookie should default to true")
	}
	if !cfg.Enabled() {
		t.Errorf("Enabled() = false for a fully-configured Config")
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestConfigFromEnv_InternalIssuerFallsBackToIssuer(t *testing.T) {
	t.Setenv(EnvIssuer, "https://kc.example.com/realms/moses")
	t.Setenv(EnvInternalIssuer, "")
	t.Setenv(EnvClientID, "c")
	t.Setenv(EnvClientSecret, "s")

	cfg := ConfigFromEnv()
	if cfg.InternalIssuer != cfg.Issuer {
		t.Errorf("InternalIssuer = %q, want fallback to Issuer %q", cfg.InternalIssuer, cfg.Issuer)
	}
}

func TestConfigFromEnv_RandomCookieSecretWhenUnset(t *testing.T) {
	t.Setenv(EnvCookieSecret, "")
	cfg := ConfigFromEnv()
	if len(cfg.CookieSecret) != 32 {
		t.Errorf("fallback CookieSecret len = %d, want 32", len(cfg.CookieSecret))
	}
}

func TestConfigFromEnv_InsecureCookieEscapeHatch(t *testing.T) {
	t.Setenv(EnvInsecureCookie, "true")
	cfg := ConfigFromEnv()
	if cfg.SecureCookie {
		t.Errorf("SecureCookie = true with MOSES_OIDC_INSECURE_COOKIE=true")
	}
}

func TestConfigEnabledAndValidate(t *testing.T) {
	cases := []struct {
		name        string
		cfg         Config
		wantEnabled bool
	}{
		{"all set", Config{Issuer: "i", ClientID: "c", ClientSecret: "s"}, true},
		{"no issuer", Config{ClientID: "c", ClientSecret: "s"}, false},
		{"no client", Config{Issuer: "i", ClientSecret: "s"}, false},
		{"no secret", Config{Issuer: "i", ClientID: "c"}, false},
		{"empty", Config{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.cfg.Enabled(); got != c.wantEnabled {
				t.Errorf("Enabled() = %v, want %v", got, c.wantEnabled)
			}
			err := c.cfg.Validate()
			if c.wantEnabled && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
			if !c.wantEnabled && err == nil {
				t.Errorf("Validate() = nil, want error")
			}
		})
	}
}

func TestExpectedAudience(t *testing.T) {
	if got := (Config{Audience: "aud", ClientID: "cid"}).expectedAudience(); got != "aud" {
		t.Errorf("expectedAudience() = %q, want %q", got, "aud")
	}
	if got := (Config{ClientID: "cid"}).expectedAudience(); got != "cid" {
		t.Errorf("expectedAudience() falls back to ClientID: got %q", got)
	}
}
