#!/usr/bin/env bash
set -euo pipefail

# clean_out_template.sh — one-shot demo strip for the fullstack-unified template.
#
# Replaces the demo feature code (the /api/v1/status endpoint + status-card
# UI) with clean twins from .template-clean/ that keep ALL Moses plumbing:
# base-path routing, health/openapi dual-mount, index.html templating
# ({{.ChartID}} placeholders + moses-config meta + browser-logger), tenant
# contract, CSRF + CORS middleware, embedding CSP, helm, config. The result
# builds and passes the remaining tests.
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

echo "==> Installing clean twins from .template-clean/ (demo endpoint + UI removed)"
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

Removed:  demo /api/v1/status endpoint (demo_routes.go stubbed), demo status
          card UI (static/index.html + app.js placeholders), demo openapi
          paths, demo platform actions.
Kept:     all Moses plumbing — base-path routing (registerAPIRoutes),
          health/openapi dual-mount, index.html templating ({{.ChartID}}
          placeholders, moses-config meta, browser-logger, <base href="./">),
          tenant contract, CSRF + CORS middleware, embedding CSP, helm chart.

Next steps:
  1. Edit moses-app.config.json — name / displayName / description.
  2. Add routes + handlers in demo_routes.go (rename freely;
     registerAPIRoutes in main.go is the single call site).
  3. Mirror every route in api/openapi.json (relative to /api/v1).
  4. Build the UI in static/ — fetch(base + '/api/v1/...') only, and mind
     the html/template {{ }} trap in index.html (skills/usage.md).
  5. Validate: go vet ./... && go test ./...
EOF
