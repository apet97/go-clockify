#!/usr/bin/env bash
#
# Regression tests for check-api-parity-matrix.sh.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/check-api-parity-matrix.sh"

tests_run=0
tests_failed=0

assert_case() {
  local name="$1"
  local want_status="$2"
  local want_pattern="$3"
  local mutator="${MUTATOR:-}"
  local dir output status

  tests_run=$((tests_run + 1))
  dir="$(mktemp -d "${TMPDIR:-/tmp}/test-api-parity-matrix.XXXXXX")"
  mkdir -p "$dir/docs/goals"
  cat > "$dir/docs/tool-catalog.json" <<'JSON'
{
  "generator": "fixture",
  "tools": [
    {
      "name": "clockify_status",
      "category": "workflow",
      "handler_kind": "native handler",
      "read_only": true,
      "destructive": false,
      "idempotent": true,
      "dry_run": false,
      "risk_class": ["read"],
      "input_schema": {
        "type": "object",
        "properties": {},
        "additionalProperties": false
      },
      "output_schema": {
        "type": "object",
        "properties": {
          "data": {"type": "object"}
        }
      }
    },
    {
      "name": "clockify_api_request",
      "handler_kind": "native handler",
      "read_only": false,
      "destructive": false,
      "idempotent": false,
      "dry_run": false,
      "risk_class": ["write"],
      "input_schema": {
        "type": "object",
        "required": ["method", "path"],
        "properties": {
          "method": {"type": "string"},
          "path": {"type": "string"},
          "query": {"type": "object"},
          "body": {"type": "object"}
        },
        "additionalProperties": false
      },
      "output_schema": {
        "type": "object",
        "properties": {
          "data": {"description": "Tool-specific payload for clockify_api_request"}
        }
      }
    }
  ]
}
JSON
  cat > "$dir/docs/goals/oneuser-tool-coverage.md" <<'MD'
# One-user tool coverage

Summary:
- Total tools: 2
- Workflow tools: 1
- Domain tools: 0
- Raw fallback tools: 1
- Fake-smoke yes: 2
- Live protocol/recovery tested yes: 1
- Live happy-path tested yes: 1

| Tool | Class | Handler | Endpoint / method | Fake smoke | Live protocol/recovery tested | Live happy-path tested | Output schema | Status | Next action |
|------|-------|---------|-------------------|-------------|--------------------------------|------------------------|---------------|--------|-------------|
| `clockify_status` | workflow | native handler | native composite | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_api_request` | raw | raw fallback | caller-supplied method/path | yes | raw_fallback_only | raw_fallback_only | generic | raw_fallback_only | keep_raw_fallback_last |
MD

  "$script" --repo-root "$dir" --write >/dev/null
  if [ -n "$mutator" ]; then
    "$mutator" "$dir"
  fi

  set +e
  output="$("$script" --repo-root "$dir" 2>&1)"
  status=$?
  set -e

  if [ "$status" -ne "$want_status" ] || ! grep -Eq "$want_pattern" <<<"$output"; then
    tests_failed=$((tests_failed + 1))
    printf 'FAIL: %s\nwanted status=%s pattern=%s\ngot status=%s output:\n%s\n' \
      "$name" "$want_status" "$want_pattern" "$status" "$output" >&2
  fi
  rm -rf "$dir"
  unset MUTATOR
}

assert_case "freshly generated fixture passes" 0 "api-parity-matrix: OK"

mut_remove_row() {
  perl -0pi -e 's/\n\| `clockify_api_request` \|[^\n]+//' "$1/docs/api-parity-matrix.md"
}
MUTATOR=mut_remove_row
assert_case "missing catalog row fails closed" 1 "matrix row count drift"

mut_stale_required_args() {
  perl -0pi -e 's/`method`, `path`/`method`/' "$1/docs/api-parity-matrix.md"
}
MUTATOR=mut_stale_required_args
assert_case "stale required args fail closed" 1 "api-parity-matrix drift"

if [ "$tests_failed" -ne 0 ]; then
  printf 'check-api-parity-matrix tests: %d/%d FAILED\n' "$tests_failed" "$tests_run" >&2
  exit 1
fi
printf 'check-api-parity-matrix tests: %d/%d OK\n' "$tests_run" "$tests_run"
