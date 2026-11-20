#!/usr/bin/env bash
# Rendered-workload checks supplement static parity. No cluster required.
set -euo pipefail
cd "$(dirname "$0")/.."
if ! command -v helm >/dev/null; then echo 'SKIP: helm unavailable'; exit 0; fi
if ! command -v ruby >/dev/null; then echo 'ERROR: ruby (stdlib YAML) needed for Helm hardening checks'; exit 1; fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for t in frontend-template backend-template fullstack-simple fullstack-unified fullstack-showcase fullstack-chat fullstack-oidc; do
  args=(--namespace s2-test)
  case "$t" in
    fullstack-showcase|fullstack-chat|fullstack-oidc)
      if helm template check "$t/helm" > "$tmp/default" 2>&1; then echo "FAIL: $t accepted sentinel"; exit 1; fi
      grep -q 'Set postgresql.auth.password' "$tmp/default"
      if helm template check "$t/helm" --set-string postgresql.auth.password= > "$tmp/empty" 2>&1; then echo "FAIL: $t accepted empty password"; exit 1; fi
      grep -q 'Set postgresql.auth.password' "$tmp/empty"
      helm template check "$t/helm" --set postgresql.enabled=false > /dev/null
      args+=(--set-string postgresql.auth.password=render-test-password)
      ;;
    fullstack-simple)
      helm template check "$t/helm" > /dev/null
      if helm template check "$t/helm" --set redis.enabled=true > "$tmp/default" 2>&1; then echo 'FAIL: redis accepted sentinel'; exit 1; fi
      grep -q 'Set redis.auth.password' "$tmp/default"
      if helm template check "$t/helm" --set redis.enabled=true --set-string redis.auth.password= > "$tmp/empty" 2>&1; then echo 'FAIL: redis accepted empty password'; exit 1; fi
      grep -q 'Set redis.auth.password' "$tmp/empty"
      args+=(--set redis.enabled=true --set-string redis.auth.password=render-test-password)
      ;;
  esac
  helm lint "$t/helm" "${args[@]}"
  helm template check "$t/helm" "${args[@]}" > "$tmp/rendered"
  ruby tests/verify_helm_security.rb "$tmp/rendered" "$t"
  # Override booleans must not be swallowed by Helm's `default` function.
  helm template check "$t/helm" "${args[@]}" --set automountServiceAccountToken=true --set securityContext.readOnlyRootFilesystem=false > "$tmp/overrides"
  grep -q 'automountServiceAccountToken: true' "$tmp/overrides"
  grep -q 'readOnlyRootFilesystem: false' "$tmp/overrides"
  echo "PASS: $t render, guards, security and overrides"
done
