# P3 · Split dense files in internal/tools

**TL;DR.** Split 7 source files and 3 test files in `internal/tools/`
along Clockify-resource lines. Mechanical, rename-only commits.
Wire/behavior/registry-order unchanged. Addresses critique item 3.

Estimated effort: **M** (2–5 days). One file per commit; ship as a series
of small PRs or one stacked PR.

## Problem

Current line counts of source files ≥ 1000 lines:

| File | Lines |
| --- | ---: |
| `firstslice.go` | 1,658 |
| `oneuser_domains.go` | 1,500 |
| `scheduling_view.go` | 1,498 |
| `invoices.go` | 1,340 |
| `oneuser_workflows.go` | 1,235 |
| `common.go` | 1,222 |
| `time_off.go` | 1,116 |

Test files ≥ 1000 lines: `oneuser_quality_test.go` (3,415),
`tools_test.go` (2,025), `time_management_test.go` (1,798),
`invoices_test.go` (1,284), `expenses_test.go` (1,098),
`reports_test.go` (1,054), `webhooks_test.go` (1,040).

These files mix several Clockify resources. Reviewing one resource
end-to-end is painful. The registry contract is **order**, not file
count — splitting by resource is wire-neutral.

## Goal

No source file in `internal/tools/` exceeds ~600 lines (test files: ~800
lines). Each file scoped to one Clockify resource family. Registry order,
descriptor count, exported symbols, and wire behavior unchanged.

## Non-goals

- Renaming any descriptor, tool, or exported symbol.
- Reorganizing the registry order.
- Refactoring handler logic during the moves.
- Splitting files that are already <600 lines.

## Decided

- **Per-file split commits are rename-only.** Each commit moves text from
  one file to another and nothing else. No symbol renames. No comment
  rewrites. No formatting changes other than what `gofmt` produces.
- **Order of work** (smallest blast radius first): `time_off.go` →
  `invoices.go` → `oneuser_workflows.go` → `common.go` →
  `oneuser_domains.go` → `firstslice.go` → `scheduling_view.go`. Tests
  follow once their parent source files are stable.
- **Target line cap:** 600 source / 800 test. A 650-line cohesive file is
  acceptable; do not split for the sake of splitting.
- **Verification gate after each split:** `go test -count=1 ./internal/tools
  ./internal/mcp && make catalog-drift && make selfinspect-drift`. If any
  fails, the split is wrong.

## Source locations to read first

| Read | Why |
|---|---|
| `internal/tools/firstslice.go:88` | `FirstSliceRegistry()` — the ordered slice. The order is the contract. |
| `internal/tools/oneuser_domains.go:20` | `FullAccessRegistry()` — the registry assembly point. |
| `internal/tools/oneuser_domains.go:30` | `RegistryForToolset(toolset string)` — tier filter. |
| `internal/tools/common.go:401` and surroundings | Helper constructors `tool`, `toolRO`, `toolRW`, `withOutputSchema`. Splits must not break callers. |

Pre-flight grep to find each resource's footprint:

```
grep -nE '^func .*(TimeOff|Holiday|Approval|Invoice|Project|Client|Task|Tag|Entry|Expense|Schedul|Group|User|Webhook|Audit)' internal/tools/*.go | sort -t: -k1,1
```

## Per-file split tables

Each table is the *destination* layout. Move the symbols listed into the
named new file. Symbol names below are illustrative — derive the actual
list per file by grep + read.

### 3.1 `time_off.go` (1,116 → ~400 each across 3 files)

| New file | Resource cluster | Symbols (illustrative) |
| --- | --- | --- |
| `time_off_requests.go` | Time-off request CRUD | `timeOffRequestsList/Create/Get/Update/Delete`, related schemas |
| `time_off_policies.go` | Policies | `timeOffPoliciesList/Get/Create/Update`, policy helpers |
| `time_off_balances.go` | Balances + approvals | `timeOffBalances`, `timeOffApprove/Deny`, balance helpers |

`time_off.go` itself: keep ≤ 50 lines if any common helpers remain, or
delete it entirely if everything moves.

### 3.2 `invoices.go` (1,340 → ~350 each across 4 files)

| New file | Resource cluster |
| --- | --- |
| `invoices.go` (kept, slimmer) | Invoice list/get/create/update/delete + mark-paid |
| `invoices_items.go` | Line items: add/list/delete (`update` is unsupported per AGENTS.md gotchas) |
| `invoices_payments.go` | Payments: create/list/delete |
| `invoices_imports.go` | `import_time` + `import_expenses` + grouping helpers |

### 3.3 `oneuser_workflows.go` (1,235 → ~400 each across 3 files)

| New file | Cluster |
| --- | --- |
| `workflows_status_and_review.go` | `clockify_status`, `clockify_review_day`, `clockify_review_week`, `clockify_tools_guide` |
| `workflows_time_capture.go` | `clockify_start_work`, `clockify_stop_work`, `clockify_switch_work`, `clockify_log_work`, `clockify_fix_entry` |
| `workflows_business.go` | `clockify_create_work_package`, `clockify_invoice_client_work`, `clockify_record_expense`, `clockify_request_time_off`, `clockify_schedule_work`, `clockify_setup_webhook`, `clockify_demo_seed`, `clockify_demo_cleanup` |

### 3.4 `common.go` (1,222 → ~600 plus per-domain helpers)

Inventory `common.go` first. Anything that is genuinely cross-cutting
(scrubbing, pagination headers, JSON-number coercion, schema tightening)
stays. Per-resource helpers (e.g. project-rate validators, expense-amount
checks) move into the resource's file.

Plan to extract:

| Move to | Helpers (illustrative) |
| --- | --- |
| `helpers_money.go` (new) | Currency conversion (minor/major), money formatting, totals normalization |
| `helpers_dates.go` (new) | Date coercion (YYYY-MM-DD → API format), end-of-day handling |
| `helpers_pagination.go` (new) | `page` / `page_size` parsing, `has_more` / `dropped` / `total_min` |
| keep in `common.go` | JSON-number coercion, schema tightening, descriptor mutators (`withOutputSchema`, etc.) |

Aim: `common.go` ≤ 700 lines after extraction.

### 3.5 `oneuser_domains.go` (1,500 → split by domain)

This file is the registry assembly. The `FullAccessRegistry()` builder is
the contract. Strategy: keep `FullAccessRegistry()` and the order tables
in `oneuser_domains.go` (≤ 300 lines). Move the per-domain descriptor
literal slices to per-domain files:

| Move to (or create) | Slices |
| --- | --- |
| `descriptors_high_value.go` | The `nativeHighValueDescriptors()` body |
| `descriptors_extras.go` | The `nativeDomainExtras()` body |
| `descriptors_timer_reports.go` | The `timerAndReportDescriptors()` body |
| `descriptors_raw.go` | The `rawAPIDescriptors()` body |

`oneuser_domains.go` after split: the assembly function + the toolset
filter + lift each per-domain slice's `func` body into its own file via
`git mv` of the function block. Confirm: `wc -l
internal/tools/oneuser_domains.go` ≤ 350.

### 3.6 `firstslice.go` (1,658 → ~400 each across ~4 files)

`FirstSliceRegistry()` returns an ordered slice of descriptors. The order
is the contract. Strategy: rename `firstslice.go` to `registry_order.go`
holding the assembly function only (≤ 200 lines). Move per-family
descriptor blocks to dedicated files:

| Move to | Cluster |
| --- | --- |
| `firstslice_entries.go` | All entry-related descriptors in the first slice |
| `firstslice_projects_clients_tasks.go` | Project/client/task descriptors |
| `firstslice_reports.go` | Report descriptors |
| `firstslice_misc.go` | Anything else cohesive (tags, users-profile, workspace-settings, …) |

### 3.7 `scheduling_view.go` (1,498 → split by capability)

| New file | Cluster |
| --- | --- |
| `scheduling_view.go` (kept, slim) | View shaping shared across scheduling tools |
| `scheduling_assignments.go` | Assignment list/create/get/update/delete (recurring route) |
| `scheduling_capacity.go` | Capacity reads + project/user totals |
| `scheduling_publish.go` | Publish workflow |

## Per-split procedure (apply for every file in §3.x)

For each entry in §3:

1. **Read** the current file end-to-end. Note function boundaries and
   any package-level vars / consts that need to migrate with the
   functions that use them.
2. **Create** the new file(s). Add the same `package tools` header and
   the imports the moved code needs.
3. **Move** function blocks with `gofmt`-safe whole-line cuts. Do not edit
   bodies. If a function references a private helper, decide which of
   the two new files the helper belongs in (one of them, not both —
   never duplicate).
4. **Run** `goimports -w internal/tools/*.go` if available, or
   `go build ./...` and fix any import errors by editing import blocks
   only. Do not change function bodies to dodge an import.
5. **Verify** with the gates below.
6. **Commit** as a single rename-only commit using the template.

### Verification gate (run after every file's split)

```
gofmt -l internal/tools | tee /tmp/gofmt-out && [ ! -s /tmp/gofmt-out ]
go vet ./internal/tools
go test -count=1 ./internal/tools ./internal/mcp
make catalog-drift
make selfinspect-drift
wc -l internal/tools/*.go | sort -rn | head -10
```

Expect:
- `gofmt -l` produces no output (file list of unformatted files is empty).
- `go vet` clean.
- Tests green.
- Drift gates clean (no diff in generated catalogs).
- `wc -l` head: the file you just split no longer appears at the top.

### Commit template (per split)

```
refactor(tools): split <oldfile>.go by <resource> into <newfile-list>

No descriptor, symbol, registry-order, or wire-behavior change. Move-only
diff. Verified:
  go test -count=1 ./internal/tools ./internal/mcp
  make catalog-drift
  make selfinspect-drift
```

### Blame-cleanliness check

After each split, spot-check with:

```
git log --follow -p internal/tools/<new-file>.go | head -20
```

The history should resolve through the rename. If it does not (because
git couldn't detect the rename due to too-small content), increase the
`-M` rename threshold:

```
git log --follow -M50 -p internal/tools/<new-file>.go | head
```

## Tests follow source (§3.8)

Once source splits are stable, do the test files in the same shape. Same
discipline: rename-only commits, one test file per commit, verify gate
identical except `make catalog-drift` is rarely affected by test moves.

| Test file | Mirror split |
| --- | --- |
| `time_management_test.go` | Split per the entry/timer/log clusters it tests |
| `invoices_test.go` | Mirror §3.2 |
| `expenses_test.go` | Split by category vs expense CRUD if applicable |
| `reports_test.go` | Split by report family (summary, detailed, weekly, money, expense, attendance, export) |
| `webhooks_test.go` | Split by lifecycle (CRUD, events, test, callback) |
| `oneuser_quality_test.go` | Split by assertion theme: live-coverage map, schema invariants, registry order, ledger validation |
| `tools_test.go` | Split by what it covers (depending on read) |

## Full validation (at the end of the series)

```
go test -count=1 ./...
make perfect
wc -l internal/tools/*.go | awk '$1 > 600 && $2 != "total" && $2 !~ /_test\.go/' | head
```

The last command should produce no source-file rows (test files allowed
up to ~800). If any source file is still > 600 lines, decide: was the
split incomplete, or is the file cohesive enough at its size? Document
the answer in the PR description.

## Rollback

Each rename-only commit reverts cleanly with `git revert <sha>`. Because
splits are file-additive (you delete-and-readd content), a revert exactly
restores the prior file. No data loss risk.

## PR description template

```
## Summary

Splits `internal/tools/<file>.go` (X lines) into:
  - <file_a>.go (Y lines) — <resource A>
  - <file_b>.go (Z lines) — <resource B>
  - <file_c>.go (W lines) — <resource C>

Rename-only diff. No descriptor, symbol, registry-order, or wire-behavior
change.

## Why

`<file>.go` mixed multiple Clockify resources. Splitting along resource
lines lowers the cost of reading one resource end-to-end and reduces the
chance of an unrelated edit during a focused change.

## Verification

- [ ] gofmt -l clean
- [ ] go vet clean
- [ ] `go test -count=1 ./internal/tools ./internal/mcp`
- [ ] `make catalog-drift` clean
- [ ] `make selfinspect-drift` clean
- [ ] `git log --follow` resolves the rename for one sample file
```

## Anti-patterns

- **Do not** rename a function during a split. Renames are a separate
  follow-up PR if at all.
- **Do not** duplicate a helper. Each helper lives in exactly one file
  after the split.
- **Do not** introduce new exports during a split. If a function was
  private and the move needs cross-file access, keep it private and pick
  one file to hold it — do not capitalize the first letter.
- **Do not** "tidy up" comments, formatting, or imports beyond what
  `gofmt`/`goimports` produces.
- **Do not** batch two file splits into one commit. One file per commit.
- **Do not** split a file that is already cohesive at 700 lines. The cap
  is a guide; cohesion wins ties.
