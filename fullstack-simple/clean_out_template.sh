#!/usr/bin/env bash
set -euo pipefail

# clean_out_template.sh — one-shot demo strip for the fullstack-simple template.
#
# Deletes the demo feature code (items CRUD + status endpoint + demo UI) and
# swaps in clean twins from .template-clean/ that keep ALL Moses plumbing:
# base-path routing, health/openapi dual-mount, tenant contract (tenant.go),
# CSRF + CORS middleware, browser-logger, nginx CSP/entrypoint, helm, config.
# The result builds and passes the remaining tests.
#
# Run it ONCE from the template root before starting real work. It is
# idempotent-safe (rm -f / cp -f only; a re-run after a partial failure
# succeeds) and it DELETES ITSELF and .template-clean/ at the end.
# No network, no git, no writes outside this directory.

cd "$(dirname "$0")"

if [ ! -d .template-clean ]; then
  echo "clean_out_template.sh: .template-clean/ not found — already cleaned out? Nothing to do." >&2
  exit 1
fi

echo "==> Removing demo files"
rm -f backend/internal/handler/items.go
rm -f backend/internal/handler/items_test.go
rm -f backend/internal/handler/status.go

echo "==> Installing clean twins from .template-clean/"
# cp -p preserves permissions; targets are overwritten in place.
(cd .template-clean && find . -type f | while IFS= read -r f; do
  rel="${f#./}"
  mkdir -p "../$(dirname "$rel")"
  cp -f -p "$f" "../$rel"
done)

echo "==> Removing clean-out machinery (self-delete)"
rm -rf .template-clean
rm -f clean_out_template.sh

cat <<'EOF'

Template cleaned out.

Removed:  demo items CRUD + status endpoint (backend), demo UI (frontend),
          demo openapi paths, demo platform actions.
Kept:     all Moses plumbing — base-path routing (buildMux), health/openapi
          dual-mount, tenant contract (internal/handler/tenant.go), CSRF +
          CORS middleware, browser-logger, nginx CSP entrypoint, helm chart.

Next steps:
  1. Edit moses-app.config.json — name / displayName / description.
  2. Add backend routes in backend/cmd/server/demo_routes.go (rename freely;
     buildMux is the single call site).
  3. Mirror every route in backend/api/openapi.json (relative to /api/v1).
  4. Wire the frontend via src/api/client.ts + hooks.ts + queryKeys.ts.
  5. Validate:
       cd backend  && go vet ./... && go test ./...
       cd frontend && npm install && npm run lint && npm test && npm run build
EOF
