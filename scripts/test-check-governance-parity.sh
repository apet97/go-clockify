#!/usr/bin/env bash
#
# test-check-governance-parity.sh — regression test for
# check-governance-parity.sh.
#
# Locks the governance-parity gate's externally-observable contract
# across its require/forbid assertion families:
#   1.  Pass: clean baseline clears all assertions
#   2.  Fail: branch-protection table missing canonical row
#   3.  Fail: branch-protection missing Doctor-strict-smoke bullet
#   4.  Fail: smoke-doctor-strict.sh buried under Test (HTTP smoke)
#   5.  Fail: GOVERNANCE.md missing `## Current state` section
#   6.  Fail: GOVERNANCE.md current-state missing required line
#   7.  Fail: GOVERNANCE.md current-state has forbidden pattern
#   8.  Fail: GOVERNANCE.md current-state has non-zero required approvals
#   9.  Fail: GOVERNANCE.md current-state forbid is case-insensitive
#   10. Fail: GOVERNANCE.md current-state forbids "Signed commits: required"
#   11. Fail: GOVERNANCE.md current-state forbids "Enforce for admins: enforced"
#   12. Pass: forbidden phrase scoped to `## Target state` (out of section)
#       must NOT trip the gate
#
# Each case builds throwaway fixtures in a per-case tmpdir, runs the
# script with cwd set to the fixture (the gate uses relative paths),
# captures combined output and exit code, and asserts both. Pure
# bash — no `go` stub needed.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-governance-parity.sh"

if [ ! -f "$script" ]; then
    echo "FAIL: script not found at $script" >&2
    exit 1
fi

tests_run=0
tests_failed=0

# write_baseline_tree lays down the minimum tree that clears every
# assertion. Cases mutate this baseline in place to drive specific
# failure paths.
write_baseline_tree() {
    local dir="$1"

    mkdir -p "$dir/docs"

    cat > "$dir/docs/branch-protection.md" <<'EOF'
# Branch protection — main

| Setting | Value |
|---|---|
| Required approvals | 0 |
| Require review from Code Owners | Disabled |
| Require signed commits | Disabled |
| Enforce for admins | Disabled |

## Required checks

- `Test (HTTP smoke)` — runs the http-smoke job and checks /health
  on the streamable transport
- `Doctor strict smoke` — runs scripts/smoke-doctor-strict.sh against
  a postgres profile
EOF

    cat > "$dir/GOVERNANCE.md" <<'EOF'
# Governance

## Current state

- Required approvals: 0 enforced.
- Code-owner reviews: disabled.
- Signed commits: disabled.
- Admin enforcement: disabled.

## Target state

Future controls land here once the team votes them in.
EOF
}

# run_case <name> <expect-exit> <expect-pattern> [extra-env-pair ...]
#
# expect-pattern is an ERE applied with grep -qE against combined
# stdout+stderr; pass an empty string to skip the assertion. The
# optional MUTATOR function (named via $MUTATOR) runs against the
# per-case fixture directory before invoking the script.
run_case() {
    local name="$1"; shift
    local expect_exit="$1"; shift
    local expect_pattern="$1"; shift

    tests_run=$((tests_run + 1))

    local dir
    dir="$(mktemp -d "${TMPDIR:-/tmp}/test-gov-parity.XXXXXX")"
    # shellcheck disable=SC2064
    trap "rm -rf \"$dir\"" RETURN

    write_baseline_tree "$dir"

    if [ -n "${MUTATOR:-}" ]; then
        "$MUTATOR" "$dir"
    fi

    local out
    local actual_exit=0
    out="$(cd "$dir" && env "$@" sh "$script" 2>&1)" || actual_exit=$?

    local pass=1
    if [ "$actual_exit" != "$expect_exit" ]; then
        pass=0
    fi
    if [ -n "$expect_pattern" ] && ! grep -qE -- "$expect_pattern" <<< "$out"; then
        pass=0
    fi

    if [ "$pass" = "1" ]; then
        printf 'PASS: %s\n' "$name"
    else
        tests_failed=$((tests_failed + 1))
        printf 'FAIL: %s\n' "$name" >&2
        printf '  expected exit=%s, got=%s\n' "$expect_exit" "$actual_exit" >&2
        printf '  expected pattern=%q\n' "$expect_pattern" >&2
        printf '  --- output ---\n%s\n  --- end ---\n' "$out" >&2
    fi

    rm -rf "$dir"
    trap - RETURN
}

# --- Case 1: clean baseline ---
MUTATOR=""
run_case "clean baseline clears all assertions" \
    0 'governance-parity: OK'

# --- Case 2: branch-protection table missing canonical row ---
mut_drop_required_approvals_row() {
    grep -v '^| Required approvals' "$1/docs/branch-protection.md" \
        > "$1/docs/branch-protection.md.new"
    mv "$1/docs/branch-protection.md.new" "$1/docs/branch-protection.md"
}
MUTATOR=mut_drop_required_approvals_row
run_case "branch-protection missing 'Required approvals' row fails closed" \
    1 'missing canonical line: Required approvals: 0'

# --- Case 3: branch-protection missing Doctor strict smoke bullet ---
mut_drop_doctor_smoke() {
    grep -v 'Doctor strict smoke' "$1/docs/branch-protection.md" \
        > "$1/docs/branch-protection.md.new"
    mv "$1/docs/branch-protection.md.new" "$1/docs/branch-protection.md"
}
MUTATOR=mut_drop_doctor_smoke
run_case "branch-protection missing 'Doctor strict smoke' bullet fails closed" \
    1 'missing required check: Doctor strict smoke'

# --- Case 4: smoke-doctor-strict.sh buried under Test (HTTP smoke) ---
# This is the burial regression — the gate exists because v1.0.0 had
# the strict-doctor required check aliased under the HTTP smoke
# bullet, which made it invisible in branch-protection-rule UI.
mut_bury_doctor_under_http() {
    cat > "$1/docs/branch-protection.md" <<'EOF'
# Branch protection — main

| Setting | Value |
|---|---|
| Required approvals | 0 |
| Require review from Code Owners | Disabled |
| Require signed commits | Disabled |
| Enforce for admins | Disabled |

## Required checks

- `Test (HTTP smoke)` — runs the http-smoke job
  also runs scripts/smoke-doctor-strict.sh under the same job
- `Doctor strict smoke` — placeholder header
EOF
}
MUTATOR=mut_bury_doctor_under_http
run_case "smoke-doctor-strict.sh buried under HTTP smoke fails closed" \
    1 'still claims smoke-doctor-strict\.sh'

# --- Case 5: GOVERNANCE.md missing `## Current state` section ---
mut_drop_current_section() {
    cat > "$1/GOVERNANCE.md" <<'EOF'
# Governance

## Target state

Future controls land here once the team votes them in.
EOF
}
MUTATOR=mut_drop_current_section
run_case "GOVERNANCE.md missing current-state section fails closed" \
    1 'missing a current-state section'

# --- Case 6: GOVERNANCE.md current-state missing required line ---
mut_drop_required_approvals_line() {
    grep -v '^- Required approvals: 0 enforced\.' "$1/GOVERNANCE.md" \
        > "$1/GOVERNANCE.md.new"
    mv "$1/GOVERNANCE.md.new" "$1/GOVERNANCE.md"
}
MUTATOR=mut_drop_required_approvals_line
run_case "GOVERNANCE.md current-state missing 'Required approvals: 0 enforced.' fails closed" \
    1 'missing: Required approvals: 0 enforced'

# --- Case 7: GOVERNANCE.md current-state has 'Code-owner reviews: enabled.' ---
mut_codeowner_enabled() {
    cat > "$1/GOVERNANCE.md" <<'EOF'
# Governance

## Current state

- Required approvals: 0 enforced.
- Code-owner reviews: disabled.
- Code-owner reviews: enabled.
- Signed commits: disabled.
- Admin enforcement: disabled.

## Target state

Future controls land here once the team votes them in.
EOF
}
MUTATOR=mut_codeowner_enabled
run_case "GOVERNANCE.md current-state forbids 'Code-owner reviews: enabled.'" \
    1 'CODEOWNERS review is required'

# --- Case 8: GOVERNANCE.md current-state has 'Required approvals: 2' ---
# require_current_line still satisfied (the `: 0 enforced.` line is
# present), but forbid_current_pattern catches the second line.
mut_required_approvals_two() {
    cat > "$1/GOVERNANCE.md" <<'EOF'
# Governance

## Current state

- Required approvals: 0 enforced.
- Required approvals: 2 (effective).
- Code-owner reviews: disabled.
- Signed commits: disabled.
- Admin enforcement: disabled.

## Target state

Future controls land here once the team votes them in.
EOF
}
MUTATOR=mut_required_approvals_two
run_case "GOVERNANCE.md current-state forbids non-zero required approvals" \
    1 'required approvals are non-zero'

# --- Case 9: forbid pattern is case-insensitive ---
# 'Code-owner reviews: ENABLED' must trip the gate even though all
# require_current_line checks pass. Without the -i flag on
# forbid_current_pattern, this case would be silently accepted.
mut_codeowner_enabled_uppercase() {
    cat > "$1/GOVERNANCE.md" <<'EOF'
# Governance

## Current state

- Required approvals: 0 enforced.
- Code-owner reviews: disabled.
- Code-owner reviews: ENABLED
- Signed commits: disabled.
- Admin enforcement: disabled.

## Target state

Future controls land here once the team votes them in.
EOF
}
MUTATOR=mut_codeowner_enabled_uppercase
run_case "GOVERNANCE.md current-state forbid is case-insensitive" \
    1 'CODEOWNERS review is required'

# --- Case 10: 'Signed commits: required' forbidden ---
mut_signed_required() {
    cat > "$1/GOVERNANCE.md" <<'EOF'
# Governance

## Current state

- Required approvals: 0 enforced.
- Code-owner reviews: disabled.
- Signed commits: disabled.
- Signed commits: required.
- Admin enforcement: disabled.

## Target state

Future controls land here once the team votes them in.
EOF
}
MUTATOR=mut_signed_required
run_case "GOVERNANCE.md current-state forbids 'Signed commits: required'" \
    1 'signed commits are required'

# --- Case 11: 'Enforce for admins: enforced' forbidden ---
mut_admin_enforced() {
    cat > "$1/GOVERNANCE.md" <<'EOF'
# Governance

## Current state

- Required approvals: 0 enforced.
- Code-owner reviews: disabled.
- Signed commits: disabled.
- Admin enforcement: disabled.
- Enforce for admins: enforced.

## Target state

Future controls land here once the team votes them in.
EOF
}
MUTATOR=mut_admin_enforced
run_case "GOVERNANCE.md current-state forbids 'Enforce for admins: enforced'" \
    1 'admin enforcement is enabled'

# --- Case 12: forbidden phrase scoped to ## Target state ---
# The gate's awk slice ends at `^## Target state`, so a forbidden
# phrase inside Target state must NOT trip the gate. This pins the
# scoping invariant — without it the gate would over-fire on any
# aspirational language about future controls.
mut_target_state_forbidden_word() {
    cat > "$1/GOVERNANCE.md" <<'EOF'
# Governance

## Current state

- Required approvals: 0 enforced.
- Code-owner reviews: disabled.
- Signed commits: disabled.
- Admin enforcement: disabled.

## Target state

The team has voted to migrate to required signed commits and to
require code-owner reviews on every PR; these aspirations are
tracked here, not in the current-state ledger above.
EOF
}
MUTATOR=mut_target_state_forbidden_word
run_case "forbidden phrase scoped to Target state must NOT trip" \
    0 'governance-parity: OK'

# --- Final report ---
echo
if [ "$tests_failed" -gt 0 ]; then
    printf 'check-governance-parity tests: %d/%d FAILED\n' "$tests_failed" "$tests_run" >&2
    exit 1
fi
printf 'check-governance-parity tests: %d/%d OK\n' "$tests_run" "$tests_run"
