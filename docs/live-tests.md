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
