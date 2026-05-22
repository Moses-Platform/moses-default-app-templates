# Using the vendored `oidcauth` middleware — Agent Skill

The `oidcauth` package lives at `backend/internal/oidcauth/`. It is
**vendored**: copy the whole directory into a template's
`backend/internal/oidcauth/` and import it from that template's own
module path. It has **no third-party Go dependencies** — RSA/JWT/JWKS
verification is built on the standard library, like `mosesproxy-go`.

## End-to-end: declare → inject → enforce → read

App-owned OIDC is a four-link chain. As an agent adding OIDC to an app,
you touch exactly two of the links (declare + enforce); the platform
owns the inject link, and your handlers do the read.

### 1. Declare — `moses-app.config.json`

```json
{
  "access": {
    "mode": "tenant",
    "roles": ["oidc-admin", "oidc-member"],
    "oidc": {
      "mode": "moses-oidc",
      "protectedPaths": ["/api/v1/me", "/api/v1/entries", "/api/v1/admin-area"],
      "publicPaths": ["/api/v1/public-info", "/health", "/api/openapi.json"]
    }
  }
}
```

- `access.oidc.mode: "moses-oidc"` opts the app in. Omit the whole
  `oidc` block (or set `mode: "none"`) and the app behaves exactly as
  before — OIDC is strictly opt-in.
- `access.roles` declares the role *vocabulary* the app understands. A
  tenant admin maps real users into these names; Moses projects the
  mapping into `resource_access.<client>.roles` on the token. Role
  names must be lowercase alnum/hyphen, 1–64 chars.
- `/health` and the OpenAPI spec are public implicitly — listing them in
  `publicPaths` is harmless and self-documenting.

### 2. Inject — the Moses platform (you do nothing)

At deploy time the platform registers a confidential Keycloak client and
injects `MOSES_OIDC_*` env vars + a K8s Secret. The Helm chart's
existing `moses.oidc` values + `oidc-secret.yaml` Secret consume them —
see `helm/values.yaml` and `helm/templates/oidc-secret.yaml`.

### 3. Enforce — wire the middleware in `main.go`

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

### 4. Read — the authenticated identity in a handler

```go
func MyHandler(w http.ResponseWriter, r *http.Request) {
    id := oidcauth.IdentityFrom(r.Context())
    // id.Subject, id.Email, id.Name, id.Username
    // id.Roles  — resource_access.<client>.roles from the ID token
    // id.Source — "session" or "moses-headers"
}
```

`id.Source` is `"session"` (validated cookie) or `"moses-headers"`
(trusted pod-to-pod call). The header path carries **no roles** —
pod-to-pod callers are authorized by the platform, not by app roles.

## Role-gating a route (authorization)

Authentication (the middleware) and authorization (your handler) are
separate steps. The middleware proves *who*; the handler enforces *what
may they do*. The reference app's `handler.AdminArea` is the model:

```go
const roleAdmin = "oidc-admin" // must match an access.roles entry

func AdminArea(w http.ResponseWriter, r *http.Request) {
    id := oidcauth.IdentityFrom(r.Context())
    if !id.HasRole(roleAdmin) {
        w.WriteHeader(http.StatusForbidden) // 403, NOT 401
        // ... explain which role is required ...
        return
    }
    // ... serve the admin-only payload ...
}
```

A signed-in user without the role gets **403** (authenticated but not
authorized), distinct from the middleware's **401** (not authenticated
at all). Test role logic without a live Keycloak via the exported
`oidcauth.ContextWithIdentity` helper — see `handler/me_test.go`.

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

## Silent SSO (the embedded-iframe case)

`/auth/silent-check` runs an authorization request with `prompt=none`.
The frontend helper `frontend/src/auth/silentSSO.ts` drives a hidden
iframe against it: an already-logged-into-Moses user is authenticated
with no login page; a not-logged-in user yields a benign
`?silent_sso=failed` marker (never an error). Fall back to
`/auth/login` when silent SSO reports `authenticated: false`.

## SameSite

The session cookie is `SameSite=Lax`. Moses embeds apps **same-site**
(under `/apps/<tenant>/<slug>/` on the Manager host), so Lax cookies are
sent in the iframe AND on a standalone custom hostname. `None` would add
CSRF surface and force `Secure`; `Strict` would drop the cookie on the
post-login redirect back from Keycloak. See the doc comment on
`sessionSameSite` in `session.go` for the full rationale.

## Security properties

- ID tokens: RS256/384/512 only — `alg:none` and HMAC algs rejected
  (closes the classic JWT alg-confusion hole).
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
`go test ./internal/handler/...` — the role-gating logic of `/me` and
`/admin-area` using the `oidcauth.ContextWithIdentity` test helper.
