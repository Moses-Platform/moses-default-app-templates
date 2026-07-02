#!/usr/bin/env bash
# CI gate: vendored base-path helper copies must be BYTE-IDENTICAL to the
# canonical shared/base-path/getBasePath.ts. Run from repo root.
#
# Exit 0 if all vendored copies are in sync. Exit non-zero with a diff if not.
#
# Like the browser-logger gate, these copies carry NO per-file VENDORED COPY
# header — they are plain `cp` mirrors of the canonical TS file, so we compare
# the whole file with `cmp`.
#
# Templates intentionally NOT covered:
#   - fullstack-simple: has no baseUrl helper (no client-side router basename).
#   - fullstack-unified: vanilla-JS frontend, no TS helper.
#   - fullstack-chat: deleted its copy (no router basename needed).
#
# Drift remediation: copy shared/base-path/getBasePath.ts over each vendored
# path:
#   cp shared/base-path/getBasePath.ts <dst>

set -euo pipefail

CANONICAL="shared/base-path/getBasePath.ts"

if [[ ! -f "${CANONICAL}" ]]; then
  echo "error: canonical ${CANONICAL} not found"
  exit 2
fi

# Templates that vendor a byte-identical copy of the base-path helper. Add
# entries here as more templates adopt.
VENDORED_PATHS=(
  "frontend-template/src/utils/baseUrl.ts"
  "fullstack-showcase/frontend/src/utils/baseUrl.ts"
  "fullstack-oidc/frontend/src/utils/baseUrl.ts"
)

drift=0

for v in "${VENDORED_PATHS[@]}"; do
  if [[ ! -f "${v}" ]]; then
    echo "error: vendored copy missing: ${v}"
    drift=1
    continue
  fi

  if ! cmp -s "${CANONICAL}" "${v}"; then
    echo "DRIFT in ${v} vs ${CANONICAL}:"
    diff -u "${CANONICAL}" "${v}" || true
    drift=1
  fi
done

if [[ ${drift} -ne 0 ]]; then
  echo ""
  echo "Vendored base-path copies are out of sync with the canonical source."
  echo "Re-sync each with:"
  echo "  cp ${CANONICAL} <vendored-path>"
  exit 1
fi

echo "All vendored base-path copies are byte-identical to the canonical source."
exit 0
