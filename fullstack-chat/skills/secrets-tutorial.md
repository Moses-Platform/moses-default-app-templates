# Declaring Runtime Secrets — fullstack-chat

This template's backend reads its own runtime secrets — most commonly an LLM
API key for chat-prompt fan-out.

The hands-on walkthrough (read the env var, declare `secrets.external[]`, the
5-step enable flow, `generate` vs hand-supplied values) ships as the embedded
**`secrets-tutorial`** agent skill — already on hand in every agent execution
pod. Copy the sibling **`moses-app.config.with-secrets.example.json`** for a
full diffable example.

**Template-specific notes:**

- This template already has `validatePlatformEnv` / `requiredPlatformEnv` in
  `backend/cmd/server/` (CHAT-0b6g). **Extend it** — add a row, don't write a
  second validator:

  ```go
  // In backend/cmd/server/validate_env.go, add to requiredPlatformEnv:
  {"LLM_API_KEY", "LLM_API_KEY — third-party LLM API key for chat fan-out", false},
  ```

- The frontend does NOT read secrets directly — it calls the backend's
  authenticated endpoints, which in turn read the env vars.

- `MOSES_CHAT_WEBHOOK_SECRET`, already required by this template, is
  **platform-injected** — it is NOT a declared external secret. Do not add it to
  `secrets.external[]`. The "never commit a value" rule still applies.

For the platform-side contract (the `awaiting_secrets` gate, image reuse on
resume, the Apps-page shield modal) see `arch.md § App-declared secrets` in the
platform repo, also surfaced via the embedded **moses-deployment-guide** skill.
