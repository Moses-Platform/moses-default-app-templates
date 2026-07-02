#!/usr/bin/env bash
#
# clean_out_template.sh — one-shot demo cleanout for the fullstack-chat template.
#
# Run this ONCE, from the template root, when you start building a real app
# on top of this template. It:
#
#   1. DELETES the demo feature code (entries feed: handlers, demo tests,
#      demo webhook sink, demo frontend tests).
#   2. REPLACES every mixed plumbing+demo file with its clean twin from
#      .template-clean/ (same paths, demo content swapped for minimal
#      placeholders + guidance comments; ALL Moses plumbing kept: path
#      contract, health/openapi dual-mount, webhook receiver,
#      /__moses/invoke proxy, X-Moses middleware, CSRF, browser-logger,
#      SDK script, base-path meta, theme init, helm, config).
#   3. Deletes .template-clean/ and finally ITSELF.
#
# The result builds and passes the remaining tests. Safe to re-run after a
# partial failure (rm -f / cp -f only). No network, no git.

set -euo pipefail
cd "$(dirname "$0")"

echo "==> fullstack-chat: cleaning out demo code"

# --- 1. Delete pure-demo files -------------------------------------------
rm -f backend/internal/handler/entries.go
rm -f backend/internal/handler/entries_test.go
rm -f backend/internal/handler/webhook_sink_demo.go
rm -f frontend/src/App.test.tsx

# --- 2. Copy clean twins over their targets ------------------------------
copy_twin() {
  # cp -f, parents guaranteed to exist (twins only shadow existing files)
  cp -f ".template-clean/$1" "$1"
  echo "    replaced $1"
}
copy_twin backend/cmd/server/demo_routes.go
copy_twin backend/cmd/server/main_test.go
copy_twin backend/internal/database/migrate_demo.go
copy_twin backend/api/openapi.json
copy_twin frontend/index.html
copy_twin frontend/src/App.tsx
copy_twin frontend/src/App.css
copy_twin frontend/src/api/client.ts
copy_twin frontend/src/api/hooks.ts
copy_twin frontend/src/api/queryKeys.ts
copy_twin moses-app.config.json

# --- 3. Self-destruct ------------------------------------------------------
rm -rf .template-clean
rm -f clean_out_template.sh

cat <<'EOF'

==> Demo cleaned out. What was removed:
    - entries feed demo (backend handlers + schema + demo tests, frontend
      feed UI + hooks + demo tests, demo platformActions)
    Kept (build on these): backend webhook receiver + CompletionSink,
    /__moses/invoke proxy, tenant gate (internal/handler/tenant.go),
    respond helpers, X-Moses middleware, CSRF, frontend src/moses/
    (invoke + hostMessages) with tests, TanStack Query scaffolding,
    browser-logger, helm chart, nginx/entrypoint path contract.

==> Next steps:
    1. Edit moses-app.config.json: displayName/description + rewrite the
       example platform action (or delete platformActions entirely).
    2. Add backend routes in backend/cmd/server/demo_routes.go and your
       schema in backend/internal/database/migrate_demo.go.
    3. Declare each new API path in backend/api/openapi.json (workspace-tool
       auto-discovery) and add UI in frontend/src/App.tsx.
    4. Verify:  cd backend && go vet ./... && go test ./...
                cd frontend && npm install && npm run lint && npm test && npm run build

    See skills/usage.md for the full KEEP / REPLACE / DELETE map and the
    env-var contract.
EOF
