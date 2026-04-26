#!/bin/sh
set -eu

# CHAT-pbup: render Moses-aware embedding policy at container start.
# See ../fullstack-simple/frontend/entrypoint.sh for the matrix; the logic
# here is intentionally identical.

: "${MOSES_BASE_PATH:=/}"
: "${MOSES_EMBEDDING_FRAMING:=}"
: "${MOSES_EMBEDDING_ALLOWED_ANCESTORS:=}"
: "${MOSES_EMBEDDING_REPORT_URI:=}"

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
      MOSES_CSP_FRAME_ANCESTORS="$MOSES_EMBEDDING_ALLOWED_ANCESTORS"
    else
      MOSES_CSP_FRAME_ANCESTORS="'self'"
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

export MOSES_BASE_PATH
export MOSES_CSP_FRAME_ANCESTORS
export MOSES_CSP_REPORT_URI
export MOSES_X_FRAME_OPTIONS_LINE

envsubst '${MOSES_BASE_PATH} ${MOSES_CSP_FRAME_ANCESTORS} ${MOSES_CSP_REPORT_URI} ${MOSES_X_FRAME_OPTIONS_LINE}' \
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
fi

exec nginx -g 'daemon off;'
