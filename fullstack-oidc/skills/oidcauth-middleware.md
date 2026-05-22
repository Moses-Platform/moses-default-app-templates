# Using the vendored `oidcauth` middleware — Agent Skill

The `oidcauth` package lives at `backend/internal/oidcauth/`. It is
**vendored**: copy the whole directory into a template's
`backend/internal/oidcauth/` and import it from that template's own
module path. It has **no third-party Go dependencies** — RSA/JWT/JWKS
verification is built on the standard library, like `mosesproxy-go`.

## Wiring it in `main.go`

```go
import "github.com/<your-module>/internal/oidcauth"

func main() {
    oidcCfg := oidcauth.ConfigFromEnv() // reads MOSES_OIDC_* env
    if err := oidcCfg.Validate(); err != nil {
        log.Printf("WARN: %v — running without OIDC enforcement", err)
    }
    auth := oidcauth.New(oidcCfg)

    mux := http.NewServeMux()
    // ... register your routes ...

    // OIDCAuth is wrapped INSIDE MosesHeaders so the header-trust
    // decision sees the X-Moses-* headers. It owns /auth/* and gates
    // protected routes before the mux runs.
    var h http.Handler = mux
    h = auth.Handler(h)
    h = middleware.MosesHeaders(h)
    // ... CORS, Logging ...
    http.ListenAndServe(":8080", h)
}
```

## Reading the authenticated identity in a handler

```go
func MyHandler(w http.ResponseWriter, r *http.Request) {
    id := oidcauth.IdentityFrom(r.Context())
    if !id.Authenticated {
        // The middleware already gates protected paths, so this is
        // only reachable on a route you did NOT mark protected.
    }
    // id.Subject, id.Email, id.Name, id.Username
    // id.Roles  — resource_access.<client>.roles from the ID token
    if id.HasRole("admin") { /* in-app authorization */ }
}
```

`id.Source` is `"session"` (validated cookie) or `"moses-headers"`
(trusted pod-to-pod call). The header path carries no roles — pod-to-pod
callers are authorized by the platform.

## Routes the middleware serves itself

`/auth/login`, `/auth/callback`, `/auth/logout`, `/auth/silent-check`
(all relative to `MOSES_BASE_PATH`). Do NOT register these in your mux.
In a multi-container template the frontend nginx must proxy `/auth/` to
the backend — see `frontend/nginx.conf` + `frontend/entrypoint.sh`.

## Config knobs

`ConfigFromEnv()` reads: `MOSES_OIDC_ISSUER`,
`MOSES_OIDC_INTERNAL_ISSUER`, `MOSES_OIDC_CLIENT_ID`,
`MOSES_OIDC_CLIENT_SECRET`, `MOSES_OIDC_AUDIENCE`,
`MOSES_OIDC_PROTECTED_PATHS`, `MOSES_OIDC_PUBLIC_PATHS`,
`MOSES_BASE_PATH`, `MOSES_OIDC_COOKIE_SECRET`,
`MOSES_OIDC_INSECURE_COOKIE`.

- **Pass-through mode**: when issuer/client/secret are absent,
  `Config.Enabled()` is false. Public routes + the header-trust path
  still work; browser requests are NOT redirected. A misconfigured
  deploy degrades visibly instead of hard-500ing.
- **`protectedPaths` empty** ⇒ deny-by-default (everything not public
  is protected). Non-empty ⇒ only the listed prefixes are gated.
- **`MOSES_OIDC_INSECURE_COOKIE=1`** drops the cookie `Secure` flag —
  local plain-HTTP dev ONLY, never on a real deploy.

## SameSite

The session cookie is `SameSite=Lax`. Moses embeds apps **same-site**
(under `/apps/<tenant>/<slug>/` on the Manager host), so Lax cookies are
sent in the iframe AND on a standalone custom hostname. `None` would add
CSRF surface and force `Secure`; `Strict` would drop the cookie on the
post-login redirect back from Keycloak. See the doc comment on
`sessionSameSite` in `session.go` for the full rationale.

## Security properties

- ID tokens: RS256/384/512 only — `alg:none` and HMAC algs rejected
  (closes the JWT alg-confusion hole).
- JWKS fetched from the **internal** issuer (in-cluster), `iss`
  validated against the **external** issuer.
- Session cookie is HMAC-signed (tamper-proof), HttpOnly (BFF: no token
  in the browser), Path-scoped to the deploy sub-path.
- PKCE `S256` always; `plain` never offered.
- `state` CSRF nonce verified constant-time on callback.
- Session expiry is capped to the sooner of the token `exp` and 8h.

## Tests

`go test ./internal/oidcauth/...` — path matching, dual-mode decision,
cookie attributes, config parsing, JWT verification (with an in-test
RSA signer), and the handshake handlers all run without a live Keycloak.
