#!/usr/bin/env bash
#
# check-api-parity-matrix.sh
#
# Generates and checks docs/api-parity-matrix.md from docs/tool-catalog.json
# plus docs/goals/oneuser-tool-coverage.md. The matrix is docs-only:
# it does not prove live calls, but it prevents stale prose about exposed
# one-user tools, arguments, wire posture, and evidence notes.

set -euo pipefail

repo_root="$(pwd)"
write=0

usage() {
  cat <<'EOF'
Usage: scripts/check-api-parity-matrix.sh [--write] [--repo-root PATH]

Options:
  --write           Regenerate docs/api-parity-matrix.md.
  --repo-root PATH  Check another repo root, used by script tests.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --write)
      write=1
      shift
      ;;
    --repo-root)
      repo_root="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

catalog="$repo_root/docs/tool-catalog.json"
coverage="$repo_root/docs/goals/oneuser-tool-coverage.md"
matrix="$repo_root/docs/api-parity-matrix.md"

if ! command -v jq >/dev/null 2>&1; then
  echo "[fail] jq is required for api parity matrix checks" >&2
  exit 1
fi
if [ ! -f "$catalog" ]; then
  echo "[fail] missing $catalog" >&2
  exit 1
fi
if [ ! -f "$coverage" ]; then
  echo "[fail] missing $coverage" >&2
  exit 1
fi

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/api-parity-matrix.XXXXXX")"
trap 'rm -rf "$tmpdir"' EXIT

tool_count="$(jq '.tools | length' "$catalog")"
workflow_count="$(jq '[.tools[] | select(.category == "workflow")] | length' "$catalog")"
raw_count="$(jq '[.tools[] | select(.name == "clockify_api_get" or .name == "clockify_api_request")] | length' "$catalog")"
domain_count=$((tool_count - workflow_count - raw_count))

awk '
  /^\| `clockify_/ {
    n = split($0, fields, "|")
    if (n >= 11) {
      tool = fields[2]
      gsub(/^[[:space:]]*`|`[[:space:]]*$/, "", tool)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", tool)
      live = fields[7]
      happy = fields[8]
      schema = fields[9]
      status = fields[10]
      next_action = fields[11]
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", live)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", happy)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", schema)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", status)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", next_action)
      print tool "\t" live "\t" happy "\t" schema "\t" status "\t" next_action
    }
  }
' "$coverage" > "$tmpdir/coverage.tsv"

jq -r '
  def ticklist:
    if length == 0 then "none"
    else (map("`" + . + "`") | join(", "))
    end;
  def props: (.input_schema.properties // {});
  def required: (.input_schema.required // []);
  def optional: ((props | keys) - required);
  def source:
    if (.name == "clockify_api_get") then "raw fallback / caller-supplied GET"
    elif (.name == "clockify_api_request") then "raw fallback / caller-supplied method"
    elif ((.method // "") != "" or (.path // "") != "") then
      ((.method // "METHOD") + " " + (.path // "path from descriptor"))
    elif (.category == "workflow") then "workflow-native composite"
    else ((.handler_kind // "native handler") + " / code-selected endpoint")
    end;
  def wire:
    [
      (if .read_only then "read" else "write" end),
      (if .destructive then "destructive" else empty end),
      (if .dry_run then "dry_run_supported" else empty end),
      (if .idempotent then "idempotent" else "non_idempotent" end),
      (if ((.risk_class // []) | length) > 0 then "risk=" + ((.risk_class // []) | join("+")) else empty end)
    ] | join("; ");
  def output:
    (.output_schema.properties.data.description // "") as $desc
    | if ($desc | startswith("Tool-specific payload for ")) then "generic envelope"
      elif (.output_schema.properties.data? != null) then "typed data envelope"
      else "envelope only"
      end;
  .tools[]
  | [
      .name,
      (.category // if (.name | test("^clockify_api_")) then "raw" else "domain" end),
      source,
      (required | ticklist),
      (optional | ticklist),
      wire,
      output
    ]
  | @tsv
' "$catalog" > "$tmpdir/catalog.tsv"

awk -F '\t' '
  BEGIN {
    while ((getline < ARGV[1]) > 0) {
      live[$1] = $2
      happy[$1] = $3
      schema[$1] = $4
      status[$1] = $5
      action[$1] = $6
    }
    ARGV[1] = ""
  }
  {
    note = "live_protocol=" live[$1] "; live_happy_path=" happy[$1] "; ledger_schema=" schema[$1] "; status=" status[$1] "; next=" action[$1]
    gsub(/\|/, "\\|", note)
    for (i = 1; i <= NF; i++) gsub(/\|/, "\\|", $i)
    printf "| `%s` | %s | %s | %s | %s | %s | %s | %s |\n", $1, $2, $3, $4, $5, $6, $7, note
  }
' "$tmpdir/coverage.tsv" "$tmpdir/catalog.tsv" > "$tmpdir/rows.md"

{
  cat <<EOF
# API parity matrix

Generated from \`docs/tool-catalog.json\` and checked against
\`docs/goals/oneuser-tool-coverage.md\`. Do not edit table rows by hand;
run \`bash scripts/check-api-parity-matrix.sh --write\` after catalog or
coverage-ledger changes.

This is the exposed one-user MCP tool surface for the local owner runtime.
Native workflow and domain tools may choose endpoints in code; raw fallback
tools intentionally accept caller-supplied paths.

Summary:
- Total tools: $tool_count
- Workflow tools: $workflow_count
- Domain tools: $domain_count
- Raw fallback tools: $raw_count

| Tool | Class | Method / source / path | Required args | Optional args | Request / wire notes | Response / output schema posture | Paid feature / live evidence note |
|------|-------|------------------------|---------------|---------------|----------------------|----------------------------------|-----------------------------------|
EOF
  cat "$tmpdir/rows.md"
} > "$tmpdir/api-parity-matrix.md"

if [ "$write" = "1" ]; then
  cp "$tmpdir/api-parity-matrix.md" "$matrix"
  echo "wrote docs/api-parity-matrix.md ($tool_count tools)"
  exit 0
fi

if [ ! -f "$matrix" ]; then
  echo "[fail] docs/api-parity-matrix.md missing; run 'bash scripts/check-api-parity-matrix.sh --write'" >&2
  exit 1
fi

expected_rows="$(grep -c '^| `clockify_' "$tmpdir/api-parity-matrix.md" || true)"
actual_rows="$(grep -c '^| `clockify_' "$matrix" || true)"
if [ "$expected_rows" -ne "$tool_count" ] || [ "$actual_rows" -ne "$tool_count" ]; then
  echo "[fail] matrix row count drift: expected $tool_count, generated $expected_rows, found $actual_rows" >&2
  exit 1
fi

if ! diff -u "$matrix" "$tmpdir/api-parity-matrix.md" > "$tmpdir/diff"; then
  echo "[fail] api-parity-matrix drift; run 'bash scripts/check-api-parity-matrix.sh --write'" >&2
  sed -n '1,120p' "$tmpdir/diff" >&2
  exit 1
fi

echo "api-parity-matrix: OK ($tool_count tools)"
