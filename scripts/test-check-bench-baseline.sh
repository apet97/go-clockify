#!/usr/bin/env bash
#
# test-check-bench-baseline.sh — regression test for check-bench-baseline.sh.
#
# Locks the baseline-validation gate's externally-observable contract:
#   1. Pass: valid baseline + workflow + go.mod returns 0
#   2. Fail: missing or empty baseline file
#   3. Fail: missing workflow file
#   4. Fail: unreadable / missing go.mod
#   5. Fail: workflow has no `./internal/...` packages to derive
#   6. Fail: baseline rows are not linux/amd64
#   7. Fail: a benchmark has fewer samples than the workflow's -count=
#   8. Fail: a workflow-listed package is missing from the baseline
#   9. Fail: a baseline package is not listed in the workflow
#   10. Override: BENCH_BASELINE_MIN_SAMPLES raises the floor
#
# Each case builds throwaway fixtures in a per-case tmpdir, runs the
# script with `BENCH_WORKFLOW=` + a positional baseline arg, captures
# combined output and exit code, and asserts both. Pure bash + awk;
# runs in milliseconds.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-bench-baseline.sh"

if [ ! -f "$script" ]; then
    echo "FAIL: script not found at $script" >&2
    exit 1
fi

tests_run=0
tests_failed=0

# write_fixtures_ok writes a known-good go.mod, workflow, and baseline
# into the given directory. Cases that test failure modes mutate
# specific files after this baseline is laid down.
write_fixtures_ok() {
    local dir="$1"
    cat > "$dir/go.mod" <<'EOF'
module github.com/test/example

go 1.25
EOF

    cat > "$dir/bench.yml" <<'EOF'
jobs:
  bench:
    steps:
      - name: Run benchmarks
        run: |
          go test -bench=. -count=2 -benchmem -run='^$' \
            ./internal/foo \
            ./internal/bar \
            > /tmp/bench-raw.txt
EOF

    # Tabs after the bench name are required for benchstat compatibility;
    # write the baseline via printf so escapes survive.
    printf 'goos: linux\n'                                                                   >  "$dir/baseline.txt"
    printf 'goarch: amd64\n'                                                                 >> "$dir/baseline.txt"
    printf 'pkg: github.com/test/example/internal/foo\n'                                     >> "$dir/baseline.txt"
    printf 'cpu: x86\n'                                                                      >> "$dir/baseline.txt"
    printf 'BenchmarkOne-2\t100\t10.0 ns/op\t0 B/op\t0 allocs/op\n'                          >> "$dir/baseline.txt"
    printf 'BenchmarkOne-2\t100\t10.1 ns/op\t0 B/op\t0 allocs/op\n'                          >> "$dir/baseline.txt"
    printf 'goos: linux\n'                                                                   >> "$dir/baseline.txt"
    printf 'goarch: amd64\n'                                                                 >> "$dir/baseline.txt"
    printf 'pkg: github.com/test/example/internal/bar\n'                                     >> "$dir/baseline.txt"
    printf 'cpu: x86\n'                                                                      >> "$dir/baseline.txt"
    printf 'BenchmarkTwo-2\t100\t20.0 ns/op\t0 B/op\t0 allocs/op\n'                          >> "$dir/baseline.txt"
    printf 'BenchmarkTwo-2\t100\t20.1 ns/op\t0 B/op\t0 allocs/op\n'                          >> "$dir/baseline.txt"
}

# run_case <name> <expected-exit> <expected-pattern> [extra-env-pair ...]
#   - <name>:             test description
#   - <expected-exit>:    0 for pass, 1 for fail
#   - <expected-pattern>: ERE matched against combined stdout+stderr
#                         (use empty string to skip the assertion)
#   - extra-env-pair:     KEY=VAL passed to the script invocation
#
# The mutator function (if any) must be set in $MUTATOR before calling
# and runs against the per-case fixture directory.
run_case() {
    local name="$1"; shift
    local expect_exit="$1"; shift
    local expect_pattern="$1"; shift

    tests_run=$((tests_run + 1))

    local dir
    dir="$(mktemp -d "${TMPDIR:-/tmp}/test-bench-baseline.XXXXXX")"
    # shellcheck disable=SC2064
    trap "rm -rf \"$dir\"" RETURN

    write_fixtures_ok "$dir"

    if [ -n "${MUTATOR:-}" ]; then
        "$MUTATOR" "$dir"
    fi

    local out
    local actual_exit=0
    # `env -i` would strip too much; instead pass through PATH and add the
    # explicit overrides this script supports.
    out="$(cd "$dir" && env BENCH_WORKFLOW="bench.yml" "$@" bash "$script" baseline.txt 2>&1)" || actual_exit=$?

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

# --- Case 1: pass case ---
MUTATOR=""
run_case "valid baseline + workflow + go.mod yields exit 0" \
    0 "bench-baseline: OK"

# --- Case 2: missing baseline file ---
mut_remove_baseline() { rm "$1/baseline.txt"; }
MUTATOR=mut_remove_baseline
run_case "missing baseline file fails closed" \
    1 "missing or empty"

# --- Case 3: empty baseline file ---
mut_empty_baseline() { : > "$1/baseline.txt"; }
MUTATOR=mut_empty_baseline
run_case "empty baseline file fails closed" \
    1 "missing or empty"

# --- Case 4: missing workflow file ---
mut_remove_workflow() { rm "$1/bench.yml"; }
MUTATOR=mut_remove_workflow
run_case "missing workflow file fails closed" \
    1 "benchmark workflow .* not found"

# --- Case 5: go.mod present but no `module` line ---
# (A wholly missing go.mod aborts on `awk: can't open file go.mod` under
# `set -e` before the script's own `[ -z "$module" ]` check fires; the
# realistic gate is "go.mod exists but is malformed".)
mut_no_module_line() {
    cat > "$1/go.mod" <<'EOF'
go 1.25
EOF
}
MUTATOR=mut_no_module_line
run_case "go.mod without module line fails closed" \
    1 "unable to read module path from go.mod"

# --- Case 6: workflow has no ./internal/ packages ---
mut_no_packages() {
    cat > "$1/bench.yml" <<'EOF'
jobs:
  bench:
    steps:
      - name: Run benchmarks
        run: |
          echo no benchmarks here \
            > /tmp/bench-raw.txt
EOF
}
MUTATOR=mut_no_packages
run_case "workflow with no ./internal/ packages fails closed" \
    1 "unable to derive benchmark packages"

# --- Case 7: wrong platform (darwin/arm64 instead of linux/amd64) ---
mut_wrong_platform() {
    sed -i.bak -e 's/^goos: linux$/goos: darwin/' -e 's/^goarch: amd64$/goarch: arm64/' "$1/baseline.txt"
    rm -f "$1/baseline.txt.bak"
}
MUTATOR=mut_wrong_platform
run_case "non-linux/amd64 baseline fails closed" \
    1 "want linux/amd64"

# --- Case 8: too few samples per benchmark (1 < count=2) ---
mut_one_sample() {
    # Drop the second BenchmarkOne row, leaving 1 sample for that bench.
    awk '
        /^BenchmarkOne-2/ {
            seen_one++
            if (seen_one == 2) next
        }
        { print }
    ' "$1/baseline.txt" > "$1/baseline.txt.new"
    mv "$1/baseline.txt.new" "$1/baseline.txt"
}
MUTATOR=mut_one_sample
run_case "fewer samples than -count= floor fails closed" \
    1 "BenchmarkOne has 1 sample.* want at least 2"

# --- Case 9: workflow lists a package missing from baseline ---
mut_missing_package() {
    # Strip the entire ./internal/bar block (4 header lines + 2 bench rows = 6).
    awk '
        /^pkg: github.com\/test\/example\/internal\/bar$/ { skip = 1 }
        /^pkg: github.com\/test\/example\/internal\/foo$/ { skip = 0 }
        skip && /^(goos:|goarch:|cpu:|Benchmark)/ { next }
        skip && /^pkg: github.com\/test\/example\/internal\/bar$/ { next }
        { print }
    ' "$1/baseline.txt" > "$1/baseline.txt.new"
    # Simpler: rewrite from scratch with only foo.
    cat > "$1/baseline.txt" <<'EOF'
goos: linux
goarch: amd64
pkg: github.com/test/example/internal/foo
cpu: x86
EOF
    printf 'BenchmarkOne-2\t100\t10.0 ns/op\t0 B/op\t0 allocs/op\n' >> "$1/baseline.txt"
    printf 'BenchmarkOne-2\t100\t10.1 ns/op\t0 B/op\t0 allocs/op\n' >> "$1/baseline.txt"
    rm -f "$1/baseline.txt.new"
}
MUTATOR=mut_missing_package
run_case "workflow package missing from baseline fails closed" \
    1 "missing benchmark package in baseline.*internal/bar"

# --- Case 10: baseline has package not listed in workflow ---
mut_extra_package() {
    # Append a third package that the workflow does not list.
    {
        printf 'goos: linux\n'
        printf 'goarch: amd64\n'
        printf 'pkg: github.com/test/example/internal/uninvited\n'
        printf 'cpu: x86\n'
        printf 'BenchmarkExtra-2\t100\t30.0 ns/op\t0 B/op\t0 allocs/op\n'
        printf 'BenchmarkExtra-2\t100\t30.1 ns/op\t0 B/op\t0 allocs/op\n'
    } >> "$1/baseline.txt"
}
MUTATOR=mut_extra_package
run_case "baseline package not in workflow fails closed" \
    1 "unexpected benchmark package in baseline.*internal/uninvited"

# --- Case 11: BENCH_BASELINE_MIN_SAMPLES raises the floor ---
MUTATOR=""
run_case "BENCH_BASELINE_MIN_SAMPLES override raises the floor" \
    1 "want at least 10" \
    BENCH_BASELINE_MIN_SAMPLES=10

# --- Final report ---
echo
if [ "$tests_failed" -gt 0 ]; then
    printf 'check-bench-baseline tests: %d/%d FAILED\n' "$tests_failed" "$tests_run" >&2
    exit 1
fi
printf 'check-bench-baseline tests: %d/%d OK\n' "$tests_run" "$tests_run"
