# Live Tests

Live tests run against a sacrificial Clockify workspace and perform real API
calls. They do not use dry runs. Never point them at a personal or production
workspace.

## Required Environment

Set these for every live run:

```sh
export CLOCKIFY_API_KEY='...'
export CLOCKIFY_WORKSPACE_ID='...'
export CLOCKIFY_RUN_LIVE_E2E=1
export CLOCKIFY_LIVE_PREFIX='MCP-LIVE-YYYYMMDD'
```

`CLOCKIFY_LIVE_PREFIX` should be unique per run so created objects are easy to
find and clean up.

Optional one-user probes use additional gates:

```sh
export CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1
export CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS=1
export CLOCKIFY_LIVE_HAPPY_PATH_CAMPAIGNS=1
```

The build-tagged optional domain campaign also requires an explicit workspace
confirmation before mutating broad domain surfaces:

```sh
export CLOCKIFY_LIVE_WORKSPACE_CONFIRM="$CLOCKIFY_WORKSPACE_ID"
```

Some optional campaign categories still require their own blast-radius gates,
such as `CLOCKIFY_LIVE_ADMIN_ENABLED=true`,
`CLOCKIFY_LIVE_BILLING_ENABLED=true`, and
`CLOCKIFY_LIVE_SETTINGS_ENABLED=true`.
`CLOCKIFY_LIVE_HAPPY_PATH_CAMPAIGNS=1` is only for paid-feature happy-path
campaigns that create real invoices, expenses, time-off records, scheduling
assignments, and webhooks in the confirmed sacrificial workspace.

## What To Run

Compile the build-tagged live suite without touching Clockify:

```sh
go test -tags=livee2e -count=0 ./tests/...
```

Run the main one-user MCP live contracts:

```sh
go test -tags=livee2e -count=1 -timeout 5m \
  -run '^(TestLiveOneUserWorkflowMCP|TestLiveRawClockifyReadSideSchemaDiff)$' \
  ./tests/...
```

Run the internal one-user live workflow probes:

```sh
go test -count=1 -timeout 10m ./internal/tools -run '^TestOneUserLive'
```

Or use the Makefile wrapper after the required environment is set:

```sh
make live-contract-local
```

## Nightly drift detection

`.github/workflows/nightly-live.yml` runs `make perfect-live` at 07:00 UTC
every day. The job uses the same sacrificial workspace credentials as local
live tests. Test-created data uses a run-specific
`MCP-LIVE-CI-${run_id}-` prefix, and the unconditional cleanup step sweeps the
broader `MCP-LIVE-CI-` prefix so stale CI leftovers do not accumulate.

On failure, the workflow opens a single rolling GitHub Issue titled
"Nightly live drift" (label: `drift`) with the run URL and the last 80 lines
of redacted output. The same issue is updated on subsequent failures and
auto-closed on the next green run.

To trigger a manual run: GitHub Actions -> "nightly-live" -> "Run workflow".

This workflow is **not** on the branch-protection required-checks list; it is a
continuous-detection signal, not a PR gate.

## Contract Coverage

| Test | What it proves |
|---|---|
| `TestLiveOneUserWorkflowMCP` | The MCP path initializes and calls workflow tools through the same one-user registry used by `cmd/clockify-mcp`. |
| `TestLiveRawClockifyReadSideSchemaDiff` | Live read-side Clockify responses still match the structs in `internal/clockify`. |
| `TestOneUserLiveWorkflow` | The internal one-user service can seed, log, start, switch, stop, fix, review, and clean up work in the configured workspace. |
| `TestOneUserLivePaidFeatureWorkflowRecovery` | Paid or high-risk workflow tools return useful success/recovery envelopes under `CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS=1`. |
| `TestOneUserLiveOptionalDomainContracts` | Optional domain tools return stable success/recovery envelopes under `CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1`. |
| `TestLiveOneUserPaidFeatureHappyPaths` | Paid-feature domain tools complete real happy paths when the workspace has the relevant features and every happy-path gate is enabled. |

## Skip Behavior

Live tests skip when required environment is missing. A skipped run is not live
evidence. The `livee2e` package includes a sentinel so an unfiltered tagged run
cannot quietly report success when every live test skipped.

## Sacrificial Workspace

Use a workspace reserved only for these tests:

1. Create a fresh Clockify workspace with no customer data.
2. Generate an API key for that workspace.
3. Store the key and workspace id only in local shell state or CI secrets.
4. Rotate the key every 90 days, and immediately after any auth-related live
   test failure.

When a live run fails, keep the prefixed objects in place until the failure has
been inspected. Cleanup can remove useful evidence too early.

## Recorded Runs

Live evidence is captured per run so reviewers can see when each test family
last passed, against which workspace shape, and under which gates. Workspace
identifiers are intentionally omitted — we record only the visible plan from
`doctor --live` and a non-identifying description.

| Date (UTC) | Commit | Workspace plan | Gates | Tests | Result | Prefix-object leftovers |
|---|---|---|---|---|---|---|
| 2026-05-22 | tested `d54d6ca6d3aa` | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1`, `CLOCKIFY_LIVE_WORKSPACE_CONFIRM=$CLOCKIFY_WORKSPACE_ID` | `make perfect-live`: core (`TestLiveOneUserWorkflowMCP`, `TestLiveRawClockifyReadSideSchemaDiff`), `internal/tools` `TestOneUserLive*`, optional-domain `TestLive*` campaign, then `make live-clean-prefix` | PASS (31.055s core `tests`, 59.624s `internal/tools`, 57.192s optional `tests`); first attempt exposed transient empty running-timer reads after `clockify_switch_work`, fixed by stop-timer retry and redacted live ID assertions | 0 prefix-object leftovers — `live-clean-prefix` deleted 12 objects, failed 0, post-delete prefix-object rescan reported `Leftovers: 0`; the failed pre-fix prefix was also swept separately (12 deleted, failed 0, leftovers 0) |
| 2026-05-15 | historical | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1`, `CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS=1`, `CLOCKIFY_LIVE_HAPPY_PATH_CAMPAIGNS=1`, `CLOCKIFY_LIVE_WORKSPACE_CONFIRM=$CLOCKIFY_WORKSPACE_ID` | `TestOneUserLiveWorkflow`, `TestOneUserLivePaidFeatureWorkflowRecovery`, `TestOneUserLiveOptionalDomainContracts`, `TestLiveOneUserWorkflowMCP` | PASS (42.7s `internal/tools`, 29.0s `tests`) | 0 (`clockify_demo_cleanup` returned ok) |
| 2026-05-15 | historical | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1`, `CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS=1`, `CLOCKIFY_LIVE_WORKSPACE_CONFIRM=$CLOCKIFY_WORKSPACE_ID` | core (`TestLiveOneUserWorkflowMCP`, `TestLiveRawClockifyReadSideSchemaDiff`), `internal/tools` `TestOneUserLive*`, optional-domain `TestLive*` campaign | PASS per sub-suite (21.3s core, 19.0s internal/tools, 39.1s optional) | 0 prefix-object leftovers after prefix-scoped sweep (48 tags + 67 clients from this session's runs archived+deleted; `clockify_demo_cleanup` plus a manual `mcp-live*`/`mcp-remaining*`/`mcp-optional*` audit confirmed 0 prefix-object leftovers) |
| 2026-05-16 | historical | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1` | `TestOneUserLiveOptionalDomainContracts` (extended to cover the new `clockify_audit_logs_search` and `clockify_entity_changes_list`) | PASS (12.8s `internal/tools`) | prefixed `mcp-*` optional-domain objects left by the contracts test; the audit-log and entity-changes probes are read-only and create nothing |
| 2026-05-16 | historical | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_PREFIX=MCP-PR110-*`, `CLOCKIFY_LIVE_WORKSPACE_CONFIRM=$CLOCKIFY_WORKSPACE_ID` | post-#110 verification - `TestLiveOneUserWorkflowMCP`, `TestOneUserLiveWorkflow`, plus targeted MCP-server tool calls (`clockify_reports_money`, `clockify_reports_export`, `clockify_status`) against the merged `main` binary | PASS (25.7s `tests`, 11.5s `internal/tools`); `clockify_reports_money` returned `ok:true` with normalized totals, confirming the PR #110 fix of the prior route-tool descriptor failure | 0 prefix-object leftovers after prefix-scoped sweep - the two workflow tests left 2 clients + 2 projects (+2 tasks) prefixed `MCP-PR110-20260516220202`; archived and deleted, and `clockify_clients_list`/`clockify_projects_list` with `name=MCP-PR110` then confirmed 0 prefix-object leftovers |
| 2026-05-17 | historical | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_API_KEY`, `CLOCKIFY_WORKSPACE_ID`, `CLOCKIFY_MAX_TOOL_RESULT_BYTES=50000`; direct local MCP stdio probe, read-only only | targeted overflow regression: `clockify_projects_list`, `clockify_time_off_policies_list`, `clockify_expenses_list`, `clockify_invoices_info`, `clockify_invoices_list`, `clockify_review_week` (`include_entries:true`) | PASS; all returned `ok:true` below 50 KB (`8662`, `7106`, `10687`, `15520`, `15520`, `36708` envelope bytes); `clockify_review_week` set `meta.truncated:true` | 0; read-only probe created no objects |
| 2026-05-18 | historical | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1`, `CLOCKIFY_LIVE_PREFIX=MCP-T4-20260518*` | `TestOneUserLiveOptionalDomainContracts` - exercises the `clockify_invoices_info` and `clockify_scheduling_publish` (dry-run) probes alongside the optional-domain surface | PASS (21.777s `internal/tools`); direct MCP confirmation: `clockify_invoices_info` returned `ok:true` (161 invoices, paged `total`/`has_more`), `clockify_scheduling_publish` dry-run returned `ok:true` ("No changes were made.") | optional-domain contract test leaves prefixed `MCP-T4-*` objects (clients/projects/tasks/tags/invoice/expense/time-off/assignment/group/holiday/webhook) in the sacrificial workspace; the `invoices_info` read and `scheduling_publish` dry-run created nothing |
| 2026-05-17 | historical | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_API_KEY`, `CLOCKIFY_WORKSPACE_ID`; live MCP stdio sweep of the rebuilt post-test-report-fix binary | json.Number/contract fix verification - `clockify_projects_list` (`page_size:3` honored), `clockify_expenses_categories_create`+`_delete` (`price_in_cents:1850` lands, not 0), `clockify_schedule_work` (upfront `needs a user` error), `clockify_invoices_payments_delete` (`dry_run:true` preview, no mutation), `clockify_entries_list` (same-day `2026-05-17` range returns 26 entries) | PASS; every fix confirmed on the live stdio wire (`json.Number` numeric args, schema/runtime parity, same-day range) | 0 (`MCPFIX-numcheck-20260517` category created then deleted) |
| 2026-05-18 | tested `5c7e92bd2e58`, merged `cb470725` (#131) | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1`, `CLOCKIFY_LIVE_WORKSPACE_CONFIRM=$CLOCKIFY_WORKSPACE_ID` | `make perfect-live`: core (`TestLiveOneUserWorkflowMCP`, `TestLiveRawClockifyReadSideSchemaDiff`), `internal/tools` `TestOneUserLive*`, optional-domain `TestLive*` campaign, then `make live-clean-prefix` | PASS (36.965s core `tests`, 57.809s `internal/tools`, 70.178s optional `tests`) | 0 prefix-object leftovers after `live-clean-prefix`; deleted 11 objects, failed 0, post-delete prefix-object rescan reported `Leftovers: 0` |
| 2026-05-19 | branch `guardrails-and-live-cleanup` (pre-merge) | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1`, `CLOCKIFY_LIVE_WORKSPACE_CONFIRM=$CLOCKIFY_WORKSPACE_ID` | `make perfect-live`: core (`TestLiveOneUserWorkflowMCP`, `TestLiveRawClockifyReadSideSchemaDiff`), `internal/tools` `TestOneUserLive*` incl. the new `TestOneUserLiveTimerHappyPaths`, optional-domain `TestLive*` campaign, then `make live-clean-prefix` | PASS (41.0s core `tests`, 64.1s `internal/tools`, 72.3s optional `tests`); `TestOneUserLiveTimerHappyPaths` drove `clockify_entries_running`/`timer_start`/`timer_status`/`timer_switch`/`timer_stop` and `clockify_projects_memberships_list` to `ok:true` | 0 prefix-object leftovers — `live-clean-prefix` deleted 13 objects, failed 0, post-delete prefix-object rescan reported `Leftovers: 0`; the scheduling assignment was matched by its linked prefixed project and deleted via the recurring-assignment route. A follow-up `CLOCKIFY_LIVE_PREFIX=MCP-LIVE-` prefix-object audit sweep cleared 249 objects accumulated from earlier sessions (104 projects, 45 invoices, 35 clients, 18 scheduling assignments, 16 holidays, 16 user-groups, 14 tags, 1 webhook), failed 0, `Leftovers: 0` prefix-object leftovers |
| 2026-05-19 | `v0.2.0` release (code tested at `239f85b`) | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1`, `CLOCKIFY_LIVE_WORKSPACE_CONFIRM=$CLOCKIFY_WORKSPACE_ID` | `make perfect-live` for the `v0.2.0` release: core (`TestLiveOneUserWorkflowMCP`, `TestLiveRawClockifyReadSideSchemaDiff`), `internal/tools` `TestOneUserLive*`, optional-domain `TestLive*` campaign, then `make live-clean-prefix` | PASS (38.1s core `tests`, 64.1s `internal/tools`, 60.7s optional `tests`) | 0 prefix-object leftovers — `live-clean-prefix` deleted 13 objects, failed 0, post-delete prefix-object rescan reported `Leftovers: 0` |

The 2026-05-18 run was executed against PR #131 head `5c7e92bd2e58` and squash-merged as
`cb470725db340949fe760399d20e67955075e4a7`. The only difference between the tested tree
and the merge commit is this `docs/live-tests.md` evidence row itself, written after the
run — there are no code changes between them. The 2026-05-19 `v0.2.0` row was run against
`239f85b`; the `v0.2.0` release commit adds only the `CHANGELOG.md` / `docs/live-tests.md`
update on top, so the tagged tree's code is identical to the tested tree.

`make live-contract-local` runs all three sub-suites back-to-back; on a
shared workspace that can trip Clockify's rate budget and surface a
transient `http2: timeout` mid-suite (the MCP returns a clean `ok:false`
recovery envelope in that case). Run the three sub-suites individually
for clean rate budgets when the combined target flakes.

When recording a new run, include the date, the visible workspace plan (from
`doctor --live`), the env-var set that was active, the named test functions,
the pass/fail result, and a prefix-object leftover count from
`clockify_demo_cleanup` plus any prefix-object audit.

## Cleaning up after live runs

`TestOneUserLiveOptionalDomainContracts` intentionally leaves the objects it creates
(clients, projects, tasks, tags, an invoice, an expense, a time-off request, a
scheduling assignment, a group, a holiday, a webhook) in the sacrificial workspace so a
failed run can be inspected.

**If a live run fails, do not clean immediately. Inspect the leftover objects first.**
Once you are done inspecting, sweep them with:

```sh
export CLOCKIFY_API_KEY=...           # sacrificial workspace key
export CLOCKIFY_WORKSPACE_ID=...      # sacrificial workspace id
export CLOCKIFY_LIVE_PREFIX=MCP-T4-... # the prefix the failed run used
export CLOCKIFY_LIVE_WORKSPACE_CONFIRM="$CLOCKIFY_WORKSPACE_ID"
make live-clean-prefix
```

`make live-clean-prefix` refuses to run unless `CLOCKIFY_LIVE_WORKSPACE_CONFIRM`
matches `CLOCKIFY_WORKSPACE_ID`. It sweeps children before parents — scheduling
assignments and time-off requests first, projects and clients last — and matches each
object by a prefixed name field, except scheduling assignments, which have no name of
their own and are matched by their linked prefixed project (by `projectId`, or by the
embedded `projectName`/`clientName`). Time-off requests are listed through the POST-only
search endpoint. After deleting it re-lists every prefixed-object collection and
prints `Leftovers: 0` for prefix-object leftovers when those sweep targets are clean; a non-zero
prefix-object leftover count or any list/delete failure exits non-zero.
`CLOCKIFY_LIVE_CLEAN_DRY_RUN=1` previews the sweep without mutating, and still
exits non-zero if a collection list fails so the preview is never silently
incomplete.

`clockify_users_invite` does not leave a sweepable object. The optional-domain campaign
invites `mcp-live-<runID>@example.com` with `send_email:false`, and the test accepts an
`ok:true` or a recovery envelope. In a workspace already at its subscription seat cap the
invite returns a clean `ok:false` ("more users than your subscription allows") and
creates nothing — verified live on 2026-05-19. If the campaign runs against a workspace
with a free seat, a successful invite can instead create a pending workspace membership;
that member is not prefixed (`mcp-live-` differs from `CLOCKIFY_LIVE_PREFIX`) and member
removal is an admin action outside the prefixed-delete sweep. It is therefore
not included in a `Leftovers: 0` prefix-object result, so audit
`clockify_users_list` with `status:PENDING` for `mcp-live-*@example.com` entries and
remove them through the Clockify admin UI.
