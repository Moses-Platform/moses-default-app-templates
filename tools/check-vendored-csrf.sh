#!/usr/bin/env bash
# CI gate: vendored csrf-go copies must stay in lockstep with each other and
# (code-wise) with the canonical shared/ source. Run from repo root.
#
# Exit 0 if all vendored copies are in sync. Exit non-zero with a diff if not.
#
# Layout: shared/csrf-go/csrf.go (package csrf) is canonical; five templates
# carry a VENDORED COPY (package middleware, or package main for
# fullstack-unified). The ONLY intentional deltas between copies/canonical:
#   - the package clause (csrf / middleware / main);
#   - vendored helpers are namespaced to avoid collisions in flat packages:
#       isUnsafeMethod -> csrfIsUnsafeMethod
#       isCrossSite    -> csrfIsCrossSite
#   - doc-comment wording (vendored copies say "Compose it like the other
#     middleware in this package", and fullstack-unified's example line is
#     unqualified `RejectCrossSiteCSRF` vs `middleware.RejectCrossSiteCSRF`);
#   - the canonical csrf_test.go has an okHandler helper + 2 extra cases the
#     vendored tests lack (pre-existing structural drift). Canonical/vendored
#     TESTS are therefore intentionally NOT cross-compared here; Check A
#     keeps the five vendored tests in lockstep with each other instead.
#
# Drift remediation: fix shared/csrf-go/csrf.go first, re-apply the two helper
# renames above, copy the body into all five vendored copies (keeping each
# file's VENDORED COPY header + package clause), and mirror the test body
# across the five vendored csrf_test.go files. If you add a SIXTH vendored
# copy, extend VENDORED_COPIES/VENDORED_TESTS below and keep its deltas to the
# documented set — new intentional deltas need a new normalization rule here,
# not a silent exemption.
#
# Portability note: the helper-rename sed deliberately uses PLAIN SUBSTRING
# patterns (the identifiers are unique substrings) — GNU-only \b word
# boundaries silently perform no substitution under BSD/macOS sed and would
# false-fail every local run on a mac while passing in Ubuntu CI.

set -euo pipefail

CANONICAL="shared/csrf-go/csrf.go"

# Reference vendored copy: the other four are compared against it (Check A),
# and it alone is compared against the canonical (Check B).
REFERENCE="backend-template/internal/middleware/csrf.go"
REFERENCE_TEST="backend-template/internal/middleware/csrf_test.go"

VENDORED_COPIES=(
  "fullstack-simple/backend/internal/middleware/csrf.go"
  "fullstack-showcase/backend/internal/middleware/csrf.go"
  "fullstack-chat/backend/internal/middleware/csrf.go"
  "fullstack-unified/csrf.go"
)
VENDORED_TESTS=(
  "fullstack-simple/backend/internal/middleware/csrf_test.go"
  "fullstack-showcase/backend/internal/middleware/csrf_test.go"
  "fullstack-chat/backend/internal/middleware/csrf_test.go"
  "fullstack-unified/csrf_test.go"
)

if [[ ! -f "${CANONICAL}" ]]; then
  echo "error: canonical ${CANONICAL} not found"
  exit 2
fi
if [[ ! -f "${REFERENCE}" ]]; then
  echo "error: reference vendored copy ${REFERENCE} not found"
  exit 2
fi

drift=0

# body FILE — print everything after the `package ...` clause. Excludes the
# per-file preamble (VENDORED COPY header / package doc) and the package
# clause itself, which are the lines allowed to differ.
body() {
  awk 'started{print} /^package /{started=1}' "$1"
}

# normalize_qualifier — fold the one comment-level qualification delta
# (fullstack-unified's example line is `RejectCrossSiteCSRF`, the package
# middleware copies say `middleware.RejectCrossSiteCSRF`). Applied to BOTH
# sides of every Check A diff so the comparison stays strict otherwise.
normalize_qualifier() {
  sed 's/middleware\.RejectCrossSiteCSRF/RejectCrossSiteCSRF/'
}

# code_only FILE — body minus comment lines and blank lines (Check B is
# comment-insensitive: doc wording legitimately differs from canonical).
code_only() {
  body "$1" | awk '!/^[[:space:]]*\/\// && !/^[[:space:]]*$/'
}

# ---------------------------------------------------------------------------
# Check A — lockstep among the 5 vendored copies (strict, incl. comments).
# ---------------------------------------------------------------------------
ref_body="$(body "${REFERENCE}" | normalize_qualifier)"
for v in "${VENDORED_COPIES[@]}"; do
  if [[ ! -f "${v}" ]]; then
    echo "error: vendored copy missing: ${v}"
    drift=1
    continue
  fi
  v_body="$(body "${v}" | normalize_qualifier)"
  if [[ -z "${v_body}" ]]; then
    echo "error: ${v} has no 'package' clause; cannot diff"
    drift=1
    continue
  fi
  if ! diff -u <(echo "${ref_body}") <(echo "${v_body}") > /dev/null; then
    echo "DRIFT (Check A) in ${v} vs ${REFERENCE}:"
    diff -u <(echo "${ref_body}") <(echo "${v_body}") || true
    drift=1
  fi
done

ref_test_body="$(body "${REFERENCE_TEST}")"
for vt in "${VENDORED_TESTS[@]}"; do
  if [[ ! -f "${vt}" ]]; then
    echo "error: vendored test copy missing: ${vt}"
    drift=1
    continue
  fi
  vt_body="$(body "${vt}")"
  if ! diff -u <(echo "${ref_test_body}") <(echo "${vt_body}") > /dev/null; then
    echo "DRIFT (Check A, test) in ${vt} vs ${REFERENCE_TEST}:"
    diff -u <(echo "${ref_test_body}") <(echo "${vt_body}") || true
    drift=1
  fi
done

# ---------------------------------------------------------------------------
# Check B — reference vendored copy vs canonical (code lines only). The
# vendored helper names are folded back to the canonical ones with PLAIN
# SUBSTRING sed (BSD-sed compatible — see the portability note up top).
# csrf.go only; tests are intentionally excluded (structural drift, see
# header).
# ---------------------------------------------------------------------------
ref_code="$(code_only "${REFERENCE}" | sed 's/csrfIsUnsafeMethod/isUnsafeMethod/g; s/csrfIsCrossSite/isCrossSite/g')"
canonical_code="$(code_only "${CANONICAL}")"
if ! diff -u <(echo "${canonical_code}") <(echo "${ref_code}") > /dev/null; then
  echo "DRIFT (Check B) in ${REFERENCE} vs ${CANONICAL} (comment-insensitive, renames folded):"
  diff -u <(echo "${canonical_code}") <(echo "${ref_code}") || true
  drift=1
fi

if [[ ${drift} -ne 0 ]]; then
  echo ""
  echo "Vendored csrf-go copies are out of sync."
  echo "Fix shared/csrf-go/csrf.go first, re-apply the helper renames"
  echo "(isUnsafeMethod->csrfIsUnsafeMethod, isCrossSite->csrfIsCrossSite),"
  echo "then copy the body into all five vendored copies, preserving each"
  echo "file's VENDORED COPY header and package clause."
  exit 1
fi

echo "All vendored csrf-go copies are in sync."
exit 0
