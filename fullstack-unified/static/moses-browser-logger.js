// Copyright (c) 2025-2026 Siemer Industries.
// SPDX-License-Identifier: BSL-1.1
//
// Moses browser-logger snippet for the fullstack-unified template.
//
// Vanilla-JS port of `shared/browser-logger/index.ts`, and the same code as the
// platform's injected client (moses-platform-prep
// backend/internal/api/browser_logger_client.js, from `(function () {` on).
// Keep the two in sync by hand; the five TS copies are byte-gated separately
// (tools/check-vendored-browser-logger.sh).
//
// Bootstrap: the page reports where it runs (`loc` = origin + pathname); the
// platform finds the app there and applies the app's own access decision. The
// request is relative and carries the viewer's session same-origin
// (credentials "same-origin": never cross-origin) — on the Moses origin and on
// the apps origin alike. A public app's visitors get a token anonymously. The
// token is short-lived: an ingest 401 bootstraps again (at most once a minute)
// and resends the batch.
//
// Optional: a <meta name="moses-config" data-api-base="…"> in index.html
// points the logger at another API origin. Leave it out on Moses (a
// cross-origin bootstrap carries no viewer session). Opt out of console
// patching with data-patch-console="false" on the <script> tag.
//
// The window.__mosesBrowserLoggerInstalled guard makes this coexist with the
// gateway-injected client: whichever loads first wins.
//
// CRASH-CAPTURE ORDERING (CHAT-il3y6): the window 'error' and
// 'unhandledrejection' listeners are attached SYNCHRONOUSLY before the
// bootstrap .then() and buffer into a bounded queue; the console patch stays
// inside the .then() because it depends on the fetched config. Do not move the
// console patch up, and do not attach a second set of window listeners.
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
  // Bound the pre-bootstrap buffer so a crash-loop before bootstrap resolves
  // can't grow memory without limit. Drop-oldest once the cap is hit.
  var PREBOOTSTRAP_BUFFER_CAP = 50;

  // A <meta name="moses-config" data-api-base="…"> may point the logger at
  // another API base; its chart/deployment ids are no longer used (the
  // platform resolves the app from `loc`).
  function readConfig() {
    try {
      var meta = document.querySelector('meta[name="moses-config"]');
      if (!meta) return null;
      var apiBase = meta.getAttribute("data-api-base") || "";
      if (apiBase.indexOf("{{") === 0) apiBase = "";
      return { apiBase: apiBase };
    } catch (_e) {
      return null;
    }
  }

  function readPatchConsoleFlag() {
    try {
      var scripts = document.getElementsByTagName("script");
      for (var i = 0; i < scripts.length; i++) {
        var src = scripts[i].getAttribute("src") || "";
        if (
          src.indexOf("moses-browser-logger.js") !== -1 ||
          src.indexOf("browser-logs/client.js") !== -1
        ) {
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
    // Idempotency latch: guarantee a single install even if this script and a
    // template-bundled moses-browser-logger both load (or the file is included
    // twice). Must precede the synchronous window-listener attach below so the
    // second run attaches no duplicate listeners.
    if (typeof window !== "undefined") {
      if (window.__mosesBrowserLoggerInstalled) return;
      window.__mosesBrowserLoggerInstalled = true;
    }

    var apiBase = cfg.apiBase || "";
    // The platform resolves the app from where the page runs and decides
    // access as for opening the app; the per-chart enabled gate applies too.
    var bootstrapUrl =
      apiBase +
      "/api/v1/browser-logs/bootstrap?loc=" +
      encodeURIComponent(
        typeof window !== "undefined"
          ? window.location.origin + window.location.pathname
          : ""
      );
    // Same-origin only: the viewer's Moses cookie never goes cross-origin.
    var bootstrapOpts = { credentials: "same-origin" };
    var REBOOTSTRAP_MIN_INTERVAL_MS = 60000;
    var lastBootstrapAt = 0;

    // --- Hoisted state (must outlive the bootstrap .then) -------------------
    var queue = [];
    var dropped = false;
    var flushReady = false;
    var flushScheduled = false;
    var ingestEndpoint = "";
    var token = "";
    var sampleRate = 1.0;

    function enqueue(level, message, stack, extra) {
      if (dropped) return;
      if (flushReady && sampleRate < 1.0 && Math.random() > sampleRate) return;
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
      if (!flushReady) {
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

    function flush() {
      flushScheduled = false;
      if (!flushReady) return;
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
        if (p && typeof p.then === "function") {
          p.then(function (res) {
            // The token expired (or its key was rotated out): bootstrap again
            // and resend this batch, bounded like the pre-bootstrap buffer.
            if (res && res.status === 401) {
              queue = events.concat(queue).slice(-PREBOOTSTRAP_BUFFER_CAP);
              rebootstrap();
            }
          }).catch(function () { /* drop — recursion-safe */ });
        }
      } catch (_e) {
        /* ignore */
      }
    }

    function rebootstrap() {
      flushReady = false;
      var now = Date.now();
      if (now - lastBootstrapAt < REBOOTSTRAP_MIN_INTERVAL_MS) {
        dropped = true;
        queue.length = 0;
        return;
      }
      fetchBootstrap().then(function (boot) {
        if (!boot) {
          dropped = true;
          queue.length = 0;
          return;
        }
        applyBootstrap(boot);
        flushReady = true;
        if (queue.length > 0) flush();
      }).catch(function () {
        dropped = true;
        queue.length = 0;
      });
    }

    // fetchBootstrap resolves to an enabled bootstrap config, or null.
    function fetchBootstrap() {
      lastBootstrapAt = Date.now();
      return fetch(bootstrapUrl, bootstrapOpts).then(function (res) {
        if (!res || !res.ok) return null;
        return res.json();
      }).then(function (boot) {
        if (!boot || !boot.enabled || !boot.token || !boot.endpoint) return null;
        return boot;
      });
    }

    function applyBootstrap(boot) {
      ingestEndpoint = absolutize(boot.endpoint, apiBase);
      sampleRate = clampSampleRate(boot.sampleRate != null ? boot.sampleRate : 1.0);
      token = boot.token;
    }

    // Window error surface --------------------------------------------------
    // Attached SYNCHRONOUSLY, before the bootstrap .then(), so a render-time
    // crash that fires window.onerror in the same tick is captured into the
    // bounded pre-bootstrap buffer rather than lost. This is the single set of
    // window listeners; do not add another after bootstrap.
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

    var bootstrapPromise;
    try {
      bootstrapPromise = fetchBootstrap();
    } catch (_e) {
      dropped = true;
      queue.length = 0;
      return;
    }

    bootstrapPromise.then(function (boot) {
      if (!boot) {
        dropped = true;
        queue.length = 0;
        return;
      }

      applyBootstrap(boot);
      var patchConsole = readPatchConsoleFlag();

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

      if (typeof document !== "undefined") {
        document.addEventListener("visibilitychange", function () {
          if (document.visibilityState === "hidden") flush();
        });
      }
      if (typeof window !== "undefined") {
        window.addEventListener("pagehide", flush);
      }

      flushReady = true;
      if (queue.length > 0) {
        flush();
      }
    }).catch(function () {
      dropped = true;
      queue.length = 0;
    });
  }

  // No <meta name="moses-config"> → empty config (same-origin API base).
  var cfg = readConfig() || { apiBase: "" };
  install(cfg);
})();
