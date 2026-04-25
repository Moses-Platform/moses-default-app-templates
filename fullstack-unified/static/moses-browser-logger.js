// Copyright (c) 2025-2026 Siemer Industries.
// SPDX-License-Identifier: BSL-1.1
//
// Moses browser-logger snippet for the fullstack-unified template (BLF-J).
//
// Vanilla-JS port of `shared/browser-logger/index.ts`. Same trust model as a
// Sentry DSN: the bootstrap endpoint hands the snippet a public, per-chart
// ingest token whose blast radius is one chart's log stream.
//
// Wiring:
//   1. main.go renders <meta name="moses-config" data-chart-id="…"
//      data-deployment-id="…" data-api-base="…"> into index.html using the
//      MOSES_CHART_ID / MOSES_DEPLOYMENT_ID / MOSES_API_BASE env vars.
//   2. index.html includes this script. By default console.error / .warn are
//      patched; opt-out by adding data-patch-console="false" to the <script>.
//
// All failures are silent — never throw out of this file or recurse into the
// patched console.

(function () {
  "use strict";

  var FORBIDDEN_HEADERS = /^(authorization|cookie|set-cookie|x-csrf-token)$/i;
  var MAX_MESSAGE_BYTES = 4000;
  var MAX_STACK_BYTES = 16000;
  var MAX_CONTEXT_BYTES = 2000;
  var QUEUE_FLUSH_THRESHOLD = 50;
  var QUEUE_FLUSH_INTERVAL_MS = 5000;

  function readConfig() {
    try {
      var meta = document.querySelector('meta[name="moses-config"]');
      if (!meta) return null;
      var chartId = meta.getAttribute("data-chart-id") || "";
      var deploymentId = meta.getAttribute("data-deployment-id") || "";
      var apiBase = meta.getAttribute("data-api-base") || "";
      // Treat unsubstituted Go-template placeholders as "absent". main.go renders
      // empty strings when env vars are missing, but defend against the literal.
      if (chartId.indexOf("{{") === 0 || deploymentId.indexOf("{{") === 0) {
        return null;
      }
      if (!chartId || !deploymentId) return null;
      return {
        chartId: chartId,
        deploymentId: deploymentId,
        apiBase: apiBase,
      };
    } catch (_e) {
      return null;
    }
  }

  function readPatchConsoleFlag() {
    try {
      var scripts = document.getElementsByTagName("script");
      for (var i = 0; i < scripts.length; i++) {
        var src = scripts[i].getAttribute("src") || "";
        if (src.indexOf("moses-browser-logger.js") !== -1) {
          var v = scripts[i].getAttribute("data-patch-console");
          if (v === "false" || v === "0") return false;
          return true;
        }
      }
    } catch (_e) {
      /* ignore */
    }
    return true;
  }

  function clip(s, max) {
    if (typeof s !== "string") s = String(s);
    return s.length <= max ? s : s.slice(0, max);
  }

  function clampSampleRate(n) {
    if (typeof n !== "number" || isNaN(n)) return 1.0;
    if (n < 0) return 0;
    if (n > 1) return 1;
    return n;
  }

  function absolutize(endpoint, apiBase) {
    if (/^https?:\/\//i.test(endpoint)) return endpoint;
    if (!apiBase) return endpoint; // same origin
    var left = apiBase.replace(/\/+$/, "");
    var right = endpoint.charAt(0) === "/" ? endpoint : "/" + endpoint;
    return left + right;
  }

  function sanitize(o) {
    if (!o || typeof o !== "object") return undefined;
    var out = {};
    var keys = [];
    try {
      keys = Object.keys(o);
    } catch (_e) {
      return undefined;
    }
    for (var i = 0; i < keys.length; i++) {
      var k = keys[i];
      if (FORBIDDEN_HEADERS.test(k)) continue;
      out[k] = o[k];
      var len = 0;
      try {
        len = JSON.stringify(out).length;
      } catch (_e) {
        // Circular value — drop the offending key and stop.
        delete out[k];
        break;
      }
      if (len > MAX_CONTEXT_BYTES) {
        delete out[k];
        break;
      }
    }
    var outKeys = [];
    try {
      outKeys = Object.keys(out);
    } catch (_e) {
      /* ignore */
    }
    return outKeys.length === 0 ? undefined : out;
  }

  function stringifyArg(v) {
    if (v === null || v === undefined) return String(v);
    if (typeof v === "string") return v;
    if (typeof v === "number" || typeof v === "boolean") return String(v);
    if (v instanceof Error) return v.stack || v.message;
    try {
      return JSON.stringify(v);
    } catch (_e) {
      return String(v);
    }
  }

  function install(cfg) {
    var apiBase = cfg.apiBase || "";
    var bootstrapUrl =
      apiBase +
      "/api/v1/browser-logs/bootstrap" +
      "?chart_id=" + encodeURIComponent(cfg.chartId) +
      "&deployment_id=" + encodeURIComponent(cfg.deploymentId);

    var fetchOpts = { credentials: "omit" };
    var bootstrapPromise;
    try {
      bootstrapPromise = fetch(bootstrapUrl, fetchOpts);
    } catch (_e) {
      return; // fetch unavailable / sync throw — silent no-op
    }

    bootstrapPromise.then(function (res) {
      if (!res || !res.ok) return;
      return res.json();
    }).then(function (boot) {
      if (!boot || !boot.enabled || !boot.token || !boot.endpoint) return;

      var ingestEndpoint = absolutize(boot.endpoint, apiBase);
      var sampleRate = clampSampleRate(boot.sampleRate != null ? boot.sampleRate : 1.0);
      var token = boot.token;
      var patchConsole = readPatchConsoleFlag();

      var queue = [];
      var flushScheduled = false;

      function enqueue(level, message, stack, extra) {
        if (sampleRate < 1.0 && Math.random() > sampleRate) return;
        var event = {
          ts: new Date().toISOString(),
          level: level,
          message: clip(String(message), MAX_MESSAGE_BYTES),
          source_kind: "client_runtime",
        };
        if (stack) event.stack = clip(stack, MAX_STACK_BYTES);
        if (typeof location !== "undefined") event.url = location.href;
        var ctx = sanitize(extra);
        if (ctx) event.context = ctx;
        queue.push(event);
        if (queue.length >= QUEUE_FLUSH_THRESHOLD) {
          flush();
        } else if (!flushScheduled) {
          flushScheduled = true;
          setTimeout(flush, QUEUE_FLUSH_INTERVAL_MS);
        }
      }

      function flush() {
        flushScheduled = false;
        if (queue.length === 0) return;
        var events = queue.splice(0, queue.length);
        try {
          var p = fetch(ingestEndpoint, {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              Authorization: "Bearer " + token,
            },
            body: JSON.stringify({ events: events }),
            keepalive: true,
            credentials: "omit",
          });
          if (p && typeof p.catch === "function") {
            p.catch(function () { /* drop — recursion-safe */ });
          }
        } catch (_e) {
          /* ignore */
        }
      }

      // Window error surface ------------------------------------------------
      if (typeof window !== "undefined") {
        window.addEventListener("error", function (e) {
          var stack;
          if (e && e.error && typeof e.error === "object" && e.error.stack) {
            stack = String(e.error.stack);
          }
          enqueue("error", (e && e.message) || "Unknown error", stack, {
            filename: e && e.filename,
            lineno: e && e.lineno,
            colno: e && e.colno,
          });
        });

        window.addEventListener("unhandledrejection", function (e) {
          var reason = e && e.reason;
          var message;
          if (reason && typeof reason === "object" && "message" in reason) {
            message = String(reason.message != null ? reason.message : reason);
          } else {
            message = String(reason);
          }
          var stack;
          if (reason && typeof reason === "object" && "stack" in reason && reason.stack != null) {
            stack = String(reason.stack);
          }
          enqueue("error", "Unhandled promise rejection: " + message, stack);
        });
      }

      // Console patching ----------------------------------------------------
      if (patchConsole && typeof console !== "undefined") {
        if (typeof console.error === "function") {
          var origErr = console.error.bind(console);
          console.error = function () {
            var args = Array.prototype.slice.call(arguments);
            try {
              enqueue("error", args.map(stringifyArg).join(" "));
            } catch (_e) {
              /* ignore */
            }
            origErr.apply(null, args);
          };
        }
        if (typeof console.warn === "function") {
          var origWarn = console.warn.bind(console);
          console.warn = function () {
            var args = Array.prototype.slice.call(arguments);
            try {
              enqueue("warn", args.map(stringifyArg).join(" "));
            } catch (_e) {
              /* ignore */
            }
            origWarn.apply(null, args);
          };
        }
      }

      // Best-effort flush on hide/unload ------------------------------------
      if (typeof document !== "undefined") {
        document.addEventListener("visibilitychange", function () {
          if (document.visibilityState === "hidden") flush();
        });
      }
      if (typeof window !== "undefined") {
        window.addEventListener("pagehide", flush);
      }
    }).catch(function () {
      /* bootstrap failure — silent */
    });
  }

  var cfg = readConfig();
  if (!cfg) return;
  install(cfg);
})();
