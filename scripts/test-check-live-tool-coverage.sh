#!/usr/bin/env bash
#
# Regression tests for check-live-tool-coverage.sh.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-live-tool-coverage.sh"

tests_run=0
tests_failed=0

pass() {
  printf 'PASS: %s\n' "$1"
}

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  tests_failed=$((tests_failed + 1))
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"
  if grep -qF -- "$needle" <<< "$haystack"; then
    pass "$label"
  else
    fail "$label"
    printf '  missing: %s\n' "$needle" >&2
  fi
}

write_fixture() {
  local dir="$1"
  mkdir -p "$dir/docs" "$dir/tests" "$dir/internal/controlplane/postgres"
  cat > "$dir/docs/tool-catalog.json" <<'JSON'
{
  "tier1": [
    {"name": "clockify_list_tools"},
    {"name": "clockify_whoami"},
    {"name": "clockify_resolve_name"}
  ],
  "tier2": [
    {"name": "clockify_list_invoices"},
    {"name": "clockify_create_invoice"}
  ]
}
JSON
  cat > "$dir/tests/e2e_live_test.go" <<'EOF'
package e2e_test

func names() []string {
	return []string{
		"clockify_whoami",
		"clockify_resolve_name",
		"clockify_list_invoices",
		"clockify_create_invoice",
	}
}
EOF
  cat > "$dir/internal/controlplane/postgres/live_audit_phases_test.go" <<'EOF'
package postgres_test
EOF
}

run_case() {
  local name="$1"
  local expect_exit="$2"
  local expect_pattern="$3"
  local mutator="${4:-}"

  tests_run=$((tests_run + 1))

  local dir
  dir="$(mktemp -d "${TMPDIR:-/tmp}/test-live-tool-coverage.XXXXXX")"
  trap 'rm -rf "$dir"' RETURN
  write_fixture "$dir"
  if [ -n "$mutator" ]; then
    "$mutator" "$dir"
  fi

  local output
  local actual_exit=0
  output="$(bash "$script" --repo-root "$dir" 2>&1)" || actual_exit=$?

  if [ "$actual_exit" = "$expect_exit" ] && grep -qE -- "$expect_pattern" <<< "$output"; then
    pass "$name"
  else
    fail "$name"
    printf '  expected exit=%s got=%s pattern=%q\n' "$expect_exit" "$actual_exit" "$expect_pattern" >&2
    printf '  --- output ---\n%s\n  --- end ---\n' "$output" >&2
  fi

  rm -rf "$dir"
  trap - RETURN
}

tests_run=$((tests_run + 1))
plan_output="$(bash "$script" --plan --repo-root "$repo_root")"
assert_contains "$plan_output" "Live tool coverage plan" "plan prints header"
assert_contains "$plan_output" "all Tier-2 tools must be named" "plan documents Tier-2 gate"
assert_contains "$plan_output" "Allowed Tier-1 local helpers" "plan documents local-only helpers"
assert_contains "$plan_output" "does not replace scheduled cron evidence" "plan distinguishes cron evidence"

run_case "clean fixture clears the gate" 0 'Summary: 0 open, 0 unknown'

drop_tier2_ref() {
  perl -0pi -e 's/\n\t\t"clockify_create_invoice",//' "$1/tests/e2e_live_test.go"
}
run_case "missing Tier-2 live reference fails closed" 1 'Tier-2 catalog tools missing livee2e source references' drop_tier2_ref

drop_tier1_ref() {
  perl -0pi -e 's/\n\t\t"clockify_resolve_name",//' "$1/tests/e2e_live_test.go"
}
run_case "missing API-backed Tier-1 live reference fails closed" 1 'API-backed Tier-1 catalog tools missing livee2e source references' drop_tier1_ref

drop_local_only_ref() {
  # clockify_list_tools is a Tier-1 local catalog helper, so the fixture
  # intentionally does not need to mention it in live tests.
  :
}
run_case "local-only Tier-1 helper is explicitly allowed" 0 'local-only helpers allowed' drop_local_only_ref

add_unknown_ref() {
  perl -0pi -e 's/"clockify_create_invoice",/"clockify_create_invoice",\n\t\t"clockify_removed_tool",/' "$1/tests/e2e_live_test.go"
}
run_case "unknown live tool reference fails closed" 1 'livee2e source mentions unknown clockify_\* tool names' add_unknown_ref

if [ "$tests_failed" -ne 0 ]; then
  printf '\ncheck-live-tool-coverage tests: %d run, %d failed\n' "$tests_run" "$tests_failed" >&2
  exit 1
fi

printf '\ncheck-live-tool-coverage tests: %d run, 0 failed\n' "$tests_run"
