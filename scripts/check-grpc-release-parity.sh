#!/usr/bin/env bash
#
# check-grpc-release-parity.sh
#
# Drift gate that keeps the private-network gRPC profile honest:
#   1. The profile doc must not claim tenant defaults to X-Tenant-ID
#      (the actual default is MCP_MTLS_TENANT_SOURCE=cert).
#   2. The profile doc must not claim Docker images include gRPC
#      unless either:
#        - .goreleaser.yaml ships clockify-mcp-grpc / -grpc-postgres
#          binaries (first-class artifact path), OR
#        - the Dockerfile / docker-image workflow accepts a GO_TAGS
#          build arg (self-build path).
#   3. If docs anywhere reference clockify-mcp-grpc-postgres, then
#      .goreleaser.yaml must contain the matching build id, and
#      scripts/check-release-assets.sh must declare the GRPC_POSTGRES
#      platforms array. Same gate for clockify-mcp-grpc.
#
# Wired into Makefile verify-core via the grpc-release-parity target,
# and into release-check, so a doc that mentions a gRPC artifact the
# release pipeline does not produce fails before tag-time.
#
# Usage:
#   bash scripts/check-grpc-release-parity.sh
#
# Exit codes:
#   0 — all gates passed
#   1 — at least one gate failed
#   2 — input files missing (script wiring bug)

set -euo pipefail

DOC="docs/deploy/profile-private-network-grpc.md"
GORELEASER=".goreleaser.yaml"
ASSETS_SCRIPT="scripts/check-release-assets.sh"
DOCKERFILE="deploy/Dockerfile"
DOCKER_WORKFLOW=".github/workflows/docker-image.yml"

fail=0

err() { echo "[fail] $*" >&2; fail=1; }
warn() { echo "[warn] $*" >&2; }

for f in "$DOC" "$GORELEASER" "$ASSETS_SCRIPT" "$DOCKERFILE"; do
    if [ ! -f "$f" ]; then
        echo "[bug] expected file missing: $f" >&2
        exit 2
    fi
done

# ---------------------------------------------------------------------------
# 1. Tenant default claim
# ---------------------------------------------------------------------------
# The wrong-default phrasing varies; the bare "defaults to X-Tenant-ID"
# substring catches the historical wording without snagging legitimate
# uses of the env var name in surrounding context (e.g. "X-Tenant-ID is
# ignored unless ...").
if grep -niE 'defaults? to .?X-Tenant-ID' "$DOC" >/dev/null; then
    err "$DOC claims tenant extraction defaults to X-Tenant-ID; the actual default is MCP_MTLS_TENANT_SOURCE=cert"
fi

# ---------------------------------------------------------------------------
# 2. Docker-image-includes-gRPC claim must be backed by a real path
# ---------------------------------------------------------------------------
have_grpc_artifact=0
if grep -qE 'id: clockify-mcp-grpc(-postgres)?$' "$GORELEASER"; then
    have_grpc_artifact=1
fi
have_dockerfile_tags=0
if grep -qE '^\s*ARG\s+GO_TAGS' "$DOCKERFILE" || \
   ( [ -f "$DOCKER_WORKFLOW" ] && grep -qE 'GO_TAGS=' "$DOCKER_WORKFLOW" ); then
    have_dockerfile_tags=1
fi

if grep -niE 'Docker.*image.*include[s]?.*gRPC|image builds include it' "$DOC" >/dev/null; then
    if [ "$have_grpc_artifact" -eq 0 ] && [ "$have_dockerfile_tags" -eq 0 ]; then
        err "$DOC claims Docker images include gRPC, but neither GoReleaser ships a -grpc artifact nor does the Dockerfile expose a GO_TAGS build arg"
    fi
fi

# ---------------------------------------------------------------------------
# 3. Doc references must match GoReleaser + asset-count script
# ---------------------------------------------------------------------------
# Search every shipped doc, not just the profile doc — the launch
# checklist and the support matrix both mention gRPC artifact names.
if grep -RIlqs 'clockify-mcp-grpc-postgres' docs deploy .github 2>/dev/null; then
    if ! grep -q 'id: clockify-mcp-grpc-postgres' "$GORELEASER"; then
        err "docs reference clockify-mcp-grpc-postgres but $GORELEASER does not declare a clockify-mcp-grpc-postgres build"
    fi
    if ! grep -q 'GRPC_POSTGRES_PLATFORMS' "$ASSETS_SCRIPT"; then
        err "docs reference clockify-mcp-grpc-postgres but $ASSETS_SCRIPT does not enumerate the GRPC_POSTGRES_PLATFORMS array"
    fi
fi
# clockify-mcp-grpc (not -grpc-postgres) gate. The grep negation
# excludes the longer variant so we do not double-count the same
# string.
if grep -RIls 'clockify-mcp-grpc[^-]' docs deploy .github 2>/dev/null \
        | grep -v -- '--' \
        | xargs -I{} grep -lE 'clockify-mcp-grpc-(linux|darwin|windows)' {} 2>/dev/null \
        | grep -q .; then
    if ! grep -qE '^\s*-\s*id:\s*clockify-mcp-grpc$' "$GORELEASER"; then
        err "docs reference clockify-mcp-grpc-* binaries but $GORELEASER does not declare a clockify-mcp-grpc build id"
    fi
    if ! grep -qE 'GRPC_PLATFORMS' "$ASSETS_SCRIPT"; then
        err "docs reference clockify-mcp-grpc-* binaries but $ASSETS_SCRIPT does not enumerate the GRPC_PLATFORMS array"
    fi
fi

# ---------------------------------------------------------------------------
# Report
# ---------------------------------------------------------------------------
if [ "$fail" -ne 0 ]; then
    echo >&2
    echo "grpc-release-parity: FAIL — fix the issues above before merging" >&2
    exit 1
fi

echo "grpc-release-parity: OK"
