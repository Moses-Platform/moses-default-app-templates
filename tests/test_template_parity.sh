#!/bin/sh
set -eu

# Template parity test — codifies the rules the comprehensive-review-agent
# verified before the templates-cleanup PR. Run from repo root or anywhere:
#
#   ./tests/test_template_parity.sh
#
# Exits non-zero on any failure. Companion to tests/test_nginx_entrypoint.sh.
#
# Each rule below is documented inline. When a rule fires a failure, the
# message names which file and what the offending content was so a human
# can fix it without re-reading this script.

REPO_ROOT="$(cd "$(dirname "$0")"/.. && pwd)"
cd "$REPO_ROOT"

PASS=0
FAIL=0

pass() {
  PASS=$((PASS + 1))
  echo "PASS: $1"
}

fail() {
  FAIL=$((FAIL + 1))
  echo "FAIL: $1"
}

# Templates and their per-folder paths. Frontend-only templates have no
# backend; the assertions below skip the missing paths gracefully.
TEMPLATES="frontend-template backend-template fullstack-simple fullstack-unified fullstack-showcase"

# ---------------------------------------------------------------------------
# 1. Helm Chart.yaml uniformity. The platform's deploy pipeline expects
#    every template's chart to be named exactly `agent-deployed-app` v1.0.0
#    so platform-injected values like nameOverride / images.<svc>.* hit the
#    expected paths.
# ---------------------------------------------------------------------------
for t in $TEMPLATES; do
  cy="$t/helm/Chart.yaml"
  if [ ! -f "$cy" ]; then
    fail "Chart.yaml: missing at $cy"
    continue
  fi
  name=$(awk '/^name:/ { print $2 }' "$cy" | tr -d '"' | tr -d "'")
  ver=$(awk '/^version:/ { print $2 }' "$cy" | tr -d '"' | tr -d "'")
  if [ "$name" = "agent-deployed-app" ]; then
    pass "Chart.yaml name: $cy → agent-deployed-app"
  else
    fail "Chart.yaml name: $cy has name=$name (expected agent-deployed-app)"
  fi
  if [ "$ver" = "1.0.0" ]; then
    pass "Chart.yaml version: $cy → 1.0.0"
  else
    fail "Chart.yaml version: $cy has version=$ver (expected 1.0.0)"
  fi
done

# ---------------------------------------------------------------------------
# 2. moses-app.config.json must NOT carry the `my-app` placeholder. The
#    template-side rename to `<template>-instance` removes the collision
#    risk for anyone who deploys a template standalone via `helm install`
#    without rewriting the file first. Per-template names are also a
#    sanity signal that someone actually customized the app config.
# ---------------------------------------------------------------------------
for t in $TEMPLATES; do
  cfg="$t/moses-app.config.json"
  if [ ! -f "$cfg" ]; then
    fail "moses-app.config.json: missing at $cfg"
    continue
  fi
  if grep -q '"name": *"my-app"' "$cfg"; then
    fail "moses-app.config.json: $cfg still has \"my-app\" placeholder"
  else
    pass "moses-app.config.json: $cfg has a real name"
  fi
done

# ---------------------------------------------------------------------------
# 3. README.md presence. fullstack-unified used to be missing one; the
#    parity rule keeps every template self-documenting for agents that
#    scaffold a new repo and need to know what they got.
# ---------------------------------------------------------------------------
for t in $TEMPLATES; do
  if [ -f "$t/README.md" ]; then
    lines=$(wc -l < "$t/README.md")
    if [ "$lines" -gt 20 ]; then
      pass "README.md: $t has $lines lines"
    else
      fail "README.md: $t has only $lines lines (expected > 20)"
    fi
  else
    fail "README.md: $t is missing"
  fi
done

# ---------------------------------------------------------------------------
# 4. Go module + Dockerfile version parity. The platform itself runs Go
#    1.24+; we keep the templates aligned so a template that adopts a
#    transitive dependency requiring `go 1.23+` doesn't fail builds.
#
#    All four `go.mod` files must declare exactly `go 1.24`. The check below
#    is exact-match — when bumping to 1.25 or later, update both
#    EXPECTED_GO_MAJOR_MINOR AND the Dockerfile golang base tag in this file
#    in the same PR (see rule #5 below).
# ---------------------------------------------------------------------------
EXPECTED_GO_MAJOR_MINOR="1.24"
for gm in $(find . -maxdepth 4 -name go.mod -not -path './node_modules/*' 2>/dev/null); do
  declared=$(awk '/^go [0-9]/ { print $2 }' "$gm")
  if [ "$declared" = "$EXPECTED_GO_MAJOR_MINOR" ]; then
    pass "go.mod: $gm → go $declared"
  else
    fail "go.mod: $gm has 'go $declared' (expected $EXPECTED_GO_MAJOR_MINOR)"
  fi
done

# ---------------------------------------------------------------------------
# 5. Dockerfile base image parity. Every Dockerfile that uses golang must
#    pin the same major.minor as go.mod. Every Dockerfile that uses alpine
#    runtime must pin the same alpine tag (3.19) so security scanning is
#    consistent.
# ---------------------------------------------------------------------------
EXPECTED_GOLANG_TAG="golang:1.24-alpine"
EXPECTED_ALPINE_TAG="alpine:3.19"

for df in $(find . -maxdepth 4 -name Dockerfile -not -path './node_modules/*' 2>/dev/null); do
  # Only check FROM lines that reference golang.
  if grep -qE '^FROM golang:' "$df"; then
    if grep -qE "^FROM $EXPECTED_GOLANG_TAG" "$df"; then
      pass "Dockerfile golang base: $df"
    else
      bad=$(grep -E '^FROM golang:' "$df" | head -1)
      fail "Dockerfile golang base: $df has '$bad' (expected $EXPECTED_GOLANG_TAG)"
    fi
  fi
  # Only check FROM lines that reference plain alpine (not nginx:alpine).
  if grep -qE '^FROM alpine:' "$df"; then
    if grep -qE "^FROM $EXPECTED_ALPINE_TAG" "$df"; then
      pass "Dockerfile alpine base: $df"
    else
      bad=$(grep -E '^FROM alpine:' "$df" | head -1)
      fail "Dockerfile alpine base: $df has '$bad' (expected $EXPECTED_ALPINE_TAG)"
    fi
  fi
done

# ---------------------------------------------------------------------------
# 6. Backend Dockerfile hardening: every backend (Go) Dockerfile must run
#    as a non-root user and declare a HEALTHCHECK. fullstack-unified used
#    to lack both — the parity rule prevents regression.
#
#    Frontend Dockerfiles are excluded: the nginx:alpine base image already
#    drops to a non-root worker after a brief root-uid startup window, and
#    the entrypoint chmods rendered config files for that worker.
# ---------------------------------------------------------------------------
for df in backend-template/Dockerfile fullstack-unified/Dockerfile fullstack-simple/backend/Dockerfile fullstack-showcase/backend/Dockerfile; do
  if [ ! -f "$df" ]; then
    fail "backend Dockerfile: missing at $df"
    continue
  fi
  if grep -qE '^USER appuser' "$df"; then
    pass "Dockerfile non-root user: $df"
  else
    fail "Dockerfile non-root user: $df is missing 'USER appuser' (USER directive must follow adduser)"
  fi
  if grep -qE '^HEALTHCHECK' "$df"; then
    pass "Dockerfile HEALTHCHECK: $df"
  else
    fail "Dockerfile HEALTHCHECK: $df is missing"
  fi
done

# ---------------------------------------------------------------------------
# 7. Reproducibility: backend Dockerfiles must NOT run `go mod tidy` at
#    build time. Tidy mutates go.sum; running it inside a Kaniko build
#    means each build can produce a different binary even with no source
#    change, and breaks air-gap registry mirrors that don't trust the
#    upstream module proxy. Use `go mod verify` instead.
# ---------------------------------------------------------------------------
for df in backend-template/Dockerfile fullstack-unified/Dockerfile fullstack-simple/backend/Dockerfile fullstack-showcase/backend/Dockerfile; do
  [ -f "$df" ] || continue
  # Strip comments before searching so the check only flags real RUN
  # directives, not commentary explaining why tidy was removed.
  if sed 's/#.*$//' "$df" | grep -qE 'go mod tidy'; then
    fail "Dockerfile reproducibility: $df runs 'go mod tidy' (use 'go mod verify' instead)"
  else
    pass "Dockerfile reproducibility: $df has no 'go mod tidy'"
  fi
done

# ---------------------------------------------------------------------------
# 8. No literal credentials in helm/values.yaml. The Moses platform's
#    SanitizeTemplateCredentials replaces __MOSES_GENERATE_PASSWORD__ with
#    a per-tenant random password during template injection. Anything else
#    (real passwords, the legacy `showcase-secret` literal, `changeme`,
#    `password`, etc.) is a security hazard if a user `helm install`s the
#    chart standalone or copies the value into another repo.
# ---------------------------------------------------------------------------
for t in $TEMPLATES; do
  vy="$t/helm/values.yaml"
  [ -f "$vy" ] || continue
  # Lines like `password: <something>` where <something> is not the
  # known-sentinel and not empty — flag them.
  bad_pw=$(awk -F: '/password:/ {
    val = $2
    sub(/#.*/, "", val)
    gsub(/^[ \t]+|[ \t]+$/, "", val)
    if (val != "" && val != "__MOSES_GENERATE_PASSWORD__" && val != "\"\"" && val != "\"__MOSES_GENERATE_PASSWORD__\"") {
      print val
    }
  }' "$vy" | head -3)
  if [ -n "$bad_pw" ]; then
    fail "helm/values.yaml: $vy contains a literal password value: $bad_pw (use __MOSES_GENERATE_PASSWORD__ or empty string)"
  else
    pass "helm/values.yaml: $vy has no literal password values"
  fi
done

# ---------------------------------------------------------------------------
# 9. nginx.conf must not emit X-XSS-Protection (deprecated/no-op in modern
#    browsers — Chrome 78+ ignores it; Firefox never implemented it).
#    Content-Security-Policy is the source of truth for framing/XSS.
# ---------------------------------------------------------------------------
for nc in $(find . -maxdepth 5 -name nginx.conf -not -path './node_modules/*' 2>/dev/null); do
  if grep -qiE 'X-XSS-Protection' "$nc"; then
    line=$(grep -niE 'X-XSS-Protection' "$nc" | head -1)
    # Allow comments that mention WHY it's omitted; only fail on actual
    # add_header directives.
    if echo "$line" | grep -qE 'add_header.*X-XSS-Protection'; then
      fail "nginx.conf: $nc emits X-XSS-Protection (deprecated): $line"
    else
      pass "nginx.conf: $nc mentions X-XSS-Protection only in a comment"
    fi
  else
    pass "nginx.conf: $nc has no X-XSS-Protection reference"
  fi
done

# ---------------------------------------------------------------------------
# 10. moses-app.config.json declares `templateApiVersion: "moses.ai/v1"` —
#     all 5 templates must be Moses-aware so the platform's Helm value
#     injection (basePath / embedding.*) takes effect.
# ---------------------------------------------------------------------------
for t in $TEMPLATES; do
  cfg="$t/moses-app.config.json"
  [ -f "$cfg" ] || continue
  if grep -q '"templateApiVersion": *"moses.ai/v1"' "$cfg"; then
    pass "templateApiVersion: $cfg → moses.ai/v1"
  else
    fail "templateApiVersion: $cfg is missing or not moses.ai/v1"
  fi
done

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
echo "----------------------------------------------------------"
echo "Template parity: $PASS passed, $FAIL failed"
echo "----------------------------------------------------------"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
