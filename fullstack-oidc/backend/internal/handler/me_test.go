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

// Me must surface the projected roles and compute is_app_admin from them.
func TestMe_ProjectsRolesAndAdminFlag(t *testing.T) {
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
	if body["is_app_admin"] != true {
		t.Errorf("is_app_admin = %v, want true (roles include oidc-admin)", body["is_app_admin"])
	}
	roles, _ := body["roles"].([]any)
	if len(roles) != 2 {
		t.Errorf("roles = %v, want 2 entries", body["roles"])
	}
}

// A signed-in user with no admin role gets is_app_admin=false.
func TestMe_NonAdminFlagFalse(t *testing.T) {
	rec := httptest.NewRecorder()
	Me(rec, withIdentity(oidcauth.Identity{
		Authenticated: true,
		Source:        "session",
		Subject:       "user-2",
		Roles:         []string{"oidc-member"},
	}))
	if decode(t, rec)["is_app_admin"] != false {
		t.Errorf("is_app_admin should be false for a non-admin user")
	}
}

// AdminArea must 403 a signed-in user who lacks the oidc-admin role.
func TestAdminArea_ForbidsNonAdmin(t *testing.T) {
	rec := httptest.NewRecorder()
	AdminArea(rec, withIdentity(oidcauth.Identity{
		Authenticated: true,
		Source:        "session",
		Subject:       "user-3",
		Roles:         []string{"oidc-member"},
	}))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if decode(t, rec)["required_role"] != roleAdmin {
		t.Errorf("403 body should name the required role")
	}
}

// AdminArea must serve a user whose token carries the oidc-admin role.
func TestAdminArea_AllowsAdmin(t *testing.T) {
	rec := httptest.NewRecorder()
	AdminArea(rec, withIdentity(oidcauth.Identity{
		Authenticated: true,
		Source:        "session",
		Subject:       "user-4",
		Roles:         []string{"oidc-admin"},
	}))
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if decode(t, rec)["ok"] != true {
		t.Errorf("admin response should report ok=true")
	}
}

// The pod-to-pod header-trust path carries no app roles, so AdminArea
// correctly 403s it — app-role authorization is a browser-session
// concern, not a pod-to-pod one.
func TestAdminArea_ForbidsHeaderTrustCaller(t *testing.T) {
	rec := httptest.NewRecorder()
	AdminArea(rec, withIdentity(oidcauth.Identity{
		Authenticated: true,
		Source:        "moses-headers",
		Subject:       "pod-caller",
		// Roles intentionally empty — header-trust carries none.
	}))
	if rec.Code != http.StatusForbidden {
		t.Errorf("header-trust caller status = %d, want 403", rec.Code)
	}
}

// PublicInfo must report the OIDC flag and the declared role vocabulary
// without requiring a session.
func TestPublicInfo_ReportsRolesAndFlag(t *testing.T) {
	rec := httptest.NewRecorder()
	PublicInfo(true)(rec, httptest.NewRequest("GET", "/api/v1/public-info", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decode(t, rec)
	if body["oidc_enabled"] != true {
		t.Errorf("oidc_enabled = %v, want true", body["oidc_enabled"])
	}
	if known, _ := body["known_roles"].([]any); len(known) == 0 {
		t.Errorf("known_roles should be non-empty")
	}
}
