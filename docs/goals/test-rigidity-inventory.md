# Test-rigidity inventory and conversion plan

Generated for P7 Task 1 from `docs/goals/test-rigidity-reduction.md`.
This is an inventory only: no test or production code was changed.

## Scope

- Repo state inspected at local `main` commit `93460e2`
  (`docs(readme): link CONTEXT.md from current docs list`).
- `docs/goals/api-reconciliation/` is unrelated and intentionally excluded.
- Fragility classifier from the P7 plan:
  - **Truth-pinning:** fails only when an observable product contract,
    handler contract, wire contract, generated artifact, or evidence ledger
    changes. Keep unless a later PR intentionally moves the contract.
  - **Fragile incidental:** can fail when behavior is unchanged but a
    descriptor annotation, generated shape, fixture ordering, or broad smoke
    fixture changes. Convert to descriptor-derived, snapshot, or property
    assertions.

## Greppable Heuristics

Commands from the plan, run from repo root:

```sh
grep -nE 'map\[string\]any\{' internal/tools/*_test.go | wc -l
grep -nE '"additionalProperties":' internal/tools/*_test.go | wc -l
grep -nE '"outputSchema":\s*map' internal/tools/*_test.go | wc -l
grep -nE 'wantCount: *15[0-9]' internal/tools/*_test.go
grep -nE 'len\(.*Registry.*\)' internal/tools/*_test.go
grep -nE 'tool-catalog\.json\|tool-catalog\.md' internal/tools/*_test.go
```

Observed output:

| Heuristic | Result |
| --- | ---: |
| `map[string]any{` in `internal/tools/*_test.go` | 1339 |
| `"additionalProperties":` in `internal/tools/*_test.go` | 5 |
| `"outputSchema":\s*map` in `internal/tools/*_test.go` | 0 |
| `wantCount: *15[0-9]` | 2 matches in `internal/tools/fullaccess_test.go` |
| `len(.*Registry.*)` | 7 matches across `fullaccess_test.go`, `product_contract_test.go`, and `schema_tighten_test.go` |
| `tool-catalog.json|tool-catalog.md` | 11 matches across catalog/doc lint and quality tests |

Top `map[string]any{` files:

| File | Matches |
| --- | ---: |
| `internal/tools/oneuser_live_test.go` | 127 |
| `internal/tools/expenses_test.go` | 109 |
| `internal/tools/time_management_test.go` | 99 |
| `internal/tools/invoices_test.go` | 92 |
| `internal/tools/reports_test.go` | 74 |
| `internal/tools/oneuser_quality_test.go` | 69 |
| `internal/tools/tools_test.go` | 57 |
| `internal/tools/webhooks_test.go` | 46 |
| `internal/tools/custom_fields_test.go` | 39 |
| `internal/tools/groups_holidays_test.go` | 33 |

Top per-test clusters from the same heuristic:

| Test | Matches |
| --- | ---: |
| `internal/tools/oneuser_live_test.go:369` `TestOneUserLiveRemainingCoverageProbes` | 85 |
| `internal/tools/expenses_test.go:154` `TestTier2_Expenses_FullSweep` | 39 |
| `internal/tools/invoices_test.go:22` `TestTier2_Invoices_FullSweep` | 37 |
| `internal/tools/oneuser_quality_test.go:2159` `TestOneUserStopWorkNoRunningTimerReturnsNoop` | 27 |
| `internal/tools/groups_holidays_test.go:25` `TestTier2_GroupsHolidays_FullSweep` | 24 |
| `internal/tools/custom_fields_test.go:14` `TestTier2_CustomFields_FullSweep` | 22 |
| `internal/tools/oneuser_live_test.go:218` `TestOneUserLiveOptionalDomainContracts` | 20 |
| `internal/tools/oneuser_quality_test.go:561` `TestOneUserWorkflowSchemasCoverActualFakeOutputs` | 14 |
| `internal/tools/schema_keyword_test.go:81` `TestSchemaSupportedKeywordsAcceptsSupportedSubset` | 13 |
| `internal/tools/fullaccess_test.go:679` `TestWorkflowPackageLogReviewAndRepeatableDemoCleanup` | 11 |

## Top 10 Hotspots

### 1. `internal/tools/fullaccess_test.go:258-418`

`TestRegistryForToolsetFiltersOwnerSurfaces` asserts exact toolset counts,
selected inclusions, selected exclusions, optional exact ordering, duplicate
protection, workflow prefixing, and tools/list registry parity. The `all=156`
and `default=16` counts are product truth, but the `core`, `business`, and
`admin` exact counts are more positional: adding an allowed domain tool or
moving a tool between subsets would fail this test before a user-visible
contract changed. Convert the non-contract subset count checks to named
membership/property assertions and leave the hard 156/16 truth in the product
contract test. Pattern: descriptor-derived/property. Effort: S.

### 2. `internal/tools/tools_test.go:16-58`

`TestFullAccessRegistryContainsCoreOneUserTools` repeats the full-registry
`156` assertion and then checks a curated list of core tool names. The curated
name set is useful, but the duplicate full count makes unrelated registry-count
changes fan out into another file. Convert this to a pure "required anchor
tools exist" property and let `product_contract_test.go` own exact startup and
advertised counts. Pattern: property. Effort: S.

### 3. `internal/tools/product_contract_test.go:90-125`

`TestDefaultToolsetEnvAdvertises16WhileRegistryLoads156` pins the binding
AGENTS.md invariant: startup loads 156 tools, default advertises 16, and `all`
advertises 156. This is rigid by design and should remain the single canonical
place for exact full/default/all counts. The only conversion worth considering
is extracting named constants if later PRs need to share the numbers without
duplicating literals. Classification: truth-pinning, keep. Pattern: none.
Effort: S if constants are extracted later.

### 4. `internal/tools/oneuser_quality_test.go:408-486`

`TestOneUserMarkdownRoutesCurrentDocsAndMarksPlatformHistory` hard-codes the
current-doc entrypoints, stale platform terms, and an allowed-doc exception
list. It guards a real truth: current docs should dominate and platform-era
language should stay quarantined. It is still incidentally fragile because any
docs reorganization or newly generated current doc can require editing a
large hand-written allowlist. Convert the allowlist into a small committed
snapshot or a docs-owned manifest and keep the test as "manifest is honored"
plus "archive dirs are skipped." Pattern: snapshot/property. Effort: M.

### 5. `internal/tools/oneuser_quality_test.go:1086-1510`

The coverage-ledger tests parse `docs/goals/oneuser-tool-coverage.md`, verify
summary counts, compare fake/live/happy-path yes rows with named evidence maps,
and check required live gates. This is lockstep by design and matches the P7
non-goal: do not rewrite the ledger discipline. It will keep cascading when
coverage cells flip, but those flips are behavioral/evidence truth, not
incidental descriptor shape. Classification: truth-pinning, keep. Pattern:
none. Effort: none.

### 6. `internal/tools/invoices_test.go:22-188`

`TestTier2_Invoices_FullSweep` covers the invoices domain through one broad
fake upstream switch and a long sequence of direct handler calls. It pins real
routes and body normalization, but a small argument-shape or response-view
change can require editing a dense multi-purpose test even when only one
handler changed. Convert the broad sweep into a thin domain smoke plus focused
per-handler table helpers for route, method, validation, and output envelope.
Pattern: property/descriptor-derived fixture. Effort: M.

### 7. `internal/tools/expenses_test.go:154-333`

`TestTier2_Expenses_FullSweep` has the same shape as the invoices sweep:
many fake upstream fixtures, multipart checks, validation probes, category
CRUD, currency lookup, and archive/delete behavior live in one test body. It
pins important behavior, but the density makes unrelated expense changes look
larger than they are. Split it into a small all-tools smoke and focused
subtests for multipart create/update, category unit-price payloads, and
archive/delete. Pattern: property/descriptor-derived fixture. Effort: M.

### 8. `internal/tools/oneuser_live_test.go:369-666`

`TestOneUserLiveRemainingCoverageProbes` is the largest `map[string]any`
cluster and drives many live coverage cells. The evidence requirement is
truth-pinning, but the inlined arguments and tool sequence are incidental
shape: adding optional descriptor annotations or changing one handler's
accepted alias can force edits inside a single very large live test. Convert
the low-risk portions to a named probe table that uses descriptor-derived
coverage args where possible, and keep bespoke live setup only where a real
entity dependency exists. Pattern: descriptor-derived fixture/property.
Effort: M.

### 9. `internal/tools/time_management_test.go:57-119`

`TestSchedulingHandlersCount` and `TestTimeOffHandlersCount` assert exact
domain-handler counts and explicit tool names. The name presence assertions
pin the useful contract; the exact `10` and `13` counts are positional and
will fail on additive domain work before the broader product contract fails.
Convert to required-name subset checks, duplicate detection, and optional
category/schema invariants. Pattern: property. Effort: S.

### 10. `internal/tools/schema_keyword_test.go:12-129`

The supported/forbidden schema keyword tables and the synthetic accepted
schema pin the JSON Schema subset accepted by the MCP/tooling boundary. Most
of this is truth-pinning, but the supported-keyword literal is a likely future
tripwire when an optional, supported annotation keyword is added. Convert only
the table ownership: keep forbidden-keyword behavior as-is, but derive the
supported set from the schema walker or a single exported compatibility list
instead of duplicating it in this test. Pattern: property/descriptor-derived.
Effort: S.

## Conversion Order

### PR-B top 5

1. `internal/tools/tools_test.go:16-58` — remove duplicate exact full-registry
   count and keep required-anchor membership.
2. `internal/tools/fullaccess_test.go:258-418` — keep subset membership checks
   but replace non-contract `core`/`business`/`admin` exact counts with
   properties.
3. `internal/tools/time_management_test.go:57-119` — replace domain handler
   count literals with required-name/property checks.
4. `internal/tools/schema_keyword_test.go:12-129` — centralize supported
   schema keyword ownership and leave forbidden-keyword behavior pinned.
5. `internal/tools/invoices_test.go:22-188` — split the broad invoice sweep
   into focused subtests or helper tables without loosening route/body truths.

### PR-C next 5

1. `internal/tools/expenses_test.go:154-333` — split the broad expense sweep
   into focused multipart/category/archive/delete probes.
2. `internal/tools/oneuser_live_test.go:369-666` — move repeatable live probes
   into descriptor-derived or named probe tables while preserving gates.
3. `internal/tools/oneuser_quality_test.go:408-486` — move platform-era docs
   exceptions to a committed manifest/snapshot.
4. `internal/tools/product_contract_test.go:90-125` — keep truth-pinning counts,
   but extract shared constants only if PR-B still needs count references.
5. `internal/tools/oneuser_quality_test.go:1086-1510` — no conversion for the
   ledger discipline; include it in PR-C's lint baseline so future incidental
   schema-map reductions do not misclassify ledger lockstep work as debt.

## Notes for PR-B/PR-C

- Do not remove tests. Convert broad literal fixtures into smaller properties
  or descriptor-derived fixtures, then prove a real behavior change still
  fails.
- Keep the coverage ledger lockstep path intact:
  `oneuser-tool-coverage.md` row + summary counts, evidence maps, parity
  matrix, and self-inspection assets remain separate truth plumbing.
- The eventual PR-C lint should use the measured post-conversion counts as
  budgets, not the raw counts in this inventory.
