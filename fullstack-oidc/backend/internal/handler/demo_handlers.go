// DEMO FILE — deleted by clean_out_template.sh. The role-gated and
// public demo handlers live here so the plumbing handlers (me.go,
// health.go, openapi.go) survive the clean-out untouched.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/moses-platform/fullstack-oidc/internal/oidcauth"
)

// roleAdmin is the app-declared role required by the admin-only demo
// route. It MUST match an entry in moses-app.config.json `access.roles`
// — that vocabulary is what a tenant admin maps real users into, and
// Moses projects the mapping into `resource_access.<client>.roles` on
// the ID token, where the oidcauth middleware lifts it onto
// Identity.Roles.
const roleAdmin = "oidc-admin"

// AdminArea is a ROLE-GATED protected route. OIDCAuth proves the caller
// is authenticated; this handler additionally enforces that the
// validated token carries the app's `oidc-admin` role. A signed-in user
// WITHOUT the role gets 403 — demonstrating that authentication
// (who you are) and authorization (what you may do) are separate steps,
// and that the second one is driven by the projected token roles.
//
// The header-trust (pod-to-pod) path carries only the roles a tenant admin
// explicitly granted to AGENT calls (Moses shield modal → Agents toggle,
// delivered as X-Moses-Roles under the gateway-auth marker). An agent call
// with no agent role grant reaches here un-roled and is correctly 403'd.
func AdminArea(w http.ResponseWriter, r *http.Request) {
	id := oidcauth.IdentityFrom(r.Context())
	w.Header().Set("Content-Type", "application/json")

	if !id.HasRole(roleAdmin) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":         "forbidden",
			"required_role": roleAdmin,
			"your_roles":    id.Roles,
			"detail": "You are signed in, but this route requires the " +
				roleAdmin + " app role. A tenant admin grants it by " +
				"mapping your user to this app's role in Moses.",
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":            true,
		"message":       "Admin-only area — your token carries the " + roleAdmin + " role.",
		"subject":       id.Subject,
		"granted_role":  roleAdmin,
		"current_roles": id.Roles,
	})
}

// PublicInfo is an always-public endpoint (listed in the app's
// access.oidc publicPaths). It reports whether OIDC enforcement is
// active without requiring a session — so an anonymous viewer can SEE
// the app's auth posture before signing in.
func PublicInfo(oidcEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"app":          "fullstack-oidc",
			"oidc_enabled": oidcEnabled,
			// known_roles echoes the app's declared role vocabulary so
			// the public landing page can explain authorization before
			// the user signs in.
			"known_roles": []string{roleAdmin, "oidc-member"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
