#!/usr/bin/env bash
#
# Regression tests for audit-branch-protection.sh. GitHub calls are
# stubbed so script-tests can verify the success and private-repo
# failure paths offline.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/audit-branch-protection.sh"

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

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"
  if grep -qF -- "$needle" <<< "$haystack"; then
    fail "$label"
    printf '  unexpected: %s\n' "$needle" >&2
  else
    pass "$label"
  fi
}

write_stub_gh() {
  local dir="$1"
  mkdir -p "$dir/bin"

  cat > "$dir/bin/gh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [ "$*" != "api repos/example/repo/branches/main/protection" ]; then
  printf 'unexpected gh args: %s\n' "$*" >&2
  exit 9
fi

if [ "${TEST_BRANCH_PROTECTION_FAIL:-0}" = "1" ]; then
  printf 'gh: Upgrade to GitHub Pro or make this repository public to enable this feature. (HTTP 403)\n' >&2
  exit 1
fi

cat <<'JSON'
{
  "required_pull_request_reviews": {"required_approving_review_count": 0},
  "required_status_checks": {
    "contexts": ["Format"],
    "checks": [
      {"context": "Doctor strict smoke", "app_id": 15368},
      {"context": "Shared-service Postgres E2E", "app_id": 15368}
    ]
  },
  "required_signatures": {"enabled": false},
  "required_linear_history": {"enabled": true},
  "enforce_admins": {"enabled": false},
  "allow_force_pushes": {"enabled": false},
  "allow_deletions": {"enabled": false},
  "required_conversation_resolution": {"enabled": true},
  "restrictions": null
}
JSON
EOF

  chmod +x "$dir/bin/gh"
}

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-audit-branch-protection.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_gh "$stub_dir"
success_output="$(GITHUB_REPOSITORY=example/repo PATH="$stub_dir/bin:$PATH" bash "$script")"
assert_contains "$success_output" '"Doctor strict smoke"' "success path projects required status checks"
assert_contains "$success_output" '"Shared-service Postgres E2E"' "success path projects required check objects"
assert_contains "$success_output" '"required_linear_history": true' "success path projects linear history"
rm -rf "$stub_dir"
trap - EXIT

tests_run=$((tests_run + 1))
stub_dir="$(mktemp -d "${TMPDIR:-/tmp}/test-audit-branch-protection-fail.XXXXXX")"
trap 'rm -rf "$stub_dir"' EXIT
write_stub_gh "$stub_dir"
failure_output=""
if failure_output="$(TEST_BRANCH_PROTECTION_FAIL=1 GITHUB_REPOSITORY=example/repo PATH="$stub_dir/bin:$PATH" bash "$script" 2>&1)"; then
  fail "private-repo API limitation exits non-zero"
else
  assert_contains "$failure_output" "unable to read main branch protection for example/repo" "private-repo failure names repo"
  assert_contains "$failure_output" "GitHub requires GitHub Pro or a public repository" "private-repo failure explains GitHub limitation"
  assert_not_contains "$failure_output" '"required_pull_request_reviews": null' "private-repo failure does not emit null snapshot"
fi
rm -rf "$stub_dir"
trap - EXIT

if [ "$tests_failed" -ne 0 ]; then
  printf '\naudit-branch-protection tests: %d run, %d failed\n' "$tests_run" "$tests_failed" >&2
  exit 1
fi

printf '\naudit-branch-protection tests: %d run, 0 failed\n' "$tests_run"
