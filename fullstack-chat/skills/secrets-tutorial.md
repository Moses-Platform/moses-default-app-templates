---
name: secrets-tutorial
description: How to declare and read runtime secrets (e.g. an LLM API key) in this template
mode: reference
priority: optional
---

# Declaring Runtime Secrets — fullstack-chat

External runtime secrets that this template's backend reads (e.g. an LLM API
key for chat-prompt fan-out) are declared once in `moses-app.config.json` and
supplied per deployment by a tenant admin via the Apps-page shield modal. The
deploy gate pauses at `awaiting_secrets` until every `required` value is
present, then auto-resumes the Helm phase by reusing the already-built image
— no rebuild on supply.

This template's frontend does NOT read secrets directly — it calls the
backend's authenticated endpoints, which in turn read the env vars below.

## Reading the env var (Go)

The template already has `validatePlatformEnv` in `backend/cmd/server/`
(CHAT-0b6g) — extend it instead of writing a new validator. The shape is the
same: a `requiredPlatformEnv` row + a startup-fatal check when MOSES_DEPLOYED=1.

```go
// In backend/cmd/server/validate_env.go, add to requiredPlatformEnv:
{"LLM_API_KEY", "LLM_API_KEY — third-party LLM API key for chat fan-out", false},

// Read it from a handler / service at request time:
apiKey := os.Getenv("LLM_API_KEY")
if apiKey == "" {
    http.Error(w, "llm not configured", http.StatusServiceUnavailable)
    return
}
```

## Declaring it (`moses-app.config.json`)

Copy the sibling `moses-app.config.with-secrets.example.json` for a full,
diffable example. The relevant block:

```json
{
  "secrets": {
    "external": [
      { "key": "llm-api-key", "envVar": "LLM_API_KEY", "description": "LLM provider API key", "required": true },
      { "key": "example-encryption-key", "envVar": "EXAMPLE_ENCRYPTION_KEY", "description": "App-data encryption key", "generate": { "type": "hex", "bytes": 32 } }
    ]
  }
}
```

`required: true` (the default) blocks the deploy until a value is supplied.
`generate` lets the platform produce the value itself — the gate treats those
as satisfied on first deploy and `required` is irrelevant for them. The
platform-generated value **persists across all subsequent deploys of the same
track**: dev and stable each get their own independent value by default
(security-first — a dev-env compromise does NOT expose the stable-env value).
Set `generate.sharedAcrossTracks: true` only for app-issued tokens that must
validate across a dev→stable promotion (rare). Only an admin Regenerate
button in the shield modal rotates a value (running pods keep the old value
until the next deploy). Use `generate` for crypto material the app uses
internally; supply external-system credentials (third-party APIs, database
passwords) by hand instead.

## How to enable

1. Copy the env-var read into your handler and add the row to `requiredPlatformEnv`.
2. Add the matching `secrets.external[]` entry to `moses-app.config.json`.
3. Commit + redeploy. Build succeeds, gate pauses at `awaiting_secrets`.
4. Tenant admin supplies the value via the Apps-page shield modal.
5. Gate auto-resumes the Helm phase via image reuse — no rebuild.

## Values never live in code

The `key`, `envVar`, `description`, and `required` flag describe **what** the
app needs. The **value** is supplied out-of-band by a tenant admin and stored
encrypted in Moses. Never commit a value to git, to `moses-app.config.json`,
or to a Helm values file. The shield modal is the only supported supply path.
The `MOSES_CHAT_WEBHOOK_SECRET` already required by this template is
**platform-injected** (not a declared external secret) — same rule applies.

---

For the platform-side contract (the `awaiting_secrets` gate, image reuse on resume, the Apps-page shield modal) see `arch.md § App-declared secrets` in the platform repo.
