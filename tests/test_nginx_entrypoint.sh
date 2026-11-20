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

# Execute each real entrypoint, with immutable image inputs and only nginx stubbed.
render_template() {
  python3 "${REPO_ROOT}/tests/run_nginx_entrypoint.py" "$1" "$2" "$3" "$4" "$5" "${6:-/}" "${7:-}"
}

assert() {
  template_dir="$1"
  framing="$2"
  ancestors="$3"
  report="$4"
  expectation="$5"
  must_not="$6"
  base_path="${7:-/}"
  moses_domain="${8:-}"

  out="$(mktemp)"
  render_template "${REPO_ROOT}/${template_dir}/nginx.conf" "$framing" "$ancestors" "$report" "$out" "$base_path" "$moses_domain"
  if ! grep -qE "$expectation" "$out"; then
    echo "FAIL: ${template_dir} framing=${framing} base=${base_path} domain=${moses_domain} expected: ${expectation}"
    echo "--- rendered ---"
    head -80 "$out"
    FAIL=$((FAIL + 1))
  else
    PASS=$((PASS + 1))
  fi
  if [ -n "$must_not" ] && grep -qE "$must_not" "$out"; then
    echo "FAIL: ${template_dir} framing=${framing} base=${base_path} domain=${moses_domain} must-not matched: ${must_not}"
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

# CHAT-pbup.16: standalone-deploy parity with the platform resolver. When
# framing=moses-only and ALLOWED_ANCESTORS is empty, the rendered CSP must
# include 'self' + the three Tauri origins so Moses Manager (which can run
# as a Tauri shell from the installer) can frame the deployed app even when
# the chart is helm-installed outside the platform.
#
# Source of truth: backend/internal/services/embedding_policy_resolver.go
# (defaultDesktopEmbedOrigins). KEEP IN SYNC.

# Case 1: moses-only + no ancestors + no MOSES_DOMAIN → four-origin default.
for tmpl in frontend-template fullstack-simple/frontend fullstack-showcase/frontend fullstack-chat/frontend fullstack-oidc/frontend; do
  assert "$tmpl" "moses-only" "" "" \
    "frame-ancestors 'self' tauri://localhost http://tauri.localhost https://tauri.localhost" \
    "https://\\*\\." "/" ""
done

# Case 2: moses-only + no ancestors + MOSES_DOMAIN=example.com → four-origin
# default PLUS https://*.example.com wildcard subdomain (mirrors the
# resolver's mosesDomain branch for non-localhost domains).
for tmpl in frontend-template fullstack-simple/frontend fullstack-showcase/frontend fullstack-chat/frontend fullstack-oidc/frontend; do
  assert "$tmpl" "moses-only" "" "" \
    "frame-ancestors 'self' tauri://localhost http://tauri.localhost https://tauri.localhost https://\\*\\.example\\.com" \
    "" "/" "example.com"
done

# Case 3: moses-only + explicit ALLOWED_ANCESTORS → that exact list, NO
# auto-merge with the chart-parity default. Explicit means explicit.
for tmpl in frontend-template fullstack-simple/frontend fullstack-showcase/frontend fullstack-chat/frontend fullstack-oidc/frontend; do
  assert "$tmpl" "moses-only" "https://other.com" "" \
    "frame-ancestors https://other\\.com;" \
    "tauri://localhost" "/" ""
done

# Case 4: moses-only + no ancestors + MOSES_DOMAIN=localhost → four-origin
# default PLUS http://localhost:* https://localhost:* (CSP3 port-wildcard,
# required because the desktop install reaches Moses through a kubectl
# port-forward on a dynamic non-default port; bare-host CSP entries match
# only the scheme's default port and would block the iframe under
# strict engines: Chromium, WebView2, macOS-Chrome). KEEP IN SYNC with
# backend/internal/services/embedding_policy_resolver.go isLocalhostDomain.
for tmpl in frontend-template fullstack-simple/frontend fullstack-showcase/frontend fullstack-chat/frontend fullstack-oidc/frontend; do
  assert "$tmpl" "moses-only" "" "" \
    "frame-ancestors 'self' tauri://localhost http://tauri.localhost https://tauri.localhost http://localhost:\\* https://localhost:\\*" \
    " https://localhost[^:]" "/" "localhost"
done

# Case 5: localhost FQDN form (trailing dot). Same shape as Case 4.
for tmpl in frontend-template fullstack-simple/frontend fullstack-showcase/frontend fullstack-chat/frontend fullstack-oidc/frontend; do
  assert "$tmpl" "moses-only" "" "" \
    "http://localhost\\.:\\* https://localhost\\.:\\*" \
    "" "/" "localhost."
done

# Case 6: *.localhost subdomain form (e.g. moses.localhost). Treated as
# localhost-y per the resolver's isLocalhostDomain match-suffix rule.
for tmpl in frontend-template fullstack-simple/frontend fullstack-showcase/frontend fullstack-chat/frontend fullstack-oidc/frontend; do
  assert "$tmpl" "moses-only" "" "" \
    "http://moses\\.localhost:\\* https://moses\\.localhost:\\*" \
    "" "/" "moses.localhost"
done

# CHAT-pbup.16: each entrypoint must reference MOSES_DOMAIN in the
# moses-only default branch. Guards against someone reverting the
# chart-parity default without the rendered-template tests above also
# tripping (e.g. by hardcoding the four-origin list back to 'self').
for ep in frontend-template/entrypoint.sh fullstack-simple/frontend/entrypoint.sh fullstack-showcase/frontend/entrypoint.sh; do
  if grep -q "MOSES_DOMAIN" "${REPO_ROOT}/${ep}" && \
     grep -q "tauri://localhost" "${REPO_ROOT}/${ep}"; then
    PASS=$((PASS + 1))
  else
    echo "FAIL: ${ep} missing CHAT-pbup.16 chart-parity default (MOSES_DOMAIN + tauri://localhost)"
    FAIL=$((FAIL + 1))
  fi
done

# CHAT-t5d1u.24 (WS-F F0c): apex/custom-hostname forwarding. Custom-hostname
# routes serve the app at the HOST ROOT with no rewrite while the pod env
# keeps the canonical MOSES_BASE_PATH, so root-relative /api/ requests must
# be forwarded onto the backend's MOSES_BASE_PATH-prefixed routes. The ROOT
# `location /api/` block therefore carries ${MOSES_BASE_PATH_PREFIX}/api/ as
# its proxy_pass URI. At a non-root base path the rendered root block must
# point at /apps/<t>/<s>/api/ (and /apps/<t>/<s>/__moses/ for fullstack-chat,
# whose backend mounts the mosesproxy InvokePath under basePath too).
for tmpl in fullstack-oidc/frontend fullstack-simple/frontend fullstack-chat/frontend fullstack-showcase/frontend; do
  assert "$tmpl" "moses-only" "" "" \
    "proxy_pass http://example-backend:8080/apps/t/x/api/;" \
    "" "/apps/t/x/"
done
assert "fullstack-chat/frontend" "moses-only" "" "" \
  "proxy_pass http://example-backend:8080/apps/t/x/__moses/;" \
  "" "/apps/t/x/"

# Regression: at the root base path the prefix is empty and the rendered
# root block must be byte-identical to the historical plain pass-through.
for tmpl in fullstack-oidc/frontend fullstack-simple/frontend fullstack-chat/frontend fullstack-showcase/frontend; do
  assert "$tmpl" "moses-only" "" "" \
    "proxy_pass http://example-backend:8080/api/;" \
    "proxy_pass http://example-backend:8080/apps" "/"
done
assert "fullstack-chat/frontend" "moses-only" "" "" \
  "proxy_pass http://example-backend:8080/__moses/;" \
  "" "/"

# CHAT-yfmwv: the sub-path API proxy MUST forward the prefix UNCHANGED. The
# backend registers its API + the mosesproxy InvokePath under MOSES_BASE_PATH
# (buildMux), so a `proxy_pass .../api/;` URI part — which makes nginx strip
# the matched location prefix — would 404 every API call through the ingress.
# Assert each real entrypoint carries the no-URI pass-through form.
for ep in fullstack-simple/frontend/entrypoint.sh fullstack-showcase/frontend/entrypoint.sh fullstack-chat/frontend/entrypoint.sh; do
  if grep -qF 'proxy_pass http://${BACKEND_SERVICE_HOST}:${BACKEND_SERVICE_PORT}/' "${REPO_ROOT}/${ep}"; then
    echo "FAIL: ${ep} sub-path proxy_pass carries a URI part — it strips MOSES_BASE_PATH and the backend 404s (CHAT-yfmwv)"
    FAIL=$((FAIL + 1))
  else
    PASS=$((PASS + 1))
  fi
done

echo
echo "Passed: $PASS  Failed: $FAIL"
[ "$FAIL" -eq 0 ]
