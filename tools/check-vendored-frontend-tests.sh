#!/usr/bin/env bash
# Every copied scaffold must retain the same executable compiler regression test.
set -euo pipefail
cd "$(dirname "$0")/.."
for d in fullstack-simple/frontend fullstack-showcase/frontend fullstack-chat/frontend fullstack-oidc/frontend; do
  for f in CompilerFixture.jsx react-compiler.test.mjs; do
    diff -u "frontend-template/tests/$f" "$d/tests/$f"
  done
done
echo 'All frontend compiler fixtures and tests are in sync.'
