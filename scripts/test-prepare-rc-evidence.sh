#!/usr/bin/env bash
#
# Regression tests for scripts/prepare-rc-evidence.sh plan-mode
# behavior. These tests intentionally do not run Group 6/7 evidence
# commands or require a real candidate tag.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/prepare-rc-evidence.sh"

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
    if grep -qF -- "$needle" <<<"$haystack"; then
        pass "$label"
    else
        fail "$label"
        printf '  missing: %s\n' "$needle" >&2
    fi
}

tests_run=$((tests_run + 1))
plan_output="$(RC_EVIDENCE_DIR=/tmp/rc-evidence-fixture bash "$script" --plan v1.2.3-rc.1)"
assert_contains "$plan_output" "Release-candidate evidence plan for v1.2.3-rc.1" "plan names the candidate tag"
assert_contains "$plan_output" "make verify-vuln" "plan includes Group 6 vuln scan"
assert_contains "$plan_output" "semgrep scan --config p/default" "plan includes Group 6 semgrep"
assert_contains "$plan_output" "git grep -n -C 5 nosemgrep" "plan includes nosemgrep context capture"
assert_contains "$plan_output" "make release-check" "plan includes Group 7 release-check"
assert_contains "$plan_output" "release-smoke.yml" "plan includes release-smoke evidence"
assert_contains "$plan_output" "Manual variant evidence" "plan includes manual variant evidence"
assert_contains "$plan_output" "FIPS variant" "plan names FIPS variant follow-up"
assert_contains "$plan_output" "scripts/check-launch-evidence-gate.sh" "plan names launch evidence gate"

tests_run=$((tests_run + 1))
invalid_output=""
if invalid_output="$(bash "$script" --plan not-a-tag 2>&1)"; then
    fail "invalid tag shape fails"
else
    if grep -q "candidate tag must look like vX.Y.Z-rc.N" <<<"$invalid_output"; then
        pass "invalid tag shape fails"
    else
        fail "invalid tag reports actionable error"
        printf '%s\n' "$invalid_output" >&2
    fi
fi

tests_run=$((tests_run + 1))
bad_semver_output=""
if bad_semver_output="$(bash "$script" --plan v1.bad.3-rc.1 2>&1)"; then
    fail "malformed semver tag fails"
else
    if grep -q "candidate tag must look like vX.Y.Z-rc.N" <<<"$bad_semver_output"; then
        pass "malformed semver tag fails"
    else
        fail "malformed semver tag reports actionable error"
        printf '%s\n' "$bad_semver_output" >&2
    fi
fi

tests_run=$((tests_run + 1))
help_output="$(bash "$script" --help)"
assert_contains "$help_output" "--trigger-release-smoke" "help documents release-smoke dispatch"
assert_contains "$help_output" "RC_EVIDENCE_ALLOW_NON_TAG" "help documents rehearsal override"

if [ "$tests_failed" -ne 0 ]; then
    printf '\nprepare-rc-evidence tests: %d run, %d failed\n' "$tests_run" "$tests_failed" >&2
    exit 1
fi

printf '\nprepare-rc-evidence tests: %d run, 0 failed\n' "$tests_run"
