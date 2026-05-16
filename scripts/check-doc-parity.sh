#!/usr/bin/env bash
#
# check-doc-parity.sh
#
# Keeps current one-user operator docs aligned with the generated runtime
# catalog and config surface. Historical/source-evidence docs are intentionally
# excluded from stale-product-language checks so old decisions can remain
# searchable without becoming current setup guidance.

set -euo pipefail

CATALOG_FILE="docs/tool-catalog.json"
NPM_PACKAGE_JSON="npm/clockify-mcp-go/package.json"
OPT_OUT="deploy/.config-parity-opt-out.txt"

fail=0

warn() { echo "[warn] $*" >&2; }
err() { echo "[fail] $*" >&2; fail=1; }

current_doc_files=()
current_public_docs=()

# Keep this list intentionally small: it is the current one-user public setup
# and operator surface. Older launch, deploy, ADR, and source-evidence docs are
# allowed to preserve historical hosted-era terms without blocking this gate.
for file in \
  README.md \
  AGENTS.md \
  docs/agent-cookbook.md \
  docs/clients.md \
  docs/api-coverage.md \
  docs/live-tests.md \
  docs/goals/oneuser-tool-coverage.md \
  docs/launch-readiness-review-may-8.md; do
  [ -f "$file" ] && current_doc_files+=("$file")
done

for file in README.md AGENTS.md docs/agent-cookbook.md docs/clients.md docs/api-coverage.md; do
  [ -f "$file" ] && current_public_docs+=("$file")
done

known_tools=""
expected_tool_count=""

if [ -f "$CATALOG_FILE" ]; then
  if ! catalog_data=$(python3 - "$CATALOG_FILE" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as fh:
    catalog = json.load(fh)

tools = catalog.get("tools")
if not isinstance(tools, list):
    raise SystemExit("catalog must contain top-level tools array")

names = []
for row in tools:
    name = row.get("name")
    if not isinstance(name, str):
        raise SystemExit("catalog tool entry missing string name")
    names.append(name)

print(len(names))
for name in sorted(set(names)):
    print(name)
PY
  ); then
    err "unable to parse flat tool catalog from $CATALOG_FILE"
  else
    expected_tool_count=$(printf "%s\n" "$catalog_data" | sed -n '1p')
    known_tools=$(printf "%s\n" "$catalog_data" | sed '1d')
    if [ "$expected_tool_count" != "156" ]; then
      err "$CATALOG_FILE tool count drift: found $expected_tool_count tools, expected 156"
    fi
  fi
else
  warn "$CATALOG_FILE missing - run 'make gen-tool-catalog'"
fi

# ---------------------------------------------------------------------------
# 1. Env-var content check
# ---------------------------------------------------------------------------

known_vars=$(
  {
    grep -rhoE '"(MCP|CLOCKIFY)_[A-Z0-9_]+"' internal/ cmd/ tests/ 2>/dev/null | sed 's/"//g'
    grep -rhoE '\b(MCP|CLOCKIFY)_[A-Z0-9_]+' .github/ 2>/dev/null || true
  } | sort -u
)

opt_out_list=""
if [ -f "$OPT_OUT" ]; then
  opt_out_list=$(grep -v '^#' "$OPT_OUT" | grep -v '^$' | awk '{print $1}' || true)
fi

if [ "${#current_doc_files[@]}" -gt 0 ]; then
  referenced_vars=$(grep -hoE '\b(MCP|CLOCKIFY)_[A-Z0-9_]{2,}' "${current_doc_files[@]}" 2>/dev/null | sort -u || true)
else
  referenced_vars=""
fi

for var in $referenced_vars; do
  if printf "%s\n" "$known_vars" | grep -qx "$var"; then continue; fi
  if printf "%s\n" "$opt_out_list" | grep -qx "$var"; then continue; fi
  case "$var" in
    *_|MCP_BEARER_TOKEN_EXAMPLE|CLOCKIFY_API_KEY_EXAMPLE) continue ;;
  esac
  err "env var referenced in current docs but not defined in code/config or opt-out: $var"
done

# ---------------------------------------------------------------------------
# 2. Tool-name content check
# ---------------------------------------------------------------------------

if [ -n "$known_tools" ] && [ "${#current_doc_files[@]}" -gt 0 ]; then
  referenced_tools=$(grep -hoE '\bclockify_[a-z0-9_]{3,}' "${current_doc_files[@]}" 2>/dev/null | sort -u || true)
  for tool in $referenced_tools; do
    case "$tool" in
      *_|clockify_mcp*|clockify_api_key*) continue ;;
    esac
    if printf "%s\n" "$known_tools" | grep -qx "$tool"; then continue; fi
    err "tool referenced in current docs but not in $CATALOG_FILE: $tool"
  done
fi

# ---------------------------------------------------------------------------
# 3. Current public docs must describe the one-user product shape
# ---------------------------------------------------------------------------

if [ -n "$expected_tool_count" ]; then
  count_files=(README.md AGENTS.md CONTRIBUTING.md SECURITY.md SUPPORT.md docs/agent-cookbook.md docs/clients.md docs/api-coverage.md)
  for file in "${count_files[@]}"; do
    [ -f "$file" ] || continue
    while IFS= read -r line; do
      line_no=${line%%:*}
      text=${line#*:}
      while IFS= read -r n; do
        [ -n "$n" ] || continue
        if [ "$n" != "$expected_tool_count" ]; then
          err "public tool-count claim drift in $file:$line_no: found $n tools, expected $expected_tool_count from $CATALOG_FILE"
        fi
      done < <(grep -oE '\b[0-9]{2,3}[[:space:]]+tools\b' <<< "$text" | grep -oE '[0-9]{2,3}' || true)
    done < <(grep -nE '\b[0-9]{2,3}[[:space:]]+tools\b' "$file" 2>/dev/null || true)
  done
fi

stale_oneuser_terms=(
  "Tier 1"
  "Tier 2"
  "policy modes"
  "multi-tenant"
  "hosted launch"
)

for needle in "${stale_oneuser_terms[@]}"; do
  [ "${#current_public_docs[@]}" -gt 0 ] || continue
  hits=$(grep -rnFi "$needle" "${current_public_docs[@]}" 2>/dev/null || true)
  if [ -n "$hits" ]; then
    while IFS= read -r line; do
      err "stale hosted/multi-user language in current one-user docs: $line"
    done <<< "$hits"
  fi
done

required_agents_terms=(
  "One local trusted user."
  'One `CLOCKIFY_API_KEY`.'
  'One required `CLOCKIFY_WORKSPACE_ID`.'
  "Stdio transport only"
  "Exactly 156 tools loaded at startup"
)
if [ -f AGENTS.md ]; then
  for pattern in "${required_agents_terms[@]}"; do
    if ! grep -qF "$pattern" AGENTS.md; then
      err "AGENTS.md must preserve one-user product contract term: $pattern"
    fi
  done
fi

# ---------------------------------------------------------------------------
# 4. README npm compatibility must match wrapper package engines when present
# ---------------------------------------------------------------------------

if [ -f "$NPM_PACKAGE_JSON" ] && grep -qE '^\| Node\.js \(npm wrapper\) \|' README.md 2>/dev/null; then
  readme_node=$(grep -E '^\| Node\.js \(npm wrapper\) \|' README.md 2>/dev/null | sed -E 's/.*\| ([0-9]+)\+ \|/\1/' | tr -d '[:space:]' || true)
  package_node=$(grep -E '"node": *">=[0-9]+"' "$NPM_PACKAGE_JSON" | sed -E 's/.*">=([0-9]+)".*/\1/' | tr -d '[:space:]' || true)
  if [ -z "$package_node" ]; then
    err "$NPM_PACKAGE_JSON missing node engine declaration"
  fi
  if [ -n "$readme_node" ] && [ -n "$package_node" ] && [ "$readme_node" != "$package_node" ]; then
    err "README Node.js (npm wrapper) compatibility ($readme_node+) does not match $NPM_PACKAGE_JSON (>=${package_node})"
  fi
fi

# ---------------------------------------------------------------------------
# 5. Dangling marker check for current operator docs
# ---------------------------------------------------------------------------

if [ "${#current_doc_files[@]}" -gt 0 ]; then
  dangling=$(grep -rnE '\b(TODO|TBD|FIXME|XXX)\b' "${current_doc_files[@]}" 2>/dev/null \
    | grep -v "^docs/adr/.*superseded" \
    | grep -v "check-doc-parity" || true)
  if [ -n "$dangling" ]; then
    while IFS= read -r line; do
      err "dangling marker in current operator doc: $line"
    done <<< "$dangling"
  fi
fi

# ---------------------------------------------------------------------------
# 6. ADR index status parity
# ---------------------------------------------------------------------------

adr_index="docs/adr/README.md"
if [ -f "$adr_index" ]; then
  while IFS= read -r row; do
    adr_id=$(awk -F'|' '{gsub(/^[[:space:]]+|[[:space:]]+$/, "", $2); print $2}' <<< "$row")
    title=$(awk -F'|' '{gsub(/^[[:space:]]+|[[:space:]]+$/, "", $3); print $3}' <<< "$row")
    rel_path=$(sed -nE 's/.*\]\(([^)]+)\).*/\1/p' <<< "$row")
    [ -n "$adr_id" ] || continue
    [ -n "$rel_path" ] || continue
    adr_path="docs/adr/$rel_path"
    if [ ! -f "$adr_path" ]; then
      err "ADR index references missing file for $adr_id: $adr_path"
      continue
    fi
    status_line=$(awk '
      tolower($0) == "## status" {
        while (getline) {
          if ($0 != "") {
            print
            exit
          }
        }
      }
    ' "$adr_path")
    status_norm=$(printf '%s' "$status_line" | tr '[:upper:]' '[:lower:]' | sed -E 's/^[*[:space:]]+//;s/[*.[:space:]]+$//')
    status_kind=""
    case "$status_norm" in
      accepted*) status_kind="accepted" ;;
      proposed*) status_kind="proposed" ;;
      superseded*) status_kind="superseded" ;;
    esac
    if [ -z "$status_kind" ]; then
      err "ADR $adr_id has unrecognised Status line in $adr_path: $status_line"
      continue
    fi

    title_norm=$(printf '%s' "$title" | tr '[:upper:]' '[:lower:]')
    case "$status_kind" in
      accepted)
        if grep -Eq '\((proposed|superseded)\)' <<< "$title_norm"; then
          err "ADR index status drift for $adr_id: file is Accepted but index title says otherwise"
        fi
        ;;
      proposed)
        if ! grep -Eq '\(proposed\)' <<< "$title_norm"; then
          err "ADR index status drift for $adr_id: file is Proposed but index title lacks '(proposed)'"
        fi
        ;;
      superseded)
        # Historical ADRs may be plainly titled in the index; accepting the
        # file's Superseded status keeps old decisions searchable without
        # forcing title churn in archived context.
        :
        ;;
    esac
  done < <(grep -E '^\|[[:space:]]*[0-9]{4}[[:space:]]*\|' "$adr_index" || true)
fi

if [ "$fail" -ne 0 ]; then
  echo >&2
  echo "doc-parity: FAIL - fix the issues above before merging" >&2
  exit 1
fi

echo "doc-parity: OK"
