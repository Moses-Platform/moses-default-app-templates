// Copyright (c) 2025-2026 Siemer Industries.
// SPDX-License-Identifier: BSL-1.1
//
// Moses browser-logger snippet (BLF-B).
//
// One-line install in a deployed-app entry point:
//
//     import { installBrowserLogger } from "@moses/browser-logger";
//     installBrowserLogger();
//
// Bootstrap: at boot the snippet asks the platform for its config (enabled
// flag, ingest token, sample rate) at `/api/v1/browser-logs/bootstrap?loc=…`,
// where `loc` is the page's origin + pathname. The platform finds the app from
// `loc` and applies the app's own access decision to the caller, exactly as
// for opening the app. The request is RELATIVE and uses credentials
// "same-origin": on the Moses origin (`/apps/<tenant>/<slug>/`) and on the
// apps origin alike it reaches the platform with the viewer's session (the
// Moses cookie or the apps-origin app pass), and it never sends a credential
// cross-origin. A public app's visitors get a token anonymously. Disabled,
// refused or unknown → the snippet silently no-ops.
//
// The token is short-lived (about 1 h) and bound to app + track + viewer. An
// ingest 401 bootstraps again — at most once a minute — and resends the
// batch; a 401 sooner than that means the token is refused for good, so the
// snippet goes silent instead of looping.
//
// VITE_MOSES_API_BASE (optional, build time) points bootstrap and ingest at
// another origin. Leave it empty on Moses: a cross-origin bootstrap carries no
// viewer session, so only public apps would get a token.
//
// The gateway may also inject the platform's copy of this logic
// (`/api/v1/browser-logs/client.js`); the install latch below makes whichever
// runs first the only one.
//
// CRASH-CAPTURE ORDERING (CHAT-il3y6): bootstrap is async (`await fetch`), but
// the most valuable errors — a render-time crash in the synchronous code that
// runs right after `installBrowserLogger()` is called — fire window.onerror
// BEFORE that fetch resolves. To avoid losing them, the window 'error' and
// 'unhandledrejection' listeners are attached SYNCHRONOUSLY at the very top of
// installBrowserLogger, before the await; their handlers push normalized
// events into a bounded pre-bootstrap buffer. Once bootstrap resolves we either
// drain that buffer through the single flush owner (enabled) or drop it
// (disabled / bootstrap error). The console.error/console.warn patching stays
// POST-bootstrap because it depends on the fetched config (patchConsole /
// sampleRate). That split is intentional and load-bearing — do not move the
// console patch up, and do not attach a second set of window listeners after
// bootstrap (that would double-count every event).

type Level = "error" | "warn" | "info" | "log";

type BootstrapConfig = {
  enabled: boolean;
  endpoint?: string;
  token?: string;
  sampleRate?: number;
};

interface QueuedEvent {
  ts: string;
  level: Level;
  message: string;
  stack?: string;
  url?: string;
  source_kind: string;
  context?: Record<string, unknown>;
}

export interface InstallOptions {
  /** Patch console.error / console.warn to also push events. Default true. */
  patchConsole?: boolean;
  /** Override the server-side sample rate. 0..1. */
  sampleRate?: number;
}

const FORBIDDEN_HEADERS = /^(authorization|cookie|set-cookie|x-csrf-token)$/i;
const MAX_MESSAGE_BYTES = 4_000;
const MAX_STACK_BYTES = 16_000;
const MAX_CONTEXT_BYTES = 2_000;
const QUEUE_FLUSH_THRESHOLD = 50;
const QUEUE_FLUSH_INTERVAL_MS = 5_000;
// Bound the pre-bootstrap buffer so a crash-loop that fires before bootstrap
// resolves can't grow memory without limit. Drop-oldest once the cap is hit.
const PREBOOTSTRAP_BUFFER_CAP = 50;
// An ingest 401 bootstraps again only if the last bootstrap is this old.
const REBOOTSTRAP_MIN_INTERVAL_MS = 60_000;
// An absolute URL's query and fragment (OAuth code/state, tokens, emails),
// keeping a trailing `:line[:column]` as stack frames carry it.
const URL_QUERY_OR_FRAGMENT = /(https?:\/\/[^\s?#)'"]+)[?#][^\s)'"]*?((?::\d+){1,2})?(?=[\s)'"]|$)/g;

export async function installBrowserLogger(opts: InstallOptions = {}): Promise<void> {
  // Idempotency latch: guarantee a single install even if this runs twice
  // (bundled snippet + gateway-injected client.js coexisting, or a double
  // import). Must precede the synchronous window-listener attach below so the
  // second call attaches no duplicate listeners. Guarded on `window` so the
  // no-window path (SSR / tests without a DOM) still runs.
  if (typeof window !== "undefined") {
    if ((window as any).__mosesBrowserLoggerInstalled) return;
    (window as any).__mosesBrowserLoggerInstalled = true;
  }

  // import.meta.env is the Vite-injected env object. Use a guarded read so
  // this file also compiles under bundlers that don't define it (the base is
  // then empty: same origin).
  const apiBase = readViteEnv().VITE_MOSES_API_BASE || "";
  const bootstrapUrl =
    apiBase +
    "/api/v1/browser-logs/bootstrap?loc=" +
    encodeURIComponent(
      typeof window !== "undefined"
        ? window.location.origin + window.location.pathname
        : ""
    );
  let lastBootstrapAt = 0;

  // --- Hoisted state (must outlive the await) -------------------------------
  // `queue` collects events both before AND after bootstrap. Before bootstrap
  // (and while a re-bootstrap runs) it is a bounded buffer; after bootstrap it
  // is drained by the flush owner. `dropped` flips true when bootstrap reports
  // disabled or errors — at that point the buffer is cleared and enqueue()
  // becomes a silent no-op.
  let queue: QueuedEvent[] = [];
  let dropped = false;
  // Post-bootstrap config. Null until (and unless) bootstrap enables logging.
  let ingestEndpoint = "";
  let token = "";
  let sampleRate = 1.0;
  let flushReady = false;
  let flushScheduled = false;
  // Bumped on every applied bootstrap: a 401 for a batch sent with an older
  // token says nothing about the current one.
  let tokenGeneration = 0;
  // The one re-bootstrap in flight; concurrent 401s wait for it.
  let rebootstrapping: Promise<void> | null = null;

  function enqueue(
    level: Level,
    message: string,
    stack?: string,
    extra?: Record<string, unknown>
  ): void {
    if (dropped) return;
    // Sampling only applies once we have a config; before bootstrap we keep
    // everything (bounded) so a pre-bootstrap crash is never sampled away.
    if (flushReady && sampleRate < 1.0 && Math.random() > sampleRate) return;
    queue.push({
      ts: new Date().toISOString(),
      level,
      message: clip(stripUrlQueries(String(message)), MAX_MESSAGE_BYTES),
      stack: stack ? clip(stripUrlQueries(stack), MAX_STACK_BYTES) : undefined,
      url: typeof location !== "undefined" ? location.origin + location.pathname : undefined,
      source_kind: "client_runtime",
      context: stripContextUrls(sanitize(extra)),
    });
    if (!flushReady) {
      // (Re-)bootstrap pending: bound the buffer, drop-oldest. No timers/flush yet.
      while (queue.length > PREBOOTSTRAP_BUFFER_CAP) {
        queue.shift();
      }
      return;
    }
    if (queue.length >= QUEUE_FLUSH_THRESHOLD) {
      flush();
    } else if (!flushScheduled) {
      flushScheduled = true;
      setTimeout(flush, QUEUE_FLUSH_INTERVAL_MS);
    }
  }

  function flush(): void {
    flushScheduled = false;
    if (!flushReady) return;
    if (queue.length === 0) return;
    const events = queue.splice(0, queue.length);
    const sentWith = tokenGeneration;
    try {
      void fetch(ingestEndpoint, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: "Bearer " + token,
        },
        body: JSON.stringify({ events }),
        keepalive: true,
        credentials: "omit",
      })
        .then((res) => {
          // The token expired (or its key was rotated out): bootstrap again
          // and resend this batch, bounded like the pre-bootstrap buffer.
          if (res && res.status === 401) {
            queue = events.concat(queue).slice(-PREBOOTSTRAP_BUFFER_CAP);
            if (sentWith !== tokenGeneration) {
              // Sent with an older token; a fresh one is already here.
              if (flushReady) flush();
              return;
            }
            void rebootstrap();
          }
        })
        .catch(() => {
          // Network or CORS failure — drop events. Logging here would
          // recurse via the patched console, so we swallow silently.
        });
    } catch {
      // fetch may throw synchronously in degenerate environments; ignore.
    }
  }

  function goSilent(): void {
    dropped = true;
    queue.length = 0;
  }

  // fetchBootstrap resolves to an enabled config, or null (disabled, refused,
  // unknown app, network error).
  async function fetchBootstrap(): Promise<BootstrapConfig | null> {
    lastBootstrapAt = Date.now();
    try {
      const res = await fetch(bootstrapUrl, { credentials: "same-origin" });
      if (!res.ok) return null;
      const cfg = (await res.json()) as BootstrapConfig;
      if (!cfg || !cfg.enabled || !cfg.token || !cfg.endpoint) return null;
      return cfg;
    } catch {
      return null;
    }
  }

  function applyBootstrap(cfg: BootstrapConfig): void {
    tokenGeneration++;
    ingestEndpoint = absolutize(cfg.endpoint ?? "", apiBase);
    sampleRate = clampSampleRate(opts.sampleRate ?? cfg.sampleRate ?? 1.0);
    token = cfg.token ?? "";
  }

  function rebootstrap(): Promise<void> {
    // Several batches refused at once share ONE re-bootstrap; each re-queued
    // its events, so the flush after it resends them all.
    if (rebootstrapping) return rebootstrapping;
    flushReady = false;
    if (Date.now() - lastBootstrapAt < REBOOTSTRAP_MIN_INTERVAL_MS) {
      // Refused by a token this fresh: it will not get better. Go silent.
      goSilent();
      return Promise.resolve();
    }
    rebootstrapping = fetchBootstrap().then((cfg) => {
      rebootstrapping = null;
      if (!cfg) {
        goSilent();
        return;
      }
      applyBootstrap(cfg);
      flushReady = true;
      if (queue.length > 0) flush();
    });
    return rebootstrapping;
  }

  // Window error surface ------------------------------------------------------
  // Attached SYNCHRONOUSLY, before the bootstrap await, so a render-time crash
  // that fires window.onerror in the same tick as installBrowserLogger() is
  // captured into the bounded pre-bootstrap buffer rather than lost. This is
  // the single set of window listeners; do not add another after bootstrap.
  if (typeof window !== "undefined") {
    window.addEventListener("error", (e: ErrorEvent) => {
      const stack = e.error && typeof e.error === "object" && "stack" in e.error
        ? String((e.error as { stack?: unknown }).stack ?? "")
        : undefined;
      enqueue("error", e.message || "Unknown error", stack, {
        filename: e.filename,
        lineno: e.lineno,
        colno: e.colno,
      });
    });

    window.addEventListener("unhandledrejection", (e: PromiseRejectionEvent) => {
      const reason = e.reason;
      const message =
        reason && typeof reason === "object" && "message" in reason
          ? String((reason as { message?: unknown }).message ?? reason)
          : String(reason);
      const stack =
        reason && typeof reason === "object" && "stack" in reason
          ? String((reason as { stack?: unknown }).stack ?? "")
          : undefined;
      enqueue("error", "Unhandled promise rejection: " + message, stack);
    });
  }

  const cfg = await fetchBootstrap();
  if (!cfg) {
    // Disabled, refused or failed — drop the buffered events and go silent.
    goSilent();
    return;
  }

  // --- Bootstrap succeeded: wire up the flush owner -------------------------
  applyBootstrap(cfg);
  const patchConsole = opts.patchConsole ?? true;

  // Console patching ---------------------------------------------------------
  // Stays POST-bootstrap: it depends on the fetched config (patchConsole /
  // sampleRate). The window listeners above are the only thing that had to
  // move before the await.
  if (patchConsole && typeof console !== "undefined") {
    const origErr = console.error.bind(console);
    console.error = (...args: unknown[]): void => {
      try {
        enqueue("error", args.map(stringifyArg).join(" "));
      } catch {
        /* ignore */
      }
      origErr(...args);
    };
    const origWarn = console.warn.bind(console);
    console.warn = (...args: unknown[]): void => {
      try {
        enqueue("warn", args.map(stringifyArg).join(" "));
      } catch {
        /* ignore */
      }
      origWarn(...args);
    };
  }

  // Best-effort flush on hide/unload — `keepalive: true` lets the request
  // outlive the page.
  if (typeof document !== "undefined") {
    document.addEventListener("visibilitychange", () => {
      if (document.visibilityState === "hidden") flush();
    });
  }
  if (typeof window !== "undefined") {
    window.addEventListener("pagehide", flush);
  }

  // Open the flush gate and DRAIN the pre-bootstrap buffer in a single send.
  // Done last so the listeners/console-patch above are fully wired before any
  // buffered crash is flushed.
  flushReady = true;
  if (queue.length > 0) {
    flush();
  }
}

// --- Internal helpers ------------------------------------------------------

interface ViteEnvShape {
  VITE_MOSES_API_BASE?: string;
}

function readViteEnv(): ViteEnvShape {
  // Use a typed indirect access so non-Vite bundlers see an empty object.
  try {
    const meta = (import.meta as unknown) as { env?: ViteEnvShape };
    return meta.env ?? {};
  } catch {
    return {};
  }
}

/** Drops the query and fragment of every absolute URL in s. */
export function stripUrlQueries(s: string): string {
  return s.replace(URL_QUERY_OR_FRAGMENT, "$1$2");
}

function stripContextUrls(
  ctx: Record<string, unknown> | undefined
): Record<string, unknown> | undefined {
  if (!ctx) return ctx;
  for (const k of Object.keys(ctx)) {
    const v = ctx[k];
    if (typeof v === "string") ctx[k] = stripUrlQueries(v);
  }
  return ctx;
}

function clip(s: string, max: number): string {
  return s.length <= max ? s : s.slice(0, max);
}

function clampSampleRate(n: number): number {
  if (typeof n !== "number" || isNaN(n)) return 1.0;
  if (n < 0) return 0;
  if (n > 1) return 1;
  return n;
}

function absolutize(endpoint: string, apiBase: string): string {
  if (/^https?:\/\//i.test(endpoint)) return endpoint;
  if (!apiBase) return endpoint; // same origin
  // apiBase may end with '/'; endpoint may start with '/'. Normalise.
  const left = apiBase.replace(/\/+$/, "");
  const right = endpoint.startsWith("/") ? endpoint : "/" + endpoint;
  return left + right;
}

/**
 * sanitize is exported for unit tests in downstream templates. It strips
 * forbidden header-shaped keys and clips the JSON body length to keep the
 * payload tight.
 */
export function sanitize(o: unknown): Record<string, unknown> | undefined {
  if (!o || typeof o !== "object") return undefined;
  const entries = Object.entries(o as Record<string, unknown>);
  const out: Record<string, unknown> = {};
  for (const [k, v] of entries) {
    if (FORBIDDEN_HEADERS.test(k)) continue;
    out[k] = v;
    let len = 0;
    try {
      len = JSON.stringify(out).length;
    } catch {
      // Circular value — drop the offending key and stop.
      delete out[k];
      break;
    }
    if (len > MAX_CONTEXT_BYTES) {
      delete out[k];
      break;
    }
  }
  return Object.keys(out).length === 0 ? undefined : out;
}

function stringifyArg(v: unknown): string {
  if (v === null || v === undefined) return String(v);
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  if (v instanceof Error) return v.stack || v.message;
  try {
    return JSON.stringify(v);
  } catch {
    return String(v);
  }
}
