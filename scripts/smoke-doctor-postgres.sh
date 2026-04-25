#!/usr/bin/env bash
#
# smoke-doctor-postgres.sh — proves the Postgres backend doctor path.
#
# Builds clockify-mcp with -tags=postgres and runs
# `doctor --strict --check-backends` against a live Postgres reachable
# at MCP_CONTROL_PLANE_DSN. Exit 0 means:
#   - Load() succeeds with the prod-postgres profile applied,
#   - strict-mode posture findings are clean,
#   - the embedded migrations (incl. 002_audit_phase) apply,
#   - the audit health round-trip in DoctorCheck(ctx) succeeds.
#
# CI uses this against an ephemeral postgres:16-alpine service. Local
# runs need a real Postgres reachable at the DSN; the script does NOT
# start one for you (Docker-in-script is too easy to get wrong; let CI
# orchestrate the service).
#
# Required env (no defaults, fail loudly so callers cannot point this at
# the wrong database by accident):
#   MCP_CONTROL_PLANE_DSN  postgres:// or postgresql:// DSN
#
# Optional env (sane defaults applied):
#   MCP_OIDC_ISSUER, MCP_OIDC_AUDIENCE, MCP_TENANT_CLAIM, CLOCKIFY_API_KEY

set -euo pipefail

if [ -z "${MCP_CONTROL_PLANE_DSN:-}" ]; then
    echo "smoke-doctor-postgres: MCP_CONTROL_PLANE_DSN is required" >&2
    exit 2
fi

case "$MCP_CONTROL_PLANE_DSN" in
    postgres://*|postgresql://*) ;;
    *)
        echo "smoke-doctor-postgres: MCP_CONTROL_PLANE_DSN must use postgres:// or postgresql:// scheme" >&2
        exit 2
        ;;
esac

if [ -n "${BIN:-}" ]; then
    cleanup_bin=0
else
    BIN="$(mktemp "${TMPDIR:-/tmp}/clockify-mcp-postgres.XXXXXX")"
    cleanup_bin=1
fi
OUT="$(mktemp "${TMPDIR:-/tmp}/doctor-postgres-out.XXXXXX")"

cleanup() {
    if [ "$cleanup_bin" -eq 1 ]; then
        rm -f "$BIN"
    fi
    rm -f "$OUT"
}
trap cleanup EXIT

go build -tags=postgres -o "$BIN" ./cmd/clockify-mcp

# env -i so the host shell does not leak unexpected MCP_* into Load().
# This mirrors smoke-doctor-strict.sh and makes CI/local identical.
set +e
env -i \
    PATH="${PATH:-/usr/bin:/bin}" \
    HOME="${HOME:-/tmp}" \
    MCP_PROFILE=prod-postgres \
    MCP_CONTROL_PLANE_DSN="$MCP_CONTROL_PLANE_DSN" \
    MCP_OIDC_ISSUER="${MCP_OIDC_ISSUER:-https://issuer.example.com}" \
    MCP_OIDC_AUDIENCE="${MCP_OIDC_AUDIENCE:-clockify-mcp}" \
    MCP_TENANT_CLAIM="${MCP_TENANT_CLAIM:-tenant_id}" \
    CLOCKIFY_API_KEY="${CLOCKIFY_API_KEY:-dummy}" \
    "$BIN" doctor --strict --check-backends >"$OUT" 2>&1
code=$?
set -e

if [ "$code" -ne 0 ]; then
    echo "smoke-doctor-postgres: doctor --strict --check-backends exit=$code (expected 0)" >&2
    cat "$OUT" >&2
    exit "$code"
fi

grep -q "Strict posture" "$OUT" || {
    echo "smoke-doctor-postgres: missing 'Strict posture' line in output" >&2
    cat "$OUT" >&2
    exit 1
}
grep -q "OK" "$OUT" || {
    echo "smoke-doctor-postgres: missing OK marker in output" >&2
    cat "$OUT" >&2
    exit 1
}
echo "OK: doctor --strict --check-backends green against Postgres"
