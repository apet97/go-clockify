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

| Date (UTC) | Workspace plan | Gates | Tests | Result | Leftovers |
|---|---|---|---|---|---|
| 2026-05-15 | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1`, `CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS=1`, `CLOCKIFY_LIVE_HAPPY_PATH_CAMPAIGNS=1`, `CLOCKIFY_LIVE_WORKSPACE_CONFIRM=$CLOCKIFY_WORKSPACE_ID` | `TestOneUserLiveWorkflow`, `TestOneUserLivePaidFeatureWorkflowRecovery`, `TestOneUserLiveOptionalDomainContracts`, `TestLiveOneUserWorkflowMCP` | PASS (42.7s `internal/tools`, 29.0s `tests`) | 0 (`clockify_demo_cleanup` returned ok) |
| 2026-05-15 | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1`, `CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS=1`, `CLOCKIFY_LIVE_WORKSPACE_CONFIRM=$CLOCKIFY_WORKSPACE_ID` | core (`TestLiveOneUserWorkflowMCP`, `TestLiveRawClockifyReadSideSchemaDiff`), `internal/tools` `TestOneUserLive*`, optional-domain `TestLive*` campaign | PASS per sub-suite (21.3s core, 19.0s internal/tools, 39.1s optional) | 0 after prefix-scoped sweep (48 tags + 67 clients from this session's runs archived+deleted; `clockify_demo_cleanup` plus a manual `mcp-live*`/`mcp-remaining*`/`mcp-optional*` audit confirmed 0 remaining) |
| 2026-05-16 | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1` | `TestOneUserLiveOptionalDomainContracts` (extended to cover the new `clockify_audit_logs_search` and `clockify_entity_changes_list`) | PASS (12.8s `internal/tools`) | prefixed `mcp-*` optional-domain objects left by the contracts test; the audit-log and entity-changes probes are read-only and create nothing |
| 2026-05-16 | BUNDLE_YEAR_2024 sacrificial | `CLOCKIFY_RUN_LIVE_E2E=1`, `CLOCKIFY_LIVE_PREFIX=MCP-PR110-*`, `CLOCKIFY_LIVE_WORKSPACE_CONFIRM=$CLOCKIFY_WORKSPACE_ID` | post-#110 verification — `TestLiveOneUserWorkflowMCP`, `TestOneUserLiveWorkflow`, plus targeted MCP-server tool calls (`clockify_reports_money`, `clockify_reports_export`, `clockify_status`) against the merged `main` binary | PASS (25.7s `tests`, 11.5s `internal/tools`); `clockify_reports_money` returned `ok:true` with normalized totals, confirming the PR #110 fix of the prior route-tool descriptor failure | 0 after prefix-scoped sweep — the two workflow tests left 2 clients + 2 projects (+2 tasks) prefixed `MCP-PR110-20260516220202`; archived and deleted, and `clockify_clients_list`/`clockify_projects_list` with `name=MCP-PR110` then confirmed 0 remaining |

`make live-contract-local` runs all three sub-suites back-to-back; on a
shared workspace that can trip Clockify's rate budget and surface a
transient `http2: timeout` mid-suite (the MCP returns a clean `ok:false`
recovery envelope in that case). Run the three sub-suites individually
for clean rate budgets when the combined target flakes.

When recording a new run, include the date, the visible workspace plan (from
`doctor --live`), the env-var set that was active, the named test functions,
the pass/fail result, and a leftover count from `clockify_demo_cleanup` plus
any prefix-scoped audit.
