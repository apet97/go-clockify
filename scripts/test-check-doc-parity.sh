#!/usr/bin/env bash
#
# Focused regression harness for scripts/check-doc-parity.sh.
#
# The checker is intentionally scoped to the current one-user product path:
# a flat generated catalog with 151 tools, minimal env surface, current
# operator docs, and historical hosted-era material excluded from the public
# one-user gate.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-doc-parity.sh"

tests_run=0
tests_failed=0

write_catalog() {
  local dir="$1"
  local count="${2:-151}"
  python3 - "$dir/docs/tool-catalog.json" "$count" <<'PY'
import json
import sys

path = sys.argv[1]
count = int(sys.argv[2])
required = [
    "clockify_status",
    "clockify_log_work",
    "clockify_review_day",
    "clockify_api_get",
    "clockify_api_request",
]
names = required[:count]
idx = 1
while len(names) < count:
    names.append(f"clockify_fixture_{idx:03d}")
    idx += 1
with open(path, "w", encoding="utf-8") as fh:
    json.dump({"tools": [{"name": name} for name in names]}, fh)
PY
}

write_baseline_tree() {
  local dir="$1"
  mkdir -p \
    "$dir/internal/config" \
    "$dir/cmd" \
    "$dir/tests" \
    "$dir/docs/adr" \
    "$dir/docs/goals" \
    "$dir/docs/archive" \
    "$dir/docs/openapi/sources" \
    "$dir/npm/clockify-mcp-go" \
    "$dir/deploy"

  cat > "$dir/internal/config/oneuser.go" <<'EOF'
package config

const (
	apiKeyEnv = "CLOCKIFY_API_KEY"
	workspaceEnv = "CLOCKIFY_WORKSPACE_ID"
	timezoneEnv = "CLOCKIFY_TIMEZONE"
	baseURLEnv = "CLOCKIFY_BASE_URL"
	toolsetEnv = "CLOCKIFY_TOOLSET"
	timeoutEnv = "CLOCKIFY_TOOL_TIMEOUT"
	inFlightEnv = "CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS"
	messageSizeEnv = "CLOCKIFY_MAX_MESSAGE_SIZE"
	rawWritesEnv = "CLOCKIFY_ENABLE_RAW_WRITES"
	webhookAllowedEnv = "CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS"
	logLevelEnv = "MCP_LOG_LEVEL"
)
EOF

  cat > "$dir/tests/live_env_test.go" <<'EOF'
package tests

const liveEnv = "CLOCKIFY_RUN_LIVE_E2E"
EOF

  write_catalog "$dir" 151

  cat > "$dir/README.md" <<'EOF'
# go-clockify

One-user local Clockify MCP with 151 tools.

| Runtime | Requirement |
| --- | --- |
| Node.js (npm wrapper) | 22+ |

Use `CLOCKIFY_API_KEY`, `CLOCKIFY_WORKSPACE_ID`, and `CLOCKIFY_TOOLSET`.
Start with `clockify_status`, then use `clockify_log_work` for time entries.
EOF

  cat > "$dir/AGENTS.md" <<'EOF'
# AGENTS

One local trusted user.
One `CLOCKIFY_API_KEY`.
One required `CLOCKIFY_WORKSPACE_ID`.
Stdio transport only.
Exactly 151 tools loaded at startup.
EOF

  cat > "$dir/docs/agent-cookbook.md" <<'EOF'
# Agent Cookbook

Use `clockify_status` before changing the pinned workspace.
EOF

  cat > "$dir/docs/clients.md" <<'EOF'
# Clients

The local stdio package exposes 151 tools.
EOF

  cat > "$dir/docs/api-coverage.md" <<'EOF'
# API Coverage

The generated catalog currently exposes 151 tools.
EOF

  cat > "$dir/docs/live-tests.md" <<'EOF'
# Live Tests

Set `CLOCKIFY_RUN_LIVE_E2E=1` only for sacrificial live validation.
EOF

  cat > "$dir/docs/goals/oneuser-tool-coverage.md" <<'EOF'
# One-User Tool Coverage

`clockify_review_day` is covered by the one-user workflow tests.
EOF

  cat > "$dir/docs/launch-readiness-review-may-8.md" <<'EOF'
# May 8 Review Ledger

Current one-user completion audit, not current launch marketing.
EOF

  cat > "$dir/docs/archive/historical.md" <<'EOF'
# Historical

Old Tier 2 and hosted launch notes are archived here.
EOF

  cat > "$dir/docs/openapi/sources/source.md" <<'EOF'
# Source

An uploaded source may mention clockify_old_source_tool without becoming current docs.
EOF

  cat > "$dir/docs/adr/README.md" <<'EOF'
| ID | Title |
| --- | --- |
| 0001 | [Accepted decision](0001-decision.md) |
EOF

  cat > "$dir/docs/adr/0001-decision.md" <<'EOF'
# Accepted decision

## Status

Accepted
EOF

  cat > "$dir/npm/clockify-mcp-go/package.json" <<'EOF'
{"engines":{"node":">=22"}}
EOF
}

run_case() {
  local name="$1"
  local want_exit="$2"
  local want_pattern="$3"
  local mutator="${4:-}"

  tests_run=$((tests_run + 1))
  local dir
  dir="$(mktemp -d)"
  write_baseline_tree "$dir"
  if [ -n "$mutator" ]; then
    "$mutator" "$dir"
  fi

  local output status
  set +e
  output="$(cd "$dir" && bash "$script" 2>&1)"
  status=$?
  set -e
  rm -rf "$dir"

  if [ "$status" -ne "$want_exit" ] || ! grep -Eq "$want_pattern" <<< "$output"; then
    tests_failed=$((tests_failed + 1))
    echo "FAIL: $name" >&2
    echo "  expected exit=$want_exit pattern=$want_pattern" >&2
    echo "  got exit=$status" >&2
    echo "  --- output ---" >&2
    sed 's/^/  /' <<< "$output" >&2
    echo "  --- end ---" >&2
  else
    echo "PASS: $name"
  fi
}

bad_catalog_shape() {
  echo '{"tier1":[],"tier2":[]}' > "$1/docs/tool-catalog.json"
}

bad_catalog_count() {
  write_catalog "$1" 150
}

undefined_env() {
  echo 'Use `CLOCKIFY_NOT_REAL`.' >> "$1/README.md"
}

undefined_env_opted_out() {
  mkdir -p "$1/deploy"
  echo "CLOCKIFY_NOT_REAL deprecated-fixture" > "$1/deploy/.config-parity-opt-out.txt"
  echo 'Use `CLOCKIFY_NOT_REAL`.' >> "$1/README.md"
}

unknown_tool() {
  echo 'Use `clockify_missing_tool`.' >> "$1/README.md"
}

remove_catalog() {
  rm "$1/docs/tool-catalog.json"
}

count_drift() {
  perl -0pi -e 's/151 tools/150 tools/' "$1/README.md"
}

stale_public_language() {
  echo 'This hosted launch uses Tier 2 policy modes.' >> "$1/docs/agent-cookbook.md"
}

historical_stale_language() {
  echo 'More multi-tenant hosted launch context.' >> "$1/docs/archive/historical.md"
}

missing_agents_contract() {
  grep -v 'Exactly 151 tools loaded at startup' "$1/AGENTS.md" > "$1/AGENTS.tmp"
  mv "$1/AGENTS.tmp" "$1/AGENTS.md"
}

node_drift() {
  echo '{"engines":{"node":">=23"}}' > "$1/npm/clockify-mcp-go/package.json"
}

dangling_marker() {
  echo 'TODO: finish current operator guidance.' >> "$1/docs/clients.md"
}

adr_status_drift() {
  perl -0pi -e 's/Accepted\n/Proposed\n/' "$1/docs/adr/0001-decision.md"
}

run_case "clean flat one-user baseline passes" 0 "doc-parity: OK"
run_case "flat catalog shape is required" 1 "unable to parse flat tool catalog" bad_catalog_shape
run_case "flat catalog must stay at 151 tools" 1 "tool count drift: found 150 tools, expected 151" bad_catalog_count
run_case "undefined current-doc env var fails" 1 "env var referenced in current docs but not defined" undefined_env
run_case "opt-out permits otherwise undefined env var" 0 "doc-parity: OK" undefined_env_opted_out
run_case "unknown current-doc tool fails" 1 "tool referenced in current docs but not in" unknown_tool
run_case "missing catalog warns and passes remaining checks" 0 "\\[warn\\].*tool-catalog\\.json missing" remove_catalog
run_case "public count drift fails" 1 "public tool-count claim drift" count_drift
run_case "stale one-user public language fails" 1 "stale hosted/multi-user language" stale_public_language
run_case "historical stale language is ignored" 0 "doc-parity: OK" historical_stale_language
run_case "AGENTS product contract is required" 1 "AGENTS\\.md must preserve one-user product contract term" missing_agents_contract
run_case "README node compatibility follows npm wrapper" 1 "README Node\\.js .* does not match" node_drift
run_case "dangling marker in current docs fails" 1 "dangling marker in current operator doc" dangling_marker
run_case "ADR index status drift fails" 1 "ADR index status drift" adr_status_drift

if [ "$tests_failed" -ne 0 ]; then
  echo >&2
  echo "check-doc-parity tests: $tests_failed/$tests_run FAILED" >&2
  exit 1
fi

echo
echo "check-doc-parity tests: $tests_run/$tests_run OK"
