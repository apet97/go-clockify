#!/usr/bin/env bash
#
# test-check-repo-hygiene.sh — regression test for check-repo-hygiene.sh.
#
# Locks the hygiene gate's externally-observable contract:
#   1.  Pass: clean tree (only README.md tracked) → exit 0
#   2.  Fail: tracked .DS_Store at root → exit 1
#   3.  Fail: tracked subdir/.DS_Store → exit 1 (`(^|/)` anchor)
#   4.  Fail: editor backup file ending in `~` → exit 1
#   5.  Fail: vim swap file `.swp` → exit 1
#   6.  Fail: tracked coverage.out → exit 1
#   7.  Fail: tracked *.exe → exit 1
#   8.  Pass: file whose name contains "orig" but does not end in
#       `.orig` must NOT trip the gate (anchor protection)
#   9.  Pass: file whose name starts with "Thumbs.db" but ends in a
#       suffix character must NOT trip the gate (exact-match anchor)
#
# The gate reads `git ls-files`, so each case builds a throwaway git
# repo in a per-case tmpdir, seeds known-clean files plus the listed
# fixture files, commits them under a synthetic identity (the gate
# itself only reads the index, but committing matches CI's invocation
# shape and guards against any future drift to a HEAD-based scan).

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-repo-hygiene.sh"

if [ ! -f "$script" ]; then
    echo "FAIL: script not found at $script" >&2
    exit 1
fi

tests_run=0
tests_failed=0

# init_git_repo lays down a git repo inside $1 with a known-clean
# README.md. Cases extend this via add_files() to track additional
# files (junk or otherwise).
init_git_repo() {
    local dir="$1"
    git init -q "$dir"
    (
        cd "$dir"
        git config user.email "test@example.invalid"
        git config user.name "test"
        git config commit.gpgsign false
        printf '# Test fixture\n' > README.md
        git add README.md
        git commit -q -m "init"
    )
}

# add_files stages and commits each path inside $1. Parent
# directories are created as needed. Files are written empty.
add_files() {
    local dir="$1"; shift
    (
        cd "$dir"
        for path in "$@"; do
            mkdir -p "$(dirname "$path")"
            : > "$path"
            git add "$path"
        done
        git commit -q -m "fixture"
    )
}

# run_case <name> <expect-exit> <expect-pattern>
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
    dir="$(mktemp -d "${TMPDIR:-/tmp}/test-check-repo-hygiene.XXXXXX")"
    # shellcheck disable=SC2064
    trap "rm -rf \"$dir\"" RETURN

    init_git_repo "$dir"

    if [ -n "${MUTATOR:-}" ]; then
        "$MUTATOR" "$dir"
    fi

    local out
    local actual_exit=0
    out="$(cd "$dir" && bash "$script" 2>&1)" || actual_exit=$?

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

# --- Case 1: clean tree ---
MUTATOR=""
run_case "clean tree clears the gate" \
    0 'repo-hygiene: OK'

# --- Case 2: tracked .DS_Store at root ---
mut_dsstore_root() {
    add_files "$1" .DS_Store
}
MUTATOR=mut_dsstore_root
run_case ".DS_Store at root fails closed" \
    1 'tracked junk files detected'

# --- Case 3: tracked subdir/.DS_Store ---
mut_dsstore_subdir() {
    add_files "$1" subdir/.DS_Store
}
MUTATOR=mut_dsstore_subdir
run_case "subdir/.DS_Store fails closed (\\(\\^\\|/\\) anchor catches subdir)" \
    1 'subdir/\.DS_Store'

# --- Case 4: editor backup ending in ~ ---
mut_backup_tilde() {
    add_files "$1" 'notes.md~'
}
MUTATOR=mut_backup_tilde
run_case "editor backup ending in ~ fails closed" \
    1 'notes\.md~'

# --- Case 5: vim swap file ---
mut_swap() {
    add_files "$1" vim.swp
}
MUTATOR=mut_swap
run_case "vim.swp fails closed (\\.swp\$ branch)" \
    1 'vim\.swp'

# --- Case 6: coverage.out ---
mut_coverage() {
    add_files "$1" coverage.out
}
MUTATOR=mut_coverage
run_case "coverage.out fails closed (named-file branch)" \
    1 'coverage\.out'

# --- Case 7: bin/clockify.exe ---
mut_exe() {
    add_files "$1" bin/clockify.exe
}
MUTATOR=mut_exe
run_case "bin/clockify.exe fails closed (\\.exe\$ branch)" \
    1 'clockify\.exe'

# --- Case 8: file with "orig" in name but not ending in .orig ---
# The gate's regex anchors `\.orig$` so files like "myorigfile.txt"
# (which contains the substring "orig" but not as a trailing
# extension) must NOT trip the gate. Without the `\.` and `$`
# anchors, a typo'd pattern would false-positive on any file
# containing "orig" anywhere — exactly the silent-overreach failure
# this case pins.
mut_origlike() {
    add_files "$1" myorigfile.txt
}
MUTATOR=mut_origlike
run_case "myorigfile.txt is not falsely flagged (\\.orig\$ anchor protects)" \
    0 'repo-hygiene: OK'

# --- Case 9: Thumbs.dbX is not Thumbs.db ---
# The gate's regex anchors `Thumbs\.db$` so a file named "Thumbs.dbX"
# (suffix added) must NOT trip the gate. Without the `$` anchor, the
# pattern would match any file whose name starts with "Thumbs.db",
# which would be wrong.
mut_thumbslike() {
    add_files "$1" Thumbs.dbX
}
MUTATOR=mut_thumbslike
run_case "Thumbs.dbX is not falsely flagged (exact-match \$ anchor)" \
    0 'repo-hygiene: OK'

# --- Final report ---
echo
if [ "$tests_failed" -gt 0 ]; then
    printf 'check-repo-hygiene tests: %d/%d FAILED\n' "$tests_failed" "$tests_run" >&2
    exit 1
fi
printf 'check-repo-hygiene tests: %d/%d OK\n' "$tests_run" "$tests_run"
