#!/usr/bin/env bash
# Body-limit source AND tests stay identical across the six self-contained apps.
# backend-template is canonical; only the unified package clause differs.
set -euo pipefail
cd "$(dirname "$0")/.."
for f in body_limit.go body_limit_test.go response_writer.go response_writer_test.go; do
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

# The same streamed-response assertions run through each app's production chain.
for d in fullstack-simple/backend/cmd/server fullstack-showcase/backend/cmd/server fullstack-chat/backend/cmd/server fullstack-oidc/backend/cmd/server fullstack-unified; do
  diff -u backend-template/cmd/server/streaming_test.go "$d/streaming_test.go"
done

# Entrypoints differ in routing; their body-limit validation must not drift.
for d in fullstack-simple/frontend fullstack-showcase/frontend fullstack-chat/frontend fullstack-oidc/frontend; do
  diff -u <(sed -n '/^# BEGIN shared request body limit/,/^# END shared request body limit/p' frontend-template/entrypoint.sh) <(sed -n '/^# BEGIN shared request body limit/,/^# END shared request body limit/p' "$d/entrypoint.sh")
done
echo 'All vendored runtime middleware, tests and body-limit validation are in sync.'
