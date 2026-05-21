# P4 · Narrow advertised default toolset; registry stays at 156

**TL;DR.** Change the default advertised surface from `all` (156 tools)
to a 16-tool everyday set. The registry still loads all 156; widening
the env exposes more. Cuts `tools/list` further on top of P1's savings.
Addresses critique item 5.

Estimated effort: **M** (2–5 days). **Depends on P1** for the
`AdvertiseOutputSchema` descriptor flag pattern; the implementation
extends the same descriptor with a tier marker.

> Product-contract change. Surface this prominently in the PR and get
> explicit maintainer approval before merging. The plan assumes approval.

## Problem

Today `CLOCKIFY_TOOLSET=all` is the default, so every MCP client sees the
full 156-tool surface, regardless of whether the user is doing daily
time-tracking or once-a-month billing. The catalog shows three categories
(domain 137, workflow 17, raw 2). The daily-use surface for a single-user
time-tracking workflow is much smaller.

After P1 lands, `tools/list` is ≈ 65k tokens for the full registry. P4
narrows the default surface to ~16 tools (~10–14k tokens), with the rest
opt-in via env. Bigger ergonomic win on top of P1.

## Goal

Default `CLOCKIFY_TOOLSET` advertises exactly 16 tools spanning daily
time-tracking. `CLOCKIFY_TOOLSET=all` continues to advertise all 156.
The registry loaded at startup is unchanged; tools not advertised are
still dispatch-callable by name.

## Non-goals

- Removing any tool from the registry.
- Blocking invocation of an unadvertised tool by name.
- Changing the meaning of existing tier values (`core`, `business`,
  `admin`, `all`) — those keep behaving as today.
- Implementing `notifications/tools/list_changed`. Deferred (see Decided).

## Decided

- **Default tier rename:** introduce a new tier `default` rather than
  silently redefining `core`. `core` keeps its current meaning so existing
  configs keep working.
- **Default tier membership (16 tools):**
  - **10 workflows:** `clockify_status`, `clockify_tools_guide`,
    `clockify_start_work`, `clockify_stop_work`, `clockify_switch_work`,
    `clockify_log_work`, `clockify_review_day`, `clockify_review_week`,
    `clockify_fix_entry`, `clockify_create_work_package`.
  - **6 domain:** `clockify_entries_running`, `clockify_entries_list`,
    `clockify_entries_get`, `clockify_entries_update`,
    `clockify_entries_delete`, `clockify_reports_summary`.
- **Default env value changes** from `"all"` to `"default"`.
- **`notifications/tools/list_changed`:** **deferred**. No runtime
  env-reload path exists today; advertised tier is fixed at startup.
- **Version:** v0.3.0, not v1.0.0. Repo is currently at v0.2.0. Pre-1.0
  permits behavioral changes. Release notes lead with this change and
  the restore command (`CLOCKIFY_TOOLSET=all`).

## Source locations to read first

| Read | Why |
|---|---|
| `internal/config/oneuser.go:27` | `DefaultToolset = "all"` — flip to `"default"`. |
| `internal/config/oneuser.go:251` | `parseToolsetEnv()` — extend the enum to accept `default`. |
| `internal/tools/oneuser_domains.go:30` | `RegistryForToolset(toolset string)` — current filtering switch. |
| `internal/tools/oneuser_domains.go:114-118` | The `case "core"/"business"/"admin"` cases. Add a `case "default"`. |
| `internal/tools/fullaccess_test.go:260` | `TestRegistryForToolsetFiltersOwnerSurfaces` — table-driven test. Add a `default` row. |
| `cmd/clockify-mcp/main.go` | `doctor` output — needs to print advertised count and full registry count separately. |

P1 must be merged first. The descriptor flag `AdvertiseOutputSchema` lives
on `mcp.ToolDescriptor`; this plan adds another flag/tag in the same
spirit.

## Implementation tasks

### Task 1 — Add a tier tag to descriptors

**File:** `internal/mcp/types.go` (the file P1 already touches).

Add to `ToolDescriptor` (decide on `Tiers []string` vs. `Tier string` —
multi-tier membership is needed because, e.g., `clockify_status` belongs
to both `default` and `core`). Recommended:

```go
// Tiers lists the toolset tiers this descriptor belongs to. Tier names:
// "default" (everyday surface), "core", "business", "admin". The "all"
// tier is implicit — every descriptor belongs to it.
Tiers []string
```

If `RegistryForToolset` is already implemented by per-descriptor inclusion
(rather than by hard-coded lists in the switch), align with that
convention. Read `internal/tools/oneuser_domains.go:30-50` to confirm
which style is in use today.

**Verify:**

```
go build ./...
```

**Commit:**

```
feat(mcp): add ToolDescriptor.Tiers for advertised-surface tagging

Sets up per-descriptor tier membership, used by RegistryForToolset to
filter the tools/list response. Default false-ish (nil) preserves the
"belongs to no tier" sentinel for future use; existing tier filtering
continues to work because it reads existing per-descriptor metadata.
```

### Task 2 — Update the config parser to accept "default"

**File:** `internal/config/oneuser.go`.

Line 27: change `DefaultToolset = "all"` to `DefaultToolset = "default"`.

Lines 251–262 (`parseToolsetEnv` and its error message): extend the valid
set to `default | core | business | admin | all`. Update the error
string to match. Touch only this function and the constant.

In the same file, find the `parseToolsetEnv` call site (line 132) — no
change needed; only the resolved string changes.

**Verify:**

```
go test -count=1 ./internal/config
```

Expect: existing tests fail until Task 4 updates them. Read
`internal/config/oneuser_test.go` to see what changed:

```
grep -n 'DefaultToolset\|"all"\|"core"\|"business"\|"admin"' internal/config/oneuser_test.go | head
```

If the existing tests assert `cfg.Toolset == "all"` for the default case
(line ~54), they will fail. Fix them in this same commit by changing the
expected value to `"default"`. Add a new positive case for `"default"`
parsing.

**Commit:**

```
feat(config): accept "default" toolset and make it the new default

CLOCKIFY_TOOLSET now accepts default|core|business|admin|all. The default
when unset shifts from "all" to "default" — a narrower advertised
surface. The registry continues to load all 156 tools; tool dispatch by
name is unaffected.
```

### Task 3 — Tag descriptors with the `default` tier

In `internal/tools/` (across the files where the 16 named tools are
constructed), add `Tiers: []string{"default", "core"}` (or whatever set
they belong to today — adding `"default"` to that set). For these names:

```
clockify_status
clockify_tools_guide
clockify_start_work
clockify_stop_work
clockify_switch_work
clockify_log_work
clockify_review_day
clockify_review_week
clockify_fix_entry
clockify_create_work_package
clockify_entries_running
clockify_entries_list
clockify_entries_get
clockify_entries_update
clockify_entries_delete
clockify_reports_summary
```

Grep to locate constructions:

```
for t in clockify_status clockify_tools_guide clockify_start_work clockify_stop_work \
         clockify_switch_work clockify_log_work clockify_review_day clockify_review_week \
         clockify_fix_entry clockify_create_work_package clockify_entries_running \
         clockify_entries_list clockify_entries_get clockify_entries_update \
         clockify_entries_delete clockify_reports_summary; do
  echo "== $t =="
  grep -n "\"$t\"" internal/tools/*.go | head -3
done
```

If the existing tier metadata is conveyed by *position in the switch* in
`RegistryForToolset` rather than a per-descriptor tag, the cleanest
implementation may be to add a `case "default"` that explicitly lists
these 16 names. In that case, skip the per-descriptor `Tiers` annotation
and use a name-based filter instead. Read the file first; pick whichever
matches the prevailing pattern.

**Verify:**

```
go build ./...
```

**Commit:**

```
feat(tools): mark the 16-tool default tier

Names: <list>. Workflows + entry CRUD + summary report. The advertised
surface under the new default tier; the registry still loads 156.
```

### Task 4 — Extend `RegistryForToolset` to handle `"default"`

**File:** `internal/tools/oneuser_domains.go`, lines 114–120.

Add the `case "default"` branch. The exact body depends on Task 3's
choice (per-descriptor `Tiers` filter vs. name-list filter). The result
must be a registry-order subslice of the 16 named tools.

The function must continue to return `s.FullAccessRegistry()` for `"all"`
(or empty/nil toolset value) and the existing slices for `"core"`,
`"business"`, `"admin"`.

**Verify:**

```
go test -count=1 -run TestRegistryForToolset ./internal/tools
```

Edit `internal/tools/fullaccess_test.go:260` (the table-driven test) to
add a `default` row asserting the 16 tools by name and `wantCount: 16`.
Use the existing table's pattern for the assertion shape.

**Commit:**

```
feat(tools): RegistryForToolset returns the 16-tool default surface

Adds a "default" case alongside core/business/admin/all. Existing tiers
unchanged. Test updated to assert membership and ordering.
```

### Task 5 — Doctor output: registry size vs. advertised count

**File:** `cmd/clockify-mcp/main.go` (locate the doctor command — grep
`doctor` in that file).

The doctor should print two distinct numbers:

```
Registry loaded:    156 tools
Advertised surface: 16 tools (toolset=default)
```

Today it likely prints one count. Add the second. Match the existing
print format (table-style, colon-aligned, or whatever style is in use —
read 10 lines around the existing output before editing).

**Verify:**

```
go build -o /tmp/clockify-mcp ./cmd/clockify-mcp
CLOCKIFY_API_KEY=test CLOCKIFY_WORKSPACE_ID=test /tmp/clockify-mcp doctor | grep -E 'Registry|Advertised|toolset'
```

Expect both lines to appear with sensible values.

**Commit:**

```
feat(doctor): show advertised count alongside registry size

Operators see both "Registry loaded: 156" and "Advertised surface: N
(toolset=X)" so the difference between dispatch-capable and
model-visible is explicit.
```

### Task 6 — README and AGENTS.md: document the new default

**Files:** `README.md`, `AGENTS.md`, `docs/protocol-notes.md`.

In `README.md`, find the "Configuration" / `CLOCKIFY_TOOLSET` row. Change
the `Default` column from `all` to `default`. Update the `Purpose` cell:

> Tool surface advertised on the wire: `default` (16 everyday tools),
> `core`, `business`, `admin`, or `all` (156). The full registry of 156
> tools is always loaded; `tools/list` advertises a subset.

Add a short subsection right below the table:

```markdown
### Restoring the previous default

Earlier versions defaulted to `CLOCKIFY_TOOLSET=all`. To keep that
behavior after upgrading, set:

    export CLOCKIFY_TOOLSET=all

The full 156-tool surface remains available; only the *advertised*
default has narrowed.
```

In `AGENTS.md`, find the "Product Contract" item that mentions
`CLOCKIFY_TOOLSET=all` (the original wording: "The default
`CLOCKIFY_TOOLSET=all` loads the complete registry"). Tighten it:

```markdown
- The default `CLOCKIFY_TOOLSET=default` advertises 16 everyday tools.
  `CLOCKIFY_TOOLSET` may also be `core`, `business`, `admin`, or `all`.
  The startup registry always loads 156 tools regardless of toolset;
  tools not advertised are still dispatch-callable by name.
```

In `docs/protocol-notes.md`, add a paragraph mirroring the P1 wire/
validation paragraph but for the advertised tier.

**Verify:**

```
git grep -n 'CLOCKIFY_TOOLSET=all\|toolset=all' README.md AGENTS.md docs/ | grep -v archive
```

Read each remaining match; confirm it's intentional (e.g. the restore-
instruction examples).

**Commit:**

```
docs: announce CLOCKIFY_TOOLSET=default as the new default

Adds a restore instruction (CLOCKIFY_TOOLSET=all) for users who want the
previous behavior. README, AGENTS.md, and docs/protocol-notes.md updated.
```

### Task 7 — Re-baseline the wire-budget test

**File:** `internal/mcp/tools_list_budget_test.go`.

The P1 budget (≤ 280 KB) was set for the full 156-tool advertised surface.
Under the new default, advertised size will drop dramatically (16 tools).
Either:

- Add a second assertion under the default toolset (~30 KB, plenty of
  headroom — pick 60 KB).
- Keep the P1 280 KB cap for the `CLOCKIFY_TOOLSET=all` case via a
  sub-test that explicitly forces `all`.

Both. Two sub-tests, two budgets.

**Verify:**

```
go test -count=1 -v -run BudgetWire ./internal/mcp
```

Read the printed sizes in both modes; confirm they match expectations:

- default → ~25–55 KB
- all → ~250–280 KB

**Commit:**

```
test(mcp): wire-budget assertions for default and all toolsets

Two sub-tests: default (cap 60 KB) and all (cap 280 KB). Pins the
ergonomic win from this change and prevents accidental regressions.
```

### Task 8 — Release notes

**File:** `CHANGELOG.md` (if present) or the release-notes file the repo
uses for tag-triggered releases. Grep:

```
ls CHANGELOG* RELEASE-NOTES* 2>/dev/null
grep -rn 'release-notes' docs/ .github/ | head
```

The repo has a tag-triggered release workflow (per recent commit
`d303659`). Find where that workflow reads release notes (likely a
`CHANGELOG.md` section or a `release-notes/` directory). Append a v0.3.0
entry:

```markdown
## v0.3.0

### Breaking — advertised toolset narrowed

The default `CLOCKIFY_TOOLSET` changes from `all` to `default`. Under
the new default, `tools/list` advertises 16 daily-time-tracking tools:
status, the timer workflows (start/stop/switch/log), the daily/weekly
review tools, fix_entry, create_work_package, tools_guide, plus entry
CRUD (running/list/get/update/delete) and the summary report.

The startup registry continues to load all 156 tools. Tools not
advertised under the new default remain dispatch-callable by name. To
restore the previous behavior, set:

    export CLOCKIFY_TOOLSET=all

Other tiers continue to behave as before: `core`, `business`, `admin`.
```

**Verify:** doc-only; visual check.

**Commit:**

```
docs: release notes for v0.3.0 default-toolset change
```

## Full validation

```
go test -count=1 ./...
make perfect
```

Manual sanity:

```
# default behavior
unset CLOCKIFY_TOOLSET
CLOCKIFY_API_KEY=test CLOCKIFY_WORKSPACE_ID=test /tmp/clockify-mcp doctor | grep -E 'Advertised|toolset'

# restored behavior
export CLOCKIFY_TOOLSET=all
CLOCKIFY_API_KEY=test CLOCKIFY_WORKSPACE_ID=test /tmp/clockify-mcp doctor | grep -E 'Advertised|toolset'
unset CLOCKIFY_TOOLSET
```

Expect the first to show "Advertised surface: 16 (toolset=default)";
the second to show "Advertised surface: 156 (toolset=all)".

## Rollback

Revert in reverse order. The key revert is Task 2 (config default
flip) — reverting that alone restores the old behavior, even if the
descriptor tags from Task 3 remain (they're additive).

## PR description template

```
## Summary

- Add `mcp.ToolDescriptor.Tiers` for per-descriptor tier membership.
- Add a new `default` tier with 16 daily-time-tracking tools.
- Change `CLOCKIFY_TOOLSET` default from `all` to `default`.
- Print "Advertised surface" alongside "Registry loaded" in doctor.
- README/AGENTS.md/protocol-notes updated; CHANGELOG entry for v0.3.0.

## Breaking change

`tools/list` defaults to 16 tools, not 156. The full registry still
loads; `CLOCKIFY_TOOLSET=all` restores the previous advertised surface.

## Why

For a single-user time-tracking workflow, a 156-tool surface burns
~158k tokens of model prompt space (with P1 applied, ~65k). The new
default surface is ~25–55 KB (~10–14k tokens). Daily ergonomics win,
no loss of capability — unadvertised tools remain dispatch-callable.

## Test plan

- [ ] `go test -count=1 ./...`
- [ ] `make perfect`
- [ ] Doctor manually: default shows 16; `CLOCKIFY_TOOLSET=all` shows 156.
- [ ] `tools/list` over stdio: confirm 16 vs 156 in both modes.
- [ ] Sanity-call an unadvertised tool by name to confirm dispatch
      still works.

## Migration

`export CLOCKIFY_TOOLSET=all` restores the previous behavior. Documented
in README under "Restoring the previous default".
```

## Anti-patterns

- **Do not** repurpose `core` as the new default. Existing configs use
  `core` and depend on its current membership.
- **Do not** remove `clockify_tools_guide` from the default tier — it's
  the model's meta-discovery escape hatch when an agent needs a tool not
  in the advertised set.
- **Do not** ship without `CLOCKIFY_TOOLSET=all` restoration documented in
  the same PR.
- **Do not** implement `notifications/tools/list_changed` in this PR.
  Deferred per Decided.
- **Do not** bump the major version. v0.3.0, not v1.0.0.
