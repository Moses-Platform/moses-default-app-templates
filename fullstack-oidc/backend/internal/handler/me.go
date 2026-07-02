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
// This is OIDC plumbing, not demo code: the SPA's auth bootstrap
// (frontend useMe/getMe) reads this route to learn whether a session
// exists and who the user is. Keep it when you strip the demo.
//
// resource_access.<client>.roles from the validated ID token are
// exposed on Identity.Roles — in-app authorization decisions (403 vs
// 200) belong in the handlers that need them, checked server-side via
// Identity.HasRole (the demo AdminArea handler in demo_handlers.go
// shows the pattern while present; it is removed by clean_out_template.sh).
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
