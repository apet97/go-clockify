#!/usr/bin/env bash
#
# test-check-go-version-parity.sh - regression test for
# check-go-version-parity.sh.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-go-version-parity.sh"

if [ ! -f "$script" ]; then
  echo "FAIL: script not found at $script" >&2
  exit 1
fi

tests_run=0
tests_failed=0

write_baseline_tree() {
  local dir="$1"

  mkdir -p "$dir/.github/workflows" "$dir/internal/transport/grpc" \
    "$dir/internal/controlplane/postgres" "$dir/cmd/clockify-mcp" \
    "$dir/docs" "$dir/deploy" \
    "$dir/go-clockify"

  cat > "$dir/go.mod" <<'EOF'
module example.test/root

go 1.25.10
EOF

  cat > "$dir/go.work" <<'EOF'
go 1.25.10

use (
    .
    ./internal/transport/grpc
    ./internal/controlplane/postgres
)
EOF

  cat > "$dir/internal/transport/grpc/go.mod" <<'EOF'
module example.test/root/internal/transport/grpc

go 1.25.10
EOF

  cat > "$dir/internal/controlplane/postgres/go.mod" <<'EOF'
module example.test/root/internal/controlplane/postgres

go 1.25.10
EOF

  # Nested local checkout noise is intentionally ignored by the gate.
  cat > "$dir/go-clockify/go.mod" <<'EOF'
module example.test/nested

go 1.20.1
EOF

  cat > "$dir/.github/workflows/ci.yml" <<'EOF'
jobs:
  test:
    steps:
      - uses: actions/setup-go@sha
        with:
          go-version: "1.25.10"
  repro:
    steps:
      - uses: actions/setup-go@sha
        with:
          go-version-file: go.mod
EOF

  cat > "$dir/.github/workflows/bench.yml" <<'EOF'
jobs:
  bench:
    steps:
      - uses: actions/setup-go@sha
        with:
          go-version: "1.25.10"
EOF

  cat > "$dir/README.md" <<'EOF'
# Test fixture

Go 1.25.10, stdlib only.

| Component | Version |
|-----------|---------|
| Go | 1.25.10 pinned |
EOF

  cat > "$dir/CONTRIBUTING.md" <<'EOF'
# Contributing

Requires the pinned Go 1.25.10 toolchain.

This project pins to **Go 1.25.10**.

`go 1.25.10`
`go-version: "1.25.10"`
EOF

  cat > "$dir/docs/support-matrix.md" <<'EOF'
# Support Matrix

Go `1.25.10` is used for local builds and CI.
EOF

  cat > "$dir/CHANGELOG.md" <<'EOF'
# Changelog

`docs/support-matrix.md` now records the Go 1.25.10 launch-candidate pin.
EOF

  cat > "$dir/cmd/clockify-mcp/fips_on.go" <<'EOF'
package main

// Go 1.25.10 (the repo's go.mod floor) ships only Enabled().
func fipsCommentFixture() {}
EOF

  cat > "$dir/deploy/Dockerfile" <<'EOF'
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa AS builder
EOF
}

run_case() {
  local name="$1"; shift
  local expect_exit="$1"; shift
  local expect_pattern="$1"; shift

  tests_run=$((tests_run + 1))

  local dir
  dir="$(mktemp -d "${TMPDIR:-/tmp}/test-go-version-parity.XXXXXX")"
  trap 'rm -rf "$dir"' RETURN

  write_baseline_tree "$dir"

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

MUTATOR=""
run_case "clean baseline clears the gate" 0 'go-version-parity: OK \(1\.25\.10\)'

mut_work_mismatch() {
  perl -0pi -e 's/go 1\.25\.10/go 1.25.9/' "$1/go.work"
}
MUTATOR=mut_work_mismatch
run_case "go.work mismatch fails closed" 1 'go\.work go directive is 1\.25\.9'

mut_submodule_mismatch() {
  perl -0pi -e 's/go 1\.25\.10/go 1.25.9/' "$1/internal/transport/grpc/go.mod"
}
MUTATOR=mut_submodule_mismatch
run_case "submodule go.mod mismatch fails closed" 1 'internal/transport/grpc/go\.mod go directive is 1\.25\.9'

mut_workflow_mismatch() {
  perl -0pi -e 's/go-version: "1\.25\.10"/go-version: "1.25.9"/' "$1/.github/workflows/bench.yml"
}
MUTATOR=mut_workflow_mismatch
run_case "workflow go-version mismatch fails closed" 1 'bench\.yml:.*go-version is 1\.25\.9'

mut_readme_mismatch() {
  perl -0pi -e 's/\| Go \| 1\.25\.10 pinned \|/| Go | 1.25.9 pinned |/' "$1/README.md"
}
MUTATOR=mut_readme_mismatch
run_case "README current Go row mismatch fails closed" 1 'README\.md missing current Go-version text'

mut_changelog_mismatch() {
  perl -0pi -e 's/Go 1\.25\.10 launch-candidate pin/Go 1.25.9 launch-candidate pin/' "$1/CHANGELOG.md"
}
MUTATOR=mut_changelog_mismatch
run_case "CHANGELOG current Go pin wording mismatch fails closed" 1 'CHANGELOG\.md missing current Go-version text'

mut_fips_comment_mismatch() {
  perl -0pi -e 's/Go 1\.25\.10 \(the repo'\''s go\.mod floor\)/Go 1.25.9 (the repo'\''s go.mod floor)/' "$1/cmd/clockify-mcp/fips_on.go"
}
MUTATOR=mut_fips_comment_mismatch
run_case "FIPS comment current Go floor mismatch fails closed" 1 'cmd/clockify-mcp/fips_on\.go missing current Go-version text'

mut_missing_digest() {
  perl -0pi -e 's/@sha256:[a-f0-9]{64}//' "$1/deploy/Dockerfile"
}
MUTATOR=mut_missing_digest
run_case "Docker builder digest missing fails closed" 1 'Dockerfile missing pinned golang'

mut_nested_checkout_only() {
  perl -0pi -e 's/go 1\\.20\\.1/go 1.19.9/' "$1/go-clockify/go.mod"
}
MUTATOR=mut_nested_checkout_only
run_case "nested go-clockify checkout is ignored" 0 'go-version-parity: OK'

if [ "$tests_failed" -ne 0 ]; then
  printf 'check-go-version-parity tests: %d/%d FAILED\n' "$tests_failed" "$tests_run" >&2
  exit 1
fi

printf 'check-go-version-parity tests: %d/%d OK\n' "$tests_run" "$tests_run"
