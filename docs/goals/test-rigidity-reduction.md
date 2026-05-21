# P7 · Reduce test rigidity so non-behavioral changes don't cascade

**TL;DR.** The 129+ test files are excellent for catching real
regressions, but many pin exact envelopes, schemas, catalog snapshots,
and generated artifacts. A descriptor field rename or an annotation
addition can require touching dozens of files. Identify the fragile
tests, convert them to snapshot-style or descriptor-derived assertions,
and add a "fragility budget" for the future. Addresses critique item 8.

Estimated effort: **L** (>5 days; ship as a 3-PR series). **Depends on
P1** (the budget-test pattern is the model to copy).

## Problem

Sample symptoms (from earlier audit work):

- Schema edits ripple into multiple `*_test.go` files because tests
  hand-write the expected schema shape rather than reading the
  descriptor.
- Catalog edits force regeneration of `docs/tool-catalog.{md,json}`
  and edits to any test that asserts a specific tool-list shape.
- Coverage-ledger flips require lockstep edits in
  `docs/goals/oneuser-tool-coverage.md`, the
  `oneUserNamedLive*Evidence` map in
  `internal/tools/oneuser_quality_test.go`, plus regeneration of the
  API parity matrix and self-inspection assets.

This is correct rigorous testing where it pins truths. It is fragile
testing where it pins incidental shape.

## Goal

Identify the top-10 fragility hotspots, classify each as "pins truth"
(keep) or "pins incidental shape" (convert), and refactor the latter to
read from descriptors / snapshots so that non-behavioral changes (a
field reorder, a new optional annotation) do not cascade.

The metric is: **after the refactor, adding one new optional descriptor
annotation touches ≤ 3 test files, not ≤ 30.**

## Non-goals

- Removing any test.
- Loosening behavioral assertions. Anything that pins observable
  behavior stays.
- Replacing all hand-written assertions with snapshots. Snapshots are
  the right tool for *generated* shapes (catalogs, schemas), not for
  every assertion.
- Rewriting the lockstep ledger discipline. The coverage ledger is
  separate plumbing (`oneuser-tool-coverage.md` + evidence maps + drift
  gates) and it's fine as-is.

## Decided

- **Fragility classifier:** a test is "fragile incidental" if it would
  fail when the production behavior is unchanged but a descriptor field
  is reordered, an optional annotation is added, or a generated artifact
  is regenerated. A test is "truth-pinning" if it would only fail when
  observable behavior changed.
- **Conversion patterns** (apply per case; pick the cheapest):
  - **Descriptor-derived fixture:** the test reads the descriptor and
    asserts the property; the assertion stays even if the descriptor
    moves.
  - **Snapshot file:** generated artifact (catalog JSON, schema export)
    is compared against a committed snapshot. Drift gate already exists
    for catalogs — extend the pattern.
  - **Property test:** assert invariants ("all schemas have
    `additionalProperties:false`", "no descriptor exposes `outputSchema`
    unless `AdvertiseOutputSchema=true`") instead of per-tool literals.
- **PR shape:** three PRs.
  - PR-A: inventory + classification report (docs only, no code).
  - PR-B: convert top-5 hotspots.
  - PR-C: convert the next 5 + add a CI lint that flags new fragile
    patterns (`assert.Equal` against a literal schema map in a test
    file).

## Source locations to read first

| Read | Why |
|---|---|
| `internal/tools/tools_test.go` (2,025 lines) | Likely a fragility hotspot. |
| `internal/tools/oneuser_quality_test.go` (3,415 lines) | Owns the live coverage map; ledger-rigidity by design but worth scanning for incidental rigidity. |
| `internal/tools/time_management_test.go` (1,798) | Probable schema/envelope literals. |
| `internal/tools/invoices_test.go` (1,284) | Same. |
| `internal/tools/fullaccess_test.go:260` | `TestRegistryForToolsetFiltersOwnerSurfaces` — table-driven, may have literal counts. |
| `internal/mcp/tools_list_budget_test.go` | The model snapshot-style test from P1 (post-merge). The pattern to imitate. |
| `Makefile` drift gates: `catalog-drift`, `selfinspect-drift`, `openapi-drift`, `raw-allowlist-drift` | Existing snapshot infrastructure. Use, don't reinvent. |

## Implementation tasks

### Task 1 — Inventory and classify (PR-A)

This task ships a doc, no code.

1. Generate a fragility report by greppable heuristics:

   ```
   # tests that hand-write JSON schema shape
   grep -nE 'map\[string\]any\{' internal/tools/*_test.go | wc -l
   grep -nE '"additionalProperties":' internal/tools/*_test.go | wc -l
   grep -nE '"outputSchema":\s*map' internal/tools/*_test.go | wc -l

   # tests that assert specific tool counts (positional rigidity)
   grep -nE 'wantCount: *15[0-9]' internal/tools/*_test.go
   grep -nE 'len\(.*Registry.*\)' internal/tools/*_test.go

   # tests that compare to inline catalog snapshots
   grep -nE 'tool-catalog\.json\|tool-catalog\.md' internal/tools/*_test.go
   ```

   Capture counts and top files. Save to
   `docs/goals/test-rigidity-inventory.md` (new file, doc-only).

2. For each top-10 hotspot, write a one-paragraph classification:
   - File + line range.
   - What it asserts.
   - Why it would fail under a non-behavioral change.
   - Conversion plan (descriptor-derived | snapshot | property).
   - Effort estimate (S/M).

3. Produce a "Conversion order" list at the bottom: the 5 cheapest +
   highest-leverage to do first (PR-B), then the next 5 (PR-C).

**Verify:**

```
wc -l docs/goals/test-rigidity-inventory.md
git status --short
```

**Commit / PR-A:**

```
docs: test-rigidity inventory and conversion plan

10 fragility hotspots classified. Top-5 conversions queued for PR-B;
next-5 for PR-C plus a CI lint.
```

### Task 2 — Convert the top-5 (PR-B)

For each of the 5 conversions identified in PR-A, do one commit:

- Read the test.
- Apply the chosen conversion pattern.
- Verify the test still fails for real behavioral changes (run it
  against a deliberate behavior-changing diff, then revert).
- Verify the test no longer fails for an incidental shape change
  (e.g. reorder a descriptor field, run the test, expect pass).
- Commit.

A representative conversion looks like this:

**Before** (hand-written schema literal):
```go
want := map[string]any{
    "type": "object",
    "additionalProperties": false,
    "properties": map[string]any{
        "workspace_id": map[string]any{"type": "string"},
        // ...
    },
}
if diff := cmp.Diff(want, descriptor.Tool.InputSchema); diff != "" {
    t.Fatalf("schema mismatch: %s", diff)
}
```

**After** (descriptor-derived invariant):
```go
schema := descriptor.Tool.InputSchema
if schema["type"] != "object" {
    t.Fatalf("type = %v, want object", schema["type"])
}
if schema["additionalProperties"] != false {
    t.Fatalf("additionalProperties = %v, want false", schema["additionalProperties"])
}
props, _ := schema["properties"].(map[string]any)
if _, ok := props["workspace_id"]; !ok {
    t.Fatalf("missing required property workspace_id")
}
// Whatever else this test is actually about — those are the behavior
// pins. Drop the rest of the literal map; that was incidental shape.
```

**Per-conversion commit template:**

```
test(tools): convert <file>:<test-name> from literal schema to invariant

Hand-written schema map removed. Test now asserts the invariant that
matters (additionalProperties=false, required props present) without
pinning every key by hand. Re-verified that a real behavior change
still trips the test.
```

After all 5 conversions, run full validation:

```
go test -count=1 -race ./...
make perfect
```

If a non-fragility test now fails because the conversion was too
aggressive, narrow the conversion. The bar is "would this fail if
behavior changed?" — if yes, the assertion stays.

### Task 3 — Convert the next 5 + add CI lint (PR-C)

Same per-conversion approach as Task 2. Then add a lint.

**File (new):** `scripts/lint-test-fragility.sh`. Shell script that
greps for the heuristics from Task 1 and fails if their count exceeds
the post-PR-B+C baseline + a tolerance.

```bash
#!/usr/bin/env bash
set -euo pipefail

# Heuristic 1: hand-written schema literals in tests.
HW_SCHEMA=$(grep -rE '"additionalProperties":' internal/tools/*_test.go | wc -l | tr -d ' ')
# Heuristic 2: tools/list shape literals (catalog-style snapshots in tests).
TL_SHAPE=$(grep -rE '"outputSchema":\s*map' internal/tools/*_test.go | wc -l | tr -d ' ')
# Heuristic 3: hardcoded full-registry counts (positional rigidity).
HARD_COUNT=$(grep -rE 'len\([^)]*Registry[^)]*\)\s*==\s*15[0-9]' internal/tools/*_test.go | wc -l | tr -d ' ')

# Baselines: write after PR-C lands; adjust upward only with a
# documented reason in the PR description.
HW_SCHEMA_BUDGET=${HW_SCHEMA_BUDGET:-NN}
TL_SHAPE_BUDGET=${TL_SHAPE_BUDGET:-NN}
HARD_COUNT_BUDGET=${HARD_COUNT_BUDGET:-NN}

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
echo "fragility budgets ok"
```

Set the budgets to the *measured* values after PR-B and PR-C land.
Replace `NN` with the actual numbers. The lint doesn't require any
existing fragility to be removed — it just prevents new fragility from
accumulating.

Wire into a Makefile target:

```
.PHONY: lint-test-fragility
lint-test-fragility:
	bash scripts/lint-test-fragility.sh
```

And add to the existing CI lint stage (find the lint workflow under
`.github/workflows/`; align with the pattern of other repo-local
linters).

**Verify:**

```
bash scripts/lint-test-fragility.sh
```

Expect: `fragility budgets ok`.

**Commit:**

```
test: add fragility budget lint script

scripts/lint-test-fragility.sh reads three heuristic counts
(hand-written schema literals, tools/list shape literals, hardcoded
registry counts) and fails the build if they exceed the post-P7
baseline. Prevents future test-rigidity accumulation.
```

## Full validation

After each of the three PRs:

```
go test -count=1 -race ./...
make perfect
bash scripts/lint-test-fragility.sh    # after PR-C
```

Manual gut check: pick a non-behavioral change — e.g. add an optional
field to `mcp.Tool` with `json:",omitempty"`. Run tests. Count the
files that need touching. Before P7: probably 5–10. After P7: should be
0–2.

## Rollback

Each commit is independently revertable. Worst-case full rollback:
`git revert` PR-C's lint commit (no behavioral impact). The conversions
in PR-B and PR-C make tests more flexible; reverting them only
re-introduces rigidity, doesn't break behavior.

## PR description templates

### PR-A

```
## Summary

Inventory of test-rigidity hotspots and a conversion plan. Docs-only.

## Findings

Top-10 fragility hotspots listed in
`docs/goals/test-rigidity-inventory.md`. Each has a one-paragraph
classification and a conversion pattern (descriptor-derived | snapshot
| property).

## Next

PR-B converts the top-5; PR-C the next-5 plus a CI lint that prevents
future regressions.

## Test plan

- [ ] `make perfect`
- [ ] Inventory doc reviewed for completeness.
```

### PR-B

```
## Summary

Converts 5 fragile tests to descriptor-derived or invariant-based
assertions. Behavior pins unchanged.

## Files touched

- <list>

## Verification

For each conversion: ran the test against a real behavior change
(reverted) to confirm it still trips, and against an incidental shape
change (reverted) to confirm it doesn't.

## Test plan

- [ ] `go test -count=1 -race ./...`
- [ ] `make perfect`
```

### PR-C

```
## Summary

Converts 5 more fragile tests plus introduces a fragility budget lint
at `scripts/lint-test-fragility.sh` wired into `make`.

## Budgets (post-PR-C)

- hand-written schema literals: <N>
- tools/list shape literals: <N>
- hardcoded registry counts: <N>

Budgets are upper bounds; lowering is unconstrained.

## Test plan

- [ ] `go test -count=1 -race ./...`
- [ ] `make perfect`
- [ ] `bash scripts/lint-test-fragility.sh` returns ok.
```

## Anti-patterns

- **Do not** loosen behavior-pinning assertions while converting
  incidental ones. The whole point is to *keep* behavior pinned tight
  and unpin only shape.
- **Do not** convert tests in `oneuser_quality_test.go` that maintain
  the live-coverage ledger. That rigidity is a feature, not a bug.
- **Do not** introduce a snapshot for every property test. Snapshots
  are for generated artifacts; invariant property tests are for
  behavioral classes.
- **Do not** raise the fragility budget without a one-paragraph
  justification in the PR description. The lint is a ratchet, not a
  speed bump.
- **Do not** skip the verification step ("real change still trips it").
  That's the only protection against accidentally weakening a test
  during conversion.
