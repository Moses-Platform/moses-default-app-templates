#!/usr/bin/env bash
set -euo pipefail

# clean_out_template.sh — one-shot demo cleanout for frontend-template.
#
# Removes the demo marketing pages (Home/About/404) and swaps in minimal
# clean replacements from .template-clean/, keeping ALL Moses platform
# plumbing intact: moses-base-path meta + router basename, no-flash theme
# init, TanStack Query data layer (client/queryClient/queryKeys/hooks),
# browser-logger, nginx CSP/entrypoint, helm and moses-app.config.json.
# The result builds and its tests pass.
#
# Run it ONCE from the template root right after cloning. It deletes
# .template-clean/ and itself at the end. Re-running after a partial
# failure is safe (rm -f / cp -f only).

cd "$(dirname "$0")"

echo "==> Removing demo pages"
rm -rf src/pages

# Guarded so a re-run after a partial earlier failure (twins already
# installed, .template-clean already gone) still succeeds.
if [ -d .template-clean ]; then
  echo "==> Installing clean replacements"
  cp -f .template-clean/src/App.tsx src/App.tsx
  cp -f .template-clean/src/App.css src/App.css
  cp -f .template-clean/src/components/Layout.tsx src/components/Layout.tsx
  cp -f .template-clean/src/api/client.ts src/api/client.ts
  cp -f .template-clean/src/api/hooks.ts src/api/hooks.ts
  cp -f .template-clean/src/api/queryKeys.ts src/api/queryKeys.ts
  cp -f .template-clean/index.html index.html
  cp -f .template-clean/moses-app.config.json moses-app.config.json
fi

echo "==> Removing clean-out machinery (one-shot)"
rm -rf .template-clean
rm -f clean_out_template.sh

cat <<'EOF'
Demo cleanout complete.

Removed: Home/About/404 demo pages (+ their CSS), the demo health read
(getHealth/useHealth), the demo "About" quickAction, demo class hooks in
App.css.
Kept:    main.tsx (router basename + QueryClientProvider + browser-logger),
         src/utils/baseUrl.ts, src/api/ transport + key factory, Layout +
         ThemeToggle, styles/theme.css tokens, nginx.conf + entrypoint.sh,
         Dockerfile, helm/.

Next steps:
  1. Edit moses-app.config.json — set name / displayName / description.
  2. Set your app title in index.html and src/components/Layout.tsx.
  3. Add pages under src/pages/ and routes in src/App.tsx
     (use <Link to="...">, never raw absolute <a href="/...">).
  4. Add typed reads/writes in src/api/client.ts + hooks in src/api/hooks.ts
     (relative paths only: 'api/v1/...').
  5. Validate: npm run lint && npm test && npm run build
EOF
