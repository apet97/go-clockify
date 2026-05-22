#!/usr/bin/env bash
# Builds clockify-mcp and smoke-tests the stdio loop:
#   - pipes initialize + tools/list + prompts/list + resources/list,
#   - closes stdin so Run() flushes and exits,
#   - asserts:
#       initialize  -> serverInfo.name == clockify-go-mcp,
#       default tools/list -> exactly 16 advertised tools,
#       all tools/list     -> cursor-paginated exactly 156 advertised tools,
#                             first=clockify_status, last=clockify_api_request,
#       no old activation/policy helper tools in any response,
#       prompts/list  -> >=1,
#       resources/list -> >=1.
#   - source MCP surface audit against tools/list:
#       every tool has a non-empty description,
#       every input property has a non-empty description,
#       every tool carries annotations.riskClass,
#       destructiveHint:true implies riskClass contains "destructive",
#       known destructive tools carry destructiveHint:true on the full surface.
#
# Product note: tools/list reports the advertised surface only. The default
# advertised surface is 16 tools, while CLOCKIFY_TOOLSET=all advertises the
# full 156-tool startup registry. In scoped toolsets, the advertised surface is
# also the callable boundary; this smoke checks the advertised stdio surface.
#
# The audit runs against the same freshly-built temp binary in both toolset
# modes, so a stale or mis-built binary that drops safety metadata fails before
# merge.
#
# Requires: jq. Installed by default on ubuntu-latest runners and via
# Homebrew on macOS.

set -euo pipefail

EXPECTED_DEFAULT_ADVERTISED_TOOLS=16
EXPECTED_ALL_ADVERTISED_TOOLS=156
BIN="${TMPDIR:-/tmp}/clockify-mcp-stdio-smoke"
OUT="$(mktemp "${TMPDIR:-/tmp}/clockify-mcp-stdio-smoke.out.XXXXXX")"
ERR="$(mktemp "${TMPDIR:-/tmp}/clockify-mcp-stdio-smoke.err.XXXXXX")"
TOOLS_OUT="$(mktemp "${TMPDIR:-/tmp}/clockify-mcp-stdio-smoke.tools.XXXXXX")"
CURRENT_LABEL=""

cleanup() {
    rm -f "$BIN" "$OUT" "$ERR" "$TOOLS_OUT"
}
trap cleanup EXIT

# audit_fail reports a source MCP surface-audit failure with the captured
# stdout for context and exits non-zero.
audit_fail() {
    echo "FAIL: ${CURRENT_LABEL:+[$CURRENT_LABEL] }$1" >&2
    echo "--- stdout ---" >&2
    cat "$OUT" >&2
    exit 1
}

if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: jq is required but not installed" >&2
    exit 2
fi

go build -o "$BIN" ./cmd/clockify-mcp

DEFAULT_REQUESTS=$'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"smoke","version":"0"}}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n{"jsonrpc":"2.0","id":3,"method":"prompts/list","params":{}}\n{"jsonrpc":"2.0","id":4,"method":"resources/list","params":{}}\n'
ALL_REQUESTS=$'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"smoke","version":"0"}}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n{"jsonrpc":"2.0","id":5,"method":"tools/list","params":{"cursor":"80"}}\n{"jsonrpc":"2.0","id":3,"method":"prompts/list","params":{}}\n{"jsonrpc":"2.0","id":4,"method":"resources/list","params":{}}\n'

run_stdio_surface() {
    local label="$1"
    local toolset="$2"
    local expected_tools="$3"
    local expect_raw_last="$4"
    local requests="$DEFAULT_REQUESTS"
    local expected_lines=4
    if [ "$expect_raw_last" = "yes" ]; then
        requests="$ALL_REQUESTS"
        expected_lines=5
    fi
    CURRENT_LABEL="$label"
    : >"$OUT"
    : >"$ERR"
    : >"$TOOLS_OUT"

    set +e
    if [ -z "$toolset" ]; then
        printf '%s' "$requests" \
            | env -u CLOCKIFY_TOOLSET \
              CLOCKIFY_API_KEY="${CLOCKIFY_API_KEY:-smoke-test-dummy}" \
              CLOCKIFY_WORKSPACE_ID="${CLOCKIFY_WORKSPACE_ID:-00000000000000000000abcd}" \
              "$BIN" \
              >"$OUT" 2>"$ERR"
    else
        printf '%s' "$requests" \
            | CLOCKIFY_TOOLSET="$toolset" \
              CLOCKIFY_API_KEY="${CLOCKIFY_API_KEY:-smoke-test-dummy}" \
              CLOCKIFY_WORKSPACE_ID="${CLOCKIFY_WORKSPACE_ID:-00000000000000000000abcd}" \
              "$BIN" \
              >"$OUT" 2>"$ERR"
    fi
    rc=$?
    set -e

    if [ "$rc" -ne 0 ]; then
        echo "FAIL: [$label] clockify-mcp exited with status $rc" >&2
        echo "--- stderr ---" >&2
        cat "$ERR" >&2
        echo "--- stdout ---" >&2
        cat "$OUT" >&2
        exit 1
    fi

    line_count=$(grep -c . "$OUT" || true)
    if [ "$line_count" -lt "$expected_lines" ]; then
        echo "FAIL: [$label] expected >=$expected_lines JSON-RPC responses on stdout, got $line_count" >&2
        echo "--- stdout ---" >&2
        cat "$OUT" >&2
        echo "--- stderr ---" >&2
        cat "$ERR" >&2
        exit 1
    fi

    init_name=$(jq -r 'select(.id == 1) | .result.serverInfo.name // empty' "$OUT")
    if [ "$init_name" != "clockify-go-mcp" ]; then
        echo "FAIL: [$label] initialize response missing result.serverInfo.name=clockify-go-mcp" >&2
        cat "$OUT" >&2
        exit 1
    fi
    echo "OK: [$label] initialize -> serverInfo.name=$init_name"

    if [ "$expect_raw_last" = "yes" ]; then
        first_next=$(jq -r 'select(.id == 2) | .result.nextCursor // empty' "$OUT")
        second_next=$(jq -r 'select(.id == 5) | .result.nextCursor // empty' "$OUT")
        if [ "$first_next" != "80" ]; then
            echo "FAIL: [$label] first tools/list page nextCursor=${first_next:-empty} (expected 80)" >&2
            exit 1
        fi
        if [ -n "$second_next" ]; then
            echo "FAIL: [$label] second tools/list page nextCursor=$second_next (expected empty)" >&2
            exit 1
        fi
        jq -s '[.[] | select(.id == 2 or .id == 5) | .result.tools[]]' "$OUT" >"$TOOLS_OUT"
    else
        jq 'select(.id == 2) | .result.tools' "$OUT" >"$TOOLS_OUT"
    fi

    tool_count=$(jq -r 'length' "$TOOLS_OUT")
    first_tool=$(jq -r '.[0].name' "$TOOLS_OUT")
    last_tool=$(jq -r '.[-1].name' "$TOOLS_OUT")
    if [ -z "$tool_count" ] || [ "$tool_count" -ne "$expected_tools" ]; then
        echo "FAIL: [$label] tools/list returned ${tool_count:-?} tools (expected exactly $expected_tools advertised tools)" >&2
        exit 1
    fi
    if [ "$first_tool" != "clockify_status" ]; then
        echo "FAIL: [$label] tools/list[0] = $first_tool (expected clockify_status)" >&2
        exit 1
    fi
    if [ "$expect_raw_last" = "yes" ] && [ "$last_tool" != "clockify_api_request" ]; then
        echo "FAIL: [$label] tools/list[-1] = $last_tool (expected clockify_api_request)" >&2
        exit 1
    fi
    echo "OK: [$label] tools/list -> exactly $tool_count advertised tools across page(s) (first=$first_tool, last=$last_tool)"

    for forbidden in clockify_activate_group clockify_activate_tool clockify_deactivate_group clockify_search_tools clockify_list_tools clockify_policy_info; do
        if grep -q "\"$forbidden\"" "$OUT"; then
            echo "FAIL: [$label] forbidden tool $forbidden present in stdio output" >&2
            exit 1
        fi
    done
    echo "OK: [$label] no old activation/policy helper tools surfaced"

    echo "Source MCP surface audit [$label]:"

    missing_desc=$(jq -r '[.[] | select((.description // "") == "") | .name] | join(", ")' "$TOOLS_OUT")
    [ -z "$missing_desc" ] || audit_fail "tools/list has tool(s) with an empty description: $missing_desc"

    missing_prop_desc=$(jq -r '[.[] as $t | ($t.inputSchema.properties // {}) | to_entries[] | select((.value.description // "") == "") | "\($t.name).\(.key)"] | join(", ")' "$TOOLS_OUT")
    [ -z "$missing_prop_desc" ] || audit_fail "tools/list has input propert(ies) with no description: $missing_prop_desc"

    missing_risk=$(jq -r '[.[] | select((.annotations.riskClass // [] | length) == 0) | .name] | join(", ")' "$TOOLS_OUT")
    [ -z "$missing_risk" ] || audit_fail "tools/list has tool(s) with no annotations.riskClass: $missing_risk"

    bad_destructive=$(jq -r '[.[] | select(.annotations.destructiveHint == true) | select((.annotations.riskClass // []) | index("destructive") | not) | .name] | join(", ")' "$TOOLS_OUT")
    [ -z "$bad_destructive" ] || audit_fail "destructiveHint:true tool(s) missing \"destructive\" in riskClass: $bad_destructive"

    echo "  OK: every advertised tool and input property has a description; every advertised tool has annotations.riskClass"

    if [ "$expect_raw_last" = "yes" ]; then
        for tool in clockify_projects_archive clockify_time_off_archive clockify_users_invite clockify_scheduling_publish; do
            is_destructive=$(jq -r --arg t "$tool" '.[] | select(.name == $t) | .annotations.destructiveHint' "$TOOLS_OUT")
            if [ "$is_destructive" != "true" ]; then
                audit_fail "$tool must carry annotations.destructiveHint=true, got \"${is_destructive:-missing}\""
            fi
        done
        echo "  OK: known destructive tools carry destructiveHint=true"
    fi

    prompt_count=$(jq -r 'select(.id == 3) | .result.prompts | length // 0' "$OUT")
    if [ -z "$prompt_count" ] || [ "$prompt_count" -lt 1 ]; then
        echo "FAIL: [$label] prompts/list returned ${prompt_count:-0} (expected >=1)" >&2
        exit 1
    fi
    echo "OK: [$label] prompts/list -> $prompt_count prompts"

    resource_count=$(jq -r 'select(.id == 4) | .result.resources | length // 0' "$OUT")
    if [ -z "$resource_count" ] || [ "$resource_count" -lt 1 ]; then
        echo "FAIL: [$label] resources/list returned ${resource_count:-0} (expected >=1)" >&2
        exit 1
    fi
    echo "OK: [$label] resources/list -> $resource_count resources"
}

run_stdio_surface "default advertised surface" "" "$EXPECTED_DEFAULT_ADVERTISED_TOOLS" "no"
run_stdio_surface "all advertised surface" "all" "$EXPECTED_ALL_ADVERTISED_TOOLS" "yes"
