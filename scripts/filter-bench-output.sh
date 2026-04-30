#!/usr/bin/env bash
#
# filter-bench-output.sh — normalize `go test -bench` output for benchstat.
#
# Some benchmarks emit slog lines while the testing package is writing a result
# row. That can split a row into:
#
#   BenchmarkName-4    2026/...
#        100  1234 ns/op ...
#
# benchstat needs the benchmark name and metrics on one line. This filter keeps
# package metadata and reconstructs those split rows while dropping log noise.

set -euo pipefail

awk '
	function emit(line) {
		sub(/[[:space:]]+$/, "", line)
		print line
	}

	/^(goos|goarch|pkg|cpu):/ || /^(PASS|ok)[[:space:]]/ {
		emit($0)
		next
	}

	/^Benchmark[^[:space:]]+[[:space:]]+[0-9]+[[:space:]]+[0-9.]+[[:space:]]+ns\/op/ {
		emit($0)
		pending = ""
		next
	}

	/^Benchmark[^[:space:]]+/ {
		pending = $1
		next
	}

	pending != "" && /^[[:space:]]+[0-9]+[[:space:]]+[0-9.]+[[:space:]]+ns\/op/ {
		sub(/^[[:space:]]+/, "")
		emit(pending "\t" $0)
		pending = ""
		next
	}
'
