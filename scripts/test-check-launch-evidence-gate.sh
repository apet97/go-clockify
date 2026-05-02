#!/usr/bin/env bash
#
# test-check-launch-evidence-gate.sh — regression test for
# check-launch-evidence-gate.sh.
#
# Contract assertions:
#   1. Pass: the real checklist (all evidence-required boxes unchecked) → exit 0
#   2. Fail: a checked box without evidence annotation → exit 1
#   3. Pass: a checked box with _Closed_ annotation → exit 0
#   4. Pass: checked box with GitHub Actions run URL → exit 0
#   5. Pass: checked box with workflow_run_id evidence → exit 0
#   6. Fail: missing checklist file → exit 1

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-launch-evidence-gate.sh"
real_checklist="$repo_root/docs/launch-candidate-checklist.md"

if [ ! -f "$script" ]; then
    echo "FAIL: script not found at $script" >&2
    exit 1
fi

tests_run=0
tests_failed=0

pass()  { tests_run=$((tests_run + 1)); echo "  PASS: $1"; }
fail()  { tests_run=$((tests_run + 1)); tests_failed=$((tests_failed + 1)); echo "  FAIL: $1"; }

# ── Test 1: real checklist passes ──────────────────────────────────

echo "== Test 1: real checklist (all evidence boxes unchecked) => OK"
if LAUNCH_CHECKLIST="$real_checklist" bash "$script" >/dev/null 2>&1; then
  pass "real checklist passes (boxes unchecked)"
else
  fail "real checklist should pass but exited non-zero"
fi

# ── Test 2: checked box without evidence => FAIL ───────────────────

echo "== Test 2: checked box without evidence annotation => FAIL"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
sed 's/^- \[ \] Two consecutive nightly runs green with no flakes/- [x] Two consecutive nightly runs green with no flakes/' \
  "$real_checklist" > "$tmp"
if LAUNCH_CHECKLIST="$tmp" bash "$script" >/dev/null 2>&1; then
  fail "checked box without evidence should fail but exited 0"
else
  pass "checked box without evidence fails"
fi

# ── Test 3: checked box with _Closed_ annotation => OK ─────────────

echo "== Test 3: checked box with _Closed_ annotation => OK"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
sed 's/^- \[ \] Two consecutive nightly runs green with no flakes/- [x] Two consecutive nightly runs green with no flakes\
      _Closed 2026-05-03 by commit abc1234_/' \
  "$real_checklist" > "$tmp"
if LAUNCH_CHECKLIST="$tmp" bash "$script" >/dev/null 2>&1; then
  pass "checked box with _Closed_ annotation passes"
else
  fail "checked box with _Closed_ annotation should pass but exited non-zero"
fi

# ── Test 4: checked box with workflow run URL => OK ────────────────

echo "== Test 4: checked box with workflow run URL => OK"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
sed 's/^- \[ \] Two consecutive nightly runs green with no flakes/- [x] Two consecutive nightly runs green with no flakes\
      https:\/\/github.com\/apet97\/go-clockify\/actions\/runs\/25240000001/' \
  "$real_checklist" > "$tmp"
if LAUNCH_CHECKLIST="$tmp" bash "$script" >/dev/null 2>&1; then
  pass "checked box with workflow run URL passes"
else
  fail "checked box with workflow run URL should pass but exited non-zero"
fi

# ── Test 5: checked box with workflow_run_id evidence => OK ──────────

echo "== Test 5: checked box with workflow_run_id evidence => OK"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
sed 's/^- \[ \] Two consecutive nightly runs green with no flakes/- [x] Two consecutive nightly runs green with no flakes\
      workflow_run_id: 25240000001/' \
  "$real_checklist" > "$tmp"
if LAUNCH_CHECKLIST="$tmp" bash "$script" >/dev/null 2>&1; then
  pass "checked box with workflow_run_id evidence passes"
else
  fail "checked box with workflow_run_id evidence should pass but exited non-zero"
fi

# ── Test 6: missing checklist file => FAIL ───────────────────────────

echo "== Test 6: missing checklist file => FAIL"
if LAUNCH_CHECKLIST="/nonexistent/checklist.md" bash "$script" >/dev/null 2>&1; then
  fail "missing checklist should fail but exited 0"
else
  pass "missing checklist fails"
fi

# ── Test 7: Group 7 box checked without evidence => FAIL ────────────

echo "== Test 7: Group 7 box checked without evidence => FAIL"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
sed 's/^- \[ \] All required workflows on .main. green: .ci.yml./- [x] All required workflows on .main. green: .ci.yml./' \
  "$real_checklist" > "$tmp"
if LAUNCH_CHECKLIST="$tmp" bash "$script" >/dev/null 2>&1; then
  fail "Group 7 checked box without evidence should fail but exited 0"
else
  pass "Group 7 checked box without evidence fails"
fi

# ── Summary ────────────────────────────────────────────────────────

echo ""
echo "Tests run: $tests_run, failures: $tests_failed"
if [ "$tests_failed" -gt 0 ]; then
  echo "FAIL" >&2
  exit 1
fi
echo "OK"
