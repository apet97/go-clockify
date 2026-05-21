#!/usr/bin/env bash
set -euo pipefail

count_matches() {
  local pattern="$1"
  (grep -hE "$pattern" internal/tools/*_test.go || true) | wc -l | tr -d '[:space:]'
}

nonempty_line_count() {
  sed '/^$/d' | wc -l | tr -d '[:space:]'
}

# Heuristic 1: hand-written schema literals in tests.
HW_SCHEMA="$(count_matches '"additionalProperties":')"
# Heuristic 2: tools/list shape literals (catalog-style snapshots in tests).
TL_SHAPE="$(count_matches '"outputSchema":[[:space:]]*map')"
# Heuristic 3: hardcoded full-registry count pins (positional rigidity).
# Search tracked tests broadly, then allow only deliberate contract/budget
# tests. The summary reports allowed pins too; otherwise "0/0" creates false
# confidence while product-contract tests still intentionally pin 156.
FULL_REGISTRY_COUNT="$(awk '/productContractFullRegistryTools[[:space:]]*=/ { print $3; exit }' internal/tools/product_contract_test.go)"
FULL_REGISTRY_COUNT="${FULL_REGISTRY_COUNT:-156}"
TEST_FILES="$(git ls-files 'internal/**/*_test.go' 'cmd/**/*_test.go' 'tests/**/*_test.go')"
HARD_COUNT_HITS="$(
  # grep exits 1 when no lines match; keep the script alive under set -e.
  # The second grep keeps this heuristic focused on registry/tools-list pins,
  # not every incidental integer in every test fixture.
  printf '%s\n' "$TEST_FILES" \
    | xargs grep -nE "\\b${FULL_REGISTRY_COUNT}\\b" 2>/dev/null \
    | grep -E 'FullAccessRegistry|RegistryForToolset|tools/list|registry|Registry loaded|Advertised surface|tools, want' \
    || true
)"
HARD_COUNT_ALLOWLIST='^(internal/tools/product_contract_test\.go|internal/tools/fullaccess_test\.go|internal/tools/oneuser_quality_test\.go|internal/tools/runtime_envelope_schema_test\.go|internal/tools/tools_list_budget_test\.go|internal/mcp/server_bench_test\.go|cmd/clockify-mcp/main_test\.go):'
UNALLOWLISTED_HARD_COUNT_HITS="$(printf '%s\n' "$HARD_COUNT_HITS" | grep -Ev "$HARD_COUNT_ALLOWLIST" || true)"
HARD_COUNT="$(printf '%s\n' "$UNALLOWLISTED_HARD_COUNT_HITS" | nonempty_line_count)"
ALLOWED_HARD_COUNT="$(printf '%s\n' "$HARD_COUNT_HITS" | grep -E "$HARD_COUNT_ALLOWLIST" | nonempty_line_count || true)"

# Baselines are the measured post-P7 PR-C values. Raise only with a documented
# reason in the PR that introduces new intentional rigidity.
HW_SCHEMA_BUDGET="${HW_SCHEMA_BUDGET:-3}"
TL_SHAPE_BUDGET="${TL_SHAPE_BUDGET:-0}"
HARD_COUNT_BUDGET="${HARD_COUNT_BUDGET:-0}"

fail=0
if [ "$HW_SCHEMA" -gt "$HW_SCHEMA_BUDGET" ]; then
  echo "fragility: hand-written schema literals=$HW_SCHEMA > budget=$HW_SCHEMA_BUDGET"
  fail=1
fi
if [ "$TL_SHAPE" -gt "$TL_SHAPE_BUDGET" ]; then
  echo "fragility: tools/list shape literals=$TL_SHAPE > budget=$TL_SHAPE_BUDGET"
  fail=1
fi
if [ "$HARD_COUNT" -gt "$HARD_COUNT_BUDGET" ]; then
  echo "fragility: unallowlisted full-registry count pins=$HARD_COUNT > budget=$HARD_COUNT_BUDGET"
  printf '%s\n' "$UNALLOWLISTED_HARD_COUNT_HITS"
  fail=1
fi

[ "$fail" -eq 0 ] || exit 1
echo "fragility budgets ok (hand-written schema literals=$HW_SCHEMA/$HW_SCHEMA_BUDGET, tools/list shape literals=$TL_SHAPE/$TL_SHAPE_BUDGET, unallowlisted full-registry count pins=$HARD_COUNT/$HARD_COUNT_BUDGET, allowed full-registry count pins=$ALLOWED_HARD_COUNT)"
