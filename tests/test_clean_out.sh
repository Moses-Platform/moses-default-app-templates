#!/bin/sh
set -eu

# Clean-out mechanism test — runs every template's clean_out_template.sh
# in a throwaway copy OUTSIDE the repo and asserts the contract from the
# clean-out spec (see root README § "Cleaning out the demo"):
#
#   - the script exits 0,
#   - the machinery self-deletes (clean_out_template.sh + .template-clean/),
#   - any Go module inside still builds (GOWORK=off go build ./...),
#   - key kept plumbing files survive (moses-app.config.json, helm/Chart.yaml).
#
# Deliberately dependency-light: no npm here (CI exercises the frontends'
# npm lint/test/build separately); Go is the only toolchain requirement,
# and Go-less templates are covered by the file assertions alone.
#
#   ./tests/test_clean_out.sh
#
# Exits non-zero on any failure. Companion to tests/test_template_parity.sh.

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

TEMPLATES="frontend-template backend-template fullstack-simple fullstack-unified fullstack-showcase fullstack-chat fullstack-oidc"

if ! command -v go >/dev/null 2>&1; then
  echo "note: go not installed — Go build assertions will be skipped"
fi

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

for t in $TEMPLATES; do
  if [ ! -d "$t" ]; then
    fail "$t: template directory missing"
    continue
  fi

  # Copy the template outside the repo. node_modules/ and dist/ are build
  # artifacts of local dev runs — irrelevant to the clean-out contract and
  # enormous, so they are excluded from the copy.
  dst="$WORK_DIR/$t"
  mkdir -p "$dst"
  (cd "$t" && tar -cf - --exclude 'node_modules' --exclude 'dist' .) | (cd "$dst" && tar -xf -)

  if [ ! -x "$dst/clean_out_template.sh" ]; then
    fail "$t: clean_out_template.sh missing or not executable in copy"
    continue
  fi

  # Run the script from the template root, capturing output for diagnosis.
  log="$WORK_DIR/$t.clean_out.log"
  if (cd "$dst" && ./clean_out_template.sh) > "$log" 2>&1; then
    pass "$t: clean_out_template.sh exited 0"
  else
    fail "$t: clean_out_template.sh exited non-zero — output follows"
    sed 's/^/    /' "$log"
    continue
  fi

  # Machinery must self-delete.
  if [ ! -e "$dst/clean_out_template.sh" ]; then
    pass "$t: clean_out_template.sh self-deleted"
  else
    fail "$t: clean_out_template.sh still present after run"
  fi
  if [ ! -e "$dst/.template-clean" ]; then
    pass "$t: .template-clean/ removed"
  else
    fail "$t: .template-clean/ still present after run"
  fi

  # Key kept plumbing files must survive the clean-out.
  for keep in moses-app.config.json helm/Chart.yaml; do
    if [ -f "$dst/$keep" ]; then
      pass "$t: kept plumbing file $keep survives"
    else
      fail "$t: kept plumbing file $keep is gone after clean-out"
    fi
  done

  # Every Go module inside the cleaned copy must still build standalone.
  if command -v go >/dev/null 2>&1; then
    for gm in "$dst/go.mod" "$dst/backend/go.mod"; do
      [ -f "$gm" ] || continue
      mod_dir=$(dirname "$gm")
      rel=${mod_dir#"$WORK_DIR/"}
      build_log="$WORK_DIR/$t.gobuild.log"
      if (cd "$mod_dir" && GOWORK=off go build ./...) > "$build_log" 2>&1; then
        pass "$t: GOWORK=off go build ./... in $rel"
      else
        fail "$t: go build failed in $rel — output follows"
        sed 's/^/    /' "$build_log"
      fi
    done
  fi
done

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
echo "----------------------------------------------------------"
echo "Clean-out mechanism: $PASS passed, $FAIL failed"
echo "----------------------------------------------------------"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
