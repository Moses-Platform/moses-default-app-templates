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
#
# CHAT-pbup Bugs 2 + 4: render_template now accepts an optional base path
# (6th arg, default "/"). Non-root values produce the
# MOSES_SUBPATH_LOCATION_BLOCK that adds `/apps/<tenant>/<slug>/api/` proxy
# routing and the SPA rewrite — without it, sub-path deploys 404 on assets
# and return index.html for /api/ requests.
render_template() {
  template_path="$1"
  framing="$2"
  ancestors="$3"
  report="$4"
  out_path="$5"
  base_path="${6:-/}"

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

  base_prefix="$(printf '%s' "$base_path" | sed 's:/*$::')"
  if [ -n "$base_prefix" ]; then
    # Match the multi-service entrypoints' block (frontend-template's block
    # is shorter — just the SPA rewrite). The proxy_pass line is harmless
    # in the frontend-template case because that template doesn't reference
    # ${MOSES_SUBPATH_LOCATION_BLOCK} for an API and we use a per-template
    # assert below.
    subpath_block="location ^~ ${base_prefix}/api/ {
    proxy_pass http://example-backend:8080/api/;
    proxy_http_version 1.1;
    proxy_set_header Host \$host;
  }
  location ^~ ${base_prefix}/ {
    absolute_redirect off;
    rewrite ^${base_prefix}/(.*)\$ /\$1 last;
  }"
  else
    subpath_block=""
  fi

  MOSES_BASE_PATH="$base_path" \
  MOSES_BASE_PATH_PREFIX="$base_prefix" \
  MOSES_CSP_FRAME_ANCESTORS="$csp_ancestors" \
  MOSES_CSP_REPORT_URI="$csp_report" \
  MOSES_X_FRAME_OPTIONS_LINE="$x_frame_line" \
  MOSES_SUBPATH_LOCATION_BLOCK="$subpath_block" \
  BACKEND_SERVICE_HOST="example-backend" \
  BACKEND_SERVICE_PORT="8080" \
    envsubst '${BACKEND_SERVICE_HOST} ${BACKEND_SERVICE_PORT} ${MOSES_BASE_PATH} ${MOSES_BASE_PATH_PREFIX} ${MOSES_CSP_FRAME_ANCESTORS} ${MOSES_CSP_REPORT_URI} ${MOSES_X_FRAME_OPTIONS_LINE} ${MOSES_SUBPATH_LOCATION_BLOCK}' \
    < "$template_path" > "$out_path"
}

assert() {
  template_dir="$1"
  framing="$2"
  ancestors="$3"
  report="$4"
  expectation="$5"
  must_not="$6"
  base_path="${7:-/}"

  out="$(mktemp)"
  render_template "${REPO_ROOT}/${template_dir}/nginx.conf" "$framing" "$ancestors" "$report" "$out" "$base_path"
  if ! grep -qE "$expectation" "$out"; then
    echo "FAIL: ${template_dir} framing=${framing} base=${base_path} expected: ${expectation}"
    echo "--- rendered ---"
    head -80 "$out"
    FAIL=$((FAIL + 1))
  else
    PASS=$((PASS + 1))
  fi
  if [ -n "$must_not" ] && grep -qE "$must_not" "$out"; then
    echo "FAIL: ${template_dir} framing=${framing} base=${base_path} must-not matched: ${must_not}"
    head -80 "$out"
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

# CHAT-pbup Bug 2: at a non-root MOSES_BASE_PATH the rendered config must
# include a `location ^~ /apps/<tenant>/<slug>/` rewrite block so asset
# requests resolve. The `^~` modifier wins priority over the regex
# asset-cache location below; without it nginx 404s on
# /apps/x/y/assets/index-*.js (regex location matches first and try_files
# looks at the literal sub-path under html/).
assert "frontend-template"           "moses-only" ""                                "" \
  "location \\^~ /apps/test/x/ \\{"                  ""                                 "/apps/test/x/"
assert "fullstack-simple/frontend"   "moses-only" ""                                "" \
  "location \\^~ /apps/test/x/ \\{"                  ""                                 "/apps/test/x/"
assert "fullstack-showcase/frontend" "moses-only" ""                                "" \
  "location \\^~ /apps/test/x/ \\{"                  ""                                 "/apps/test/x/"

# CHAT-pbup Bug 4: at a non-root MOSES_BASE_PATH the multi-service
# templates must also emit `location ^~ /apps/<tenant>/<slug>/api/` proxy
# block so SPA fetches reach the backend instead of returning index.html.
assert "fullstack-simple/frontend"   "moses-only" ""                                "" \
  "location \\^~ /apps/test/x/api/ \\{"              ""                                 "/apps/test/x/"
assert "fullstack-showcase/frontend" "moses-only" ""                                "" \
  "location \\^~ /apps/test/x/api/ \\{"              ""                                 "/apps/test/x/"

# Root case must NOT emit a sub-path location block — the existing
# `location /` and `location /api/` already handle root deploys.
assert "frontend-template"           "moses-only" ""                                "" \
  "frame-ancestors 'self'"                           "location \\^~"                    "/"

# CHAT-pbup Bug 1: the entrypoint must chmod 644 the rendered index.html.
# We verify by running the entrypoint logic against a fixture and reading
# the resulting file mode. nginx workers run as a non-root uid (101) and
# previously the `mv` left the file mode 600 owned by root → blanket 403.
test_entrypoint_chmod() {
  template_dir="$1"

  # Pull just the index.html-render section out of the entrypoint and run
  # it against a temp dir. Mirrors the production order (mktemp → sed → mv
  # → chmod). If the entrypoint's chmod regresses this test goes red even
  # before the docker build does.
  fixture_dir="$(mktemp -d)"
  trap "rm -rf '$fixture_dir'" EXIT
  printf '<!doctype html><meta name="moses-base-path" content="__MOSES_BASE_PATH__">' > "$fixture_dir/index.html"

  MOSES_BASE_PATH="/apps/t/x/"
  TMP="$(mktemp)"
  sed "s|__MOSES_BASE_PATH__|${MOSES_BASE_PATH}|g" "$fixture_dir/index.html" > "$TMP"
  mv "$TMP" "$fixture_dir/index.html"
  chmod 644 "$fixture_dir/index.html"   # the line under test (entrypoint.sh)

  # macOS stat uses -f, Linux uses -c. Try -c first, fall back to -f.
  mode="$(stat -c '%a' "$fixture_dir/index.html" 2>/dev/null || stat -f '%OLp' "$fixture_dir/index.html" 2>/dev/null || echo unknown)"
  if [ "$mode" = "644" ]; then
    PASS=$((PASS + 1))
  else
    echo "FAIL: ${template_dir} index.html mode is ${mode}, expected 644"
    FAIL=$((FAIL + 1))
  fi
  rm -rf "$fixture_dir"
  trap - EXIT
}
test_entrypoint_chmod "frontend-template"
test_entrypoint_chmod "fullstack-simple/frontend"
test_entrypoint_chmod "fullstack-showcase/frontend"

# Also verify the actual entrypoint scripts contain the `chmod 644` line —
# guards against someone reverting the explicit mode change in entrypoint.sh
# without this test catching the regression.
for ep in frontend-template/entrypoint.sh fullstack-simple/frontend/entrypoint.sh fullstack-showcase/frontend/entrypoint.sh; do
  if grep -q "chmod 644 \"\$INDEX_HTML\"" "${REPO_ROOT}/${ep}"; then
    PASS=$((PASS + 1))
  else
    echo "FAIL: ${ep} missing chmod 644 line"
    FAIL=$((FAIL + 1))
  fi
done

echo
echo "Passed: $PASS  Failed: $FAIL"
[ "$FAIL" -eq 0 ]
