#!/usr/bin/env bash
#
# test-check-coverage.sh — regression test for check-coverage.sh.
#
# Locks the coverage gate's externally-observable contract:
#   1. Pass: stub total + per-package coverage above floors → exit 0
#   2. Fail: global floor violated → exit 1
#   3. Override: COVERAGE_GLOBAL_FLOOR raises the global floor
#   4. Fail: per-package floor violated → exit 1
#   5. Fail: a configured floor has no coverage report → exit 1
#   6. Edge: empty COVERAGE_FLOORS still passes if global cleared
#   7. COVERAGE_SKIP_RUN with no profile present → exit 1
#   8. COVERAGE_SKIP_RUN with profile present → exit 0
#   9. Custom COVERAGE_OUT path is honoured
#   10. Multi-package: failures are reported, passes are not silenced
#
# The script under test invokes `go` for both the initial coverage
# run (`go test ... -coverprofile=...`) and the per-package report
# (`go test -cover ./pkg`). Each case prepends a per-case directory
# to PATH that contains a fake `go` shell stub keyed off env vars
# (FAKE_TOTAL, FAKE_COV_<pkg-with-slashes-replaced-by-underscores>).
# The stub honours the no-skip path by creating the requested
# coverprofile so the file-existence check downstream succeeds.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-coverage.sh"

if [ ! -f "$script" ]; then
    echo "FAIL: script not found at $script" >&2
    exit 1
fi

tests_run=0
tests_failed=0

# Stub body for `go`. Written once into each per-case tmpdir.
read -r -d '' STUB <<'STUB_EOF' || true
#!/usr/bin/env bash
sub="$1"
shift || true
case "$sub" in
    tool)
        # `go tool cover -func=PATH` — emit a synthetic profile summary
        # ending in `total:\t(statements)\tPCT%`.
        printf 'pkg/foo.go:1:\tF1\t100.0%%\n'
        printf 'total:\t(statements)\t%s%%\n' "${FAKE_TOTAL:-80.5}"
        ;;
    test)
        # Two flavours:
        #   (a) initial `go test -race ... -coverprofile=PATH ./internal/...`
        #   (b) per-package `go test -cover ./pkg1 ./pkg2 ...`
        # Differentiate via the presence of `-coverprofile=` in argv.
        is_initial=0
        is_per_pkg=0
        for arg in "$@"; do
            case "$arg" in
                -coverprofile=*) is_initial=1 ;;
                -cover) is_per_pkg=1 ;;
            esac
        done
        if [ "$is_initial" = "1" ]; then
            for arg in "$@"; do
                case "$arg" in
                    -coverprofile=*)
                        out="${arg#-coverprofile=}"
                        : > "$out"
                        ;;
                esac
            done
            exit 0
        fi
        if [ "$is_per_pkg" = "1" ]; then
            for arg in "$@"; do
                case "$arg" in
                    ./*)
                        local_pkg="${arg#./}"
                        var="FAKE_COV_$(printf '%s' "$local_pkg" | tr '/' '_')"
                        cov="${!var:-95.0}"
                        if [ "$cov" = "missing" ]; then
                            continue
                        fi
                        printf 'ok\tgithub.com/apet97/go-clockify/%s\t0.001s\tcoverage: %s%% of statements\n' "$local_pkg" "$cov"
                        ;;
                esac
            done
            exit 0
        fi
        echo "fake-go: unsupported `go test` invocation: $*" >&2
        exit 1
        ;;
    *)
        echo "fake-go: unsupported subcommand: $sub" >&2
        exit 1
        ;;
esac
STUB_EOF

# write_fake_go installs the stub in $1 and makes it executable.
write_fake_go() {
    local dir="$1"
    printf '%s\n' "$STUB" > "$dir/go"
    chmod +x "$dir/go"
}

# run_case <name> <expect-exit> <expect-pattern> [extra-env-pair ...]
#
# The optional MUTATOR function (named via $MUTATOR) runs against the
# per-case tmpdir before invoking the script under test.
run_case() {
    local name="$1"; shift
    local expect_exit="$1"; shift
    local expect_pattern="$1"; shift

    tests_run=$((tests_run + 1))

    local dir
    dir="$(mktemp -d "${TMPDIR:-/tmp}/test-check-coverage.XXXXXX")"
    # shellcheck disable=SC2064
    trap "rm -rf \"$dir\"" RETURN

    write_fake_go "$dir"

    if [ -n "${MUTATOR:-}" ]; then
        "$MUTATOR" "$dir"
    fi

    local out
    local actual_exit=0
    out="$(cd "$dir" && env PATH="$dir:$PATH" "$@" bash "$script" 2>&1)" || actual_exit=$?

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

# --- Case 1: pass case (defaults clear default-ish floors) ---
MUTATOR=""
run_case "default total + coverage clears configured floors" \
    0 "all coverage floors cleared" \
    COVERAGE_GLOBAL_FLOOR=70 \
    COVERAGE_FLOORS="internal/foo=80 internal/bar=80" \
    FAKE_TOTAL=85.0 \
    FAKE_COV_internal_foo=90.0 \
    FAKE_COV_internal_bar=90.0

# --- Case 2: global floor violation ---
MUTATOR=""
run_case "global floor violation fails closed" \
    1 "total coverage 50.* below 71.* floor" \
    COVERAGE_FLOORS="" \
    FAKE_TOTAL=50.0

# --- Case 3: override global floor upward ---
MUTATOR=""
run_case "COVERAGE_GLOBAL_FLOOR override rejects below-threshold total" \
    1 "total coverage 80.* below 99.* floor" \
    COVERAGE_GLOBAL_FLOOR=99 \
    COVERAGE_FLOORS="" \
    FAKE_TOTAL=80.0

# --- Case 4: per-package floor violation ---
MUTATOR=""
run_case "per-package floor violation fails closed" \
    1 "internal/foo coverage 10.* below 50.* floor" \
    COVERAGE_GLOBAL_FLOOR=70 \
    COVERAGE_FLOORS="internal/foo=50" \
    FAKE_TOTAL=85.0 \
    FAKE_COV_internal_foo=10.0

# --- Case 5: a configured floor has no coverage report ---
MUTATOR=""
run_case "missing per-package report fails closed" \
    1 "no coverage report for internal/foo" \
    COVERAGE_GLOBAL_FLOOR=70 \
    COVERAGE_FLOORS="internal/foo=50" \
    FAKE_TOTAL=85.0 \
    FAKE_COV_internal_foo=missing

# --- Case 6: whitespace-only COVERAGE_FLOORS reaches the no-floors branch ---
# (The script uses `${COVERAGE_FLOORS:-default}`, so an outright empty
# string falls back to the default list. Operators who want to skip
# per-package gates have to pass a whitespace-only override; the test
# locks that contract.)
MUTATOR=""
run_case "whitespace-only COVERAGE_FLOORS hits no-floors branch" \
    0 "no per-package floors configured" \
    COVERAGE_FLOORS=" " \
    FAKE_TOTAL=85.0

# --- Case 7: COVERAGE_SKIP_RUN with no pre-existing profile ---
MUTATOR=""
run_case "skip-run with missing profile fails closed" \
    1 "coverage profile .* not found" \
    COVERAGE_SKIP_RUN=1

# --- Case 8: COVERAGE_SKIP_RUN with pre-existing profile ---
mut_precreate_profile() {
    : > "$1/coverage.out"
}
MUTATOR=mut_precreate_profile
run_case "skip-run with pre-existing profile honoured" \
    0 "all coverage floors cleared" \
    COVERAGE_SKIP_RUN=1 \
    COVERAGE_GLOBAL_FLOOR=70 \
    COVERAGE_FLOORS="internal/foo=80" \
    FAKE_TOTAL=85.0 \
    FAKE_COV_internal_foo=90.0

# --- Case 9: custom COVERAGE_OUT path honoured ---
mut_precreate_custom_profile() {
    : > "$1/custom-cov.out"
}
MUTATOR=mut_precreate_custom_profile
run_case "custom COVERAGE_OUT path honoured" \
    0 "all coverage floors cleared" \
    COVERAGE_OUT=custom-cov.out \
    COVERAGE_SKIP_RUN=1 \
    COVERAGE_GLOBAL_FLOOR=70 \
    COVERAGE_FLOORS="internal/foo=80" \
    FAKE_TOTAL=85.0 \
    FAKE_COV_internal_foo=90.0

# --- Case 10: multi-package, only some fail ---
MUTATOR=""
run_case "multi-package failure flags only the violator" \
    1 "internal/foo coverage 10.* below 50.* floor" \
    COVERAGE_GLOBAL_FLOOR=70 \
    COVERAGE_FLOORS="internal/foo=50 internal/bar=50" \
    FAKE_TOTAL=85.0 \
    FAKE_COV_internal_foo=10.0 \
    FAKE_COV_internal_bar=80.0

# --- Final report ---
echo
if [ "$tests_failed" -gt 0 ]; then
    printf 'check-coverage tests: %d/%d FAILED\n' "$tests_failed" "$tests_run" >&2
    exit 1
fi
printf 'check-coverage tests: %d/%d OK\n' "$tests_run" "$tests_run"
