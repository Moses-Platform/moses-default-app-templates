#!/usr/bin/env bash
# CI gate: vendored browser-logger copies must be BYTE-IDENTICAL to the
# canonical shared/browser-logger/index.ts. Run from repo root.
#
# Exit 0 if all vendored copies are in sync. Exit non-zero with a diff if not.
#
# Unlike the mosesproxy-go gate, these copies carry NO per-file VENDORED COPY
# header — they are plain `cp` mirrors of the canonical TS file, so we compare
# the whole file with `cmp`.
#
# Note: fullstack-unified/static/moses-browser-logger.js is a hand-maintained
# vanilla-JS PORT of the same logic (different language), not a byte-copy, so
# it is intentionally NOT covered by this byte-identity gate.
#
# Drift remediation: copy shared/browser-logger/index.ts over each vendored
# path:
#   cp shared/browser-logger/index.ts <dst>

set -euo pipefail

CANONICAL="shared/browser-logger/index.ts"

if [[ ! -f "${CANONICAL}" ]]; then
  echo "error: canonical ${CANONICAL} not found"
  exit 2
fi

# Templates that vendor a byte-identical copy of the TS logger. Add entries
# here as more templates adopt.
VENDORED_PATHS=(
  "frontend-template/src/moses-browser-logger.ts"
  "fullstack-simple/frontend/src/moses-browser-logger.ts"
  "fullstack-chat/frontend/src/moses-browser-logger.ts"
  "fullstack-showcase/frontend/src/moses-browser-logger.ts"
  "fullstack-oidc/frontend/src/moses-browser-logger.ts"
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
  echo "Vendored browser-logger copies are out of sync with the canonical source."
  echo "Re-sync each with:"
  echo "  cp ${CANONICAL} <vendored-path>"
  exit 1
fi

echo "All vendored browser-logger copies are byte-identical to the canonical source."
exit 0
