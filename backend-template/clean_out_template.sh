#!/usr/bin/env bash
set -euo pipefail

# clean_out_template.sh — one-shot demo cleanout for backend-template.
#
# Removes the demo feature code (Item CRUD, platform-info endpoint and its
# Moses Platform API client) and swaps in minimal clean replacements from
# .template-clean/, keeping ALL Moses platform plumbing intact: base-path
# routing contract, /health + /api/openapi.json dual-mount, X-Moses header
# middleware, CSRF guard, embedding headers, tenant helpers, helm chart and
# moses-app.config.json. The result builds and its tests pass.
#
# Run it ONCE from the template root right after cloning. It deletes
# .template-clean/ and itself at the end. Re-running after a partial
# failure is safe (rm -f / cp -f only).

cd "$(dirname "$0")"

echo "==> Removing demo feature code"
rm -f internal/handler/items.go
rm -f internal/handler/items_tenant_test.go
rm -f internal/handler/platform.go
rm -rf internal/model
rm -rf internal/platform

# Guarded so a re-run after a partial earlier failure (twins already
# installed, .template-clean already gone) still succeeds.
if [ -d .template-clean ]; then
  echo "==> Installing clean replacements"
  cp -f .template-clean/cmd/server/demo_routes.go cmd/server/demo_routes.go
  cp -f .template-clean/cmd/server/main_test.go cmd/server/main_test.go
  cp -f .template-clean/api/openapi.json api/openapi.json
  cp -f .template-clean/moses-app.config.json moses-app.config.json
  cp -f .template-clean/moses-app.config.with-secrets.example.json moses-app.config.with-secrets.example.json
fi

echo "==> Removing clean-out machinery (one-shot)"
rm -rf .template-clean
rm -f clean_out_template.sh

cat <<'EOF'
Demo cleanout complete.

Removed: Item CRUD (handler/model/tests), platform-info endpoint + Moses
Platform API client, demo route registrations, demo OpenAPI paths, the
moses-platform integrations block.
Kept:    main.go plumbing (base-path contract, health/openapi dual-mount,
         middleware chain), internal/handler/{health,openapi,tenant}.go,
         internal/middleware/*, internal/config/*, helm/, Dockerfile.

Next steps:
  1. Edit moses-app.config.json — set name / displayName / description.
  2. Add your routes in cmd/server/demo_routes.go (registerDemoRoutes).
  3. Describe them in api/openapi.json (paths relative to /api/v1,
     no /health entry) — each operationId becomes an MCP tool.
  4. Add per-route tests in cmd/server/main_test.go.
  5. Validate: go vet ./... && go test ./...
EOF
