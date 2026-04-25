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
// At build time, the Moses platform's KanikoBuildService bakes three Vite
// env vars into the bundle:
//
//   - VITE_MOSES_CHART_ID       — the chart this build belongs to (UUID)
//   - VITE_MOSES_DEPLOYMENT_ID  — the per-build deployment id (UUID, the
//                                 agent_pod_executions row that produced this
//                                 image)
//   - VITE_MOSES_API_BASE       — empty string for same-origin / desktop;
//                                 absolute URL when the platform sits on a
//                                 different host (e.g. production with
//                                 multi-domain ingress)
//
// At boot the snippet hits BLF-A's bootstrap endpoint to fetch its config
// (per-chart enabled flag + ingest token + sample rate). When the env vars
// are missing (built outside Moses, or chart-id stripped) it silently no-ops.

type Level = "error" | "warn" | "info" | "log";

type BootstrapConfig = {
  enabled: boolean;
  endpoint?: string;
  token?: string;
  sampleRate?: number;
};

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

export async function installBrowserLogger(opts: InstallOptions = {}): Promise<void> {
  // import.meta.env is the Vite-injected env object. Use a guarded read so
  // this file also compiles under bundlers that don't define it (the call
  // becomes a no-op and the snippet stays silent).
  const env = readViteEnv();
  const chartId = env.VITE_MOSES_CHART_ID;
  const deploymentId = env.VITE_MOSES_DEPLOYMENT_ID;
  if (!chartId || !deploymentId) {
    return;
  }

  const apiBase = env.VITE_MOSES_API_BASE || "";
  const bootstrapUrl =
    apiBase +
    "/api/v1/browser-logs/bootstrap" +
    "?chart_id=" + encodeURIComponent(chartId) +
    "&deployment_id=" + encodeURIComponent(deploymentId);

  let cfg: BootstrapConfig | null = null;
  try {
    const res = await fetch(bootstrapUrl, { credentials: "omit" });
    if (!res.ok) return;
    cfg = (await res.json()) as BootstrapConfig;
  } catch {
    return;
  }
  if (!cfg || !cfg.enabled || !cfg.token || !cfg.endpoint) {
    return;
  }

  const ingestEndpoint = absolutize(cfg.endpoint, apiBase);
  const sampleRate = clampSampleRate(opts.sampleRate ?? cfg.sampleRate ?? 1.0);
  const patchConsole = opts.patchConsole ?? true;
  const token = cfg.token;

  const queue: unknown[] = [];
  let flushScheduled = false;

  function enqueue(
    level: Level,
    message: string,
    stack?: string,
    extra?: Record<string, unknown>
  ): void {
    if (sampleRate < 1.0 && Math.random() > sampleRate) return;
    queue.push({
      ts: new Date().toISOString(),
      level,
      message: clip(String(message), MAX_MESSAGE_BYTES),
      stack: stack ? clip(stack, MAX_STACK_BYTES) : undefined,
      url: typeof location !== "undefined" ? location.href : undefined,
      source_kind: "client_runtime",
      context: sanitize(extra),
    });
    if (queue.length >= QUEUE_FLUSH_THRESHOLD) {
      flush();
    } else if (!flushScheduled) {
      flushScheduled = true;
      setTimeout(flush, QUEUE_FLUSH_INTERVAL_MS);
    }
  }

  function flush(): void {
    flushScheduled = false;
    if (queue.length === 0) return;
    const events = queue.splice(0, queue.length);
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
      }).catch(() => {
        // Network or CORS failure — drop events. Logging here would
        // recurse via the patched console, so we swallow silently.
      });
    } catch {
      // fetch may throw synchronously in degenerate environments; ignore.
    }
  }

  // Window error surface ------------------------------------------------------
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

  // Console patching ---------------------------------------------------------
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
}

// --- Internal helpers ------------------------------------------------------

interface ViteEnvShape {
  VITE_MOSES_CHART_ID?: string;
  VITE_MOSES_DEPLOYMENT_ID?: string;
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
