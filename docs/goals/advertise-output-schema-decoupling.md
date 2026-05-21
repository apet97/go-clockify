# P1 · Decouple advertised output schema from internal validation

**TL;DR.** Drop `outputSchema` from `tools/list` for ~141 of 156 tools.
Keep server-side validation untouched. Net: `tools/list` shrinks from
**158k → ~65k tokens** (≈93k saved). Addresses critique items 1 and 4.

Estimated effort: **S** (≤1 day). No upstream dependency. Required for
P4 (narrow default toolset).

## Problem (with measurements)

Measured from `docs/tool-catalog.json`:

| Category | Tools | output_schema bytes | Disposition |
| --- | ---: | ---: | --- |
| domain | 137 | 366 KB | opt-out |
| workflow | 17 | 64 KB | 14 opt-in, 3 opt-out |
| raw | 2 | 4 KB | opt-out |

`outputSchema` is optional per MCP spec. The current `mcp.Tool` struct
(`internal/mcp/types.go:109`) already declares it `omitempty`:

```go
type Tool struct {
    ...
    OutputSchema map[string]any `json:"outputSchema,omitempty"`
    ...
}
```

So removing it from the wire is "set the field to nil on the wire copy".
The descriptor still holds the full schema for internal use.

## Goal

`tools/list` advertises `outputSchema` only for the 14 workflow tools listed
below. Everything else: `outputSchema` omitted from the wire response;
server-side validation behavior unchanged; generated dev catalog unchanged.

## Non-goals

- Removing or merging any of the 156 tools.
- Loosening server-side output validation.
- Changing `CLOCKIFY_TOOLSET` defaults (P4).
- Editing the generated catalog (`docs/tool-catalog.{md,json}`) — it stays
  the full dev reference.

## Decided

- **Opt-in (14 workflows):** `clockify_status`,
  `clockify_create_work_package`, `clockify_log_work`,
  `clockify_start_work`, `clockify_stop_work`, `clockify_switch_work`,
  `clockify_review_day`, `clockify_review_week`, `clockify_fix_entry`,
  `clockify_invoice_client_work`, `clockify_record_expense`,
  `clockify_request_time_off`, `clockify_schedule_work`,
  `clockify_setup_webhook`.
- **Opt-out (everything else, 142 tools):** all `domain` tools (137), both
  `raw` tools, and the 3 workflows that don't benefit (`clockify_tools_guide`
  — text-shaped; `clockify_demo_seed`, `clockify_demo_cleanup` — rarely
  called).
- **Wire-budget cap:** `tools/list` minified ≤ **280 KB**. Measured target
  ≈ 260 KB. Re-baseline once after first real measurement.
- **Implementation surface:** new field on `mcp.ToolDescriptor`; wire copy
  in `internal/mcp/tools.go:toolsListResultJSONBytes`.

## Source locations to read first

| Read | Why |
|---|---|
| `internal/mcp/types.go:109` | `mcp.Tool` struct. Confirm `OutputSchema` field has `json:"outputSchema,omitempty"`. |
| `internal/mcp/tools.go:84` | `toolsListResultJSONBytes()` — single-marshal hot path. Wire copy goes here. |
| `internal/mcp/tools_list_budget_test.go` | Existing budget test. You will extend it. |
| `internal/tools/common.go:401` | `mcp.Tool{}` constructor (`tool` / `toolRO` / `toolRW`). Helpers compose descriptors. |
| `internal/tools/common.go:542` | `withOutputSchema` and surrounding annotation helpers — pattern for descriptor mutation. |
| `internal/tools/oneuser_workflows.go` | Where the 14 opt-in workflows are declared. Locate their `mcp.ToolDescriptor` constructions. |
| `docs/protocol-notes.md` | Where you add the one-paragraph note about wire-vs-validation decoupling. |

Run before starting:

```
grep -n 'AdvertiseOutputSchema\|advertiseOutputSchema' internal/ -R || echo "expected: no matches (greenfield change)"
go test -count=1 ./internal/mcp ./internal/tools
```

The grep should return no matches — this field doesn't exist yet.

## Implementation tasks

### Task 1 — Add the descriptor flag

**File:** `internal/mcp/types.go`.

In the `ToolDescriptor` struct (search nearby `type ToolDescriptor`), add:

```go
// AdvertiseOutputSchema controls whether Tool.OutputSchema appears in
// tools/list responses. Default false: server-side validation still uses
// the schema, but it is omitted from the wire to shrink tools/list.
AdvertiseOutputSchema bool
```

Place the field after the existing flags (next to `ReadOnlyHint` etc., if
present). Keep the comment.

**Verify:**

```
go build ./...
```

**Commit:**

```
feat(mcp): add Tool descriptor AdvertiseOutputSchema flag

Prepare for decoupling advertised output schema from internal validation.
Default false; future commits set true for the 14 opt-in workflow tools.
```

### Task 2 — Strip outputSchema on the wire copy

**File:** `internal/mcp/tools.go`, function `toolsListResultJSONBytes` (line
~84). Before the JSON marshal, walk the descriptor slice and produce a wire
copy with `OutputSchema` zeroed for any descriptor where
`AdvertiseOutputSchema == false`.

Pattern (read the existing function to align with its current locking and
caching):

```go
wire := make([]mcp.Tool, 0, len(descs))
for _, d := range descs {
    t := d.Tool          // value copy
    if !d.AdvertiseOutputSchema {
        t.OutputSchema = nil
    }
    wire = append(wire, t)
}
// marshal `wire` to JSON exactly the way the existing function marshals tools today
```

If the function currently builds JSON via a different intermediate type, keep
the same approach — just clear `OutputSchema` on the copy before the marshal.

**Verify:**

```
go build ./...
go test -count=1 -run TestToolsList ./internal/mcp
```

Expect existing tests to still pass — no descriptor has opted in yet, so this
commit removes `outputSchema` from every entry. The existing budget test may
or may not have a literal numeric threshold; if it has one that expected
schemas in place, expect it to either still pass (smaller is fine) or report
a new lower size. Read the test before committing.

**Commit:**

```
feat(mcp): omit outputSchema from tools/list unless descriptor opts in

The wire copy zeroes Tool.OutputSchema when the descriptor's
AdvertiseOutputSchema flag is false. Server-side validation is unaffected
because it reads the descriptor, not the wire response.
```

### Task 3 — Opt in the 14 workflow descriptors

**File:** `internal/tools/oneuser_workflows.go` (and any sibling file that
constructs the workflow `mcp.ToolDescriptor` values — locate them with
`grep -n 'clockify_status\|clockify_review_day\|clockify_log_work' internal/tools/`).

For each of these 14 tool names, set `AdvertiseOutputSchema: true` on the
descriptor:

```
clockify_status
clockify_create_work_package
clockify_log_work
clockify_start_work
clockify_stop_work
clockify_switch_work
clockify_review_day
clockify_review_week
clockify_fix_entry
clockify_invoice_client_work
clockify_record_expense
clockify_request_time_off
clockify_schedule_work
clockify_setup_webhook
```

The convention will be one of:

```go
{Tool: withOutputSchema(toolRO(...), envelope...), AdvertiseOutputSchema: true, Handler: ...}
```

or, if you prefer a helper to avoid repetition (only if a helper would
clearly improve readability — otherwise inline is fine):

```go
func advertised(d mcp.ToolDescriptor) mcp.ToolDescriptor {
    d.AdvertiseOutputSchema = true
    return d
}
```

Wrap each of the 14 with `advertised(...)`. Do not touch other workflows
(`clockify_tools_guide`, `clockify_demo_seed`, `clockify_demo_cleanup`).
Do not touch any domain or raw tool.

**Verify:**

```
go build ./...
go vet ./...
go test -count=1 ./internal/tools ./internal/mcp
```

Expect green.

**Commit:**

```
feat(tools): advertise outputSchema for the 14 composed workflow tools

These workflows synthesize results the model chains on, so the schema
helps the agent navigate the response. All domain and raw tools, plus
the 3 non-composed workflows (tools_guide, demo_seed, demo_cleanup),
remain non-advertising.
```

### Task 4 — Extend the wire-budget test

**File:** `internal/mcp/tools_list_budget_test.go`.

Add an assertion that the minified `tools/list` JSON is ≤ 280 KB. Use the
existing test's harness — read the first ~50 lines of the file to copy the
pattern. The new assertion is roughly:

```go
const wireBudgetBytes = 280 * 1024
if len(raw) > wireBudgetBytes {
    t.Fatalf("tools/list wire size %d exceeds budget %d (~%d KB over)",
        len(raw), wireBudgetBytes, (len(raw)-wireBudgetBytes)/1024)
}
```

Also print the measured size on success so future regressions are easy to
spot:

```go
t.Logf("tools/list wire size: %d bytes (%.1f KB)", len(raw), float64(len(raw))/1024.0)
```

**Verify:**

```
go test -count=1 -v -run BudgetWire ./internal/mcp
```

Read the printed size and record it. Expected: 200–280 KB. If it's
significantly larger than 280 KB, investigate before adjusting the cap —
that probably means Task 3 opted in tools that shouldn't be opt-in.

**Commit:**

```
test(mcp): cap tools/list minified wire size at 280 KB

Pins the win from outputSchema decoupling. Measured size on this commit:
<paste from t.Logf output>.
```

### Task 5 — Document the decoupling

**File:** `docs/protocol-notes.md`.

Append a section (locate the appropriate place by reading the existing
structure — likely after the "Tool results" section or near other JSON-RPC
shape notes):

```
## Wire vs. validation: outputSchema

`tools/list` advertises `outputSchema` only for the 14 composed workflow
tools whose synthesized response shape helps an agent chain calls. Domain
CRUD, raw fallback, and the non-composed workflows (`clockify_tools_guide`,
`clockify_demo_seed`, `clockify_demo_cleanup`) omit `outputSchema` on the
wire. Server-side output validation is unchanged: every tool still
validates its result against the schema in its `mcp.ToolDescriptor`. The
generated dev catalog at `clockify://mcp/tool-catalog` exposes full
schemas for clients that want them.
```

**Verify:** rendering check by `cat docs/protocol-notes.md | wc -l` is
sufficient. The doc has no automated lint.

**Commit:**

```
docs(protocol-notes): document outputSchema decoupling
```

### Task 6 — Drift regeneration

The generated catalog should still include full schemas — the catalog
documents descriptors, not the wire. But run drift gates anyway:

```
make catalog-drift
```

Expect: clean (no diff). If it complains, you accidentally touched the
descriptor *content* in Task 3 (you should have only added a flag). Reread
your Task-3 diff.

```
make perfect
```

Expect: green.

If drift gates surface other regenerated assets, regenerate per `AGENTS.md`:

```
make gen-tool-catalog
make sync-selfinspect-assets
```

But do not regenerate to "fix" something you didn't intend to change. Read
the drift output first.

## Full validation

```
go test -count=1 ./...
make perfect
```

Bench check (optional but informative — `tools/list` is on the hot path):

```
go test -bench=BenchmarkDispatchToolsListLarge -benchmem -count=3 ./internal/mcp
```

Compare to baseline if recorded; record new baseline if appropriate.

## Rollback

Each task is its own commit. To revert: `git revert <task-commit-sha>` in
reverse order. The descriptor flag default is `false`, so reverting Task 3
restores all-tools-advertise behavior without further work.

## PR description template

```
## Summary

- Add `mcp.ToolDescriptor.AdvertiseOutputSchema bool` (default false).
- Strip `outputSchema` from `tools/list` on the wire copy when the
  descriptor doesn't opt in.
- Opt in the 14 composed workflow tools.
- Cap minified `tools/list` size at 280 KB in
  `internal/mcp/tools_list_budget_test.go`.
- Document the wire-vs-validation distinction in `docs/protocol-notes.md`.

## Measured impact

- Before: ~158k tokens (~632 KB minified) for 156 tools.
- After: <measured>k tokens (<measured> KB minified).
- Net: <measured>k tokens reclaimed on every `tools/list` exchange.

## Internal validation: unchanged

The descriptor still carries the full `OutputSchema`; validation paths
read from there. Only the wire copy is stripped.

## Test plan

- [ ] `go test -count=1 ./...`
- [ ] `make perfect`
- [ ] `internal/mcp/tools_list_budget_test.go` records the new size in
      `t.Logf`.
- [ ] Manual: `clockify-mcp` over stdio, send `tools/list`, eyeball that
      domain tool entries no longer carry `outputSchema`.
```

## Anti-patterns

- **Do not** advertise output schema on domain CRUD tools. The savings come
  from those 137 entries.
- **Do not** delete or mutate any tool's `OutputSchema` in the descriptor.
  Validation depends on it.
- **Do not** edit `docs/tool-catalog.{md,json}` by hand. If they drift,
  regenerate.
- **Do not** bundle multiple tasks into one commit. Each task is one commit.
- **Do not** widen the opt-in list during implementation. The 14 names are
  decided. If you want to change the set, propose it in the PR description.
