# Live-Validation Campaign — Continuation Handoff

Date: 2026-05-02 (status note added 2026-05-03)
Branch: `test/full-live-workspace-validation` (12 commits ahead of `main`,
pushed to `origin`)
Draft PR: https://github.com/apet97/go-clockify/pull/53

This doc tells the next agent (or maintainer) exactly what state the
live-validation campaign is in, what tests pass, what bugs were
surfaced, what's left to do, and how to re-run everything locally.

## Status update — 2026-05-03 (post-PR #53–#58 and exhaustive follow-up)

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
- **#7 — scheduling 10 tools "wrong host"**: **fixed.**
  PR #53 fixed `list_assignments` (`/all` suffix + `start`/`end`).
  PR #55 repointed `filter_schedule_capacity` to per-user totals
  and removed the phantom `list_schedules` tool. The 2026-05-03
  cleanup removed the matching phantom `get_` and `create_`
  schedule tools (no `/scheduling/{id}` or `POST /scheduling`
  surface exists). The four assignment-CRUD tools now use the
  documented recurring-assignment routes under
  `/scheduling/assignments/recurring`; `clockify_get_assignment`
  scans the supported `/assignments/all` date-range endpoint because
  Clockify exposes no single-assignment GET route.
- **#8 — `list_time_off_requests` GET→POST**: **fixed in PR #53.**
- **#9 — `get_user_group` 405**: **fixed** (handler scans the LIST
  response).
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
**numeric / unit questions** (invoice `unitPrice` raw minor units,
expense raw amount/total pass-through, expense `projectId`
optional-vs-required, shared-reports non-`SUMMARY` filter
requirements) are now documented in `docs/api-coverage.md` under
"Known API contract notes" rather than tracked here.

The "Bug inventory" and "Remaining work" sections below are
preserved verbatim as the historical campaign artifact. Treat them
as a record of what got found, not as a current task list — the
task list is in `docs/launch-candidate-checklist.md` and
`docs/api-coverage.md`.

Follow-up branch `test/exhaustive-live-coverage-followup` extends the
manual sacrificial-workspace suite so every one of the 121 generated
catalog tools is named in `tests/e2e_live*.go` and exercised through
the MCP path. New coverage includes the remaining Tier-1 CRUD/logging
tools, full invoice CRUD/report/item probes, shared-report
create/update/export/delete, user-admin group operations with owner
safety dry-runs, webhook CRUD using the live `webhookEvent` body
shape, time-off request/policy/balance probes, and approvals
period-submit/get/status probes. Some of these are deliberately
asserted as upstream 4xx / permission / unsupported-route outcomes
rather than success paths; see `docs/api-coverage.md` for the current
evidence table.

## Branch state

- Historical branch tip: see `git log -1 --oneline` on
  `test/full-live-workspace-validation`.
- Status: superseded by merged PRs #53-#57 and the follow-up fixes
  recorded in `docs/api-coverage.md`.
- Current task list: use `docs/api-coverage.md` and
  `docs/launch-candidate-checklist.md`, not the historical
  "Remaining work" section below.

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
| `tests/e2e_live_t2_scheduling_test.go` | `TestLiveT2SchedulingRecurringCRUD` | 1 flow | recurring assignment create/get/update/delete on the live routes |
| `tests/e2e_live_t2_expenses_test.go` | `TestLiveT2ExpensesCRUD` | 5 | category CRUD + expense create; category delete archive constraint remains pinned |
| `tests/e2e_live_t2_custom_fields_test.go` | `TestLiveT2CustomFieldsCRUD` | 7 | seeds project; cap-skips field tests when workspace full |
| `tests/e2e_live_t2_groups_holidays_test.go` | `TestLiveT2GroupsHolidaysCRUD` | 7 | user-group CRUD + holiday create/delete path |
| `tests/e2e_live_t2_project_admin_test.go` | `TestLiveT2ProjectAdminCRUD` | 6 | template / estimate / memberships / archive |
| `tests/e2e_live_policy_modes_test.go` | `TestLivePolicyModes` | 5 | parametric `create_client` per policy mode |
| `tests/e2e_live_pagination_test.go` | `TestLivePaginationOnTags` | 3 | seed 11 tags + pagination meta + walk |
| `tests/e2e_live_tier1_remaining_crud_test.go` | `TestLiveTier1RemainingCRUD` | 1 flow | remaining Tier-1 CRUD/log/search coverage |
| `tests/e2e_live_t2_invoices_test.go` | `TestLiveT2InvoicesCRUD` | 1 flow | all invoice tools; update item pinned as unsupported 405 |
| `tests/e2e_live_t2_shared_reports_test.go` | `TestLiveT2SharedReportsCRUDAndExports` | 1 flow | SUMMARY CRUD/export plus conditional DETAILED/WEEKLY export |
| `tests/e2e_live_t2_user_admin_test.go` | `TestLiveT2UserAdminCRUDAndOwnerSafety` | 1 flow | user-admin group CRUD, membership add/remove, owner dry-run safety |
| `tests/e2e_live_t2_webhooks_test.go` | `TestLiveT2WebhooksCRUD` | 1 flow | webhook CRUD using live singular-event contract |
| `tests/e2e_live_t2_time_off_approvals_test.go` | `TestLiveT2TimeOffRemainingTools`, `TestLiveT2ApprovalsRemainingTools` | 2 | time-off and approvals probes with success and concrete 4xx outcomes |

## Historical bug inventory (13 findings, now closed or documented)

The original campaign pinned these findings as inverted assertions.
Those annotations have since been replaced by success-path coverage or
by explicit upstream/workspace-state limitations. Keep this section as
provenance for the fixes; use `docs/api-coverage.md` for current
coverage status.

### List-shape mismatches (handler reads `[]map[string]any` but upstream wraps)

1. **`clockify_list_invoices`, `clockify_invoice_report`** —
   upstream returns `{total, invoices:[…]}`. Closed by envelope
   handling in `internal/tools/tier2_invoices.go`.
2. **`clockify_list_expenses`, `clockify_expense_report`** —
   upstream returns `{expenses:{expenses:[…]}}` (double-nested).
   Closed by nested-envelope handling in `internal/tools/tier2_expenses.go`.
3. **`clockify_list_expense_categories`** — upstream returns
   `{count, categories:[…]}`. Closed by category envelope handling.
4. **`clockify_list_webhooks`** — upstream returns
   `{workspaceWebhookCount, webhooks:[…]}`. Closed by webhook
   envelope handling.

### Wrong-endpoint / wrong-host (handler routes wrong)

5. **`clockify_list_webhook_events`** — handler hits
   `/workspaces/{id}/webhooks/events` but the events route is
   per-webhook (`/webhooks/{webhookId}/events`); response is 400
   "Webhook doesn't belong to Workspace". Closed by returning the
   static workspace event enum.
6. **All 6 `shared_reports` tools** — handler routes via
   `api.clockify.me/.../shared-reports*`; Clockify exposes shared
   reports on `reports.api.clockify.me`. Closed by the reports-host
   client path and live request-shape fixes.
7. **Scheduling assignment tools** — original handlers targeted
   unsupported assignment paths and phantom schedule routes. Closed
   by using live `/assignments/all` list/totals routes, removing
   phantom schedule tools, and routing assignment CRUD through
   recurring-assignment endpoints.

### Method / verb mismatches

8. **`clockify_list_time_off_requests`** — handler GETs
   `/time-off/requests` but upstream returns 405 "Request method
   'GET' is not supported"; the endpoint requires a POST search
   body. Closed by POST search handling.
9. **`clockify_get_user_group`** — upstream returns 405 on the
   per-id GET; only mutating verbs are supported. Closed by scanning
   the supported list response.
10. **`clockify_set_project_memberships`** — handler PUTs to
    `/projects/{id}/memberships` but upstream returns 405; v1 API
    uses PATCH replace semantics. Closed by switching to PATCH and
    reading memberships from the full project response.

### Content-type / body-shape mismatches

11. **`clockify_create_expense`** — handler POSTs
    `application/json` but upstream rejects with 415; expenses
    require `multipart/form-data` (verified by direct probe).
    Closed by threading multipart body support through the client.
12. **`clockify_create_holiday`** — handler sends `{name, date,
    recurring?}` flat; upstream wants nested
    `datePeriod:{startDate, endDate}` plus `userIds`/`userGroupIds`.
    Closed by using `datePeriod`, `occursAnnually`, and required
    assignment envelopes.

### Descriptor drift

13. **`clockify_create_custom_field`** — descriptor advertises
    "TEXT, NUMBER, DROPDOWN, CHECKBOX, LINK"; upstream enum is
    `{TXT, NUMBER, DROPDOWN_SINGLE, DROPDOWN_MULTIPLE, CHECKBOX,
    LINK}`. Closed by advertising the live enum values.

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
- `TestLiveTier2ReadOnlySweep` (NEW — 22 subtests)
- `TestLiveT2SchedulingRecurringCRUD` (NEW — recurring assignment CRUD)
- `TestLiveT2ExpensesCRUD` (NEW — 5 subtests, mixed)
- `TestLiveT2CustomFieldsCRUD` (NEW — 7 subtests; cap-skips on this workspace until pruned)
- `TestLiveT2GroupsHolidaysCRUD` (NEW — 7 subtests)
- `TestLiveT2ProjectAdminCRUD` (NEW — 6 subtests)
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
| Holidays | 0 | Create/delete path live-tested; cleanup uses direct DELETE |
| Expense categories | **7** | Upstream requires archival before delete; archive flag is not writable via API on this Clockify version. Documented in `docs/api-coverage.md` as a known workspace-state limitation. Names: `mcp-live-*-exp-cat-0[-renamed]`, plus `mcp-live-probe-cat-archived`. Maintainer must clean these via the Clockify UI. |

## Historical remaining work (superseded)

### Closed handler work

The numbered handler bugs above are no longer pending. Current live
coverage gaps, raw numeric-unit notes, and upstream
workspace-state limitations are tracked in `docs/api-coverage.md`.

### Workspace state work (no code change required)

8. **Prune the 50 existing custom fields** in the sacrificial
   workspace so `TestLiveT2CustomFieldsCRUD` exercises a real CRUD
   path. The test is currently cap-skip-tolerant.
9. **Manually delete the 7 orphan expense categories** via the
   Clockify UI (archive in UI → DELETE accepted). Names are listed
   above.

### Exhaustive follow-up coverage (landed after the historical campaign)

10. Remaining Tier-1 CRUD/logging/search tools are covered by
    `TestLiveTier1RemainingCRUD`.
11. Invoice CRUD, invoice items, send dry-run, mark-paid dry-run and
    real mark-paid, and invoice reports are covered by
    `TestLiveT2InvoicesCRUD`.
12. Shared-report SUMMARY create/update/export/delete and conditional
    DETAILED/WEEKLY export probes are covered by
    `TestLiveT2SharedReportsCRUDAndExports`.
13. User-admin user-group CRUD, membership add/remove, owner
    deactivate dry-run, and update-role unsupported-route evidence
    are covered by `TestLiveT2UserAdminCRUDAndOwnerSafety`.
14. Webhook CRUD is covered by `TestLiveT2WebhooksCRUD`; the handler
    now uses `webhookEvent`, `triggerSourceType`, and `triggerSource`
    instead of the old plural `events` array.
15. Time-off policy/request/balance/status tools and approvals tools
    are covered by `TestLiveT2TimeOffRemainingTools` and
    `TestLiveT2ApprovalsRemainingTools`, with permission and
    unsupported-route responses pinned where the sacrificial
    workspace rejects the operation.

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

The original campaign reserved separate webhook-registration and
external-side-effect gate names for deferred phases. The exhaustive
follow-up instead uses the existing explicit full-surface/admin/billing
gates plus `CLOCKIFY_LIVE_WORKSPACE_CONFIRM`; keep any future cron
promotion behind a separate blast-radius review.

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
- The historical pinned bugs are now tracked as closed or as explicit
  upstream/workspace-state limitations in `docs/api-coverage.md`.

## Files to read first if you're picking this up

1. `docs/api-coverage.md` — campaign findings and bug inventory
2. `tests/live_helpers_test.go` — campaign harness (~250 lines)
3. `tests/e2e_live_t2_readonly_test.go` — 22-tool sweep
4. The PR description on https://github.com/apet97/go-clockify/pull/53
5. AGENTS.md — binding rules that this work respected
