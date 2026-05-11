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
CEIL_FAIL_OUT="$(mktemp "${TMPDIR:-/tmp}/doctor-strict-ceiling.XXXXXX")"

cleanup() {
    if [ "$cleanup_bin" -eq 1 ]; then
        rm -f "$BIN"
    fi
    rm -f "$OK_OUT" "$FAIL_OUT" "$CEIL_FAIL_OUT"
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
        MCP_DEFAULT_TENANT_ID="prod-fallback-disabled" \
        CLOCKIFY_API_KEY="dummy" \
        "$@"
}

doctor_env "$BIN" doctor --strict >"$OK_OUT"
grep -q "Strict posture" "$OK_OUT"
grep -q "OK" "$OK_OUT"

# Negative case 1 — broad-policy strict gate. CLOCKIFY_POLICY=standard
# under prod-postgres must trip the strict broad-policy finding
# (exit 3). MCP_TENANT_POLICY_CEILING is raised to "standard" so the
# ADR 0021 process<=ceiling config-load gate (added in PR #99 review)
# does not pre-empt the broad-policy gate this smoke is specifically
# exercising. The two gates are intentionally orthogonal: the ceiling
# gate catches process-vs-ceiling pair misconfiguration; the
# broad-policy gate catches "hosted profile + broader-than-
# time_tracking_safe policy" regardless of whether the ceiling is
# aligned.
set +e
doctor_env CLOCKIFY_POLICY=standard MCP_TENANT_POLICY_CEILING=standard "$BIN" doctor --strict >"$FAIL_OUT" 2>&1
code=$?
set -e

if [ "$code" -ne 3 ]; then
    echo "expected doctor --strict to exit 3 for strict findings, got $code"
    cat "$FAIL_OUT"
    exit 1
fi
grep -q "CLOCKIFY_POLICY" "$FAIL_OUT"

# Negative case 2 — ADR 0021 ceiling gate. CLOCKIFY_POLICY broader
# than MCP_TENANT_POLICY_CEILING must be rejected at config load
# (exit 2) before the strict-mode gate is even reached. This pins
# the FromEnv-level guardrail introduced in PR #99 review fix-forward.
set +e
doctor_env CLOCKIFY_POLICY=standard MCP_TENANT_POLICY_CEILING=time_tracking_safe "$BIN" doctor --strict >"$CEIL_FAIL_OUT" 2>&1
ceil_code=$?
set -e

if [ "$ceil_code" -ne 2 ]; then
    echo "expected doctor to exit 2 (Load error) when CLOCKIFY_POLICY exceeds MCP_TENANT_POLICY_CEILING, got $ceil_code"
    cat "$CEIL_FAIL_OUT"
    exit 1
fi
if ! grep -q "exceeds" "$CEIL_FAIL_OUT"; then
    echo "expected 'exceeds' in error output, got:"
    cat "$CEIL_FAIL_OUT"
    exit 1
fi

echo "OK: doctor --strict positive and negative smokes passed (broad-policy + ceiling gate)"
