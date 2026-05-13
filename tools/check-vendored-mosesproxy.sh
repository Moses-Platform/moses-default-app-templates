#!/usr/bin/env bash
# CI gate: vendored mosesproxy-go copies must match the canonical shared/ source
# (modulo the VENDORED COPY header comment). Run from repo root.
#
# Exit 0 if all vendored copies are in sync. Exit non-zero with a diff if not.
#
# Drift remediation: copy shared/mosesproxy-go/proxy.go (and proxy_test.go) into
# the consumer's internal/mosesproxy/, preserving the VENDORED COPY header.

set -euo pipefail

CANONICAL_DIR="shared/mosesproxy-go"
CANONICAL_PROXY="${CANONICAL_DIR}/proxy.go"
CANONICAL_TEST="${CANONICAL_DIR}/proxy_test.go"

if [[ ! -f "${CANONICAL_PROXY}" ]]; then
  echo "error: canonical ${CANONICAL_PROXY} not found"
  exit 2
fi

# Templates that vendor the proxy. Add entries here as more templates adopt.
VENDORED_PATHS=(
  "fullstack-chat/backend/internal/mosesproxy"
)

drift=0

for v in "${VENDORED_PATHS[@]}"; do
  vproxy="${v}/proxy.go"
  vtest="${v}/proxy_test.go"

  if [[ ! -f "${vproxy}" ]]; then
    echo "error: vendored copy missing: ${vproxy}"
    drift=1
    continue
  fi

  # Compare both files from the `package mosesproxy` line onward. The
  # vendored file's preamble is the VENDORED COPY header + the same
  # package-level doc as the canonical (one contiguous // block in both
  # cases); diffing from the package clause down ignores both headers
  # uniformly and surfaces any code drift.
  vendored_body=$(awk 'started{print} /^package mosesproxy/{started=1; print}' "${vproxy}")
  canonical_body=$(awk 'started{print} /^package mosesproxy/{started=1; print}' "${CANONICAL_PROXY}")

  if [[ -z "${vendored_body}" ]]; then
    echo "error: ${vproxy} has no 'package mosesproxy' clause; cannot diff"
    drift=1
    continue
  fi

  if ! diff -u <(echo "${canonical_body}") <(echo "${vendored_body}") > /dev/null; then
    echo "DRIFT in ${vproxy} vs ${CANONICAL_PROXY}:"
    diff -u <(echo "${canonical_body}") <(echo "${vendored_body}") || true
    drift=1
  fi

  if [[ -f "${CANONICAL_TEST}" ]]; then
    if [[ ! -f "${vtest}" ]]; then
      echo "error: vendored test copy missing: ${vtest}"
      drift=1
      continue
    fi
    if ! diff -u "${CANONICAL_TEST}" "${vtest}" > /dev/null; then
      echo "DRIFT in ${vtest} vs ${CANONICAL_TEST}:"
      diff -u "${CANONICAL_TEST}" "${vtest}" || true
      drift=1
    fi
  fi
done

if [[ ${drift} -ne 0 ]]; then
  echo ""
  echo "Vendored mosesproxy-go copies are out of sync with the canonical source."
  echo "Re-sync with:"
  echo "  cp ${CANONICAL_PROXY} fullstack-chat/backend/internal/mosesproxy/proxy.go  # preserve VENDORED COPY header"
  exit 1
fi

echo "All vendored mosesproxy-go copies are in sync with the canonical source."
exit 0
