#!/usr/bin/env bash
#
# test-check-launch-evidence-gate.sh — regression test for
# check-launch-evidence-gate.sh.
#
# Contract assertions:
#   1. Pass: the real checklist (open external boxes unchecked, or
#      checked only when carrying evidence) → exit 0
#   2. Fail: a checked box without evidence annotation → exit 1
#   3. Pass: a checked box with _Closed_ annotation → exit 0
#   4. Pass: checked box with GitHub Actions run URL → exit 0
#   5. Pass: checked box with workflow_run_id evidence → exit 0
#   6. Fail: missing checklist file → exit 1
#   7. Fail: Group 7 checked box without evidence → exit 1
#   8. Fail: Group 6 checked box without evidence → exit 1
#   9. Fail: Group 7 release-artifact box checked without evidence → exit 1

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

echo "== Test 1: real checklist (external evidence boxes guarded) => OK"
if LAUNCH_CHECKLIST="$real_checklist" bash "$script" >/dev/null 2>&1; then
  pass "real checklist passes"
else
  fail "real checklist should pass but exited non-zero"
fi

# ── Test 2: checked box without evidence => FAIL ───────────────────

echo "== Test 2: checked box without evidence annotation => FAIL"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat > "$tmp" <<'EOF'
- [x] Two consecutive nightly runs green with no flakes
      _Tracking: evidence intentionally missing._
EOF
if LAUNCH_CHECKLIST="$tmp" bash "$script" >/dev/null 2>&1; then
  fail "checked box without evidence should fail but exited 0"
else
  pass "checked box without evidence fails"
fi

# ── Test 3: checked box with _Closed_ annotation => OK ─────────────

echo "== Test 3: checked box with _Closed_ annotation => OK"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat > "$tmp" <<'EOF'
- [x] Two consecutive nightly runs green with no flakes
      _Closed 2026-05-03 by commit abc1234_
EOF
if LAUNCH_CHECKLIST="$tmp" bash "$script" >/dev/null 2>&1; then
  pass "checked box with _Closed_ annotation passes"
else
  fail "checked box with _Closed_ annotation should pass but exited non-zero"
fi

# ── Test 4: checked box with workflow run URL => OK ────────────────

echo "== Test 4: checked box with workflow run URL => OK"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat > "$tmp" <<'EOF'
- [x] Two consecutive nightly runs green with no flakes
      https://github.com/apet97/go-clockify/actions/runs/25240000001
EOF
if LAUNCH_CHECKLIST="$tmp" bash "$script" >/dev/null 2>&1; then
  pass "checked box with workflow run URL passes"
else
  fail "checked box with workflow run URL should pass but exited non-zero"
fi

# ── Test 5: checked box with workflow_run_id evidence => OK ──────────

echo "== Test 5: checked box with workflow_run_id evidence => OK"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat > "$tmp" <<'EOF'
- [x] Two consecutive nightly runs green with no flakes
      workflow_run_id: 25240000001
EOF
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
#
# After the post-v1.2.1 closure-annotation pass, the real Group 7
# "All required workflows on `main` green" row in
# launch-candidate-checklist.md carries an inline `_Closed
# YYYY-MM-DD …_` annotation (the row stays unchecked per the
# operator's "do not tick" rule, but it has the proper evidence
# annotation alongside). The historical `[ ]` -> `[x]` substitution
# against the real checklist no longer reproduces the
# "checked-without-evidence" anti-pattern because the gate sees the
# annotation and passes. Construct a minimal synthetic checklist
# instead — a checked Group 7 box with no evidence URL or `_Closed_`
# annotation within 8 lines — so the negative test's invariant
# ("checked Group 7 box without evidence must fail the gate") stays
# load-bearing regardless of the real checklist's state. Mirrors the
# Test 8 / Test 9 synthetic-fixture pattern.

echo "== Test 7: Group 7 box checked without evidence => FAIL"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat > "$tmp" <<'EOF'
# Launch candidate checklist — synthetic fixture for evidence-gate test 7

## 7. CI / release readiness

- [x] All required workflows on `main` green: `ci.yml`, `codeql.yml`,
      `dependency-review.yml`, `mutation.yml`, `reproducibility.yml`.
- [ ] Other gates not exercised by this test.
EOF
if LAUNCH_CHECKLIST="$tmp" bash "$script" >/dev/null 2>&1; then
  fail "Group 7 checked box without evidence should fail but exited 0"
else
  pass "Group 7 checked box without evidence fails"
fi

# ── Test 8: Group 6 box checked without evidence => FAIL ────────────
#
# After v1.2.1-rc.3 closures, the real Group 6 verify-vuln box is
# already `[x]` with evidence on main, so the historical
# `[ ]` -> `[x]` substitution against the real checklist became a
# no-op. Construct a minimal synthetic checklist instead — a checked
# Group 6 box with no evidence text within 8 lines — so the negative
# test's invariant ("checked Group 6 box without evidence URL or
# _Closed_ annotation must fail the gate") stays load-bearing
# regardless of the real checklist's state.

echo "== Test 8: Group 6 box checked without evidence => FAIL"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat > "$tmp" <<'EOF'
# Launch candidate checklist — synthetic fixture for evidence-gate test 8

## 6. Security and policy review

- [x] `make verify-vuln` green for the candidate tag (govulncheck).

## 7. CI / release readiness

- [ ] All required workflows on `main` green: `ci.yml`, `codeql.yml`,
EOF
if LAUNCH_CHECKLIST="$tmp" bash "$script" >/dev/null 2>&1; then
  fail "Group 6 checked box without evidence should fail but exited 0"
else
  pass "Group 6 checked box without evidence fails"
fi

# ── Test 9: Group 7 release-artifact box checked without evidence => FAIL ──
#
# Same pattern as Test 8: the real Release-artefacts box is now `[x]`
# with evidence after Lane 3's rc.3 closure, so the historical
# `[ ]` -> `[x]` substitution is a no-op. Use a synthetic minimal
# fixture so the gate's "release-artifact checked without evidence"
# negative test stays meaningful.

echo "== Test 9: Group 7 release-artifact box checked without evidence => FAIL"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat > "$tmp" <<'EOF'
# Launch candidate checklist — synthetic fixture for evidence-gate test 9

## 7. CI / release readiness

- [x] Release artefacts: signed binaries (cosign, plus SLSA when GitHub artifact attestations are available), SBOMs, container image, and SHA256SUMS.txt are all present on the GitHub Release.

- [ ] All required workflows on `main` green: `ci.yml`, `codeql.yml`,
EOF
if LAUNCH_CHECKLIST="$tmp" bash "$script" >/dev/null 2>&1; then
  fail "Group 7 release-artifact box without evidence should fail but exited 0"
else
  pass "Group 7 release-artifact box without evidence fails"
fi

# ── Summary ────────────────────────────────────────────────────────

echo ""
echo "Tests run: $tests_run, failures: $tests_failed"
if [ "$tests_failed" -gt 0 ]; then
  echo "FAIL" >&2
  exit 1
fi
echo "OK"
