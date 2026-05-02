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
#   1. Every env var referenced in docs/ or README.md appears in
#      config.go OR is
#      explicitly listed as deprecated in
#      deploy/.config-parity-opt-out.txt. A doc referencing a
#      removed env var misleads operators.
#   2. Every tool name referenced in operator-facing docs or README.md
#      exists in docs/tool-catalog.json. ADR narrative is excluded so
#      hypothetical rename examples do not trigger false positives.
#      Prevents runbooks pointing at tools we never shipped or already
#      removed.
#   3. Public-surface banned strings do not appear in shipped
#      docs/README.
#   4. README npm compatibility matches the published npm package's
#      engine requirement.
#   5. No dangling TODO / TBD / FIXME / XXX in
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
DOC_FILES_TOP=(README.md docs/support-matrix.md docs/upgrade-checklist.md docs/verify-release.md docs/production-readiness.md docs/clients.md)
NPM_PACKAGE_JSON="npm/clockify-mcp-go/package.json"

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

# Find every MCP_* / CLOCKIFY_* token in docs/README.
# `docs/superpowers/` is excluded — that subtree holds AI-assisted design
# specs which by definition describe future state (env vars to be added in
# upcoming PRs). They are planning artifacts, not operator documentation,
# and including them would force every brainstorm-spec commit to land
# alongside its corresponding implementation, defeating the point of writing
# the spec first.
referenced_vars=$(grep -rhoE --exclude-dir=superpowers '\b(MCP|CLOCKIFY)_[A-Z0-9_]{2,}' \
                   "${DOC_DIRS[@]}" 2>/dev/null | sort -u || true)
referenced_vars="$referenced_vars"$'\n'"$(grep -hoE '\b(MCP|CLOCKIFY)_[A-Z0-9_]{2,}' "${DOC_FILES_TOP[@]}" 2>/dev/null | sort -u || true)"
referenced_vars=$(printf "%s\n" "$referenced_vars" | sort -u | sed '/^$/d')

for var in $referenced_vars; do
  if echo "$known_vars" | grep -qx "$var"; then continue; fi
  if echo "$opt_out_list" | grep -qx "$var"; then continue; fi
  # Trailing underscore = grep stopped at a non-word boundary inside
  # a wider env-var name (e.g. an ASCII-art diagram broke a long var
  # over two lines, or doc prose used `MCP_FOO_*` as a glob). Skip;
  # the full name will appear elsewhere in the same doc and be
  # validated through that match.
  case "$var" in
    *_) continue ;;
  esac
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

  strict_tool_files=(README.md)
  while IFS= read -r -d '' file; do
    strict_tool_files+=("$file")
  done < <(find docs -type f -name '*.md' ! -path 'docs/adr/*' ! -path 'docs/superpowers/*' -print0)

  referenced_tools=$(
    grep -hoE '\bclockify_[a-z0-9_]{3,}' "${strict_tool_files[@]}" 2>/dev/null \
      | sort -u || true
  )

  for tool in $referenced_tools; do
    # Trailing underscore = the regex matched a glob-prefix
    # reference like `clockify_list_*` or `clockify_get_*` in
    # narrative prose; the asterisk is not [a-z0-9_] so the
    # capture truncates at the underscore. These are wildcards,
    # not real tool names — skip without faulting.
    case "$tool" in
      *_) continue ;;
    esac
    # Skip common non-tool prefixes / snippets that share the clockify_ stem.
    case "$tool" in
      clockify_mcp*|clockify_outage*|clockify_upstream*|clockify_policy|clockify_api_key*|clockify_admin) continue ;;
    esac
    if echo "$known_tools" | grep -qx "$tool"; then continue; fi
    err "tool referenced in operator docs but not in $CATALOG_FILE: $tool"
  done
else
  warn "$CATALOG_FILE missing — run 'make gen-tool-catalog'"
fi

# ---------------------------------------------------------------------------
# 3. Banned stale public-surface strings
# ---------------------------------------------------------------------------

banned_strings=(
  "clockify_activate_group"
  "clockify_activate_tool"
  "@anycli/clockify-mcp-go"
  "defaulting to a preview if the \`dry_run\` parameter is omitted"
  "Destructive tools run through a dry-run interceptor by default"
  "through the dry-run interceptor unless the caller opts into"
)

for needle in "${banned_strings[@]}"; do
  hits=$(grep -rnF "$needle" "${DOC_DIRS[@]}" "${DOC_FILES_TOP[@]}" 2>/dev/null || true)
  if [ -n "$hits" ]; then
    while IFS= read -r line; do
      err "banned stale public-surface string found: $line"
    done <<< "$hits"
  fi
done

# ---------------------------------------------------------------------------
# 4. README npm compatibility must match published package engines
# ---------------------------------------------------------------------------

if [ ! -f "$NPM_PACKAGE_JSON" ]; then
  err "npm package file missing: $NPM_PACKAGE_JSON"
else
  # `|| true` is load-bearing: when the row / engine line is missing,
  # grep returns 1, and under `set -euo pipefail` the substitution
  # would otherwise abort silently before the dedicated `[ -z … ]`
  # err lines below could fire — leaving CI to fail closed with no
  # diagnostic.
  readme_node=$(grep -E '^\| Node\.js \(npm wrapper\) \|' README.md | sed -E 's/.*\| ([0-9]+)\+ \|/\1/' | tr -d '[:space:]' || true)
  package_node=$(grep -E '"node": *">=[0-9]+"' "$NPM_PACKAGE_JSON" | sed -E 's/.*">=([0-9]+)".*/\1/' | tr -d '[:space:]' || true)
  if [ -z "$readme_node" ]; then
    err "README.md missing Node.js (npm wrapper) compatibility row"
  fi
  if [ -z "$package_node" ]; then
    err "$NPM_PACKAGE_JSON missing node engine declaration"
  fi
  if [ -n "$readme_node" ] && [ -n "$package_node" ] && [ "$readme_node" != "$package_node" ]; then
    err "README Node.js (npm wrapper) compatibility ($readme_node+) does not match $NPM_PACKAGE_JSON (>=${package_node})"
  fi
fi

# ---------------------------------------------------------------------------
# 5. TODO / TBD / FIXME / XXX check
# ---------------------------------------------------------------------------

dangling=$(grep -rnE --exclude-dir=superpowers '\b(TODO|TBD|FIXME|XXX)\b' "${DOC_DIRS[@]}" "${DOC_FILES_TOP[@]}" 2>/dev/null \
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
