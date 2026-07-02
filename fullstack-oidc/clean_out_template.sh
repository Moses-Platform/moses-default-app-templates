#!/usr/bin/env bash
# clean_out_template.sh — one-shot demo strip for the fullstack-oidc template.
#
# Deletes the demo feature code (entries, shared-notes, admin-area,
# public-info + the seven demo pages) and copies clean minimal twins from
# .template-clean/ over the mixed files, leaving all Moses platform plumbing
# intact: the vendored oidcauth middleware, /api/v1/me session introspection,
# the auth pill + silent-SSO bootstrap, health/openapi dual-mounts, X-Moses
# middleware, browser-logger, nginx/entrypoint CSP, helm chart, and config.
# The result builds and passes the remaining tests.
#
# Run it ONCE from the template root, before writing any code. It is
# idempotent-safe (rm -f / cp -f only; re-running after a partial failure
# succeeds) and DELETES ITSELF (plus .template-clean/) at the end.
set -euo pipefail

cd "$(dirname "$0")"

if [ ! -d .template-clean ]; then
  echo "clean_out_template.sh: .template-clean/ not found — already cleaned?" >&2
  exit 1
fi

echo "==> Removing demo backend code"
rm -f backend/cmd/server/demo_routes.go # replaced by stub twin below
rm -f backend/internal/handler/entries.go
rm -f backend/internal/handler/shared.go
rm -f backend/internal/handler/shared_test.go
rm -f backend/internal/handler/demo_handlers.go
rm -f backend/internal/handler/demo_handlers_test.go
rm -f backend/internal/database/migrate_demo.go # replaced by empty-schema twin below

echo "==> Removing demo frontend code"
rm -rf frontend/src/pages
rm -f frontend/src/components/NavIcons.tsx

echo "==> Removing demo skill doc"
rm -f skills/oidc-overview.md

echo "==> Installing clean twins"
cp -f .template-clean/backend/cmd/server/demo_routes.go backend/cmd/server/demo_routes.go
cp -f .template-clean/backend/cmd/server/main_test.go backend/cmd/server/main_test.go
cp -f .template-clean/backend/internal/database/migrate_demo.go backend/internal/database/migrate_demo.go
cp -f .template-clean/backend/api/openapi.json backend/api/openapi.json
cp -f .template-clean/frontend/src/App.tsx frontend/src/App.tsx
cp -f .template-clean/frontend/src/App.test.tsx frontend/src/App.test.tsx
cp -f .template-clean/frontend/src/components/Layout.tsx frontend/src/components/Layout.tsx
cp -f .template-clean/frontend/src/api/client.ts frontend/src/api/client.ts
cp -f .template-clean/frontend/src/api/hooks.ts frontend/src/api/hooks.ts
cp -f .template-clean/frontend/src/api/queryKeys.ts frontend/src/api/queryKeys.ts
cp -f .template-clean/frontend/index.html frontend/index.html
cp -f .template-clean/moses-app.config.json moses-app.config.json

echo "==> Self-cleanup"
rm -rf .template-clean
rm -f clean_out_template.sh

cat <<'EOF'

Demo cleaned out. What remains is a minimal, fully-integrated OIDC
relying-party app (oidcauth middleware, /api/v1/me, auth pill, silent SSO,
helm, config). Next steps:

  1. Edit moses-app.config.json identity (name, displayName, description)
     and access.oidc lists as you add routes.
  2. Add backend routes in backend/cmd/server/demo_routes.go (now a stub)
     and handlers in backend/internal/handler/.
  3. Add your schema in backend/internal/database/migrate_demo.go.
  4. Add frontend pages under src/pages/, routes in src/App.tsx, data
     hooks in src/api/ (client.ts + hooks.ts + queryKeys.ts).
  5. Update backend/api/openapi.json paths for every route you add.
  6. Verify:
       cd backend  && go vet ./... && go test ./...
       cd frontend && npm install && npm run lint && npm test && npm run build

See skills/usage.md for the env-var contract and the two common pitfalls
(gateway-auth marker, session-lifetime cliff).
EOF
