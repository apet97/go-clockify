#!/usr/bin/env bash
#
# Regression tests for scripts/collect-license-evidence.sh. These use a
# PATH-stubbed `go` so the tests stay offline and do not depend on the
# caller's module cache.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/collect-license-evidence.sh"

tests_run=0
tests_failed=0

fail() {
  echo "FAIL: $*" >&2
  tests_failed=$((tests_failed + 1))
}

pass() {
  echo "PASS: $*"
}

make_fixture() {
  local dir="$1"
  mkdir -p "$dir/bin" "$dir/root" "$dir/dep-with-license" "$dir/dep-without-license" "$dir/cmd/clockify-mcp"
  printf 'MIT fixture\n' > "$dir/root/LICENSE"
  printf 'Apache fixture\n' > "$dir/dep-with-license/LICENSE.txt"
  cat > "$dir/bin/go" <<EOF
#!/usr/bin/env bash
set -euo pipefail
if [ "\${1:-}" != "list" ]; then
  echo "unexpected go command: \$*" >&2
  exit 2
fi
printf 'example.com/root||%s/root|true\n' "$dir"
printf 'example.com/dep-with-license|v1.2.3|%s/dep-with-license|false\n' "$dir"
printf 'example.com/dep-without-license|v9.9.9|%s/dep-without-license|false\n' "$dir"
EOF
  chmod +x "$dir/bin/go"
}

run_case() {
  local name="$1"
  shift
  tests_run=$((tests_run + 1))
  if "$@"; then
    pass "$name"
  else
    fail "$name"
  fi
}

case_plan_mentions_variants() {
  local dir out
  dir="$(mktemp -d "${TMPDIR:-/tmp}/license-evidence-test.XXXXXX")"
  # shellcheck disable=SC2064
  trap "rm -rf \"$dir\"" RETURN
  make_fixture "$dir"
  out="$(PATH="$dir/bin:$PATH" bash "$script" --repo-root "$dir" --plan)"
  grep -qF "License evidence plan" <<< "$out" &&
    grep -qF "grpc-postgres" <<< "$out" &&
    grep -qF "does not close B.08/L-10" <<< "$out"
}

case_run_reports_license_candidates() {
  local dir out
  dir="$(mktemp -d "${TMPDIR:-/tmp}/license-evidence-test.XXXXXX")"
  # shellcheck disable=SC2064
  trap "rm -rf \"$dir\"" RETURN
  make_fixture "$dir"
  out="$(PATH="$dir/bin:$PATH" bash "$script" --repo-root "$dir")"
  grep -qF "Variant: default" <<< "$out" &&
    grep -qF "example.com/dep-with-license" <<< "$out" &&
    grep -qF "license_candidates:" <<< "$out" &&
    grep -qF "dep-with-license/LICENSE.txt" <<< "$out" &&
    grep -qF "Summary:" <<< "$out"
}

case_fail_missing_license_exits_nonzero() {
  local dir out status
  dir="$(mktemp -d "${TMPDIR:-/tmp}/license-evidence-test.XXXXXX")"
  # shellcheck disable=SC2064
  trap "rm -rf \"$dir\"" RETURN
  make_fixture "$dir"
  status=0
  out="$(PATH="$dir/bin:$PATH" bash "$script" --repo-root "$dir" --fail-missing-license 2>&1)" || status=$?
  [ "$status" -ne 0 ] &&
    grep -qF "example.com/dep-without-license" <<< "$out" &&
    grep -qF "license_candidates: <none found>" <<< "$out"
}

run_case "plan names variants and legal boundary" case_plan_mentions_variants
run_case "run reports module license candidates" case_run_reports_license_candidates
run_case "strict mode fails on missing license candidate" case_fail_missing_license_exits_nonzero

echo
if [ "$tests_failed" -ne 0 ]; then
  printf 'collect-license-evidence tests: %d/%d FAILED\n' "$tests_failed" "$tests_run" >&2
  exit 1
fi
printf 'collect-license-evidence tests: %d/%d OK\n' "$tests_run" "$tests_run"
