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

## How it reaches Moses

At boot the snippet calls `/api/v1/browser-logs/bootstrap?loc=<origin+pathname>`
with `credentials: "same-origin"`. The platform finds the app from `loc` and
applies the app's own access decision to the caller, exactly as for opening
the app:

- a signed-in viewer of a workspace or restricted app gets a token bound to
  that app, its track and that viewer; an outsider gets nothing;
- a public app's visitors get a token anonymously;
- logging disabled, an unknown app, or any refusal: the snippet stays silent.

The request is relative, so it works on the Moses origin
(`/apps/<tenant>/<slug>/`) and on the separate apps origin alike: there the
apps listener forwards `/api/v1/browser-logs/*` to Moses with the viewer's
app pass. No credential is ever sent cross-origin.

Tokens are short-lived (about 1 h). An ingest `401` bootstraps again, at most
once a minute, and resends the batch (bounded to 50 events). A `401` sooner
than a minute after a bootstrap means the token is refused for good; the
snippet then goes silent instead of looping.

The platform build still bakes `VITE_MOSES_CHART_ID` / `VITE_MOSES_DEPLOYMENT_ID`;
the snippet ignores them. The only build-time setting is optional:

| Var                   | Meaning                                                     |
| --------------------- | ----------------------------------------------------------- |
| `VITE_MOSES_API_BASE` | Origin of the Moses API. Leave empty (same origin) on Moses. |

Set it only for an app you host yourself against a Moses elsewhere: a
cross-origin bootstrap carries no viewer session, so only a public app gets a
token that way. Building outside Moses is safe: bootstrap fails and the
snippet goes quiet.

## Why a vendored copy and not only the gateway's

The Envoy gateway injects the platform's copy of this logic
(`<script src="/api/v1/browser-logs/client.js" defer>`) into app pages, but
not into every page: not into public apps, preview tracks, apps on their own
hostname, Traefik or ingress-nginx installs, or a gateway without the
injector. The vendored copy covers those. When both load, the first install
wins (`window.__mosesBrowserLoggerInstalled`), so nothing is reported twice.

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
page load and on every re-bootstrap, so flipping the switch takes effect on
the next visit (or within the hour for an open tab).

## Vanilla-JS port

The `fullstack-unified` template is plain Go + ES5+ JavaScript with no
bundler, so it cannot consume this TypeScript module. It ships an inline
port at
[`fullstack-unified/static/moses-browser-logger.js`](../../fullstack-unified/static/moses-browser-logger.js)
that mirrors this snippet's behaviour 1:1. Its body is the same code as the
platform's injected client (`moses-platform-prep`
`backend/internal/api/browser_logger_client.js`); keep the three in sync by
hand. It needs no server-rendered config. An optional
`<meta name="moses-config" data-api-base="…">` sets the API origin, like
`VITE_MOSES_API_BASE` above.

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
