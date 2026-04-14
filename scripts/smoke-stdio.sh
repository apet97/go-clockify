#!/usr/bin/env bash
# Builds clockify-mcp and smoke-tests the stdio transport:
#   - launches the server in stdio mode,
#   - pipes an `initialize` then `tools/list` JSON-RPC request to stdin,
#   - closes stdin (EOF signals Run() to exit the scanner loop),
#   - reads stdout, asserts:
#       line 1 is a valid initialize response with
#         result.serverInfo.name == "clockify-go-mcp",
#       line 2 is a valid tools/list response with at least one tool.
#
# Stdio is the default transport per the MCP spec and the framing layer
# in internal/mcp/server.go is newline-delimited JSON. It had been
# unexercised in CI (only http, streamable_http, and grpc-under-tag were
# smoked) until this script was added — a regression in the stdio framing
# layer would have required a user bug report to surface.
#
# Env vars consumed:
#   CLOCKIFY_API_KEY   Dummy value is fine; stdio mode never talks to
#                      the real Clockify API during the smoke.
#   MCP_TRANSPORT      Forced to `stdio` by the script.
#
# Requires: jq (for JSON parsing). Installed by default on
# ubuntu-latest runners and available via Homebrew on macOS.

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

# Two newline-delimited JSON-RPC requests. Closing stdin after the
# second request triggers the EOF path in Run() (internal/mcp/server.go
# line ~354), which flushes pending responses and exits the loop.
REQUESTS=$'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n'

set +e
printf '%s' "$REQUESTS" \
    | MCP_TRANSPORT=stdio \
      CLOCKIFY_API_KEY=smoke-test-dummy \
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

# Expect at least two non-empty JSON lines on stdout.
line_count=$(grep -c . "$OUT" || true)
if [ "$line_count" -lt 2 ]; then
    echo "FAIL: expected >=2 JSON-RPC responses on stdout, got $line_count" >&2
    echo "--- stdout ---" >&2
    cat "$OUT" >&2
    echo "--- stderr ---" >&2
    cat "$ERR" >&2
    exit 1
fi

# Pair responses by id (order isn't contractually guaranteed, but jq
# lets us pick by id cheaply).
init_name=$(jq -r 'select(.id == 1) | .result.serverInfo.name // empty' "$OUT")
if [ "$init_name" != "clockify-go-mcp" ]; then
    echo "FAIL: initialize response missing result.serverInfo.name=clockify-go-mcp" >&2
    echo "--- stdout ---" >&2
    cat "$OUT" >&2
    exit 1
fi
echo "OK: initialize returned serverInfo.name=$init_name"

tool_count=$(jq -r 'select(.id == 2) | .result.tools | length' "$OUT")
if [ -z "$tool_count" ] || [ "$tool_count" -lt 1 ]; then
    echo "FAIL: tools/list returned ${tool_count:-?} tools (expected >=1)" >&2
    echo "--- stdout ---" >&2
    cat "$OUT" >&2
    exit 1
fi
echo "OK: tools/list returned $tool_count tools"
