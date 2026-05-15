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

catalog_contract_errors="$(
  jq -r '
    def err($msg): $msg;
    if (.tools | type) != "array" then
      err("catalog must use top-level tools array")
    else empty end,
    if has("tier1") or has("tier2") then
      err("catalog must not use legacy tier1/tier2 top-level shape")
    else empty end,
    if ((.tools // []) | length) == 0 then
      err("catalog tools array must not be empty")
    else empty end,
    ((.tools // []) | group_by(.name)[] | select(length > 1) | err("duplicate tool name: " + .[0].name)),
    ((.tools // [])[] | . as $tool |
      [
        (if (($tool.name // "") | test("^clockify_")) then empty else err(($tool.name // "<missing>") + ": name must start with clockify_") end),
        (if (($tool.description // "") | length) > 0 then empty else err(($tool.name // "<missing>") + ": description is required") end),
        (if ($tool.handler_kind // "") != "" then empty else err($tool.name + ": handler_kind is required") end),
        (if ($tool.read_only | type) == "boolean" then empty else err($tool.name + ": read_only boolean is required") end),
        (if ($tool.destructive | type) == "boolean" then empty else err($tool.name + ": destructive boolean is required") end),
        (if ($tool.idempotent | type) == "boolean" then empty else err($tool.name + ": idempotent boolean is required") end),
        (if ($tool.dry_run | type) == "boolean" then empty else err($tool.name + ": dry_run boolean is required") end),
        (if ($tool.risk_class | type) == "array" then empty else err($tool.name + ": risk_class array is required") end),
        (if ($tool.input_schema | type) == "object" then empty else err($tool.name + ": input_schema object is required") end),
        (if ($tool.output_schema | type) == "object" then empty else err($tool.name + ": output_schema object is required") end),
        (if (($tool.output_schema.required // []) | index("ok")) and (($tool.output_schema.required // []) | index("action")) then empty else err($tool.name + ": output_schema must require ok and action") end),
        (if ($tool.annotations | type) == "object" then empty else err($tool.name + ": annotations object is required") end),
        (if ($tool.annotations.readOnlyHint == $tool.read_only) then empty else err($tool.name + ": annotations.readOnlyHint must mirror read_only") end),
        (if ($tool.annotations.destructiveHint == $tool.destructive) then empty else err($tool.name + ": annotations.destructiveHint must mirror destructive") end),
        (if ($tool.annotations.idempotentHint == $tool.idempotent) then empty else err($tool.name + ": annotations.idempotentHint must mirror idempotent") end),
        (if ($tool.annotations.dryRun == $tool.dry_run) then empty else err($tool.name + ": annotations.dryRun must mirror dry_run") end),
        (if ($tool.annotations.handlerKind == $tool.handler_kind) then empty else err($tool.name + ": annotations.handlerKind must mirror handler_kind") end),
        (if (($tool.risk_class - ($tool.annotations.riskClass // [])) | length) == 0 then empty else err($tool.name + ": annotations.riskClass must include risk_class") end)
      ][])
  ' "$catalog"
)"
if [ -n "$catalog_contract_errors" ]; then
  echo "[fail] tool catalog contract drift:" >&2
  sed 's/^/  - /' <<<"$catalog_contract_errors" >&2
  exit 1
fi

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/api-parity-matrix.XXXXXX")"
trap 'rm -rf "$tmpdir"' EXIT

tool_count="$(jq '.tools | length' "$catalog")"
workflow_count="$(jq '[.tools[] | select(.category == "workflow")] | length' "$catalog")"
raw_count="$(jq '[.tools[] | select(.name == "clockify_api_get" or .name == "clockify_api_request")] | length' "$catalog")"
domain_count=$((tool_count - workflow_count - raw_count))

raw_fallback_errors="$(
  jq -r --argjson tool_count "$tool_count" '
    def err($msg): $msg;
    def idx($name):
      first(range(0; .tools | length) as $i | select(.tools[$i].name == $name) | $i);
    [
      (idx("clockify_api_get") // -1),
      (idx("clockify_api_request") // -1)
    ] as $raw_indexes
    | if ($raw_indexes | sort) != [($tool_count - 2), ($tool_count - 1)] then
        err("raw fallback tools must be the final two catalog entries")
      else empty end,
      (.tools[] | select(.name == "clockify_api_get") | . as $tool |
        if $tool.read_only != true or $tool.idempotent != true or $tool.annotations.openWorldHint != true or (($tool.input_schema.required // []) | index("path") | not) then
          err("clockify_api_get raw fallback safety metadata drift")
        else empty end),
      (.tools[] | select(.name == "clockify_api_request") | . as $tool |
        if $tool.read_only != false or $tool.idempotent != false or $tool.annotations.openWorldHint != true or (($tool.input_schema.required // []) | sort) != ["method", "path"] then
          err("clockify_api_request raw fallback safety metadata drift")
        else empty end)
  ' "$catalog"
)"
if [ -n "$raw_fallback_errors" ]; then
  echo "[fail] raw fallback catalog drift:" >&2
  sed 's/^/  - /' <<<"$raw_fallback_errors" >&2
  exit 1
fi

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
