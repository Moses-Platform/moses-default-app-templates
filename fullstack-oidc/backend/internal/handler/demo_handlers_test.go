// DEMO TEST FILE — deleted by clean_out_template.sh together with
// demo_handlers.go. Shared helpers (withIdentity, decode) live in
// me_test.go, which survives the clean-out.
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moses-platform/fullstack-oidc/internal/oidcauth"
)

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

// The pod-to-pod header-trust path carries only explicitly-granted agent
// roles, so AdminArea correctly 403s an un-roled header-trust caller.
func TestAdminArea_ForbidsHeaderTrustCaller(t *testing.T) {
	rec := httptest.NewRecorder()
	AdminArea(rec, withIdentity(oidcauth.Identity{
		Authenticated: true,
		Source:        "moses-headers",
		Subject:       "pod-caller",
		// Roles intentionally empty — no agent role grant.
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
