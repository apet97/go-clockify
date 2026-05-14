#!/usr/bin/env bash
# Builds clockify-mcp and smoke-tests the stdio loop:
#   - pipes initialize + tools/list + prompts/list + resources/list,
#   - closes stdin so Run() flushes and exits,
#   - asserts:
#       initialize  -> serverInfo.name == clockify-go-mcp,
#       tools/list  -> >=100 tools, first=clockify_status, last=clockify_api_request,
#       no forbidden activation/policy tools in any response,
#       prompts/list  -> >=1,
#       resources/list -> >=1.
#
# Requires: jq. Installed by default on ubuntu-latest runners and via
# Homebrew on macOS.

set -euo pipefail

BIN="${TMPDIR:-/tmp}/clockify-mcp-stdio-smoke"
OUT="$(mktemp "${TMPDIR:-/tmp}/clockify-mcp-stdio-smoke.out.XXXXXX")"
ERR="$(mktemp "${TMPDIR:-/tmp}/clockify-mcp-stdio-smoke.err.XXXXXX")"

cleanup() {
    rm -f "$BIN" "$OUT" "$ERR"
}
trap cleanup EXIT

if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: jq is required but not installed" >&2
    exit 2
fi

go build -o "$BIN" ./cmd/clockify-mcp

REQUESTS=$'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"smoke","version":"0"}}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n{"jsonrpc":"2.0","id":3,"method":"prompts/list","params":{}}\n{"jsonrpc":"2.0","id":4,"method":"resources/list","params":{}}\n'

set +e
printf '%s' "$REQUESTS" \
    | CLOCKIFY_API_KEY="${CLOCKIFY_API_KEY:-smoke-test-dummy}" \
      CLOCKIFY_WORKSPACE_ID="${CLOCKIFY_WORKSPACE_ID:-00000000000000000000abcd}" \
      "$BIN" \
      >"$OUT" 2>"$ERR"
rc=$?
set -e

if [ "$rc" -ne 0 ]; then
    echo "FAIL: clockify-mcp exited with status $rc" >&2
    echo "--- stderr ---" >&2
    cat "$ERR" >&2
    echo "--- stdout ---" >&2
    cat "$OUT" >&2
    exit 1
fi

line_count=$(grep -c . "$OUT" || true)
if [ "$line_count" -lt 4 ]; then
    echo "FAIL: expected >=4 JSON-RPC responses on stdout, got $line_count" >&2
    echo "--- stdout ---" >&2
    cat "$OUT" >&2
    echo "--- stderr ---" >&2
    cat "$ERR" >&2
    exit 1
fi

init_name=$(jq -r 'select(.id == 1) | .result.serverInfo.name // empty' "$OUT")
if [ "$init_name" != "clockify-go-mcp" ]; then
    echo "FAIL: initialize response missing result.serverInfo.name=clockify-go-mcp" >&2
    cat "$OUT" >&2
    exit 1
fi
echo "OK: initialize -> serverInfo.name=$init_name"

tool_count=$(jq -r 'select(.id == 2) | .result.tools | length' "$OUT")
first_tool=$(jq -r 'select(.id == 2) | .result.tools[0].name' "$OUT")
last_tool=$(jq -r 'select(.id == 2) | .result.tools[-1].name' "$OUT")
if [ -z "$tool_count" ] || [ "$tool_count" -lt 100 ]; then
    echo "FAIL: tools/list returned ${tool_count:-?} tools (expected >=100)" >&2
    exit 1
fi
if [ "$first_tool" != "clockify_status" ]; then
    echo "FAIL: tools/list[0] = $first_tool (expected clockify_status)" >&2
    exit 1
fi
if [ "$last_tool" != "clockify_api_request" ]; then
    echo "FAIL: tools/list[-1] = $last_tool (expected clockify_api_request)" >&2
    exit 1
fi
echo "OK: tools/list -> $tool_count tools (first=$first_tool, last=$last_tool)"

for forbidden in clockify_activate_group clockify_activate_tool clockify_deactivate_group clockify_search_tools clockify_list_tools clockify_policy_info; do
    if grep -q "\"$forbidden\"" "$OUT"; then
        echo "FAIL: forbidden tool $forbidden present in stdio output" >&2
        exit 1
    fi
done
echo "OK: no forbidden activation/policy tools surfaced"

prompt_count=$(jq -r 'select(.id == 3) | .result.prompts | length // 0' "$OUT")
if [ -z "$prompt_count" ] || [ "$prompt_count" -lt 1 ]; then
    echo "FAIL: prompts/list returned ${prompt_count:-0} (expected >=1)" >&2
    exit 1
fi
echo "OK: prompts/list -> $prompt_count prompts"

resource_count=$(jq -r 'select(.id == 4) | .result.resources | length // 0' "$OUT")
if [ -z "$resource_count" ] || [ "$resource_count" -lt 1 ]; then
    echo "FAIL: resources/list returned ${resource_count:-0} (expected >=1)" >&2
    exit 1
fi
echo "OK: resources/list -> $resource_count resources"
