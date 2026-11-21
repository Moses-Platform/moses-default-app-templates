#!/usr/bin/env bash
# Rendered-workload checks supplement static parity. No cluster required.
set -euo pipefail
cd "$(dirname "$0")/.."
if ! command -v helm >/dev/null; then echo 'SKIP: helm unavailable'; exit 0; fi
if ! command -v ruby >/dev/null; then echo 'ERROR: ruby (stdlib YAML) needed for Helm hardening checks'; exit 1; fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for t in frontend-template backend-template fullstack-simple fullstack-unified fullstack-showcase fullstack-chat fullstack-oidc; do
  # Include every envFrom source together; the canonical signing Secret must win.
  args=(--namespace s2-test
    --set-string secrets.secretName=app-extra
    --set-string secrets.configMapName=app-config
    --set-string 'secrets.secretNames[0]=app-runtime'
    --set-string 'secrets.secretNames[1]=canonical-interapp')
  kind=none
  case "$t" in
    fullstack-showcase|fullstack-chat|fullstack-oidc) kind=postgresql ;;
    fullstack-simple) kind=redis ;;
  esac
  disabled=("${args[@]}")
  if [ "$kind" != none ]; then
    disabled+=(--set "$kind.enabled=false")
    if helm template check "$t/helm" "${args[@]}" --set "$kind.enabled=true" > "$tmp/missing" 2>&1; then
      echo "FAIL: $t accepted missing Secret reference"; exit 1
    fi
    grep -q "$kind.auth.existingSecret is required" "$tmp/missing"
    if helm template check "$t/helm" "${args[@]}" --set "$kind.enabled=true" --set-string "$kind.auth.existingSecret=" > "$tmp/empty" 2>&1; then
      echo "FAIL: $t accepted empty Secret reference"; exit 1
    fi
    grep -q "$kind.auth.existingSecret is required" "$tmp/empty"
    args+=(--set "$kind.enabled=true" --set-string "$kind.auth.existingSecret=bundled-credentials")
  fi
  helm lint "$t/helm" "${disabled[@]}"
  helm template check "$t/helm" "${disabled[@]}" > "$tmp/disabled"
  ruby tests/verify_helm_secret_refs.rb "$tmp/disabled" none password "$t"
  helm lint "$t/helm" "${args[@]}"
  helm template check "$t/helm" "${args[@]}" > "$tmp/rendered"
  ruby tests/verify_helm_security.rb "$tmp/rendered" "$t"
  ruby tests/verify_helm_secret_refs.rb "$tmp/rendered" "$kind" password "$t"
  if [ "$kind" != none ]; then
    # Prove both the custom key and explicit empty-key default reach all consumers.
    for key in custom-key ''; do
      helm template check "$t/helm" "${args[@]}" --set-string "$kind.auth.passwordKey=$key" > "$tmp/custom-key"
      ruby tests/verify_helm_secret_refs.rb "$tmp/custom-key" "$kind" "${key:-password}" "$t"
    done
  fi
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
