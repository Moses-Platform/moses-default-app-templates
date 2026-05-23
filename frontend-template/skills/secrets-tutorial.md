# Declaring Runtime Secrets — frontend-template

**This template has no backend.** It is a static React + Vite + nginx app.

Declared external secrets (`moses-app.config.json` → `secrets.external[]`)
are injected as env vars **into a Linux pod at runtime**. They are NOT shipped
to the browser. JavaScript running in the user's browser cannot read a server
env var; anything baked into a JS bundle by Vite at build time would be
trivially extractable from `view-source` and is therefore unsafe for any
genuine secret material.

If your app needs an API key, JWT signing key, or encryption key, **fork a
fullstack template instead**:

- `fullstack-simple` — split frontend + Go backend, no database
- `fullstack-unified` — single Go binary serving the frontend + an API
- `fullstack-showcase` — full reference with PostgreSQL
- `fullstack-chat` — chat-roundtrip + LLM API integration
- `fullstack-oidc` — OIDC relying-party + app-issued JWTs

Each ships a `skills/secrets-tutorial.md` with the Go snippet, a sibling
`moses-app.config.with-secrets.example.json`, and the read+validate pattern.

For the platform-side contract (the `awaiting_secrets` gate, image reuse on
resume, the Apps-page shield modal) see `arch.md § App-declared secrets` in
the platform repo.
