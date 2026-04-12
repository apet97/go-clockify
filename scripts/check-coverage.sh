#!/usr/bin/env bash
# Runs go test with coverage and enforces global + per-package floors.
# Called from CI and from `make cover-check` so both use identical logic.
#
# Environment variables:
#   COVERAGE_GLOBAL_FLOOR   Minimum total coverage % (default: 55)
#   COVERAGE_FLOORS         Space-separated "pkg=floor" pairs, e.g.
#                           "internal/mcp=62 internal/config=78".
#                           Default matches the current CI floors.
#   COVERAGE_SKIP_RUN       If non-empty, reuse existing coverage.out.
#   COVERAGE_OUT            Coverage profile path (default: coverage.out).
#
# Exit codes:
#   0  all floors cleared
#   1  one or more floors violated or tests failed

set -euo pipefail

GLOBAL_FLOOR="${COVERAGE_GLOBAL_FLOOR:-55}"
COVERAGE_OUT="${COVERAGE_OUT:-coverage.out}"
FLOORS_DEFAULT="internal/mcp=62 internal/config=78 internal/enforcement=85 internal/ratelimit=70 internal/logging=85 internal/jsonschema=85"
FLOORS="${COVERAGE_FLOORS:-$FLOORS_DEFAULT}"

if [ -z "${COVERAGE_SKIP_RUN:-}" ]; then
    echo "== coverage run (./internal/...) =="
    go test -race -count=1 -timeout 120s -coverprofile="$COVERAGE_OUT" ./internal/...
fi

if [ ! -f "$COVERAGE_OUT" ]; then
    echo "ERROR: coverage profile $COVERAGE_OUT not found" >&2
    exit 1
fi

TOTAL=$(go tool cover -func="$COVERAGE_OUT" | awk '/^total:/ {print $NF}' | tr -d '%')
printf 'Total coverage: %s%%\n' "$TOTAL"

if ! awk -v cov="$TOTAL" -v floor="$GLOBAL_FLOOR" 'BEGIN { exit (cov + 0 < floor + 0) ? 1 : 0 }'; then
    printf 'FAIL: total coverage %s%% below %s%% floor\n' "$TOTAL" "$GLOBAL_FLOOR" >&2
    exit 1
fi

PKG_LIST=""
for entry in $FLOORS; do
    pkg="${entry%%=*}"
    PKG_LIST="$PKG_LIST ./$pkg"
done

if [ -z "$PKG_LIST" ]; then
    printf 'OK: global floor cleared; no per-package floors configured\n'
    exit 0
fi

REPORT=$(go test -cover $PKG_LIST)
printf '%s\n' "$REPORT"

violations=0
for entry in $FLOORS; do
    pkg="${entry%%=*}"
    floor="${entry##*=}"
    cov=$(printf '%s\n' "$REPORT" | awk -v target="github.com/apet97/go-clockify/${pkg}" '$1 == "ok" && $2 == target { gsub("%", "", $5); print $5 }')
    if [ -z "$cov" ]; then
        printf 'FAIL: no coverage report for %s\n' "$pkg" >&2
        violations=$((violations + 1))
        continue
    fi
    if ! awk -v cov="$cov" -v floor="$floor" 'BEGIN { exit (cov + 0 < floor + 0) ? 1 : 0 }'; then
        printf 'FAIL: %s coverage %s%% below %s%% floor\n' "$pkg" "$cov" "$floor" >&2
        violations=$((violations + 1))
    else
        printf 'OK: %s %s%% >= %s%%\n' "$pkg" "$cov" "$floor"
    fi
done

if [ "$violations" -gt 0 ]; then
    printf '%d package(s) below coverage floor\n' "$violations" >&2
    exit 1
fi

printf 'OK: all coverage floors cleared (global %s%% >= %s%%)\n' "$TOTAL" "$GLOBAL_FLOOR"
