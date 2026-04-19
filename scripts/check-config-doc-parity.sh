#!/usr/bin/env bash
#
# check-config-doc-parity.sh — PR-blocking gate
#
# Runs cmd/gen-config-docs and fails if the generated artefacts
# (cmd/clockify-mcp/help_generated.go and the CONFIG-TABLE block in
# README.md) have drifted from internal/config/AllSpecs().
#
# This is how we keep the registry as the single source of truth:
# editing spec.go without regenerating docs cannot merge.
#
# Usage:
#   bash scripts/check-config-doc-parity.sh
#
# Exit codes:
#   0 — generated artefacts match internal/config/AllSpecs()
#   1 — drift detected; developer must run `go run ./cmd/gen-config-docs
#       -mode=all` and commit the regenerated files.

set -euo pipefail

echo "== config-doc-parity =="

go run ./cmd/gen-config-docs -mode=all

if ! git diff --quiet -- README.md cmd/clockify-mcp/help_generated.go; then
  echo >&2 "[fail] config-doc-parity: generated docs are out of sync with internal/config/spec.go"
  echo >&2
  echo >&2 "       Fix:"
  echo >&2 "         go run ./cmd/gen-config-docs -mode=all"
  echo >&2 "         git add README.md cmd/clockify-mcp/help_generated.go"
  echo >&2
  echo >&2 "       Diff:"
  git --no-pager diff -- README.md cmd/clockify-mcp/help_generated.go | head -80 >&2 || true
  exit 1
fi

echo "config-doc-parity: OK"
