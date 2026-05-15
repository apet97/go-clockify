#!/usr/bin/env bash
#
# check-live-tool-coverage.sh
#
# Static guard for the live-test coverage story in docs/api-coverage.md.
# This does not prove live tests ran or passed. It fails if the generated
# catalog gains a workflow/domain tool name that is not at least named by the
# livee2e source bundle, so manual/scheduled live evidence cannot silently
# drift away from the startup catalog.

set -euo pipefail

repo_root="$(pwd)"
plan=0

usage() {
  cat <<'EOF'
Usage: scripts/check-live-tool-coverage.sh [--plan] [--repo-root PATH]

Checks that:
  - every workflow/domain catalog tool is referenced by livee2e source files;
  - raw fallback tools are not required as typed workflow/domain coverage;
  - livee2e files do not mention unknown clockify_* tool names.

This is a static coverage guard only. Scheduled live-contract evidence still
comes from .github/workflows/live-contract.yml cron runs.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --plan)
      plan=1
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

if [ "$plan" = "1" ]; then
  cat <<'EOF'
Live tool coverage plan

- Source of truth: docs/tool-catalog.json.
- Live-test source scan: tests/e2e_live*.go plus the Postgres
  live_audit_phases_test.go contract.
- Gate: all workflow/domain tools must be named in livee2e source.
- Raw fallback tools are checked by catalog-order and api-parity gates; they
  may appear in contract tests but are not required as typed live coverage.
- Gate: livee2e source must not carry unknown clockify_* tool names.

This does not replace scheduled cron evidence. It only guards the static
coverage inventory against catalog/test drift.
EOF
  exit 0
fi

catalog="$repo_root/docs/tool-catalog.json"
if [ ! -f "$catalog" ]; then
  echo "[fail] docs/tool-catalog.json missing" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "[fail] jq is required for live tool coverage checks" >&2
  exit 1
fi
if ! command -v rg >/dev/null 2>&1; then
  echo "[fail] rg is required for live tool coverage checks" >&2
  exit 1
fi

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/check-live-tool-coverage.XXXXXX")"
trap 'rm -rf "$tmpdir"' EXIT

catalog_contract_errors="$(
  jq -r '
    if (.tools | type) != "array" then
      "catalog must use top-level tools array"
    else empty end,
    if has("tier1") or has("tier2") then
      "catalog must not use legacy tier1/tier2 top-level shape"
    else empty end,
    ((.tools // [])[] | select((.name // "") == "") | "catalog tool name is required"),
    ((.tools // [])[] | select((.category // "") | test("^(workflow|domain|raw)$") | not) | (.name // "<missing>") + ": category must be workflow, domain, or raw")
  ' "$catalog"
)"
if [ -n "$catalog_contract_errors" ]; then
  echo "[fail] tool catalog contract drift:" >&2
  sed 's/^/       /' <<<"$catalog_contract_errors" >&2
  exit 1
fi

jq -r '.tools[] | select(.category == "workflow" or .category == "domain") | .name' "$catalog" | sort -u > "$tmpdir/live-required.txt"
jq -r '.tools[] | select(.category == "raw") | .name' "$catalog" | sort -u > "$tmpdir/raw.txt"
jq -r '.tools[] | .name' "$catalog" | sort -u > "$tmpdir/all.txt"

live_files=()
while IFS= read -r file; do
  live_files+=("$file")
done < <(find "$repo_root/tests" -maxdepth 1 -name 'e2e_live*.go' -type f 2>/dev/null | sort)
if [ -f "$repo_root/internal/controlplane/postgres/live_audit_phases_test.go" ]; then
  live_files+=("$repo_root/internal/controlplane/postgres/live_audit_phases_test.go")
fi

if [ "${#live_files[@]}" -eq 0 ]; then
  echo "[fail] no livee2e source files found" >&2
  exit 1
fi

rg -o --no-filename 'clockify_[a-z0-9_]+' "${live_files[@]}" | sort -u > "$tmpdir/live-refs.txt" || true
if [ -f "$repo_root/internal/tools/oneuser_quality_test.go" ]; then
  awk '
    /^func oneUserNamedLiveEvidence\(\)/ { in_func = 1 }
    in_func { print }
    in_func && /^}/ { exit }
  ' "$repo_root/internal/tools/oneuser_quality_test.go" \
    | rg -o --no-filename 'clockify_[a-z0-9_]+' >> "$tmpdir/live-refs.txt" || true
fi
sort -u "$tmpdir/live-refs.txt" -o "$tmpdir/live-refs.txt"

comm -23 "$tmpdir/live-required.txt" "$tmpdir/live-refs.txt" > "$tmpdir/missing-live-required.txt"
comm -13 "$tmpdir/all.txt" "$tmpdir/live-refs.txt" > "$tmpdir/unknown-live-refs.txt"

open=0
unknown=0

required_count="$(wc -l < "$tmpdir/live-required.txt" | tr -d ' ')"
raw_count="$(wc -l < "$tmpdir/raw.txt" | tr -d ' ')"
catalog_count="$(wc -l < "$tmpdir/all.txt" | tr -d ' ')"

if [ -s "$tmpdir/missing-live-required.txt" ]; then
  open=$((open + 1))
  echo "[open] workflow/domain catalog tools missing livee2e source references"
  sed 's/^/       tool: /' "$tmpdir/missing-live-required.txt"
  echo "       action: add a livee2e probe, documented unsupported/permission probe, or explicit coverage rationale before claiming full-catalog live coverage."
else
  echo "[closed] all ${required_count} workflow/domain catalog tools are named in livee2e source"
fi

echo "[closed] raw fallback tools are not required as typed live coverage (${raw_count} raw fallback tools)"

if [ -s "$tmpdir/unknown-live-refs.txt" ]; then
  open=$((open + 1))
  echo "[open] livee2e source mentions unknown clockify_* tool names"
  sed 's/^/       tool: /' "$tmpdir/unknown-live-refs.txt"
  echo "       action: update the test reference or regenerate the catalog so removed/renamed tools do not masquerade as live coverage."
else
  echo "[closed] no unknown clockify_* live-test references"
fi

echo
echo "Summary: ${open} open, ${unknown} unknown"
echo "Catalog: ${catalog_count} tools (${required_count} workflow/domain live-required, ${raw_count} raw fallback)"

if [ "$open" -ne 0 ] || [ "$unknown" -ne 0 ]; then
  exit 1
fi
