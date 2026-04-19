#!/usr/bin/env bash
# Builds clockify-mcp and smoke-tests the HTTP transport:
#   /health must return 200 (server liveness).
#   /ready  must return 200 or 503 (503 is expected when the dummy API
#           key cannot reach Clockify; the test only verifies reachability).
#
# Environment variables:
#   SMOKE_HTTP_PORT  Port to bind (default: 8091)

set -euo pipefail

BIN="${TMPDIR:-/tmp}/clockify-mcp-smoke"
PORT="${SMOKE_HTTP_PORT:-8091}"
PID=""

cleanup() {
    if [ -n "$PID" ]; then
        kill "$PID" 2>/dev/null || true
    fi
    rm -f "$BIN"
}
trap cleanup EXIT

go build -o "$BIN" ./cmd/clockify-mcp

MCP_TRANSPORT=http \
MCP_HTTP_BIND="127.0.0.1:$PORT" \
MCP_BEARER_TOKEN=smoke-test-token-1234567890 \
MCP_METRICS_AUTH_MODE=none \
CLOCKIFY_API_KEY=smoke-test-dummy \
"$BIN" &
PID=$!

sleep 2

if ! curl -sf "http://127.0.0.1:$PORT/health" >/dev/null; then
    echo "FAIL: /health not reachable or non-200" >&2
    exit 1
fi
echo "OK: /health returned 200"

code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/ready")
case "$code" in
    200|503)
        echo "OK: /ready returned $code"
        ;;
    *)
        echo "FAIL: unexpected /ready status: $code" >&2
        exit 1
        ;;
esac
