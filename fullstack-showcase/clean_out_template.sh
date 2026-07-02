#!/usr/bin/env bash
#
# clean_out_template.sh — one-shot demo strip for the fullstack-showcase
# template.
#
# Deletes all showcase/demo code (backend demo handlers + model, frontend
# demo pages/components, the showcase-overview skill) and copies clean
# minimal twins from .template-clean/ over the mixed files (demo routes,
# demo schema, OpenAPI paths, frontend data layer, app shell, configs).
# All Moses platform plumbing survives: path contract, health/openapi
# dual-mount, X-Moses middleware, CSRF, CORS allowlist, browser-logger,
# nginx CSP/entrypoint, Helm chart incl. PostgreSQL, tenant helpers,
# DB connect/retry/28P01 fail-fast.
#
# Run it ONCE from the template root. It removes .template-clean/ and
# DELETES ITSELF at the end. Re-running after a partial failure is safe
# (rm -f / cp -f only). No network, no git, no writes outside this dir.
set -euo pipefail

cd "$(dirname "$0")"

if [ ! -d .template-clean ]; then
  echo "ERROR: .template-clean/ not found — already cleaned out?" >&2
  exit 1
fi

echo "==> Removing demo backend code"
rm -f backend/internal/handler/moses_info.go
rm -f backend/internal/handler/capabilities.go
rm -f backend/internal/handler/capabilities_test.go
rm -f backend/internal/handler/notes.go
rm -f backend/internal/handler/notes_test.go
rm -f backend/internal/handler/notes_tenant_test.go
rm -f backend/internal/handler/users.go
rm -rf backend/internal/model

echo "==> Removing demo frontend code"
rm -rf frontend/src/pages
rm -f frontend/src/components/FeatureCard.tsx frontend/src/components/FeatureCard.css
rm -f frontend/src/components/FlowDiagram.tsx frontend/src/components/FlowDiagram.css
rm -f frontend/src/components/NotesPanel.tsx frontend/src/components/NotesPanel.css
rm -f frontend/src/components/UserList.tsx

echo "==> Removing demo skills"
rm -f skills/showcase-overview.md

echo "==> Installing clean twins"
# cp -p preserves perms; every twin path mirrors its target path.
(cd .template-clean && find . -type f -print0) | while IFS= read -r -d '' f; do
  target="${f#./}"
  mkdir -p "$(dirname "$target")"
  cp -f -p ".template-clean/$target" "$target"
  echo "    replaced $target"
done

echo "==> Removing cleanout machinery (self-delete)"
rm -rf .template-clean
rm -f clean_out_template.sh

cat <<'EOF'

Demo cleanout complete. Removed: showcase pages/components, demo handlers
(moses-info, capabilities, notes, users), demo model, demo schema, demo
config actions, showcase-overview skill.

Next steps:
  1. Edit moses-app.config.json identity fields (displayName, description).
  2. Add backend routes in backend/cmd/server/demo_routes.go and your
     schema in backend/internal/database/migrate_demo.go.
  3. Mirror routes in backend/api/openapi.json (paths relative to /api/v1;
     the cmd/server tests enforce spec<->mux consistency).
  4. Add frontend pages + hooks (src/api/client.ts, hooks.ts, queryKeys.ts).
  5. Validate:
       cd backend  && go vet ./... && go test ./...
       cd frontend && npm install && npm run lint && npm test && npm run build

Reference patterns (platform API key consumption, per-table tenant
rewrite, env-var contract): skills/usage.md
EOF
