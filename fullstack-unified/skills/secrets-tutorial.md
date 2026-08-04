# Declaring Runtime Secrets

This template's backend may need its own runtime secrets — a third-party API
key, an app-data encryption key, an app-issued JWT signing key.

**The hands-on walkthrough now ships as a first-class embedded agent skill,
`secrets-tutorial`** (always loaded into agent execution pods). It covers:

- reading the env var in Go (extend `validatePlatformEnv` / fail-fast pattern),
- the `secrets.external[]` block + `generate` vs hand-supplied values,
- the 5-step enable flow (declare → redeploy → `awaiting_secrets` gate →
  admin supplies via the Apps-page shield modal → auto-resume via image reuse).

Copy the sibling **`moses-app.config.with-secrets.example.json`** for a full
diffable example.

For the platform-side contract (the `awaiting_secrets` gate, image reuse on
resume, the shield modal) see `arch.md § App-declared secrets` in the platform
repo, also surfaced via the embedded **moses-deployment-guide** skill.
