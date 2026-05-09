#!/usr/bin/env bash
#
# check-live-tool-coverage.sh
#
# Static guard for the live-test coverage story in docs/api-coverage.md.
# This does not prove live tests ran or passed. It fails if the generated
# catalog gains a tool name that is not at least named by the livee2e source
# bundle, so manual/scheduled live evidence cannot silently drift away from
# the 128-tool catalog.

set -euo pipefail

repo_root="$(pwd)"
plan=0

usage() {
  cat <<'EOF'
Usage: scripts/check-live-tool-coverage.sh [--plan] [--repo-root PATH]

Checks that:
  - every Tier-2 catalog tool is referenced by livee2e source files;
  - every API-backed Tier-1 catalog tool is referenced by livee2e source files;
  - Tier-1 local tool-surface helpers are explicitly allowed instead of
    pretending to make live Clockify calls;
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
- Gate: all Tier-2 tools must be named in livee2e source.
- Gate: all API-backed Tier-1 tools must be named in livee2e source.
- Allowed Tier-1 local helpers: activate_group, activate_tool,
  deactivate_group, list_tools. These mutate only the MCP tool surface or
  query the local catalog, so unit/contract tests are the right evidence.
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

jq -r '.tier1[]?.name' "$catalog" | sort -u > "$tmpdir/tier1.txt"
jq -r '.tier2[]?.name' "$catalog" | sort -u > "$tmpdir/tier2.txt"
jq -r '(.tier1[]?, .tier2[]?) | .name' "$catalog" | sort -u > "$tmpdir/all.txt"

cat > "$tmpdir/tier1-local-only.txt" <<'EOF'
clockify_activate_group
clockify_activate_tool
clockify_deactivate_group
clockify_list_tools
EOF
sort -u "$tmpdir/tier1-local-only.txt" -o "$tmpdir/tier1-local-only.txt"

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

cat "$tmpdir/live-refs.txt" "$tmpdir/tier1-local-only.txt" | sort -u > "$tmpdir/live-or-local.txt"
comm -23 "$tmpdir/tier2.txt" "$tmpdir/live-refs.txt" > "$tmpdir/missing-tier2.txt"
comm -23 "$tmpdir/tier1.txt" "$tmpdir/live-or-local.txt" > "$tmpdir/missing-tier1.txt"
comm -13 "$tmpdir/all.txt" "$tmpdir/live-refs.txt" > "$tmpdir/unknown-live-refs.txt"

open=0
unknown=0

tier2_count="$(wc -l < "$tmpdir/tier2.txt" | tr -d ' ')"
tier1_count="$(wc -l < "$tmpdir/tier1.txt" | tr -d ' ')"
local_count="$(wc -l < "$tmpdir/tier1-local-only.txt" | tr -d ' ')"

if [ -s "$tmpdir/missing-tier2.txt" ]; then
  open=$((open + 1))
  echo "[open] Tier-2 catalog tools missing livee2e source references"
  sed 's/^/       tool: /' "$tmpdir/missing-tier2.txt"
  echo "       action: add a livee2e probe, documented unsupported/permission probe, or explicit coverage rationale before claiming full-catalog live coverage."
else
  echo "[closed] all ${tier2_count} Tier-2 catalog tools are named in livee2e source"
fi

if [ -s "$tmpdir/missing-tier1.txt" ]; then
  open=$((open + 1))
  echo "[open] API-backed Tier-1 catalog tools missing livee2e source references"
  sed 's/^/       tool: /' "$tmpdir/missing-tier1.txt"
  echo "       action: add a read-only or sacrificial live probe, or move the tool into the local-only allowlist with a documented rationale."
else
  echo "[closed] all API-backed Tier-1 catalog tools are named in livee2e source (${local_count} local-only helpers allowed)"
fi

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
echo "Catalog: ${tier1_count} Tier-1 tools, ${tier2_count} Tier-2 tools"

if [ "$open" -ne 0 ] || [ "$unknown" -ne 0 ]; then
  exit 1
fi
