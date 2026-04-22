#!/usr/bin/env bash
# Verifies build-tag wiring for the clockify-mcp binary:
#   1. Default build has zero gated symbols (otel/pprof/grpc) and go.mod
#      has zero rows referencing the gated modules (ADR 009, ADR 012).
#   2. Each build tag (otel, grpc, pprof, pprof,otel, fips) builds cleanly,
#      the tagged binary actually links the gated symbols, and tagged tests
#      pass in the sub-packages that opt in.
#
# Environment variables:
#   SKIP_FIPS   If set, skip the FIPS section entirely. Useful for local
#               runs where the Go toolchain is not GOFIPS140-capable.
#   FIPS_ONLY   If set, run ONLY the FIPS section. Used by `make verify-fips`.
#
# CI calls this with no flags so everything runs.

set -euo pipefail

TMPROOT="${TMPDIR:-/tmp}"
TMP_DEFAULT="${TMPROOT%/}/clockify-mcp-default"
TMP_GRPC="${TMPROOT%/}/clockify-mcp-grpc"
TMP_PPROF="${TMPROOT%/}/clockify-mcp-pprof"
TMP_FIPS="${TMPROOT%/}/clockify-mcp-fips"
TMP_PG="${TMPROOT%/}/clockify-mcp-postgres"

cleanup() {
    rm -f "$TMP_DEFAULT" "$TMP_GRPC" "$TMP_PPROF" "$TMP_FIPS" "$TMP_PG"
}
trap cleanup EXIT

check_symbol_absent() {
    local binary="$1" pattern="$2" label="$3" count
    count=$(go tool nm "$binary" | grep -c "$pattern" || true)
    printf '%s symbol count: %s\n' "$label" "$count"
    if [ "$count" -ne 0 ]; then
        printf 'FAIL: default build leaked %s symbols\n' "$label" >&2
        return 1
    fi
}

check_symbol_present() {
    local binary="$1" pattern="$2" label="$3" count
    count=$(go tool nm "$binary" | grep -c "$pattern" || true)
    printf '%s symbol count: %s\n' "$label" "$count"
    if [ "$count" -eq 0 ]; then
        printf 'FAIL: tagged build did not link %s\n' "$label" >&2
        return 1
    fi
}

if [ -z "${FIPS_ONLY:-}" ]; then
    echo "== default build =="
    go build ./...
    go build -o "$TMP_DEFAULT" ./cmd/clockify-mcp

    check_symbol_absent "$TMP_DEFAULT" opentelemetry "opentelemetry"
    check_symbol_absent "$TMP_DEFAULT" 'net/http/pprof' "net/http/pprof"
    check_symbol_absent "$TMP_DEFAULT" 'google.golang.org/grpc' "google.golang.org/grpc"
    check_symbol_absent "$TMP_DEFAULT" 'jackc/pgx' "jackc/pgx"

    echo "== go.mod parity =="
    otel_rows=$(grep -c 'go.opentelemetry.io' go.mod || true)
    grpc_rows=$(grep -c 'google.golang.org/grpc' go.mod || true)
    pgx_rows=$(grep -c 'jackc/pgx' go.mod || true)
    printf 'go.opentelemetry.io rows: %s\n' "$otel_rows"
    printf 'google.golang.org/grpc rows: %s\n' "$grpc_rows"
    printf 'jackc/pgx rows: %s\n' "$pgx_rows"
    if [ "$otel_rows" -ne 0 ]; then
        echo "FAIL: go.mod leaked OpenTelemetry rows (ADR 009)" >&2
        exit 1
    fi
    if [ "$grpc_rows" -ne 0 ]; then
        echo "FAIL: go.mod leaked gRPC rows (ADR 012)" >&2
        exit 1
    fi
    if [ "$pgx_rows" -ne 0 ]; then
        echo "FAIL: go.mod leaked jackc/pgx rows (ADR 0001 / ADR 0011)" >&2
        exit 1
    fi

    echo "== -tags=otel =="
    go build -tags=otel ./...
    go test -tags=otel -count=1 ./internal/tracing/...
    # Exercise the OTel sub-module tests in addition to the facade.
    # The sub-module lives in its own go.mod so top-level `go test ./...`
    # does not descend into it; this is the only place that runs
    # `TestInstallEmitsSpanToOTLPEndpoint` and any other span-emission
    # regression gates.
    (cd internal/tracing/otel && go build ./... && go vet ./... && go test -count=1 ./...)

    echo "== -tags=grpc =="
    go build -tags=grpc ./...
    (cd internal/transport/grpc && go build ./... && go vet ./... && go test -count=1 ./...)
    go build -tags=grpc -o "$TMP_GRPC" ./cmd/clockify-mcp
    check_symbol_present "$TMP_GRPC" 'google.golang.org/grpc' "google.golang.org/grpc"

    echo "== -tags=pprof =="
    go build -tags=pprof ./...
    go build -tags=pprof -o "$TMP_PPROF" ./cmd/clockify-mcp
    check_symbol_present "$TMP_PPROF" 'net/http/pprof' "net/http/pprof"
    go test -tags=pprof -count=1 ./internal/mcp/...

    echo "== -tags=pprof,otel =="
    go build -tags=pprof,otel ./...

    echo "== -tags=grpc,otel =="
    # Combinatorial: gRPC transport plus OTel exporters. Verifies the
    # otel interceptor path compiles when both tags are active — this
    # matters because the OTel wiring is a thin adapter over a stdlib
    # fallback, and the adapter only shows up under -tags=otel.
    go build -tags=grpc,otel ./...

    echo "== -tags=postgres =="
    go build -tags=postgres ./...
    (cd internal/controlplane/postgres && go build -tags=postgres ./... && go vet -tags=postgres ./...)
    go build -tags=postgres -o "$TMP_PG" ./cmd/clockify-mcp
    check_symbol_present "$TMP_PG" 'jackc/pgx' "jackc/pgx"
fi

if [ -n "${SKIP_FIPS:-}" ]; then
    echo "== -tags=fips (skipped via SKIP_FIPS) =="
    exit 0
fi

echo "== -tags=fips (GOFIPS140=latest) =="
export GOFIPS140=latest
go build -tags=fips ./...
go build -tags=fips -o "$TMP_FIPS" ./cmd/clockify-mcp
output=$("$TMP_FIPS" --version 2>&1 || true)
echo "$output"
if ! echo "$output" | grep -q "fips140_enabled"; then
    echo "FAIL: -tags=fips binary did not log fips140_enabled on startup" >&2
    exit 1
fi
go test -tags=fips -count=1 -timeout 120s ./...

echo "== -tags=fips,grpc (GOFIPS140=latest) =="
# Combinatorial: FIPS + gRPC. Verifies that the gRPC transport
# builds cleanly under GOFIPS140 — a real risk path because the
# gRPC dependency chain transitively pulls in crypto choices
# that need the FIPS-capable primitives.
go build -tags=fips,grpc ./...
