#!/usr/bin/env bash
# Body-limit source AND tests stay identical across the six self-contained apps.
# backend-template is canonical; only the unified package clause differs.
set -euo pipefail
cd "$(dirname "$0")/.."
for f in body_limit.go body_limit_test.go; do
  for d in fullstack-simple/backend/internal/middleware fullstack-showcase/backend/internal/middleware fullstack-chat/backend/internal/middleware fullstack-oidc/backend/internal/middleware fullstack-unified; do
    diff -u <(sed 's/^package main$/package middleware/' "backend-template/internal/middleware/$f") <(sed 's/^package main$/package middleware/' "$d/$f")
  done
done
# Shared logging implementation (backend-template additionally logs tenant context).
for f in logging.go logging_test.go; do
  for d in fullstack-simple/backend/internal/middleware fullstack-showcase/backend/internal/middleware fullstack-oidc/backend/internal/middleware fullstack-unified; do
    diff -u <(sed 's/^package main$/package middleware/' "fullstack-chat/backend/internal/middleware/$f") <(sed 's/^package main$/package middleware/' "$d/$f")
  done
done
echo 'All vendored runtime middleware and tests are in sync.'
