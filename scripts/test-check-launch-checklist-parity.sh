#!/usr/bin/env bash
#
# test-check-launch-checklist-parity.sh — regression test for
# check-launch-checklist-parity.sh.
#
# Locks the launch-checklist gate's externally-observable contract
# across both layers. Layer 1 is source greps (checklist + cmd source);
# Layer 2 builds clockify-mcp via PATH-stubbed `go` and asserts that
# every flag the checklist names is also documented in `--help`.
# Cases:
#
#   Layer 1
#   1.  Pass: clean baseline
#   2.  Fail: checklist file absent
#   3.  Fail: checklist missing the strict prod-postgres backtick line
#   4.  Fail: checklist mentions --check-backends, source missing
#       `case a == "--check-backends"`
#   5.  Fail: checklist mentions --check-backends, source missing
#       `checkBackends` symbol
#   6.  Fail: checklist mentions --check-backends, source missing
#       `func backendDoctorFindings` implementation
#
#   Layer 2 (with PATH-stubbed `go`)
#   7.  Fail: --help missing "clockify-mcp doctor"
#   8.  Fail: --help missing "prod-postgres"
#   9.  Fail: --help missing "--strict"
#   10. Fail: --help missing "--profile="
#   11. Fail: checklist references --check-backends, --help missing it
#   12. Fail: checklist references --allow-broad-policy, --help missing it
#
#   Edges
#   13. Fail: `go build` exits non-zero (gate reports build failure)
#   14. Fail: Go absent on PATH (gate calls err with "Go toolchain
#       not on PATH" — fail-closed, NOT warn-only)
#
# The `go` stub is a bash script installed at $tmpdir/go. The gate's
# `command -v go` resolves it via PATH. On a `go build -o BIN
# ./cmd/clockify-mcp` invocation, the stub writes a fake binary to
# BIN that cats a per-case help fixture file. FAKE_BUILD_FAILS=1
# forces the stub to exit non-zero on `build`. For the Go-absent
# case, PATH is restricted to /usr/bin:/bin (real go lives in
# /opt/homebrew/bin or similar; verified absent from system paths).

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-launch-checklist-parity.sh"

if [ ! -f "$script" ]; then
    echo "FAIL: script not found at $script" >&2
    exit 1
fi

tests_run=0
tests_failed=0

# Stub body for `go`. On `go build -o BIN ...` the stub writes a
# fake binary to BIN that, when run, cats the file pointed at by
# $HELP_FIXTURE_FILE. The literal absolute path is baked into the
# fake binary at write time via printf %q so the binary stays
# self-contained.
read -r -d '' STUB <<'STUB_EOF' || true
#!/usr/bin/env bash
sub="${1:-}"
shift || true
case "$sub" in
    build)
        out=""
        while [ $# -gt 0 ]; do
            if [ "$1" = "-o" ]; then
                out="${2:-}"
                shift 2 || break
            else
                shift
            fi
        done
        if [ "${FAKE_BUILD_FAILS:-0}" = "1" ]; then
            echo "stub-go: simulated build failure" >&2
            exit 1
        fi
        if [ -z "${out}" ]; then
            echo "stub-go: -o not specified" >&2
            exit 1
        fi
        if [ -z "${HELP_FIXTURE_FILE:-}" ] || [ ! -f "${HELP_FIXTURE_FILE}" ]; then
            echo "stub-go: HELP_FIXTURE_FILE unset or missing" >&2
            exit 1
        fi
        printf '#!/usr/bin/env bash\ncat %q\n' "${HELP_FIXTURE_FILE}" > "${out}"
        chmod +x "${out}"
        exit 0
        ;;
    "")
        echo "stub-go: missing subcommand" >&2
        exit 1
        ;;
    *)
        # The gate only ever calls `go build`. Other subcommands
        # (version, env, etc.) are no-ops to keep the stub forgiving.
        exit 0
        ;;
esac
STUB_EOF

# write_fake_go installs the stub at $1/go and makes it executable.
write_fake_go() {
    local dir="$1"
    printf '%s\n' "$STUB" > "$dir/go"
    chmod +x "$dir/go"
}

# Default help-text fixture content. Includes every string the gate
# greps for in Layer 2: "clockify-mcp doctor", "prod-postgres",
# "--strict", "--profile=", "--check-backends", "--allow-broad-policy".
DEFAULT_HELP=$(cat <<'HELP_EOF'
clockify-mcp — Model Context Protocol server for Clockify

Subcommands:
  clockify-mcp doctor       Run preflight checks
  clockify-mcp serve        Start the MCP transport

Doctor flags:
  --profile=<name>          Doctor profile (e.g. prod-postgres)
  --strict                  Fail on warnings as well as errors
  --check-backends          Probe configured Clockify backends
  --allow-broad-policy      Permit standard policy in production
HELP_EOF
)

# Default checklist content. Mentions both --check-backends and
# --allow-broad-policy so every conditional branch (Layer 1 and
# Layer 2) fires under the baseline.
DEFAULT_CHECKLIST=$(cat <<'CHK_EOF'
# Public Hosted Launch Checklist

Operators must verify before flipping the routing layer:

1. Tier-0 doctor invariant. The command
   `clockify-mcp doctor --profile=prod-postgres --strict` exits 0
   against the production database snapshot.

2. Backend reachability. Run with --check-backends to probe every
   configured Clockify backend before routing live traffic.

3. Policy posture. The deployment must NOT use --allow-broad-policy
   in steady-state operation; document the override in the runbook.
CHK_EOF
)

# Default cmd/clockify-mcp source. Contains the three sentinel
# substrings the gate greps for at Layer 1: parse case, option
# symbol, and func implementation. Source can be syntactically
# invalid Go — the gate uses `grep -Rqs`, never `go vet`.
DEFAULT_MAIN=$(cat <<'MAIN_EOF'
package main

func parseDoctorArgs(args []string) doctorOptions {
    var opts doctorOptions
    for _, a := range args {
        switch {
        case a == "--check-backends":
            opts.checkBackends = true
        }
    }
    return opts
}

func backendDoctorFindings() []finding {
    return nil
}
MAIN_EOF
)

# write_baseline_tree writes the tmpdir layout with all strings
# the gate cares about present in their canonical positions.
# Cases mutate this baseline in place (replacing one of the
# fixture files) to drive specific failure paths.
write_baseline_tree() {
    local dir="$1"

    mkdir -p "$dir/docs/release" "$dir/cmd/clockify-mcp" "$dir/.fixture"

    printf '%s\n' "$DEFAULT_CHECKLIST" \
        > "$dir/docs/release/public-hosted-launch-checklist.md"
    printf '%s\n' "$DEFAULT_MAIN" > "$dir/cmd/clockify-mcp/main.go"
    printf '%s\n' "$DEFAULT_HELP" > "$dir/.fixture/help.txt"

    write_fake_go "$dir"
}

# run_case <name> <expect-exit> <expect-pattern> [extra-env-pair ...]
#
# expect-pattern is an ERE applied with grep -qE against combined
# stdout+stderr; pass an empty string to skip the assertion. The
# optional MUTATOR function (named via $MUTATOR) runs against the
# per-case fixture directory before invoking the script. The script
# is invoked with cwd inside the fixture (gate uses relative paths)
# and PATH prefixed with $dir so the stub `go` is resolved first.
# Extra env-var pairs are passed via env <pair>... bash "$script".
run_case() {
    local name="$1"; shift
    local expect_exit="$1"; shift
    local expect_pattern="$1"; shift

    tests_run=$((tests_run + 1))

    local dir
    dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-checklist.XXXXXX")"
    # shellcheck disable=SC2064
    trap "rm -rf \"$dir\"" RETURN

    write_baseline_tree "$dir"

    if [ -n "${MUTATOR:-}" ]; then
        "$MUTATOR" "$dir"
    fi

    local out
    local actual_exit=0
    out="$(cd "$dir" \
        && env PATH="$dir:/usr/bin:/bin" \
               HELP_FIXTURE_FILE="$dir/.fixture/help.txt" \
               "$@" \
               bash "$script" 2>&1)" || actual_exit=$?

    local pass=1
    if [ "$actual_exit" != "$expect_exit" ]; then pass=0; fi
    if [ -n "$expect_pattern" ] && ! grep -qE -- "$expect_pattern" <<< "$out"; then pass=0; fi

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

# ============================================================
# Layer 1 cases (source greps)
# ============================================================

# --- Case 1: clean baseline ---
MUTATOR=""
run_case "clean baseline clears Layer 1 + Layer 2" \
    0 'launch-checklist-parity: OK'

# --- Case 2: checklist file absent ---
mut_drop_checklist() {
    rm "$1/docs/release/public-hosted-launch-checklist.md"
}
MUTATOR=mut_drop_checklist
run_case "checklist file absent fails closed" \
    1 'missing checklist'

# --- Case 3: checklist missing strict-doctor backtick line ---
mut_drop_strict_line() {
    cat > "$1/docs/release/public-hosted-launch-checklist.md" <<'EOF'
# Public Hosted Launch Checklist

Operators must verify before flipping the routing layer:

1. Backend reachability. Run with --check-backends to probe every
   configured Clockify backend before routing live traffic.
EOF
}
MUTATOR=mut_drop_strict_line
run_case "checklist missing strict-doctor backtick line fails closed" \
    1 'must require.*prod-postgres'

# --- Case 4: source missing `case a == "--check-backends"` ---
mut_drop_case_statement() {
    cat > "$1/cmd/clockify-mcp/main.go" <<'EOF'
package main

func parseDoctorArgs(args []string) doctorOptions {
    var opts doctorOptions
    for _, a := range args {
        if a == "--something-else" {
            opts.checkBackends = true
        }
    }
    return opts
}

func backendDoctorFindings() []finding {
    return nil
}
EOF
}
MUTATOR=mut_drop_case_statement
run_case "source missing 'case a == \"--check-backends\"' fails closed" \
    1 'does not parse it'

# --- Case 5: source missing `checkBackends` symbol ---
mut_drop_checkbackends_symbol() {
    cat > "$1/cmd/clockify-mcp/main.go" <<'EOF'
package main

func parseDoctorArgs(args []string) doctorOptions {
    var opts doctorOptions
    for _, a := range args {
        switch {
        case a == "--check-backends":
            opts.somethingElse = true
        }
    }
    return opts
}

func backendDoctorFindings() []finding {
    return nil
}
EOF
}
MUTATOR=mut_drop_checkbackends_symbol
run_case "source missing 'checkBackends' symbol fails closed" \
    1 'no checkBackends option'

# --- Case 6: source missing `func backendDoctorFindings` ---
mut_drop_func() {
    cat > "$1/cmd/clockify-mcp/main.go" <<'EOF'
package main

func parseDoctorArgs(args []string) doctorOptions {
    var opts doctorOptions
    for _, a := range args {
        switch {
        case a == "--check-backends":
            opts.checkBackends = true
        }
    }
    return opts
}
EOF
}
MUTATOR=mut_drop_func
run_case "source missing 'func backendDoctorFindings' fails closed" \
    1 'no backendDoctorFindings'

# ============================================================
# Layer 2 cases (PATH-stubbed go + help fixture)
# ============================================================

# Helper to rewrite the help fixture with one substring removed.
make_help_minus() {
    local dir="$1"
    local needle="$2"
    grep -vF -- "$needle" "$dir/.fixture/help.txt" > "$dir/.fixture/help.txt.new"
    mv "$dir/.fixture/help.txt.new" "$dir/.fixture/help.txt"
}

# --- Case 7: --help missing "clockify-mcp doctor" ---
mut_help_no_doctor() {
    make_help_minus "$1" "clockify-mcp doctor"
}
MUTATOR=mut_help_no_doctor
run_case "Layer 2: --help missing 'clockify-mcp doctor' fails closed" \
    1 'does not advertise the doctor subcommand'

# --- Case 8: --help missing "prod-postgres" ---
mut_help_no_prod_postgres() {
    make_help_minus "$1" "prod-postgres"
}
MUTATOR=mut_help_no_prod_postgres
run_case "Layer 2: --help missing 'prod-postgres' fails closed" \
    1 'does not list the prod-postgres profile'

# --- Case 9: --help missing "--strict" ---
mut_help_no_strict() {
    make_help_minus "$1" "--strict"
}
MUTATOR=mut_help_no_strict
run_case "Layer 2: --help missing '--strict' fails closed" \
    1 'does not document --strict'

# --- Case 10: --help missing "--profile=" ---
mut_help_no_profile() {
    make_help_minus "$1" "--profile="
}
MUTATOR=mut_help_no_profile
run_case "Layer 2: --help missing '--profile=' fails closed" \
    1 'does not document --profile=<name>'

# --- Case 11: checklist has --check-backends, help doesn't ---
mut_help_no_check_backends() {
    make_help_minus "$1" "--check-backends"
}
MUTATOR=mut_help_no_check_backends
run_case "Layer 2: --help missing '--check-backends' fails closed" \
    1 '--help does not document it'

# --- Case 12: checklist has --allow-broad-policy, help doesn't ---
mut_help_no_allow_broad() {
    make_help_minus "$1" "--allow-broad-policy"
}
MUTATOR=mut_help_no_allow_broad
run_case "Layer 2: --help missing '--allow-broad-policy' fails closed" \
    1 '--help does not document it'

# ============================================================
# Edge cases
# ============================================================

# --- Case 13: go build exits non-zero ---
MUTATOR=""
run_case "go build failure reports executable parity check skip-fail" \
    1 'go build .* failed' \
    FAKE_BUILD_FAILS=1

# --- Case 14: Go absent on PATH ---
# To make `command -v go` return false the test PATH must contain
# zero `go` binaries. The Ubuntu CI runner image ships go at
# /usr/bin/go, so the obvious PATH=/usr/bin:/bin shortcut does not
# isolate from it. Build a per-case sandbox-bin/ with only the two
# external binaries the gate's Go-absent path actually invokes
# (grep + rm — every other call is a bash builtin) and run with
# PATH set exclusively to that sandbox. The gate reaches the
# `command -v go` check, returns false, calls err — fail=1, exit 1.
# This pins the fail-closed behaviour: Go-absent is not warn-only.
run_case_no_go() {
    local name="$1"; shift
    local expect_exit="$1"; shift
    local expect_pattern="$1"; shift

    tests_run=$((tests_run + 1))

    local dir
    dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-checklist.XXXXXX")"
    # shellcheck disable=SC2064
    trap "rm -rf \"$dir\"" RETURN

    write_baseline_tree "$dir"
    rm "$dir/go"

    # Build the sandbox PATH with bash + grep + rm. /usr/bin/grep
    # and /bin/rm are present on both macOS and Ubuntu CI runners;
    # $BASH is the absolute path of the bash currently running this
    # test (brew /opt/homebrew/bin/bash on macOS, /usr/bin/bash on
    # Ubuntu) — env -i needs bash on PATH because it consults the
    # new PATH to exec the command. The symlinks resolve through
    # the canonical paths so the gate's `grep -E`, `grep -F`,
    # `grep -Rqs`, and the cleanup trap's `rm -f` all keep working
    # while `command -v go` finds nothing.
    mkdir -p "$dir/sandbox-bin"
    ln -s "$BASH" "$dir/sandbox-bin/bash"
    ln -s /usr/bin/grep "$dir/sandbox-bin/grep"
    ln -s /bin/rm "$dir/sandbox-bin/rm"

    local out
    local actual_exit=0
    out="$(cd "$dir" \
        && env -i HOME="$HOME" PATH="$dir/sandbox-bin" \
               bash "$script" 2>&1)" || actual_exit=$?

    local pass=1
    if [ "$actual_exit" != "$expect_exit" ]; then pass=0; fi
    if [ -n "$expect_pattern" ] && ! grep -qE -- "$expect_pattern" <<< "$out"; then pass=0; fi

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
run_case_no_go "Go absent on PATH fails closed (not warn-only)" \
    1 'Go toolchain not on PATH'

# --- Final report ---
echo
if [ "$tests_failed" -gt 0 ]; then
    printf 'check-launch-checklist-parity tests: %d/%d FAILED\n' "$tests_failed" "$tests_run" >&2
    exit 1
fi
printf 'check-launch-checklist-parity tests: %d/%d OK\n' "$tests_run" "$tests_run"
