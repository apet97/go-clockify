# Live-Validation Campaign — Continuation Handoff

Date: 2026-05-02 (status note added 2026-05-03)
Branch: `test/full-live-workspace-validation` (12 commits ahead of `main`,
pushed to `origin`)
Draft PR: https://github.com/apet97/go-clockify/pull/53

This doc tells the next agent (or maintainer) exactly what state the
live-validation campaign is in, what tests pass, what bugs were
surfaced, what's left to do, and how to re-run everything locally.

## Status update — 2026-05-03 (post-PR #53–#56)

The bulk of the bug inventory below was closed across four merged
PRs (#53–#56) and one cleanup branch removing the matching phantom
schedule tools. The status, item-by-item against the numbered list:

- **#1, #2, #3, #4 — list-shape envelopes** (`list_invoices`,
  `invoice_report`, `list_expenses`, `expense_report`,
  `list_expense_categories`, `list_webhooks`): **fixed in PR #53.**
- **#5 — `list_webhook_events` route**: **fixed in PR #53** (handler
  now returns the static enum; the dedicated `/events` route was
  proven not to exist).
- **#6 — shared_reports host / route**: **fixed in PRs #53 and #56**
  (host moved to `reports.api.clockify.me`; write/export tools
  rewired to `type`/`filter` body keys, ws-prefixed PUT/DELETE,
  bare-id GET, and binary-aware export envelope).
- **#7 — scheduling 10 tools "wrong host"**: **partially fixed.**
  PR #53 fixed `list_assignments` (`/all` suffix + `start`/`end`).
  PR #55 repointed `filter_schedule_capacity` to per-user totals
  and removed the phantom `list_schedules` tool. The 2026-05-03
  cleanup removed the matching phantom `get_` and `create_`
  schedule tools (no `/scheduling/{id}` or `POST /scheduling`
  surface exists). The four assignment-CRUD tools (get / create /
  update / delete on `/assignments/{id}`) still hit a path that
  was 404 in the probe lab matrix and remain pinned in
  `TestLiveT2BlockedGroups` — they're a candidate for a future
  probe + re-route batch but are not in scope for the PR #53–#56
  wave.
- **#8 — `list_time_off_requests` GET→POST**: **fixed in PR #53.**
- **#9 — `get_user_group` 405**: **fixed in PR #53** (handler scans
  the LIST response).
- **#10 — `set_project_memberships` PUT→PATCH + envelope**: **fixed
  in PR #53** (PATCH semantics; full project response; REPLACE
  semantics pinned in tests).
- **#11 — `create_expense` multipart**: **fixed in PR #53**
  (multipart body builder threaded through the client).
- **#12 — `create_holiday` body shape**: **fixed in PR #53**
  (`datePeriod.{startDate,endDate}` + `users.ids`/`userGroups.ids`
  + `occursAnnually`).
- **#13 — `create_custom_field` enum**: **fixed in PR #53** (enum
  widened to `{TXT, NUMBER, DROPDOWN_SINGLE, DROPDOWN_MULTIPLE,
  CHECKBOX, LINK}`).

Not changed: the binding rules in this doc (manual livee2e is **not**
launch evidence; live-contract.yml stays untouched; nothing here
ticks Group 1/6/7 boxes on the launch-candidate checklist). Unresolved
**numeric / unit questions** (invoice `unitPrice` cents, expense
`amount`/`total` scaling, expense `projectId` optional-vs-required,
shared-reports non-`SUMMARY` filter requirements) are now documented
in `docs/api-coverage.md` under "Known unresolved API contract
questions" rather than tracked here.

The "Bug inventory" and "Remaining work" sections below are
preserved verbatim as the historical campaign artifact. Treat them
as a record of what got found, not as a current task list — the
task list is in `docs/launch-candidate-checklist.md` and
`docs/api-coverage.md`.

## Branch state

- Branch tip: see `git log -1 --oneline` on `test/full-live-workspace-validation`
- Status: pushed to `origin/test/full-live-workspace-validation`
- PR: open, **draft** — do not merge until the bug inventory is
  triaged. The PR description is the canonical change summary.

## Commits (oldest → newest)

```
45df606 test(livee2e): add prefix-isolated harness for sacrificial-workspace campaign
84d63dd test(livee2e): cover the 13 Tier-1 read-only tools that lacked live evidence
5e528d2 test(livee2e): add Tier-2 read-only sweep, surfacing 6 handler/upstream shape bugs
8a31682 test(livee2e): cover expense-category create/update; pin upstream constraints and a handler bug
48f3633 test(livee2e): cover custom_fields CRUD; add archive-then-delete project cleanup helper
b98f0d8 test(livee2e): pin per-tool 404 status for shared_reports + scheduling (wrong-host blockers)
41cb749 test(livee2e): cover user_group CRUD; pin holiday handler shape bug + per-id GET 405
8b2b352 test(livee2e): cover project_admin templates/estimates/archive; pin memberships PUT 405
3de13cd test(livee2e): pin per-tool policy gate across all 5 modes via live create_client
c8e7fc6 test(livee2e): pin pagination meta envelope and seeded-set discoverability on list_tags
8af7ce5 docs(api-coverage): record live-validation campaign findings and bug inventory
6888d5b test(livee2e): require name+archived in client cleanup PUT body
```

Each commit is atomic with a `Why:` and `Verified:` trailer; drift
checks (flip-assert-red-restore-green) recorded in `Verified:` for
every non-trivial test addition per AGENTS.md:127-129.

## Live tests added

All gated by `//go:build livee2e` and live in `tests/`:

| Test file | Test func | Subtests | Notes |
|---|---|---|---|
| `tests/live_helpers_test.go` | (helpers) | – | `setupLiveCampaign`, prefix, cleanup registry, `activateTier2`, raw client primitives, archive-then-delete for projects + clients |
| `tests/e2e_live_tier1_readonly_test.go` | `TestLiveTier1ReadOnly` | 13 | All 13 previously-uncovered Tier-1 read-only tools |
| `tests/e2e_live_t2_readonly_test.go` | `TestLiveTier2ReadOnlySweep` | 22 | Read-only sweep across all 11 Tier-2 groups |
| `tests/e2e_live_t2_blocked_groups_test.go` | `TestLiveT2BlockedGroups` | 11 | shared_reports + scheduling pinned-error contracts |
| `tests/e2e_live_t2_expenses_test.go` | `TestLiveT2ExpensesCRUD` | 5 | category CRUD + 3 pinned errors |
| `tests/e2e_live_t2_custom_fields_test.go` | `TestLiveT2CustomFieldsCRUD` | 7 | seeds project; cap-skips field tests when workspace full |
| `tests/e2e_live_t2_groups_holidays_test.go` | `TestLiveT2GroupsHolidaysCRUD` | 7 | user-group CRUD; pinned-errors for `get_user_group` and `create_holiday` |
| `tests/e2e_live_t2_project_admin_test.go` | `TestLiveT2ProjectAdminCRUD` | 6 | template / estimate / archive; pinned-error for `set_project_memberships` |
| `tests/e2e_live_policy_modes_test.go` | `TestLivePolicyModes` | 5 | parametric `create_client` per policy mode |
| `tests/e2e_live_pagination_test.go` | `TestLivePaginationOnTags` | 3 | seed 11 tags + pagination meta + walk |

## Bug inventory (13 findings, each pinned as an inverted assertion)

A pinned-error assertion fails the moment its target is fixed,
forcing the fixer to delete the annotation. Each entry below names
the surfaced symptom and the most likely fix; full diagnostic context
is in `docs/api-coverage.md` "Bug inventory surfaced by the
campaign" and in the pinning subtest's inline comment.

### List-shape mismatches (handler reads `[]map[string]any` but upstream wraps)

1. **`clockify_list_invoices`, `clockify_invoice_report`** —
   upstream returns `{total, invoices:[…]}`. Pinned in
   `TestLiveTier2ReadOnlySweep`. Likely fix:
   `internal/tools/tier2_invoices.go` deserialise wrapping struct.
2. **`clockify_list_expenses`, `clockify_expense_report`** —
   upstream returns `{expenses:{expenses:[…]}}` (double-nested).
   Pinned in `TestLiveTier2ReadOnlySweep` and
   `TestLiveT2ExpensesCRUD`. Likely fix:
   `internal/tools/tier2_expenses.go`.
3. **`clockify_list_expense_categories`** — upstream returns
   `{count, categories:[…]}`. Pinned in
   `TestLiveTier2ReadOnlySweep`.
4. **`clockify_list_webhooks`** — upstream returns
   `{workspaceWebhookCount, webhooks:[…]}`. Pinned in
   `TestLiveTier2ReadOnlySweep`. Likely fix:
   `internal/tools/tier2_webhooks.go`.

### Wrong-endpoint / wrong-host (handler routes wrong)

5. **`clockify_list_webhook_events`** — handler hits
   `/workspaces/{id}/webhooks/events` but the events route is
   per-webhook (`/webhooks/{webhookId}/events`); response is 400
   "Webhook doesn't belong to Workspace". Pinned in
   `TestLiveTier2ReadOnlySweep`.
6. **All 6 `shared_reports` tools** — handler routes via
   `api.clockify.me/.../shared-reports*`; Clockify exposes shared
   reports on `reports.api.clockify.me`. Pinned in
   `TestLiveT2BlockedGroups` and `TestLiveTier2ReadOnlySweep`.
   Likely fix: thread reports base URL through
   `internal/clockify/client.go`.
7. **All 10 `scheduling` tools** — same root-cause class:
   scheduling lives on a separate Clockify scheduling host. Pinned
   in `TestLiveT2BlockedGroups` and `TestLiveTier2ReadOnlySweep`.

### Method / verb mismatches

8. **`clockify_list_time_off_requests`** — handler GETs
   `/time-off/requests` but upstream returns 405 "Request method
   'GET' is not supported"; the endpoint requires a POST search
   body. Pinned in `TestLiveTier2ReadOnlySweep`.
9. **`clockify_get_user_group`** — upstream returns 405 on the
   per-id GET; only mutating verbs are supported. Pinned in
   `TestLiveT2GroupsHolidaysCRUD`. Likely fix: scan LIST response.
10. **`clockify_set_project_memberships`** — handler PUTs to
    `/projects/{id}/memberships` but upstream returns 405; v1 API
    uses PATCH or a different subroute. Pinned in
    `TestLiveT2ProjectAdminCRUD`.

### Content-type / body-shape mismatches

11. **`clockify_create_expense`** — handler POSTs
    `application/json` but upstream rejects with 415; expenses
    require `multipart/form-data` (verified by direct curl probe).
    Pinned in `TestLiveT2ExpensesCRUD`. Likely fix: thread a
    multipart body through `internal/clockify/client.go`.
12. **`clockify_create_holiday`** — handler sends `{name, date,
    recurring?}` flat; upstream wants nested
    `datePeriod:{startDate, endDate}` plus `userIds`/`userGroupIds`.
    Pinned in `TestLiveT2GroupsHolidaysCRUD`.

### Descriptor drift

13. **`clockify_create_custom_field`** — descriptor advertises
    "TEXT, NUMBER, DROPDOWN, CHECKBOX, LINK"; upstream enum is
    `{TXT, NUMBER, DROPDOWN_SINGLE, DROPDOWN_MULTIPLE, CHECKBOX,
    LINK}`. Detected during `TestLiveT2CustomFieldsCRUD` work; the
    test sends `TXT` to work around. Likely fix: descriptor
    docstring update or handler-side translation.

## Tests that currently pass (success path, against the sacrificial workspace)

Run: `go test -tags=livee2e -count=1 -timeout 10m ./tests/...`
(with the env file sourced — see below). Wall-clock 18.4 s.

- `TestE2EReadOnly` (existing)
- `TestE2EErrors` (existing)
- `TestE2EMutating` (existing)
- `TestLiveDryRunDoesNotMutate` (existing)
- `TestLivePolicyTimeTrackingSafeBlocksProjectCreate` (existing)
- `TestLiveReadSideSchemaDiff` (existing)
- `TestLiveTier1ReadOnly` (NEW — all 13 subtests)
- `TestLiveTier2ReadOnlySweep` (NEW — 22 subtests, mixed success/pinned)
- `TestLiveT2BlockedGroups` (NEW — 11 pinned-error subtests)
- `TestLiveT2ExpensesCRUD` (NEW — 5 subtests, mixed)
- `TestLiveT2CustomFieldsCRUD` (NEW — 7 subtests; cap-skips on this workspace until pruned)
- `TestLiveT2GroupsHolidaysCRUD` (NEW — 7 subtests, mixed)
- `TestLiveT2ProjectAdminCRUD` (NEW — 6 subtests, mixed)
- `TestLivePolicyModes` (NEW — all 5 mode sub-cases)
- `TestLivePaginationOnTags` (NEW — 3 subtests)
- `TestLiveContractSkipSentinel` (existing — no-skip guarantee)

## Cleanup / orphan state

After the post-campaign sweep:

| Entity | Orphans | Notes |
|---|---|---|
| Clients | 0 | All swept via name+archived PUT → DELETE |
| Projects (active + archived) | 0 | `rawArchiveAndDeleteProject` works |
| Tags | 0 | DELETE accepted directly |
| User groups | 0 | DELETE accepted directly |
| Holidays | 0 | (no holidays got created — `create_holiday` is broken) |
| Expense categories | **7** | Upstream requires archival before delete; archive flag is not writable via API on this Clockify version. Documented in `docs/api-coverage.md` as a known workspace-state limitation. Names: `mcp-live-*-exp-cat-0[-renamed]`, plus `mcp-live-probe-cat-archived`. Maintainer must clean these via the Clockify UI. |

## Remaining work

### Convert pinned-error tests into success-path tests

Each bug listed above keeps a pinned-error assertion. When a fix
lands, the assertion will fire (because the error string disappears)
and the fixer must delete the annotation. The work, in priority order:

1. **Fix the 4 list-shape mismatches** (#1–#4 above). Each is a
   surgical handler change in `internal/tools/tier2_*.go`. After the
   fix, replace the `expectErr` annotation in
   `TestLiveTier2ReadOnlySweep` with a real shape assertion (e.g.,
   "data is a slice of N elements"). Recommend grouping all 4 fixes
   into one PR since they're the same shape problem.
2. **Fix `clockify_list_webhook_events`** (#5). Reroute handler URL
   to `/webhooks/{webhookId}/events`; the descriptor will need a
   `webhook_id` argument since the resource is per-webhook now.
3. **Decide architecture for shared_reports + scheduling** (#6, #7).
   These are wider changes — they need a second base URL threaded
   through `internal/clockify.Client`. Do this in its own design
   discussion + ADR.
4. **Fix the 3 method-mismatch handlers** (#8, #9, #10). Each is a
   surgical handler change. After fix, swap the pinned-error
   contract for a success-path assertion in the matching test.
5. **Fix `clockify_create_expense`** (#11). Multipart body builder
   in `internal/clockify/client.go`; non-trivial. After fix, the
   downstream `TestLiveT2ExpensesCRUD` tests can be extended to a
   full create → get → update → delete cycle (the test is currently
   blocked by this bug).
6. **Fix `clockify_create_holiday`** (#12). Reshape body to wrap
   datePeriod and provide sensible defaults.
7. **Fix `clockify_create_custom_field` descriptor** (#13). Update
   descriptor enum docstring or add handler translation; remove the
   TXT-vs-TEXT comment in `TestLiveT2CustomFieldsCRUD` once done.

### Workspace state work (no code change required)

8. **Prune the 50 existing custom fields** in the sacrificial
   workspace so `TestLiveT2CustomFieldsCRUD` exercises a real CRUD
   path. The test is currently cap-skip-tolerant.
9. **Manually delete the 7 orphan expense categories** via the
   Clockify UI (archive in UI → DELETE accepted). Names are listed
   above.

### Future test additions (not blockers, just gaps)

10. Live tests for the 8 remaining Tier-1 tools that still lack
    coverage: `clockify_log_time`, `clockify_update_entry`,
    `clockify_find_and_update_entry`, `clockify_switch_project`,
    `clockify_create_task`, plus the rest. Each is a small
    additive test mirroring the existing patterns.
11. Approvals CRUD test (Phase 11 was skipped). Requires a
    timesheet to operate on. Sends external email even on the
    sacrificial workspace; should gate behind a future
    external-side-effects env var (see reserved gate names below).
12. Webhook CRUD (Phase 13 was skipped). Even though
    `list_webhooks` is broken, `create_webhook` against
    `https://example.com/...` may work if URL validation passes.
    The campaign safety rule prefers to skip this since we don't
    control the destination.
13. Time-off policy + request CRUD (Phase 10 was skipped). Same
    email-side-effect reasoning.

### Workflow integration (separate review)

14. **Do not** extend `.github/workflows/live-contract.yml`'s
    `-run` regex without a separate cron-blast-radius review. The
    new tests are local-only / manual-dispatch-only by design.
    Adding any of them to the cron must consider the surface they
    exercise (some surfaces would mail nightly).
15. **Do not** mark this work as Group 1 / 6 / 7 evidence on
    `docs/launch-candidate-checklist.md`. AGENTS.md:114-118 binds:
    no launch-readiness claim until scheduled cron + candidate-tag
    security walk-through + release/sigstore/SLSA evidence
    coexist. None of those exist here.

## Re-running locally

The campaign uses a 600-permission env file at
`/tmp/clockify-livetest.env`. The file's variable names are public
(below); the API key is not committed and must be re-supplied by the
operator. The variable list:

Variables consumed by the test code today (gates that the harness
or per-domain tests read directly):

```
CLOCKIFY_RUN_LIVE_E2E=1
CLOCKIFY_API_KEY=<REDACTED — sacrificial-workspace key>
CLOCKIFY_WORKSPACE_ID=<workspace id, see api-coverage.md>
CLOCKIFY_LIVE_WORKSPACE_CONFIRM=<must equal CLOCKIFY_WORKSPACE_ID>
CLOCKIFY_LIVE_WRITE_ENABLED=true
CLOCKIFY_LIVE_FULL_SURFACE_ENABLED=true
CLOCKIFY_LIVE_ADMIN_ENABLED=true
CLOCKIFY_LIVE_BILLING_ENABLED=true
CLOCKIFY_LIVE_SETTINGS_ENABLED=true
```

Reserved gate names for deferred phases (defined in the original
campaign plan, present in the env file, but not yet read by any test
because their phases were intentionally skipped — see "Future test
additions" above): a webhook-registration gate (Phase 13 — webhooks)
and an external-side-effects gate (Phases 10/11 — time_off and
approvals which trigger email). When those phases land, the tests
will start gating on them and they should be added to this list.

The `CLOCKIFY_LIVE_WORKSPACE_CONFIRM` second-factor check is a
deliberate defense against a misconfigured shell mutating a wrong
workspace — the harness `t.Fatal`s if it doesn't equal
`CLOCKIFY_WORKSPACE_ID` exactly.

Run sequence (from inside `go-clockify/`):

```sh
source /tmp/clockify-livetest.env
go test -tags=livee2e -count=1 -timeout 10m ./tests/...
```

Narrow re-runs after a handler fix:

```sh
# After fixing list-shape mismatches:
go test -tags=livee2e -count=1 -run '^TestLiveTier2ReadOnlySweep$' ./tests/...

# After fixing create_expense multipart bug:
go test -tags=livee2e -count=1 -run '^TestLiveT2ExpensesCRUD$' ./tests/...

# After any test edit, before commit:
make check
make doc-parity
make config-doc-parity
make catalog-drift
git diff --check
```

## What this work does NOT do

- It does NOT close any box on `docs/launch-candidate-checklist.md`
  Group 1, 6, or 7. The launch-evidence-gate
  (`scripts/check-launch-evidence-gate.sh`) is satisfied because
  no such box was ticked.
- It does NOT extend `.github/workflows/live-contract.yml`. The
  cron's `-run` regex is anchored and the new tests stay
  local-only / manual-dispatch-only by design (cron blast radius).
- It does NOT relax any policy or dry-run default. AGENTS.md:119-124
  is binding.
- It does NOT fix the bugs it pinned. Each fix is a separate change
  per the priority list above.

## Files to read first if you're picking this up

1. `docs/api-coverage.md` — campaign findings and bug inventory
2. `tests/live_helpers_test.go` — campaign harness (~250 lines)
3. `tests/e2e_live_t2_readonly_test.go` — 22-tool sweep (good
   starting point for understanding the pinned-error pattern)
4. The PR description on https://github.com/apet97/go-clockify/pull/53
5. AGENTS.md — binding rules that this work respected
