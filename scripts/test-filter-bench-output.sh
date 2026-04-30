#!/usr/bin/env bash
#
# test-filter-bench-output.sh — regression test for filter-bench-output.sh.
#
# Locks the filter's externally-observable contract:
#   - benchstat metadata (goos / goarch / pkg / cpu) passes through
#   - "ok" footer with trailing whitespace passes through
#   - complete benchmark rows pass through unchanged
#   - slog/log noise unrelated to a benchmark row is dropped
#   - a benchmark row split by slog contamination is reassembled into one
#     benchstat-compatible row (name + metrics, joined by a tab)
#
# Pure bash + a single invocation of the filter; runs in milliseconds.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
filter="$repo_root/scripts/filter-bench-output.sh"

if [ ! -x "$filter" ] && [ ! -f "$filter" ]; then
    echo "FAIL: filter script not found at $filter" >&2
    exit 1
fi

# Heredoc with explicit \t escapes via $'...' so tabs survive intact.
input=$'goos: darwin
goarch: arm64
pkg: github.com/apet97/go-clockify/internal/foo
cpu: Apple M1
BenchmarkSimple-8   \t  100000\t     12345 ns/op\t     128 B/op\t       2 allocs/op
2026/04/30 14:30:00 INFO some slog noise here
BenchmarkSplit-8   \t2026/04/30 14:30:01 INFO contamination during row write
   500000\t      234.5 ns/op\t      48 B/op\t       1 allocs/op
random unmatched log spam
BenchmarkPlain-8   \t  200000\t      567 ns/op
ok  \tgithub.com/apet97/go-clockify/internal/foo\t2.345s'

expected=$'goos: darwin
goarch: arm64
pkg: github.com/apet97/go-clockify/internal/foo
cpu: Apple M1
BenchmarkSimple-8   \t  100000\t     12345 ns/op\t     128 B/op\t       2 allocs/op
BenchmarkSplit-8\t500000\t      234.5 ns/op\t      48 B/op\t       1 allocs/op
BenchmarkPlain-8   \t  200000\t      567 ns/op
ok  \tgithub.com/apet97/go-clockify/internal/foo\t2.345s'

actual="$(printf '%s\n' "$input" | bash "$filter")"

if [ "$actual" = "$expected" ]; then
    echo "filter-bench-output: OK"
    exit 0
fi

echo "FAIL: filter-bench-output produced unexpected output" >&2
echo "--- expected ---" >&2
printf '%s\n' "$expected" >&2
echo "--- actual ---" >&2
printf '%s\n' "$actual" >&2
echo "--- diff (expected vs actual) ---" >&2
diff <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") >&2 || true
exit 1
