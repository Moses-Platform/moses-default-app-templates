# `@moses/browser-logger`

Tiny TypeScript snippet that ships in every Moses default app template. It
captures runtime errors, unhandled promise rejections, and (optionally)
`console.error` / `console.warn` output, and posts them to the Moses
platform's browser-log ingest endpoint (BLF-A).

## Install

Add the snippet as the first statement in your app's entry file:

```ts
// src/main.tsx (or wherever your app boots)
import { installBrowserLogger } from "@moses/browser-logger";
installBrowserLogger();

// ...rest of your app
```

The call is fire-and-forget — it returns a `Promise<void>` you can ignore.

## Build-time configuration

The snippet expects three Vite env vars, baked in at build time by Moses's
in-cluster image build service:

| Var                          | Source                                     | Required |
| ---------------------------- | ------------------------------------------ | -------- |
| `VITE_MOSES_CHART_ID`        | The chart this build belongs to (UUID)    | recommended |
| `VITE_MOSES_DEPLOYMENT_ID`   | `agent_pod_executions.id` for this build   | recommended |
| `VITE_MOSES_API_BASE`        | Absolute URL to Moses API (empty = same origin) | no |

When both of the first two are baked in, the snippet bootstraps with
`chart_id` + `deployment_id`. When either is missing (build args not
forwarded, chart-id stripped, or a non-Vite build), the snippet no longer
self-disables: it bootstraps with a `loc=<origin+pathname>` param so the
Moses platform resolves the chart/deployment identity server-side from the
request URL. The per-chart "Browser error logging" toggle still gates the
stream — a chart with logging disabled bootstraps to not-enabled and the
snippet stays silent. Building outside Moses (e.g. a local `vite build` dry
run) is still safe: the bootstrap request just fails/404s and the snippet
goes quiet.

For local dev against a running Moses, set these in `.env.local`:

```
VITE_MOSES_CHART_ID=<chart-uuid>
VITE_MOSES_DEPLOYMENT_ID=<deployment-uuid>
VITE_MOSES_API_BASE=http://localhost
```

## Cache-friendly Dockerfile pattern

In your template's `Dockerfile`, declare these as `ARG` only inside the
final build stage — **not** above `npm install` — so dependency layers stay
cached across builds:

```dockerfile
FROM node:20-alpine AS deps
COPY package.json package-lock.json* ./
RUN npm install

FROM deps AS build
COPY . .
ARG VITE_MOSES_CHART_ID
ARG VITE_MOSES_DEPLOYMENT_ID
ARG VITE_MOSES_API_BASE
RUN VITE_MOSES_CHART_ID=$VITE_MOSES_CHART_ID \
    VITE_MOSES_DEPLOYMENT_ID=$VITE_MOSES_DEPLOYMENT_ID \
    VITE_MOSES_API_BASE=$VITE_MOSES_API_BASE \
    npx vite build
```

## What gets sent

Each event is a small JSON object:

```json
{
  "ts": "2026-04-25T10:30:00.000Z",
  "level": "error",
  "message": "Cannot read properties of undefined",
  "stack": "TypeError: ...",
  "url": "https://app.example.com/dashboard",
  "source_kind": "client_runtime",
  "context": { "filename": "...", "lineno": 42, "colno": 7 }
}
```

Events are batched (up to 50) and flushed every 5s, on `visibilitychange:hidden`,
or on `pagehide`. The fetch uses `keepalive: true` so it survives unload.

## What does NOT get sent

- `Authorization`, `Cookie`, `Set-Cookie`, `X-CSRF-Token` keys are stripped
  from any `context` object before send.
- `message` is clipped to 4 KB; `stack` to 16 KB; `context` JSON to 2 KB.
- Sample rate (server-controlled, default 1.0) gates per-event emission
  client-side.

## Per-chart toggles

Workspace admins control browser logging via chart settings:
- "Browser error logging" — turn the entire stream off without redeploying.
- "PII scrubbing" — server-side regex strip of emails, phone numbers,
  CC-shaped digit runs, and IPv4 addresses from `message` and `stack`.

Bootstrap (`/api/v1/browser-logs/bootstrap`) reads the toggles on every
page load, so flipping the switch takes effect on the next visit.

## Vanilla-JS port (BLF-J)

The `fullstack-unified` template is plain Go + ES5+ JavaScript with no
bundler, so it cannot consume this TypeScript module. It ships an inline
port at
[`fullstack-unified/static/moses-browser-logger.js`](../../fullstack-unified/static/moses-browser-logger.js)
that mirrors this snippet's behaviour and config surface 1:1, but reads
its config from a server-rendered `<meta name="moses-config">` tag
populated by `main.go` from the `MOSES_CHART_ID` / `MOSES_DEPLOYMENT_ID`
/ `MOSES_API_BASE` env vars. The two snippets stay in sync — change one,
review the other.

## Vendored copies (byte-identical contract)

`shared/browser-logger/index.ts` is the **canonical source**. It is NOT
imported across packages — each React template vendors a **byte-identical
copy** at `<template>/frontend/src/moses-browser-logger.ts`:

- `frontend-template`, `fullstack-simple`, `fullstack-chat`,
  `fullstack-showcase`, `fullstack-oidc`.

There is no symlink or bundler alias. **To change the logger: edit
`shared/browser-logger/index.ts`, then re-copy it over all five vendored
copies** (do NOT edit a copy directly). The vanilla-JS port
(`fullstack-unified/static/moses-browser-logger.js`) is a separate
language port — keep it behaviourally in sync by hand (see above).

A CI gate enforces the byte-identity:
[`tools/check-vendored-browser-logger.sh`](../../tools/check-vendored-browser-logger.sh)
(wired via `.github/workflows/vendored-browser-logger-drift.yml`) fails the
build if any vendored TS copy drifts from the canonical source. Run it
locally after editing:

```sh
tools/check-vendored-browser-logger.sh
```
