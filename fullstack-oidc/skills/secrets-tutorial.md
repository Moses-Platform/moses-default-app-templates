# Declaring Runtime Secrets — fullstack-oidc

This template's backend reads its own **app-owned** runtime secrets — most
commonly a signing key for the JWTs the app itself mints downstream.

The hands-on walkthrough (read the env var, declare `secrets.external[]`, the
5-step enable flow, `generate` vs hand-supplied values) ships as the embedded
**`secrets-tutorial`** agent skill — already on hand in every agent execution
pod. Copy the sibling **`moses-app.config.with-secrets.example.json`** for a
full diffable example (it declares a `generate: { type: rsa }` JWT signing key).

**Template-specific note — do NOT declare the OIDC client secret:**

The OIDC client secret used by this template's BFF flow is **platform-managed**
and injected via `MOSES_OIDC_*` env vars when `access.oidc.mode == "moses-oidc"`.
Do NOT declare it under `secrets.external[]`. The `secrets-tutorial` skill is
for **app-owned** secrets such as the signing key for tokens the app itself
mints — not for the platform-managed OIDC client secret.

The template's `internal/config.Validate()` already fail-fasts on
`MOSES_TENANT_ID` when `MOSES_DEPLOYED=1` — extend that same pattern for your
app-owned secrets.

For the platform-side contract (the `awaiting_secrets` gate, image reuse on
resume, the Apps-page shield modal) see `arch.md § App-declared secrets` in the
platform repo, also surfaced via the embedded **moses-deployment-guide** skill.
