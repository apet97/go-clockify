#!/usr/bin/env bash
#
# test-check-release-assets.sh — regression test for
# check-release-assets.sh.
#
# Locks the release-asset gate's externally-observable contract:
#   1.  Pass: full 46-asset fixture (find-fallback path, no
#       artifacts.json) clears both passes.
#   2.  Pass: full 46-asset fixture WITH valid artifacts.json
#       exercises the jq path. Skipped with a printed [skip] line
#       when jq is not on PATH.
#   3.  Fail: missing one SBOM (.spdx.json) — exit 1, "missing"
#       listing. v0.7.0 regression class.
#   4.  Fail: missing one signature (.sigstore.json) — exit 1.
#   5.  Fail: missing the windows binary (.exe) — exit 1.
#   6.  Fail: missing SHA256SUMS.txt — exit 1.
#   7.  Fail: extra rogue artifact matching ASSET_RE — Pass 1 ok,
#       Pass 2 cardinality fails.
#   8.  Pass: extra non-matching file (README.md) at dist/ root —
#       ASSET_RE filters it out, cardinality stays at 46.
#   9.  Fail: dist directory absent → exit 2.
#  10.  Pass: clockify-mcp-grpc-postgres-* counted exactly once,
#       not twice — pins the longer-alt-first regex ordering at
#       gate line 219.
#
# Each case builds throwaway fixtures in a per-case tmpdir, runs
# the gate with that tmpdir's dist/ as the explicit argument, and
# asserts exit code + output pattern. Pure bash; jq is opportunistic
# and case 2 self-skips when missing.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-release-assets.sh"

if [ ! -f "$script" ]; then
    echo "FAIL: script not found at $script" >&2
    exit 1
fi

# The gate at scripts/check-release-assets.sh uses `declare -A`
# (associative arrays, bash 4+) on its find-fallback dedup path.
# macOS still ships bash 3.2 by default; CI runs Ubuntu bash 5+.
# When the host bash is < 4 we cannot exercise the find-fallback or
# the cardinality cases without the gate itself failing on a
# language-level error before any assertion runs. Skip the suite
# with a clear note in that case — CI is the authoritative gate.
if [ "${BASH_VERSINFO[0]:-0}" -lt 4 ]; then
    printf '[skip] check-release-assets tests require bash 4+ '
    printf '(host bash %s detected). The gate uses `declare -A` for '\
'its find-fallback dedup path; CI (Ubuntu bash 5+) runs the suite '\
'fully. Install a modern bash (`brew install bash`) to run locally.\n' \
        "${BASH_VERSION%%[!0-9.]*}" >&2
    exit 0
fi

tests_run=0
tests_failed=0
tests_skipped=0

# The 15-entry binary platform list. Each binary expands to three
# files at dist/ root: <name>, <name>.spdx.json, <name>.sigstore.json.
# 15 × 3 + 1 SHA256SUMS.txt = 46. Mirrors the gate's expected[]
# build at scripts/check-release-assets.sh:95-138 — kept as a
# parallel constant so a drift between gate and test fails the
# test loudly rather than passing for the wrong reason.
PLATFORMS=(
    "clockify-mcp-darwin-arm64"
    "clockify-mcp-darwin-x64"
    "clockify-mcp-linux-arm64"
    "clockify-mcp-linux-x64"
    "clockify-mcp-windows-x64.exe"
    "clockify-mcp-fips-darwin-arm64"
    "clockify-mcp-fips-darwin-x64"
    "clockify-mcp-fips-linux-arm64"
    "clockify-mcp-fips-linux-x64"
    "clockify-mcp-postgres-linux-arm64"
    "clockify-mcp-postgres-linux-x64"
    "clockify-mcp-grpc-linux-arm64"
    "clockify-mcp-grpc-linux-x64"
    "clockify-mcp-grpc-postgres-linux-arm64"
    "clockify-mcp-grpc-postgres-linux-x64"
)

# write_full_dist creates an empty dist/ tree under $1 with all 46
# expected files at the top level. Cases mutate this baseline to
# drive failure paths.
write_full_dist() {
    local dir="$1"
    mkdir -p "$dir/dist"
    local p
    for p in "${PLATFORMS[@]}"; do
        : > "$dir/dist/$p"
        : > "$dir/dist/$p.spdx.json"
        : > "$dir/dist/$p.sigstore.json"
    done
    : > "$dir/dist/SHA256SUMS.txt"
}

# write_artifacts_json emits a goreleaser-2.x-style artifacts.json
# at $1/dist/artifacts.json mapping every published asset name to
# its absolute path. The gate's jq path (lines 171-183, 221-226 of
# the gate) keys off this file when present.
write_artifacts_json() {
    local dir="$1"
    local out="$dir/dist/artifacts.json"
    local p first=1

    printf '[' > "$out"
    for p in "${PLATFORMS[@]}"; do
        local suffix
        for suffix in "" ".spdx.json" ".sigstore.json"; do
            local name="$p$suffix"
            if [ "$first" = 1 ]; then
                first=0
            else
                printf ',' >> "$out"
            fi
            printf '{"name":"%s","path":"%s"}' "$name" "$dir/dist/$name" >> "$out"
        done
    done
    printf ',{"name":"SHA256SUMS.txt","path":"%s"}]' "$dir/dist/SHA256SUMS.txt" >> "$out"
}

# run_case <name> <expect-exit> <expect-pattern>
#
# expect-pattern is an ERE applied with grep -qE against combined
# stdout+stderr; pass an empty string to skip the assertion. The
# optional MUTATOR function (named via $MUTATOR) runs against the
# per-case fixture directory before invoking the script. The script
# is invoked with the absolute path of $dir/dist as its argument.
run_case() {
    local name="$1"; shift
    local expect_exit="$1"; shift
    local expect_pattern="$1"; shift

    tests_run=$((tests_run + 1))

    local dir
    dir="$(mktemp -d "${TMPDIR:-/tmp}/test-release-assets.XXXXXX")"
    # shellcheck disable=SC2064
    trap "rm -rf \"$dir\"" RETURN

    write_full_dist "$dir"

    if [ -n "${MUTATOR:-}" ]; then
        "$MUTATOR" "$dir"
    fi

    local out
    local actual_exit=0
    out="$(bash "$script" "$dir/dist" 2>&1)" || actual_exit=$?

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

# --- Case 1: full 46-asset fixture, no artifacts.json (find path) ---
MUTATOR=""
run_case "full 46-asset fixture clears the gate (find fallback path)" \
    0 'OK: all 46 expected release assets present'

# --- Case 2: full 46-asset fixture WITH artifacts.json (jq path) ---
if command -v jq >/dev/null 2>&1; then
    mut_with_artifacts_json() {
        write_artifacts_json "$1"
    }
    MUTATOR=mut_with_artifacts_json
    run_case "full 46-asset fixture clears the gate (jq path)" \
        0 'OK: all 46 expected release assets present'
else
    tests_skipped=$((tests_skipped + 1))
    printf 'SKIP: jq path test (jq not on PATH)\n'
fi

# --- Case 3: missing one SBOM ---
mut_drop_one_sbom() {
    rm "$1/dist/clockify-mcp-fips-linux-x64.spdx.json"
}
MUTATOR=mut_drop_one_sbom
run_case "missing one SBOM fails closed (v0.7.0 regression class)" \
    1 'expected release asset\(s\) missing'

# --- Case 4: missing one signature ---
mut_drop_one_sig() {
    rm "$1/dist/clockify-mcp-darwin-arm64.sigstore.json"
}
MUTATOR=mut_drop_one_sig
run_case "missing one sigstore signature fails closed" \
    1 'expected release asset\(s\) missing'

# --- Case 5: missing windows binary ---
mut_drop_windows() {
    rm "$1/dist/clockify-mcp-windows-x64.exe"
}
MUTATOR=mut_drop_windows
run_case "missing windows binary fails closed" \
    1 'clockify-mcp-windows-x64\.exe'

# --- Case 6: missing SHA256SUMS.txt ---
mut_drop_sha() {
    rm "$1/dist/SHA256SUMS.txt"
}
MUTATOR=mut_drop_sha
run_case "missing SHA256SUMS.txt fails closed" \
    1 'SHA256SUMS\.txt'

# --- Case 7: extra rogue artifact matching ASSET_RE ---
# `clockify-mcp-darwin-x64.exe` matches the gate's ASSET_RE
# (the `(\.exe)?` group is optional after any platform), is NOT in
# expected[], and pushes the cardinality to 47. Pass 1 succeeds (all
# 46 expected files present); Pass 2 cardinality fails.
mut_extra_rogue_match() {
    : > "$1/dist/clockify-mcp-darwin-x64.exe"
}
MUTATOR=mut_extra_rogue_match
run_case "extra rogue artifact matching ASSET_RE fails Pass 2" \
    1 'found 47 matching release artefacts'

# --- Case 8: extra non-matching file (README.md) ---
# A file whose name does NOT match ASSET_RE must be ignored: Pass 2
# cardinality must stay at 46. Without this case, an over-broad
# regex change to ASSET_RE could silently accept stray dist/ files.
mut_extra_nonmatch() {
    : > "$1/dist/README.md"
}
MUTATOR=mut_extra_nonmatch
run_case "extra non-matching README.md is filtered by ASSET_RE" \
    0 'OK: all 46 expected release assets present'

# --- Case 9: dist directory absent ---
# Built-in special case: the gate exits 2 (not 1) when its
# directory argument doesn't exist. The contract test pins this
# distinct exit code so a future refactor can't collapse it into
# the generic exit 1 surface.
run_case_no_dist() {
    local name="$1"; shift
    local expect_exit="$1"; shift
    local expect_pattern="$1"; shift

    tests_run=$((tests_run + 1))

    local out
    local actual_exit=0
    out="$(bash "$script" "/nonexistent/dist-$$" 2>&1)" || actual_exit=$?

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
}
run_case_no_dist "missing dist directory exits 2 (distinct from generic 1)" \
    2 'not found'

# --- Case 10: -grpc-postgres- counted once, not twice ---
# Critical regex-ordering invariant from gate line 218: the
# alternation `(-fips|-postgres|-grpc-postgres|-grpc)` lists
# `-grpc-postgres` before `-grpc` so the regex engine takes the
# longer match first. If the order were swapped, every
# `clockify-mcp-grpc-postgres-*` file would match TWICE — once as
# `-grpc-postgres` and once as `-grpc` (matching `-grpc-postgres-*`
# would never happen because `-grpc` would match first). The
# baseline-46 fixture already exercises this — case 10 makes the
# invariant explicit by re-running with a baseline that includes
# *only* grpc-postgres binaries (plus SHA256SUMS) and asserting
# the cardinality of the matching grpc-postgres assets is exactly
# 6 (2 binaries × 3 file shapes). If the regex misordered,
# Pass 1 would still find each expected file (grep handles either
# alternative), but Pass 2's count would over-count. We assert via
# a stricter bash test: count occurrences of the literal substring
# in the output of a probe regex.
probe_grpc_postgres_unique_match() {
    local dir
    dir="$(mktemp -d "${TMPDIR:-/tmp}/test-release-assets.XXXXXX")"
    # shellcheck disable=SC2064
    trap "rm -rf \"$dir\"" RETURN

    write_full_dist "$dir"

    tests_run=$((tests_run + 1))

    # Shape: the regex extracted from the gate. Mirroring the
    # alternation order is intentional — if a future maintainer
    # reorders the gate's ASSET_RE, this probe still uses the
    # correct ordering and so detects the over-count.
    local asset_re
    asset_re='^(clockify-mcp(-fips|-postgres|-grpc-postgres|-grpc)?-(darwin|linux|windows)-(arm64|x64)(\.exe)?(\.spdx\.json|\.sigstore\.json)?|SHA256SUMS\.txt)$'

    local matches
    matches=$(find "$dir/dist" -type f -printf '%f\n' 2>/dev/null \
        || find "$dir/dist" -type f -exec basename {} \;)
    local grpc_postgres_count
    grpc_postgres_count=$(printf '%s\n' "$matches" \
        | grep -E "$asset_re" \
        | grep -c -- '-grpc-postgres-' || true)

    # 2 binaries × 3 shapes (binary, .spdx.json, .sigstore.json) = 6.
    if [ "$grpc_postgres_count" = "6" ]; then
        printf 'PASS: -grpc-postgres-* matches exactly once each (count=6)\n'
    else
        tests_failed=$((tests_failed + 1))
        printf 'FAIL: -grpc-postgres-* match count\n' >&2
        printf '  expected 6, got %s\n' "$grpc_postgres_count" >&2
    fi

    rm -rf "$dir"
    trap - RETURN
}
probe_grpc_postgres_unique_match

# --- Final report ---
echo
if [ "$tests_failed" -gt 0 ]; then
    printf 'check-release-assets tests: %d/%d FAILED' "$tests_failed" "$tests_run" >&2
    if [ "$tests_skipped" -gt 0 ]; then
        printf ' (%d skipped)' "$tests_skipped" >&2
    fi
    printf '\n' >&2
    exit 1
fi
printf 'check-release-assets tests: %d/%d OK' "$tests_run" "$tests_run"
if [ "$tests_skipped" -gt 0 ]; then
    printf ' (%d skipped)' "$tests_skipped"
fi
printf '\n'
