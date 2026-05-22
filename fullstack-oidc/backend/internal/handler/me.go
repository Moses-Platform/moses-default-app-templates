package handler

import (
	"encoding/json"
	"net/http"

	"github.com/moses-platform/fullstack-oidc/internal/oidcauth"
)

// Me returns the authenticated principal. The OIDCAuth middleware has
// already gated this route (it is protected), so an unauthenticated
// caller never reaches here — but the handler still reads the Identity
// out of the request context to demonstrate the contract.
//
// resource_access.<client>.roles from the validated ID token are
// exposed on Identity.Roles for in-app authorization decisions.
func Me(w http.ResponseWriter, r *http.Request) {
	id := oidcauth.IdentityFrom(r.Context())

	resp := map[string]any{
		"authenticated": id.Authenticated,
		"source":        id.Source,
		"subject":       id.Subject,
		"email":         id.Email,
		"name":          id.Name,
		"username":      id.Username,
		"roles":         id.Roles,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// PublicInfo is an always-public endpoint (listed in the app's
// access.oidc publicPaths). It reports whether OIDC enforcement is
// active without requiring a session.
func PublicInfo(oidcEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"app":          "fullstack-oidc",
			"oidc_enabled": oidcEnabled,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
