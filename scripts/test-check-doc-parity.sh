#!/usr/bin/env bash
#
# test-check-doc-parity.sh — regression test for check-doc-parity.sh.
#
# Locks the doc-parity gate's externally-observable contract across
# all five phases (env-var content, tool-name catalog match, banned
# strings, README↔npm engines parity, dangling markers):
#
#   1.  Pass: clean baseline tree clears all five phases
#   2.  Phase 1 fail: doc references undefined env var
#   3.  Phase 1 pass: opt-out file allows the otherwise-undefined var
#   4.  Phase 1 pass: inline allowlist (MCP_BEARER_TOKEN_EXAMPLE)
#   5.  Phase 1 pass: docs/superpowers/ excluded from scan
#   6.  Phase 2 fail: doc references unknown tool
#   7.  Phase 2 pass: tool-prefix allowlist (clockify_mcp_*)
#   8.  Phase 2 soft: missing tool-catalog warns, gate still passes
#   9.  Phase 3 fail: banned public-surface string in a doc
#   10. Phase 4 fail: README Node compat row disagrees with package.json
#   11. Phase 4 fail: README missing the Node compat row entirely
#   12. Phase 5 fail: dangling marker in operator doc
#   13. Phase 5 pass: marker inside docs/adr/*-superseded.md is filtered
#   14. Phase 4 fail: package.json missing engines.node declaration
#
# Each case builds throwaway fixtures in a per-case tmpdir, runs the
# script with cwd set to the fixture (the script under test uses
# relative paths exclusively), captures combined output and exit
# code, and asserts both. Pure bash; no `go` stub needed because the
# gate itself shells out to nothing.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-doc-parity.sh"

if [ ! -f "$script" ]; then
    echo "FAIL: script not found at $script" >&2
    exit 1
fi

tests_run=0
tests_failed=0

# write_baseline_tree lays down the minimum tree that passes all five
# phases. Cases mutate this baseline in place to exercise specific
# failure paths.
write_baseline_tree() {
    local dir="$1"

    mkdir -p "$dir/internal/config" "$dir/cmd" "$dir/tests" \
             "$dir/.github/workflows" "$dir/deploy" \
             "$dir/docs/runbooks" "$dir/docs/deploy" "$dir/docs/adr" \
             "$dir/npm/clockify-mcp-go"

    # Phase 1 known_vars source: stringy literals so the regex
    # '"(MCP|CLOCKIFY)_[A-Z0-9_]+"' matches inside a Go file.
    cat > "$dir/internal/config/config.go" <<'EOF'
package config

const (
    fooEnv = "MCP_FOO"
    barEnv = "CLOCKIFY_BAR"
)
EOF

    # CI-only env var that the script picks up via the .github/ scan.
    cat > "$dir/.github/workflows/live.yml" <<'EOF'
jobs:
  live:
    env:
      CLOCKIFY_LIVE_TOKEN: dummy
EOF

    # Empty opt-out: header only.
    cat > "$dir/deploy/.config-parity-opt-out.txt" <<'EOF'
# Test fixture opt-out
EOF

    # Phase 2 known_tools: catalog with one tool entry that matches
    # the script's '"name": *"clockify_[a-z0-9_]+"' regex.
    cat > "$dir/docs/tool-catalog.json" <<'EOF'
{
  "tier1": [
    {"name": "clockify_list_workspaces"}
  ]
}
EOF

    # Phase 4 inputs: package.json with engines.node + README compat row.
    cat > "$dir/npm/clockify-mcp-go/package.json" <<'EOF'
{
  "name": "clockify-mcp-go",
  "engines": {
    "node": ">=18"
  }
}
EOF

    cat > "$dir/README.md" <<'EOF'
# Test fixture README

Configures MCP_FOO at startup; surfaces clockify_list_workspaces.

## Compatibility

| Component | Version |
|---|---|
| Node.js (npm wrapper) | 18+ |
EOF

    # DOC_FILES_TOP entry inside the strict scan: must contain no
    # markers, banned strings, unknown tools, or undefined env vars.
    cat > "$dir/docs/support-matrix.md" <<'EOF'
# Support matrix

See README for compatibility details.
EOF
}

# run_case <name> <expect-exit> <expect-pattern> [extra-env-pair ...]
#
# expect-pattern is an ERE applied with grep -qE against combined
# stdout+stderr; pass an empty string to skip the assertion.
# The optional MUTATOR function (named via $MUTATOR) runs against
# the per-case fixture directory before invoking the script.
run_case() {
    local name="$1"; shift
    local expect_exit="$1"; shift
    local expect_pattern="$1"; shift

    tests_run=$((tests_run + 1))

    local dir
    dir="$(mktemp -d "${TMPDIR:-/tmp}/test-doc-parity.XXXXXX")"
    # shellcheck disable=SC2064
    trap "rm -rf \"$dir\"" RETURN

    write_baseline_tree "$dir"

    if [ -n "${MUTATOR:-}" ]; then
        "$MUTATOR" "$dir"
    fi

    local out
    local actual_exit=0
    out="$(cd "$dir" && env "$@" bash "$script" 2>&1)" || actual_exit=$?

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
run_case "clean baseline clears all five phases" \
    0 'doc-parity: OK'

# --- Case 2: undefined env var ref in docs ---
mut_undefined_env() {
    printf '\nFurther reading: see MCP_GHOST notes.\n' >> "$1/README.md"
}
MUTATOR=mut_undefined_env
run_case "Phase 1: undefined env var reference fails closed" \
    1 'env var referenced in docs but not defined'

# --- Case 3: opt-out file allows MCP_GHOST ---
mut_opt_out_ghost() {
    printf '\nFurther reading: see MCP_GHOST notes.\n' >> "$1/README.md"
    printf 'MCP_GHOST   placeholder for fixture\n' \
        >> "$1/deploy/.config-parity-opt-out.txt"
}
MUTATOR=mut_opt_out_ghost
run_case "Phase 1: opt-out file allows otherwise-undefined var" \
    0 'doc-parity: OK'

# --- Case 4: inline allowlist (MCP_BEARER_TOKEN_EXAMPLE) ---
mut_bearer_example() {
    printf 'Example: MCP_BEARER_TOKEN_EXAMPLE=changeme\n' \
        > "$1/docs/runbooks/x.md"
}
MUTATOR=mut_bearer_example
run_case "Phase 1: inline example allowlist is honoured" \
    0 'doc-parity: OK'

# --- Case 5: docs/superpowers/ excluded ---
mut_superpowers_ghost() {
    mkdir -p "$1/docs/superpowers"
    printf '# Future spec\n\nIntroduces MCP_GHOST in a later release.\n' \
        > "$1/docs/superpowers/spec.md"
}
MUTATOR=mut_superpowers_ghost
run_case "Phase 1: docs/superpowers/ is excluded from scan" \
    0 'doc-parity: OK'

# --- Case 6: unknown clockify_* tool token ---
mut_unknown_tool() {
    printf '\nNote: clockify_ghost_tool was renamed last month.\n' \
        >> "$1/README.md"
}
MUTATOR=mut_unknown_tool
run_case "Phase 2: unknown tool token fails closed" \
    1 'tool referenced in operator docs but not in'

# --- Case 7: tool-prefix allowlist (clockify_mcp_*) ---
mut_tool_allowlist() {
    printf '\nInternal helper: clockify_mcp_internal handles bootstrap.\n' \
        >> "$1/README.md"
}
MUTATOR=mut_tool_allowlist
run_case "Phase 2: tool-prefix allowlist is honoured" \
    0 'doc-parity: OK'

# --- Case 8: missing tool-catalog warns, gate passes ---
mut_remove_catalog() {
    rm "$1/docs/tool-catalog.json"
}
MUTATOR=mut_remove_catalog
run_case "Phase 2: missing tool-catalog warns, gate still passes" \
    0 '\[warn\].*tool-catalog\.json'

# --- Case 9: banned public-surface string ---
# `@anycli/clockify-mcp-go` deliberately uses a hyphen, not an
# underscore, so it does NOT also trip Phase 2's `\bclockify_…` regex.
mut_banned_string() {
    printf 'Legacy hint: see @anycli/clockify-mcp-go in older releases.\n' \
        > "$1/docs/runbooks/x.md"
}
MUTATOR=mut_banned_string
run_case "Phase 3: banned stale public-surface string fails closed" \
    1 'banned stale public-surface string'

# --- Case 10: README ↔ package.json Node mismatch ---
mut_node_mismatch() {
    cat > "$1/npm/clockify-mcp-go/package.json" <<'EOF'
{
  "name": "clockify-mcp-go",
  "engines": {
    "node": ">=20"
  }
}
EOF
}
MUTATOR=mut_node_mismatch
run_case "Phase 4: README Node compat does not match package.json fails closed" \
    1 'does not match'

# --- Case 11: README missing Node compat row ---
# Phase 4's `readme_node=$(grep | sed | tr)` pipeline is followed by
# `|| true` so grep returning 1 (row absent) does not abort the
# substitution under `set -euo pipefail` — the dedicated
# `[ -z "$readme_node" ]` err line below it must remain reachable.
mut_missing_node_row() {
    grep -vF '| Node.js (npm wrapper) |' "$1/README.md" > "$1/README.md.new"
    mv "$1/README.md.new" "$1/README.md"
}
MUTATOR=mut_missing_node_row
run_case "Phase 4: README missing Node compat row fails closed" \
    1 'missing Node\.js .* compatibility row'

# --- Case 12: dangling marker in operator runbook ---
mut_dangling_marker() {
    printf '# Runbook\n\n- TODO follow up before release\n' \
        > "$1/docs/runbooks/x.md"
}
MUTATOR=mut_dangling_marker
run_case "Phase 5: dangling marker in operator doc fails closed" \
    1 'dangling marker in operator doc'

# --- Case 13: marker inside docs/adr/*-superseded.md is filtered ---
mut_superseded_marker() {
    printf '# Superseded ADR\n\n- TODO drop this once renamed\n' \
        > "$1/docs/adr/0001-x-superseded.md"
}
MUTATOR=mut_superseded_marker
run_case "Phase 5: marker in docs/adr/*-superseded.md is filtered" \
    0 'doc-parity: OK'

# --- Case 14: package.json missing engines.node declaration ---
# Parallel to case 11 for the package_node branch. Lives under the
# same `|| true` contract: without the trailing `|| true` on Phase
# 4's package_node pipeline, this branch would silently abort.
mut_missing_engines_node() {
    cat > "$1/npm/clockify-mcp-go/package.json" <<'EOF'
{
  "name": "clockify-mcp-go"
}
EOF
}
MUTATOR=mut_missing_engines_node
run_case "Phase 4: package.json missing engines.node fails closed" \
    1 'missing node engine declaration'

# --- Final report ---
echo
if [ "$tests_failed" -gt 0 ]; then
    printf 'check-doc-parity tests: %d/%d FAILED\n' "$tests_failed" "$tests_run" >&2
    exit 1
fi
printf 'check-doc-parity tests: %d/%d OK\n' "$tests_run" "$tests_run"
