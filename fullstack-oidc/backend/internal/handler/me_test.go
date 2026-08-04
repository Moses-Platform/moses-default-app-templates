package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moses-platform/fullstack-oidc/internal/oidcauth"
)

// withIdentity returns a request whose context carries the given
// Identity, as the oidcauth middleware would populate it.
func withIdentity(id oidcauth.Identity) *http.Request {
	r := httptest.NewRequest("GET", "/api/v1/me", nil)
	return r.WithContext(oidcauth.ContextWithIdentity(r.Context(), id))
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response not JSON: %v (body=%q)", err, rec.Body.String())
	}
	return m
}

// Me must surface the identity projection, including the roles the
// middleware lifted from resource_access.<client>.roles.
func TestMe_ProjectsIdentityAndRoles(t *testing.T) {
	rec := httptest.NewRecorder()
	Me(rec, withIdentity(oidcauth.Identity{
		Authenticated: true,
		Source:        "session",
		Subject:       "user-1",
		Email:         "u@example.com",
		Roles:         []string{"oidc-admin", "oidc-member"},
	}))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decode(t, rec)
	if body["authenticated"] != true {
		t.Errorf("authenticated = %v, want true", body["authenticated"])
	}
	if body["subject"] != "user-1" || body["email"] != "u@example.com" {
		t.Errorf("identity projection wrong: %v", body)
	}
	if body["source"] != "session" {
		t.Errorf("source = %v, want session", body["source"])
	}
	roles, _ := body["roles"].([]any)
	if len(roles) != 2 {
		t.Errorf("roles = %v, want 2 entries", body["roles"])
	}
}

// With no Identity on the context (as if the route were un-gated), Me
// reports the zero identity rather than erroring — the middleware is
// the gate, the handler just projects.
func TestMe_ZeroIdentityWhenUngated(t *testing.T) {
	rec := httptest.NewRecorder()
	Me(rec, httptest.NewRequest("GET", "/api/v1/me", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if decode(t, rec)["authenticated"] != false {
		t.Errorf("authenticated should be false for a bare context")
	}
}
