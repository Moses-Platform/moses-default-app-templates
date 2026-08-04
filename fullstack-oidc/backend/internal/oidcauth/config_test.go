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
	t.Setenv(EnvIssuer, "https://kc.example.com/auth/realms/moses/")
	t.Setenv(EnvInternalIssuer, "http://keycloak.moses.svc:8080/auth/realms/moses")
	t.Setenv(EnvClientID, "app-client")
	t.Setenv(EnvClientSecret, "s3cr3t")
	t.Setenv(EnvAudience, "app-audience")
	t.Setenv(EnvBasePath, "/apps/tenant/slug/")
	t.Setenv(EnvProtectedPaths, "/api/private,/admin")
	t.Setenv(EnvPublicPaths, "/api/v1/info")
	t.Setenv(EnvCookieSecret, "deadbeefdeadbeefdeadbeefdeadbeef")

	cfg := ConfigFromEnv()

	if cfg.Issuer != "https://kc.example.com/auth/realms/moses" {
		t.Errorf("Issuer trailing slash not trimmed: %q", cfg.Issuer)
	}
	if cfg.InternalIssuer != "http://keycloak.moses.svc:8080/auth/realms/moses" {
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

func TestConfigFromEnv_GatewayAuthSecret(t *testing.T) {
	// Set -> read (and trimmed) into Config.GatewayAuthSecret.
	t.Setenv(EnvGatewayAuthSecret, "  shared-gateway-secret  ")
	cfg := ConfigFromEnv()
	if cfg.GatewayAuthSecret != "shared-gateway-secret" {
		t.Errorf("GatewayAuthSecret = %q, want trimmed %q", cfg.GatewayAuthSecret, "shared-gateway-secret")
	}
}

func TestConfigFromEnv_GatewayAuthSecretUnsetIsEmpty(t *testing.T) {
	// Unset -> empty -> header-trust path disabled (fail-safe).
	t.Setenv(EnvGatewayAuthSecret, "")
	cfg := ConfigFromEnv()
	if cfg.GatewayAuthSecret != "" {
		t.Errorf("GatewayAuthSecret = %q, want empty when env unset", cfg.GatewayAuthSecret)
	}
}

func TestConfigFromEnv_CookieNamespace(t *testing.T) {
	// Set -> read (and trimmed) into Config.CookieNamespace, which pins the
	// cookie NAME to a stable per-(chart,track) token (app-OIDC cookie-overflow fix).
	t.Setenv(EnvCookieNamespace, "  app-2837eb92-dev  ")
	cfg := ConfigFromEnv()
	if cfg.CookieNamespace != "app-2837eb92-dev" {
		t.Errorf("CookieNamespace = %q, want trimmed %q", cfg.CookieNamespace, "app-2837eb92-dev")
	}
}

func TestConfigFromEnv_CookieNamespaceUnsetIsEmpty(t *testing.T) {
	// Unset -> empty -> cookieSuffix() falls back to sha256(ClientID) (legacy).
	t.Setenv(EnvCookieNamespace, "")
	cfg := ConfigFromEnv()
	if cfg.CookieNamespace != "" {
		t.Errorf("CookieNamespace = %q, want empty when env unset", cfg.CookieNamespace)
	}
}

func TestConfigFromEnv_PublicURL(t *testing.T) {
	// Set -> read (and trimmed) into Config.PublicURL. The middleware
	// prefers this over r.Host when building OIDC redirect_uri values
	// (envoy strips :port from Host on the in-cluster hop).
	t.Setenv(EnvPublicURL, "  http://localhost:9877  ")
	cfg := ConfigFromEnv()
	if cfg.PublicURL != "http://localhost:9877" {
		t.Errorf("PublicURL = %q, want trimmed %q", cfg.PublicURL, "http://localhost:9877")
	}
}

func TestConfigFromEnv_PublicURLUnsetIsEmpty(t *testing.T) {
	// Unset -> empty -> absoluteURL falls back to r.Host derivation
	// (preserves behaviour for non-platform deployments).
	t.Setenv(EnvPublicURL, "")
	cfg := ConfigFromEnv()
	if cfg.PublicURL != "" {
		t.Errorf("PublicURL = %q, want empty when env unset", cfg.PublicURL)
	}
}

func TestConfigFromEnv_InternalIssuerFallsBackToIssuer(t *testing.T) {
	t.Setenv(EnvIssuer, "https://kc.example.com/auth/realms/moses")
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

func TestConfigFromEnv_PublicURLs(t *testing.T) {
	t.Setenv(EnvPublicURLs, " https://moses-manager.eu, https://platform.moses-manager.eu ")
	cfg := ConfigFromEnv()
	want := []string{"https://moses-manager.eu", "https://platform.moses-manager.eu"}
	if !reflect.DeepEqual(cfg.PublicURLs, want) {
		t.Fatalf("PublicURLs = %#v, want %#v", cfg.PublicURLs, want)
	}
}

func TestConfigFromEnv_PublicURLsUnsetIsNil(t *testing.T) {
	t.Setenv(EnvPublicURLs, "")
	if cfg := ConfigFromEnv(); cfg.PublicURLs != nil {
		t.Fatalf("PublicURLs = %#v, want nil when unset", cfg.PublicURLs)
	}
}
