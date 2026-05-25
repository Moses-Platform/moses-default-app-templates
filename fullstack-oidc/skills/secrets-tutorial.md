# Declaring Runtime Secrets — fullstack-oidc

External runtime secrets that this template's backend reads (e.g. a signing
key for app-issued JWTs) are declared once in `moses-app.config.json` and
supplied per deployment by a tenant admin via the Apps-page shield modal. The
deploy gate pauses at `awaiting_secrets` until every `required` value is
present, then auto-resumes the Helm phase by reusing the already-built image
— no rebuild on supply.

> **Note.** The OIDC client secret used by this template's BFF flow is
> **platform-managed** and injected via `MOSES_OIDC_*` env vars when
> `access.oidc.mode == "moses-oidc"`. Do NOT declare it under
> `secrets.external[]`. This skill is for **app-owned** secrets such as the
> signing key for tokens the app itself mints downstream.

This template's frontend does NOT read secrets directly — it calls the
backend's protected endpoints, which in turn read the env vars below.

## Reading the env var (Go)

The template's `internal/config.Validate()` already fail-fasts on
`MOSES_TENANT_ID` when `MOSES_DEPLOYED=1`. Extend the same pattern for your
app-owned secrets:

```go
import (
    "log"
    "os"
)

jwtKey := os.Getenv("JWT_SIGNING_KEY")
if jwtKey == "" && os.Getenv("MOSES_DEPLOYED") == "1" {
    log.Fatalf("JWT_SIGNING_KEY env is required when MOSES_DEPLOYED=1 — declare it in moses-app.config.json secrets.external[] and supply via the Apps-page shield modal")
}
```

## Declaring it (`moses-app.config.json`)

Copy the sibling `moses-app.config.with-secrets.example.json` for a full,
diffable example. The relevant block:

```json
{
  "secrets": {
    "external": [
      { "key": "jwt-signing-key", "envVar": "JWT_SIGNING_KEY", "description": "App-issued JWT signing key (RSA)", "generate": { "type": "rsa", "rsaBits": 2048 } },
      { "key": "example-encryption-key", "envVar": "EXAMPLE_ENCRYPTION_KEY", "description": "App-data encryption key", "generate": { "type": "hex", "bytes": 32 } }
    ]
  }
}
```

`required: true` (the default) blocks the deploy until a value is supplied.
`generate` lets the platform produce the value itself — the gate treats those
as satisfied on first deploy and `required` is irrelevant for them. The
platform-generated value **persists across all subsequent deploys**; only an
admin Regenerate button in the shield modal rotates it (running pods keep the
old value until the next deploy). Use `generate` for crypto material the app
uses internally; supply external-system credentials (third-party APIs,
database passwords) by hand instead.

## How to enable

1. Copy the env-var read + fail-fast snippet into your code.
2. Add the matching `secrets.external[]` entry to `moses-app.config.json`.
3. Commit + redeploy. Build succeeds, gate pauses at `awaiting_secrets`.
4. Tenant admin supplies the value via the Apps-page shield modal.
5. Gate auto-resumes the Helm phase via image reuse — no rebuild.

## Values never live in code

The `key`, `envVar`, `description`, and `required` flag describe **what** the
app needs. The **value** is supplied out-of-band by a tenant admin and stored
encrypted in Moses. Never commit a value to git, to `moses-app.config.json`,
or to a Helm values file. The shield modal is the only supported supply path.

---

For the platform-side contract (the `awaiting_secrets` gate, image reuse on resume, the Apps-page shield modal) see `arch.md § App-declared secrets` in the platform repo.
