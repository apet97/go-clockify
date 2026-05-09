#!/usr/bin/env bash
#
# check-go-version-parity.sh
#
# Fails when the repo's Go patch pin drifts across module files,
# workflow setup-go pins, or current public support docs. Historical
# benchmark snapshots and archived notes are intentionally out of scope.

set -euo pipefail

fail=0

err() {
  echo "[fail] $*" >&2
  fail=1
}

if [ ! -f go.mod ]; then
  err "go.mod missing"
  echo "go-version-parity: FAIL" >&2
  exit 1
fi

expected="$(awk '$1 == "go" { print $2; exit }' go.mod)"
if [ -z "$expected" ]; then
  err "unable to read Go version from go.mod"
  echo "go-version-parity: FAIL" >&2
  exit 1
fi
expected_minor="${expected%.*}"

check_go_directive() {
  local file="$1"
  local found
  found="$(awk '$1 == "go" { print $2; exit }' "$file")"
  if [ -z "$found" ]; then
    err "$file missing go directive"
    return
  fi
  if [ "$found" != "$expected" ]; then
    err "$file go directive is $found, expected $expected"
  fi
}

if [ -f go.work ]; then
  check_go_directive go.work
fi

while IFS= read -r -d '' file; do
  check_go_directive "$file"
done < <(find . \
  \( -path './.git' -o -path './go-clockify' -o -path './.local' -o -path './.review' \) -prune -o \
  -name go.mod -print0)

workflow_hits=0
while IFS= read -r line; do
  [ -n "$line" ] || continue
  workflow_hits=$((workflow_hits + 1))
  file="${line%%:*}"
  rest="${line#*:}"
  line_no="${rest%%:*}"
  text="${rest#*:}"
  found="$(sed -E 's/.*go-version:[[:space:]]*"?([^"# ]+)"?.*/\1/' <<< "$text")"
  if [ "$found" != "$expected" ]; then
    err "$file:$line_no go-version is $found, expected $expected"
  fi
done < <(grep -RInE 'go-version:[[:space:]]*"?[0-9]+\.[0-9]+\.[0-9]+' .github/workflows 2>/dev/null || true)

if [ "$workflow_hits" -eq 0 ]; then
  err "no explicit setup-go go-version pins found in .github/workflows"
fi

current_doc_patterns=(
  "README.md:Go ${expected}, stdlib only"
  "README.md:| Go | ${expected} pinned |"
  "CONTRIBUTING.md:Requires the pinned Go ${expected} toolchain"
  "CONTRIBUTING.md:This project pins to **Go ${expected}**"
  "CONTRIBUTING.md:\`go ${expected}\`"
  "CONTRIBUTING.md:\`go-version: \"${expected}\"\`"
  "docs/support-matrix.md:Go \`${expected}\`"
  "CHANGELOG.md:docs/support-matrix.md\` now records the Go ${expected}"
  "cmd/clockify-mcp/fips_on.go:Go ${expected} (the repo's go.mod floor)"
)

for entry in "${current_doc_patterns[@]}"; do
  file="${entry%%:*}"
  pattern="${entry#*:}"
  if [ ! -f "$file" ]; then
    err "current Go-version doc file missing: $file"
    continue
  fi
  if ! grep -Fq "$pattern" "$file"; then
    err "$file missing current Go-version text: $pattern"
  fi
done

if ! grep -Eq "FROM --platform=\\\$BUILDPLATFORM golang:${expected_minor//./\\.}-bookworm@sha256:[0-9a-f]{64} AS builder" deploy/Dockerfile 2>/dev/null; then
  err "deploy/Dockerfile missing pinned golang:${expected_minor}-bookworm builder digest"
fi

if [ "$fail" != "0" ]; then
  echo "go-version-parity: FAIL" >&2
  exit 1
fi

echo "go-version-parity: OK ($expected)"
