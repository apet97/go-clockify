#!/usr/bin/env bash
set -euo pipefail

count_matches() {
  local pattern="$1"
  (grep -hE "$pattern" internal/tools/*_test.go || true) | wc -l | tr -d '[:space:]'
}

# Heuristic 1: hand-written schema literals in tests.
HW_SCHEMA="$(count_matches '"additionalProperties":')"
# Heuristic 2: tools/list shape literals (catalog-style snapshots in tests).
TL_SHAPE="$(count_matches '"outputSchema":[[:space:]]*map')"
# Heuristic 3: hardcoded full-registry counts (positional rigidity).
HARD_COUNT="$(count_matches 'len\([^)]*Registry[^)]*\)[[:space:]]*==[[:space:]]*15[0-9]')"

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
  echo "fragility: hardcoded registry counts=$HARD_COUNT > budget=$HARD_COUNT_BUDGET"
  fail=1
fi

[ "$fail" -eq 0 ] || exit 1
echo "fragility budgets ok (hand-written schema literals=$HW_SCHEMA/$HW_SCHEMA_BUDGET, tools/list shape literals=$TL_SHAPE/$TL_SHAPE_BUDGET, hardcoded registry counts=$HARD_COUNT/$HARD_COUNT_BUDGET)"
