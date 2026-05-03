#!/usr/bin/env bash
#
# Regression tests for scripts/claude-campaign.sh dry-run planning.
# These tests intentionally do not create git worktrees or open iTerm.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/claude-campaign.sh"

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

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"
  if grep -qF -- "$needle" <<<"$haystack"; then
    fail "$label"
    printf '  unexpected: %s\n' "$needle" >&2
  else
    pass "$label"
  fi
}

tests_run=$((tests_run + 1))
plan_output="$(cd "$repo_root" && bash "$script" --dry-run --campaign-id fixture)"
assert_contains "$plan_output" "claude-campaign plan" "dry-run prints plan header"
assert_contains "$plan_output" "model=opus" "dry-run defaults to Opus"
assert_contains "$plan_output" "lane=code-quality branch=campaign/fixture/code-quality" "dry-run includes code-quality lane"
assert_contains "$plan_output" "lane=performance branch=campaign/fixture/performance" "dry-run includes performance lane"
assert_contains "$plan_output" "lane=stability branch=campaign/fixture/stability" "dry-run includes stability lane"
assert_contains "$plan_output" "lane=observability branch=campaign/fixture/observability" "dry-run includes observability lane"
assert_contains "$plan_output" "lane=ai-agent-ux branch=campaign/fixture/ai-agent-ux" "dry-run includes AI-agent UX lane"
assert_contains "$plan_output" "claude --model opus" "dry-run command invokes Claude with Opus"
assert_contains "$plan_output" '$(cat ' "dry-run command inserts prompt file content"

tests_run=$((tests_run + 1))
subset_output="$(cd "$repo_root" && bash "$script" --dry-run --campaign-id fixture2 --lanes performance --model opus)"
assert_contains "$subset_output" "lane=performance branch=campaign/fixture2/performance" "subset dry-run includes requested lane"
assert_not_contains "$subset_output" "lane=code-quality" "subset dry-run excludes unrequested lane"

tests_run=$((tests_run + 1))
invalid_lane_output=""
if invalid_lane_output="$(cd "$repo_root" && bash "$script" --dry-run --campaign-id fixture --lanes unknown 2>&1)"; then
  fail "invalid lane fails"
else
  if grep -qF "unknown lane 'unknown'" <<<"$invalid_lane_output"; then
    pass "invalid lane fails"
  else
    fail "invalid lane reports actionable error"
    printf '%s\n' "$invalid_lane_output" >&2
  fi
fi

tests_run=$((tests_run + 1))
invalid_id_output=""
if invalid_id_output="$(cd "$repo_root" && bash "$script" --dry-run --campaign-id '../bad' 2>&1)"; then
  fail "invalid campaign id fails"
else
  if grep -qF "campaign id must contain only" <<<"$invalid_id_output"; then
    pass "invalid campaign id fails"
  else
    fail "invalid campaign id reports actionable error"
    printf '%s\n' "$invalid_id_output" >&2
  fi
fi

if [ "$tests_failed" -ne 0 ]; then
  printf '\nclaude-campaign tests: %d run, %d failed\n' "$tests_run" "$tests_failed" >&2
  exit 1
fi

printf '\nclaude-campaign tests: %d run, 0 failed\n' "$tests_run"
