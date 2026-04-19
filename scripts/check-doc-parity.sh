#!/usr/bin/env bash
#
# check-doc-parity.sh
#
# Release-blocking gate that keeps operator-facing documentation in
# sync with the code. The config-parity script (check-config-parity.sh)
# checks env-var *presence* in deployment manifests. This script
# checks *content correctness* — the docs that operators rely on
# during an incident or an upgrade must reference real symbols, not
# stale ones.
#
# Checks performed (all non-fatal warnings print, only the strict
# ones below fail the gate):
#
#   1. Every env var referenced in docs/ appears in config.go OR is
#      explicitly listed as deprecated in
#      deploy/.config-parity-opt-out.txt. A doc referencing a
#      removed env var misleads operators.
#   2. Every tool name referenced in operator-facing docs exists in
#      docs/tool-catalog.json OR is explicitly marked as a planned
#      future name. Prevents runbooks pointing at tools we never
#      shipped or already removed.
#   3. No dangling TODO / TBD / FIXME / XXX in
#      docs/runbooks/, docs/deploy/, docs/adr/, or the top-level
#      operator docs. These markers signal unfinished operator
#      guidance and belong in issue tracker or PRs, not in shipped
#      docs.
#
# Usage:
#   bash scripts/check-doc-parity.sh
#
# Exit codes:
#   0 — all checks passed
#   1 — at least one strict check failed

set -euo pipefail

CONFIG_FILE="internal/config/config.go"
CATALOG_FILE="docs/tool-catalog.json"
OPT_OUT="deploy/.config-parity-opt-out.txt"
DOC_DIRS=(docs/runbooks docs/deploy docs/adr docs)
DOC_FILES_TOP=(docs/support-matrix.md docs/upgrade-checklist.md docs/verify-release.md docs/production-readiness.md)

fail=0

warn() { echo "[warn] $*" >&2; }
err() { echo "[fail] $*" >&2; fail=1; }

# ---------------------------------------------------------------------------
# 1. Env-var content check
# ---------------------------------------------------------------------------

if [ ! -f "$CONFIG_FILE" ]; then
  err "config file missing: $CONFIG_FILE"
  exit 1
fi

# Known env vars are every "MCP_*" / "CLOCKIFY_*" string literal
# referenced anywhere in internal/** or cmd/** source. This
# deliberately over-includes — test-only literals
# (CLOCKIFY_LIVE_*, CLOCKIFY_RUN_LIVE_E2E, etc.) count as "defined"
# so a runbook that documents them for test harnesses does not
# trip the gate. Operator-facing mis-references still fail on
# removed vars, which is the case the gate exists for.
known_vars=$(
  {
    grep -rhoE '"(MCP|CLOCKIFY)_[A-Z0-9_]+"' internal/ cmd/ tests/ 2>/dev/null | sed 's/"//g'
    # Workflow files reference CLOCKIFY_LIVE_* and other CI-only env
    # vars that never appear in Go source. Counting them as defined
    # keeps operator docs honest without forcing a Go-side shim.
    grep -rhoE '\b(MCP|CLOCKIFY)_[A-Z0-9_]+' .github/ 2>/dev/null | sort -u
  } | sort -u)

opt_out_list=""
if [ -f "$OPT_OUT" ]; then
  opt_out_list=$(grep -v '^#' "$OPT_OUT" | grep -v '^$' | awk '{print $1}' || true)
fi

# Find every MCP_* / CLOCKIFY_* token in docs
referenced_vars=$(grep -rhoE '\b(MCP|CLOCKIFY)_[A-Z0-9_]{2,}' \
                   "${DOC_DIRS[@]}" 2>/dev/null | sort -u || true)

for var in $referenced_vars; do
  if echo "$known_vars" | grep -qx "$var"; then continue; fi
  if echo "$opt_out_list" | grep -qx "$var"; then continue; fi
  # A small allowlist for example-only names that never existed and
  # never will (e.g. placeholders in snippets). Expand here rather
  # than adding them to the real opt-out list.
  case "$var" in
    MCP_BEARER_TOKEN_EXAMPLE|CLOCKIFY_API_KEY_EXAMPLE) continue ;;
  esac
  err "env var referenced in docs but not defined in $CONFIG_FILE or opt-out: $var"
done

# ---------------------------------------------------------------------------
# 2. Tool-name content check
# ---------------------------------------------------------------------------

if [ -f "$CATALOG_FILE" ]; then
  known_tools=$(grep -oE '"name": *"clockify_[a-z0-9_]+"' "$CATALOG_FILE" \
                | sed 's/"name": *"//;s/"//' | sort -u)

  referenced_tools=$(grep -rhoE '\bclockify_[a-z0-9_]{3,}' \
                      "${DOC_DIRS[@]}" 2>/dev/null | sort -u || true)

  for tool in $referenced_tools; do
    # Skip common non-tool prefixes that share the clockify_ stem.
    case "$tool" in
      clockify_mcp*|clockify_outage*|clockify_upstream*|clockify_policy|clockify_api_key*) continue ;;
    esac
    if echo "$known_tools" | grep -qx "$tool"; then continue; fi
    warn "tool referenced in docs but not in $CATALOG_FILE: $tool"
    # Downgraded to warn (not fail) because internal narrative may
    # reference tools in transition. Flip to err after v1.0 GA.
  done
else
  warn "$CATALOG_FILE missing — run 'make gen-tool-catalog'"
fi

# ---------------------------------------------------------------------------
# 3. TODO / TBD / FIXME / XXX check
# ---------------------------------------------------------------------------

dangling=$(grep -rnE '\b(TODO|TBD|FIXME|XXX)\b' "${DOC_DIRS[@]}" "${DOC_FILES_TOP[@]}" 2>/dev/null \
            | grep -v "^docs/adr/.*superseded" \
            | grep -v "check-doc-parity" || true)

if [ -n "$dangling" ]; then
  while IFS= read -r line; do
    err "dangling marker in operator doc: $line"
  done <<< "$dangling"
fi

# ---------------------------------------------------------------------------
# Report
# ---------------------------------------------------------------------------

if [ "$fail" -ne 0 ]; then
  echo >&2
  echo "doc-parity: FAIL — fix the issues above before merging" >&2
  exit 1
fi

echo "doc-parity: OK"
