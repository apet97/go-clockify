#!/usr/bin/env bash
#
# check-bench-baseline.sh — validate the committed CI benchmark baseline.
#
# The weekly benchmark workflow compares fresh ubuntu-latest samples against
# internal/benchdata/baseline.txt. This gate fails when that committed baseline
# is missing workflow packages, mixes in non-linux/amd64 output, or has too few
# samples for benchstat to make a useful comparison.

set -euo pipefail

baseline="${1:-internal/benchdata/baseline.txt}"
workflow="${BENCH_WORKFLOW:-.github/workflows/bench.yml}"

echo "== bench-baseline =="

if [ ! -s "$baseline" ]; then
	echo "[fail] $baseline missing or empty" >&2
	exit 1
fi

if [ ! -f "$workflow" ]; then
	echo "[fail] benchmark workflow $workflow not found" >&2
	exit 1
fi

module=$(awk '$1 == "module" { print $2; exit }' go.mod)
if [ -z "$module" ]; then
	echo "[fail] unable to read module path from go.mod" >&2
	exit 1
fi

count=$(awk '
	/go test -bench=\. / {
		for (i = 1; i <= NF; i++) {
			if ($i ~ /^-count=/) {
				sub(/^-count=/, "", $i)
				print $i
				exit
			}
		}
	}
' "$workflow")
sample_floor="${BENCH_BASELINE_MIN_SAMPLES:-${count:-6}}"

expected_pkgs=$(awk '
	/go test -bench=\. / { in_bench = 1 }
	in_bench {
		for (i = 1; i <= NF; i++) {
			token = $i
			gsub(/\\$/, "", token)
			if (token ~ /^\.\/internal\//) print token
		}
	}
	in_bench && /> \/tmp\/bench-raw\.txt/ { exit }
' "$workflow" | sed 's#^\./##' | sort -u)

if [ -z "$expected_pkgs" ]; then
	echo "[fail] unable to derive benchmark packages from $workflow" >&2
	exit 1
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

awk_status=0
awk -v module="$module" -v floor="$sample_floor" '
	/^goos: / {
		goos = $2
		next
	}
	/^goarch: / {
		goarch = $2
		next
	}
	/^pkg: / {
		pkg = $2
		seen_pkg[pkg] = 1
		pkg_goos[pkg] = goos
		pkg_goarch[pkg] = goarch
		next
	}
	/^Benchmark/ {
		name = $1
		sub(/-[0-9]+$/, "", name)
		key = pkg "\t" name
		samples[key]++
		bench_pkg[key] = pkg
		bench_name[key] = name
		next
	}
	END {
		for (pkg in seen_pkg) {
			print pkg > "/dev/stderr"
			if (pkg_goos[pkg] != "linux" || pkg_goarch[pkg] != "amd64") {
				printf("[fail] %s baseline is %s/%s, want linux/amd64\n", pkg, pkg_goos[pkg], pkg_goarch[pkg]) > "/dev/stderr"
				fail = 1
			}
		}
		for (key in samples) {
			if (samples[key] < floor) {
				printf("[fail] %s %s has %d sample(s), want at least %d\n", bench_pkg[key], bench_name[key], samples[key], floor) > "/dev/stderr"
				fail = 1
			}
		}
	exit fail
	}
' "$baseline" 2>"$tmpdir/baseline-report" || awk_status=$?

actual_pkgs=$(grep -E '^github\.com/.+/internal/' "$tmpdir/baseline-report" | sort -u || true)
cat "$tmpdir/baseline-report" | grep -E '^\[fail\]' >&2 || true

missing=0
while IFS= read -r pkg; do
	[ -z "$pkg" ] && continue
	full_pkg="$module/$pkg"
	if ! grep -qxF "$full_pkg" <<< "$actual_pkgs"; then
		echo "[fail] missing benchmark package in baseline: $full_pkg" >&2
		missing=1
	fi
done <<< "$expected_pkgs"

extra=0
while IFS= read -r full_pkg; do
	[ -z "$full_pkg" ] && continue
	local_pkg="${full_pkg#"$module"/}"
	if ! grep -qxF "$local_pkg" <<< "$expected_pkgs"; then
		echo "[fail] unexpected benchmark package in baseline: $full_pkg" >&2
		extra=1
	fi
done <<< "$actual_pkgs"

if [ "$awk_status" -ne 0 ] || [ "$missing" -ne 0 ] || [ "$extra" -ne 0 ]; then
	exit 1
fi

echo "bench-baseline: OK ($baseline, min samples: $sample_floor)"
