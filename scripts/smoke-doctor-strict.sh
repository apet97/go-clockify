#!/usr/bin/env bash
# Builds clockify-mcp and smoke-tests the hosted strict doctor gate.
# The positive and negative cases use synthetic env only; this remains
# an offline config/posture check and does not require Postgres.

set -euo pipefail

if [ -n "${BIN:-}" ]; then
    cleanup_bin=0
else
    BIN="$(mktemp "${TMPDIR:-/tmp}/clockify-mcp-doctor-strict.XXXXXX")"
    cleanup_bin=1
fi
OK_OUT="$(mktemp "${TMPDIR:-/tmp}/doctor-strict-ok.XXXXXX")"
FAIL_OUT="$(mktemp "${TMPDIR:-/tmp}/doctor-strict-fail.XXXXXX")"

cleanup() {
    if [ "$cleanup_bin" -eq 1 ]; then
        rm -f "$BIN"
    fi
    rm -f "$OK_OUT" "$FAIL_OUT"
}
trap cleanup EXIT

go build -o "$BIN" ./cmd/clockify-mcp

doctor_env() {
    env -i \
        PATH="${PATH:-/usr/bin:/bin}" \
        HOME="${HOME:-/tmp}" \
        MCP_PROFILE=prod-postgres \
        MCP_CONTROL_PLANE_DSN="postgres://user:pass@localhost:5432/clockify?sslmode=disable" \
        MCP_OIDC_ISSUER="https://issuer.example.com" \
        MCP_OIDC_AUDIENCE="clockify-mcp" \
        MCP_TENANT_CLAIM="tenant_id" \
        CLOCKIFY_API_KEY="dummy" \
        "$@"
}

doctor_env "$BIN" doctor --strict >"$OK_OUT"
grep -q "Strict posture" "$OK_OUT"
grep -q "OK" "$OK_OUT"

set +e
doctor_env CLOCKIFY_POLICY=standard "$BIN" doctor --strict >"$FAIL_OUT" 2>&1
code=$?
set -e

if [ "$code" -ne 3 ]; then
    echo "expected doctor --strict to exit 3 for strict findings, got $code"
    cat "$FAIL_OUT"
    exit 1
fi

grep -q "CLOCKIFY_POLICY" "$FAIL_OUT"
echo "OK: doctor --strict positive and negative smokes passed"
