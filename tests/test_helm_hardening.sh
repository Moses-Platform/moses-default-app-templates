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
  case "$t" in
    frontend-template|backend-template|fullstack-unified)
      key=env
      container=agent-deployed-app
      if [ "$t" = fullstack-unified ]; then key=globalEnv; container=app; fi
      ruby tests/verify_helm_body_limit.rb "$tmp/rendered" 1048576 "$container"
      helm template check "$t/helm" "${args[@]}" --set-string "$key.MOSES_MAX_BODY_BYTES=52428800" > "$tmp/body-limit"
      ruby tests/verify_helm_body_limit.rb "$tmp/body-limit" 52428800 "$container"
      ;;
    fullstack-simple|fullstack-showcase|fullstack-chat|fullstack-oidc)
      ruby tests/verify_helm_body_limit.rb "$tmp/rendered" 1048576
      # Retained service-local values must fail, even with a global override.
      for service in frontend backend; do
        for value in 52428800 '' 0; do
          ruby -ryaml -e 'v=YAML.load_file(ARGV[0]); s=v.fetch("services").find { |s| s["name"] == ARGV[1] }; (s["env"] ||= {})["MOSES_MAX_BODY_BYTES"]=ARGV[2]; puts({"services"=>v["services"]}.to_yaml)' "$t/helm/values.yaml" "$service" "$value" > "$tmp/service-override.yaml"
          for global in '' 52428800; do
            override=("${args[@]}")
            if [ -n "$global" ]; then override+=(--set-string "globalEnv.MOSES_MAX_BODY_BYTES=$global"); fi
            if helm template check "$t/helm" "${override[@]}" -f "$tmp/service-override.yaml" > "$tmp/legacy-limit" 2>&1; then
              echo "FAIL: $t accepted $service service-local body limit '$value' (global='$global')"; exit 1
            fi
            grep -Fq 'move MOSES_MAX_BODY_BYTES to globalEnv: one value now applies to the frontend proxy and the backend' "$tmp/legacy-limit"
          done
        done
      done
      # Move the retained 50 MiB value to globalEnv; both containers receive it.
      helm template check "$t/helm" "${args[@]}" --set-string globalEnv.MOSES_MAX_BODY_BYTES=52428800 > "$tmp/body-limit"
      ruby tests/verify_helm_body_limit.rb "$tmp/body-limit" 52428800
      ;;
  esac
  echo "PASS: $t render, guards, security and overrides"
done
