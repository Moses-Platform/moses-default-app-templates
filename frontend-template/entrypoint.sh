#!/bin/sh
set -eu

# CHAT-pbup: render Moses-aware embedding policy at container start.
# See ../fullstack-simple/frontend/entrypoint.sh for the matrix; the logic
# here is intentionally identical.
#
# CHAT-pbup.16: when framing=moses-only with no explicit
# MOSES_EMBEDDING_ALLOWED_ANCESTORS, default to the same parity origin set
# the platform resolver emits (see
# moses-platform-prep/backend/internal/services/embedding_policy_resolver.go).
# Keeps standalone helm install (no platform-supplied ancestors) embeddable
# from Moses Manager + the Tauri installer shell. KEEP IN SYNC with the
# resolver — when the chart's frame-ancestors line at
# chart/templates/ingressroute.yaml ~line 109 changes, this list must too.

: "${MOSES_BASE_PATH:=/}"
: "${MOSES_EMBEDDING_FRAMING:=}"
: "${MOSES_EMBEDDING_ALLOWED_ANCESTORS:=}"
: "${MOSES_EMBEDDING_REPORT_URI:=}"
: "${MOSES_DOMAIN:=}"

# Default per appType (this is a frontend template) when unset.
if [ -z "$MOSES_EMBEDDING_FRAMING" ]; then
  MOSES_EMBEDDING_FRAMING="moses-only"
fi

case "$MOSES_EMBEDDING_FRAMING" in
  public)
    MOSES_CSP_FRAME_ANCESTORS="*"
    MOSES_X_FRAME_OPTIONS=""
    ;;
  denied)
    MOSES_CSP_FRAME_ANCESTORS="'none'"
    MOSES_X_FRAME_OPTIONS="DENY"
    ;;
  moses-only|*)
    if [ -n "$MOSES_EMBEDDING_ALLOWED_ANCESTORS" ]; then
      # Explicit override wins: the platform (or operator) supplied a
      # concrete ancestors list — use it verbatim, no auto-merge.
      MOSES_CSP_FRAME_ANCESTORS="$MOSES_EMBEDDING_ALLOWED_ANCESTORS"
    else
      # Standalone-or-no-platform-context default: mirror the platform
      # resolver's chart-parity moses-only default. Always include the
      # Tauri origins (Moses Manager runs in a Tauri shell in the
      # installer); per-domain entries depend on whether MOSES_DOMAIN is
      # localhost-y (port-wildcard required) or production-style.
      MOSES_CSP_FRAME_ANCESTORS="'self' tauri://localhost http://tauri.localhost https://tauri.localhost"
      if [ -n "$MOSES_DOMAIN" ]; then
        case "$MOSES_DOMAIN" in
          localhost|localhost.|localhost:*|*.localhost|*.localhost.)
            # Standalone install reached via `kubectl port-forward` on a
            # dynamic non-default port. CSP3: a bare host matches only the
            # scheme's default port (80/443); the ':*' port-wildcard is
            # required for the deployed-app iframe to render under strict
            # engines (Chromium / WebView2 / macOS-Chrome). KEEP IN SYNC
            # with the platform resolver's localhost branch in
            # backend/internal/services/embedding_policy_resolver.go.
            MOSES_CSP_FRAME_ANCESTORS="${MOSES_CSP_FRAME_ANCESTORS} http://${MOSES_DOMAIN}:* https://${MOSES_DOMAIN}:*"
            ;;
          *)
            # Production-style domain: subdomain wildcard only.
            MOSES_CSP_FRAME_ANCESTORS="${MOSES_CSP_FRAME_ANCESTORS} https://*.${MOSES_DOMAIN}"
            ;;
        esac
      fi
    fi
    MOSES_X_FRAME_OPTIONS=""
    ;;
esac

if [ -n "$MOSES_EMBEDDING_REPORT_URI" ]; then
  MOSES_CSP_REPORT_URI="report-uri ${MOSES_EMBEDDING_REPORT_URI};"
else
  MOSES_CSP_REPORT_URI=""
fi

if [ -n "$MOSES_X_FRAME_OPTIONS" ]; then
  MOSES_X_FRAME_OPTIONS_LINE="add_header X-Frame-Options \"${MOSES_X_FRAME_OPTIONS}\" always;"
else
  MOSES_X_FRAME_OPTIONS_LINE=""
fi

# CHAT-pbup Bug 2: render a sub-path location block so /apps/<tenant>/<slug>/...
# requests resolve to the same SPA + assets as a root deploy. Vite emits
# relative paths in index.html so the browser fetches assets at
# /apps/<tenant>/<slug>/assets/index-XXX.js — without an explicit location
# match, those requests fall through to `try_files` which 404s before
# reaching `location /`. We strip the prefix via rewrite + break and
# delegate to the existing static / SPA handlers.
#
# When MOSES_BASE_PATH="/" the prefix is empty and the block is omitted —
# the existing `location /` already handles the root case.
MOSES_BASE_PATH_PREFIX="$(printf '%s' "$MOSES_BASE_PATH" | sed 's:/*$::')"
if [ -n "$MOSES_BASE_PATH_PREFIX" ]; then
  # `^~` makes this a non-regex prefix location with priority over the
  # regex asset-cache `location ~* \.(js|css|...)$`. Without `^~` the
  # regex location wins and try_files looks for /usr/share/nginx/html/
  # apps/<tenant>/<slug>/assets/foo.js (the literal sub-path) and 404s.
  # The `last` rewrite re-enters location matching with the stripped URI,
  # so the regex location then serves the asset from the html root.
  MOSES_SUBPATH_LOCATION_BLOCK="location ^~ ${MOSES_BASE_PATH_PREFIX}/ {
    absolute_redirect off;
    rewrite ^${MOSES_BASE_PATH_PREFIX}/(.*)\$ /\$1 last;
  }"
else
  MOSES_SUBPATH_LOCATION_BLOCK=""
fi

export MOSES_BASE_PATH
export MOSES_BASE_PATH_PREFIX
export MOSES_CSP_FRAME_ANCESTORS
export MOSES_CSP_REPORT_URI
export MOSES_X_FRAME_OPTIONS_LINE
export MOSES_SUBPATH_LOCATION_BLOCK

envsubst '${MOSES_BASE_PATH} ${MOSES_BASE_PATH_PREFIX} ${MOSES_CSP_FRAME_ANCESTORS} ${MOSES_CSP_REPORT_URI} ${MOSES_X_FRAME_OPTIONS_LINE} ${MOSES_SUBPATH_LOCATION_BLOCK}' \
  < /etc/nginx/nginx.conf.template > /etc/nginx/nginx.conf

# Render the Moses-base-path meta tag into index.html so the SPA's
# getBasePath() helper picks it up at runtime. Vite emits relative asset
# paths (base: './') so the assets work regardless of the prefix; what we
# need to inject is the BASE_PATH for the React Router basename.
INDEX_HTML="/usr/share/nginx/html/index.html"
if [ -f "$INDEX_HTML" ]; then
  # Replace the <meta name="moses-base-path"> placeholder if present.
  # Templates ship index.html with content="__MOSES_BASE_PATH__".
  TMP="$(mktemp)"
  sed "s|__MOSES_BASE_PATH__|${MOSES_BASE_PATH}|g" "$INDEX_HTML" > "$TMP"
  mv "$TMP" "$INDEX_HTML"
  # mktemp creates files mode 0600 owned by root; nginx workers run as a
  # non-root uid (101) and would 403 on every fresh container without this.
  chmod 644 "$INDEX_HTML"
fi

exec nginx -g 'daemon off;'
