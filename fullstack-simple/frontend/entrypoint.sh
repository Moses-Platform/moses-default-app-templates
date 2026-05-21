#!/bin/sh
set -eu

# CHAT-pbup: render Moses-aware embedding policy at container start.
# The platform passes MOSES_BASE_PATH + MOSES_EMBEDDING_FRAMING +
# MOSES_EMBEDDING_ALLOWED_ANCESTORS + MOSES_EMBEDDING_REPORT_URI through Helm
# values. We translate the policy into nginx add_header directives and let
# envsubst fill the template.
#
# Framing matrix (matches the chart's chart/templates/ingressroute.yaml):
#   public      -> Content-Security-Policy: frame-ancestors *;
#                  X-Frame-Options omitted (CSP is the source of truth).
#   moses-only  -> Content-Security-Policy: frame-ancestors <ancestors>;
#                  When ancestors is empty, fall back to the chart-parity
#                  default ('self' + Tauri origins + wildcard subdomain when
#                  MOSES_DOMAIN is set) — mirrors the platform resolver in
#                  embedding_policy_resolver.go (CHAT-pbup.16). KEEP IN SYNC.
#   denied      -> Content-Security-Policy: frame-ancestors 'none';
#                  X-Frame-Options: DENY.
#
# The template's appType (frontend / hybrid) defaults to "moses-only" when
# the env var is empty, mirroring the platform's resolver default.

: "${MOSES_BASE_PATH:=/}"
: "${MOSES_EMBEDDING_FRAMING:=}"
: "${MOSES_EMBEDDING_ALLOWED_ANCESTORS:=}"
: "${MOSES_EMBEDDING_REPORT_URI:=}"
: "${MOSES_DOMAIN:=}"

# Default per appType (this is a frontend/hybrid template) when unset.
if [ -z "$MOSES_EMBEDDING_FRAMING" ]; then
  MOSES_EMBEDDING_FRAMING="moses-only"
fi

# Backend service discovery (already established in this template).
: "${BACKEND_SERVICE_HOST:=agent-deployed-app-backend}"
: "${BACKEND_SERVICE_PORT:=8080}"

# Build the CSP frame-ancestors directive value + companion X-Frame-Options.
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
      # resolver's chart-parity moses-only default. Tauri origins are
      # unconditional (Moses Manager runs in a Tauri shell from the
      # installer); per-domain entries are only added when MOSES_DOMAIN
      # is set so a bare standalone deploy doesn't emit dangling refs.
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
    # X-Frame-Options is only safe to emit alongside CSP frame-ancestors when
    # the policy resolves to a single same-origin source. Skipping it for
    # multi-origin / wildcard cases per the CHAT-pbup acceptance.
    MOSES_X_FRAME_OPTIONS=""
    ;;
esac

# Optional report-uri directive
if [ -n "$MOSES_EMBEDDING_REPORT_URI" ]; then
  MOSES_CSP_REPORT_URI="report-uri ${MOSES_EMBEDDING_REPORT_URI};"
else
  MOSES_CSP_REPORT_URI=""
fi

# Pre-render an X-Frame-Options add_header directive (or empty line).
if [ -n "$MOSES_X_FRAME_OPTIONS" ]; then
  MOSES_X_FRAME_OPTIONS_LINE="add_header X-Frame-Options \"${MOSES_X_FRAME_OPTIONS}\" always;"
else
  MOSES_X_FRAME_OPTIONS_LINE=""
fi

# CHAT-pbup Bugs 2 + 4: render sub-path-aware location blocks at container
# start so /apps/<tenant>/<slug>/api/... reaches the backend proxy and asset
# requests at the same prefix resolve to the SPA's static files. The blocks
# are empty when MOSES_BASE_PATH=/ (standalone deploys); the existing root
# `location /` and `location /api/` already handle that case.
MOSES_BASE_PATH_PREFIX="$(printf '%s' "$MOSES_BASE_PATH" | sed 's:/*$::')"
if [ -n "$MOSES_BASE_PATH_PREFIX" ]; then
  # `^~` priority needed so these prefix locations win over the regex
  # asset-cache location below — without it the regex matches first and
  # try_files 404s on /usr/share/nginx/html/apps/<tenant>/<slug>/assets/...
  # The `last` rewrite re-enters location matching with the stripped URI.
  MOSES_SUBPATH_LOCATION_BLOCK="location ^~ ${MOSES_BASE_PATH_PREFIX}/api/ {
    # CHAT-yfmwv: proxy_pass with NO URI part — forward the full prefixed path
    # UNCHANGED so it reaches the backend's MOSES_BASE_PATH-mounted routes
    # (buildMux registers the API under basePath). A '.../api/' URI part would
    # strip the matched location prefix and the backend would 404.
    proxy_pass http://${BACKEND_SERVICE_HOST}:${BACKEND_SERVICE_PORT};
    proxy_http_version 1.1;
    proxy_set_header Host \$host;
    proxy_set_header X-Real-IP \$remote_addr;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto \$scheme;

    # CHAT-pxeo.12: X-Moses-Tenant-ID is audit-only / caller-context.
    # Backend reads MOSES_TENANT_ID from env for storage keys; the
    # forwarded header drives the backend's 403 cross-check on writes.
    proxy_pass_header X-Moses-Tenant-ID;
    proxy_pass_header X-Moses-User-ID;
    proxy_pass_header X-Moses-Chart-ID;
    proxy_pass_header X-Moses-Request-ID;
  }
  location ^~ ${MOSES_BASE_PATH_PREFIX}/ {
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
export BACKEND_SERVICE_HOST
export BACKEND_SERVICE_PORT

# Substitute env vars into the nginx config.
envsubst '${BACKEND_SERVICE_HOST} ${BACKEND_SERVICE_PORT} ${MOSES_BASE_PATH} ${MOSES_BASE_PATH_PREFIX} ${MOSES_CSP_FRAME_ANCESTORS} ${MOSES_CSP_REPORT_URI} ${MOSES_X_FRAME_OPTIONS_LINE} ${MOSES_SUBPATH_LOCATION_BLOCK}' \
  < /etc/nginx/nginx.conf.template > /etc/nginx/nginx.conf

# CHAT-pbup: render Moses-base-path meta tag into index.html so the React
# Router basename helper picks up the correct value at runtime.
INDEX_HTML="/usr/share/nginx/html/index.html"
if [ -f "$INDEX_HTML" ]; then
  TMP="$(mktemp)"
  sed "s|__MOSES_BASE_PATH__|${MOSES_BASE_PATH}|g" "$INDEX_HTML" > "$TMP"
  mv "$TMP" "$INDEX_HTML"
  # mktemp creates files mode 0600 owned by root; nginx workers run as a
  # non-root uid (101) and would 403 on every fresh container without this.
  chmod 644 "$INDEX_HTML"
fi

# Start nginx
exec nginx -g 'daemon off;'
