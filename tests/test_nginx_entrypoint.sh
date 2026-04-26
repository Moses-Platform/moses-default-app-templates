#!/bin/sh
set -eu

# CHAT-pbup: smoke test for the Moses-aware nginx entrypoint scripts.
# Renders each template's nginx.conf under several MOSES_EMBEDDING_FRAMING
# combinations and grep-asserts the output. Run with:
#
#   ./tests/test_nginx_entrypoint.sh
#
# Exits non-zero on any failure.

REPO_ROOT="$(cd "$(dirname "$0")"/.. && pwd)"
PASS=0
FAIL=0

if ! command -v envsubst >/dev/null 2>&1; then
  echo "SKIP: envsubst not installed (install gettext: brew install gettext / apt-get install gettext)"
  exit 0
fi

# Inline copy of the entrypoint policy logic — kept in sync with each
# template's entrypoint.sh. If you edit the entrypoint matrix, mirror the
# change here.
render_template() {
  template_path="$1"
  framing="$2"
  ancestors="$3"
  report="$4"
  out_path="$5"

  if [ -z "$framing" ]; then
    framing="moses-only"
  fi

  case "$framing" in
    public)
      csp_ancestors="*"
      x_frame=""
      ;;
    denied)
      csp_ancestors="'none'"
      x_frame="DENY"
      ;;
    moses-only|*)
      if [ -n "$ancestors" ]; then
        csp_ancestors="$ancestors"
      else
        csp_ancestors="'self'"
      fi
      x_frame=""
      ;;
  esac

  if [ -n "$report" ]; then
    csp_report="report-uri ${report};"
  else
    csp_report=""
  fi

  if [ -n "$x_frame" ]; then
    x_frame_line="add_header X-Frame-Options \"${x_frame}\" always;"
  else
    x_frame_line=""
  fi

  MOSES_BASE_PATH="/" \
  MOSES_CSP_FRAME_ANCESTORS="$csp_ancestors" \
  MOSES_CSP_REPORT_URI="$csp_report" \
  MOSES_X_FRAME_OPTIONS_LINE="$x_frame_line" \
  BACKEND_SERVICE_HOST="example-backend" \
  BACKEND_SERVICE_PORT="8080" \
    envsubst '${BACKEND_SERVICE_HOST} ${BACKEND_SERVICE_PORT} ${MOSES_BASE_PATH} ${MOSES_CSP_FRAME_ANCESTORS} ${MOSES_CSP_REPORT_URI} ${MOSES_X_FRAME_OPTIONS_LINE}' \
    < "$template_path" > "$out_path"
}

assert() {
  template_dir="$1"
  framing="$2"
  ancestors="$3"
  report="$4"
  expectation="$5"
  must_not="$6"

  out="$(mktemp)"
  render_template "${REPO_ROOT}/${template_dir}/nginx.conf" "$framing" "$ancestors" "$report" "$out"
  if ! grep -qE "$expectation" "$out"; then
    echo "FAIL: ${template_dir} framing=${framing} expected: ${expectation}"
    echo "--- rendered ---"
    head -40 "$out"
    FAIL=$((FAIL + 1))
  else
    PASS=$((PASS + 1))
  fi
  if [ -n "$must_not" ] && grep -qE "$must_not" "$out"; then
    echo "FAIL: ${template_dir} framing=${framing} must-not matched: ${must_not}"
    head -40 "$out"
    FAIL=$((FAIL + 1))
  fi
  rm -f "$out"
}

# frontend-template
assert "frontend-template"           "public"     ""                                "" \
  "frame-ancestors \\*"                              "^[^#]*add_header X-Frame-Options"
assert "frontend-template"           "denied"     ""                                "" \
  "frame-ancestors 'none'"                           ""
assert "frontend-template"           "moses-only" "https://moses.example.com"       "" \
  "frame-ancestors https://moses.example.com"        ""
assert "frontend-template"           "moses-only" "'self'"                          "/csp" \
  "report-uri /csp;"                                 ""

# fullstack-simple frontend
assert "fullstack-simple/frontend"   "public"     ""                                "" \
  "frame-ancestors \\*"                              "^[^#]*add_header X-Frame-Options"
assert "fullstack-simple/frontend"   "denied"     ""                                "" \
  "frame-ancestors 'none'"                           ""

# fullstack-showcase frontend
assert "fullstack-showcase/frontend" "moses-only" "https://moses.example.com"       "" \
  "frame-ancestors https://moses.example.com"        ""

echo
echo "Passed: $PASS  Failed: $FAIL"
[ "$FAIL" -eq 0 ]
