# Live contract tests

The nightly **Live contract** workflow (`.github/workflows/live-contract.yml`)
runs the build-tagged `livee2e` test suite against a dedicated sacrificial
Clockify workspace. Its job is to catch upstream drift — response shape
changes, auth policy changes, rate-limit behavior changes — before those
changes break customer integrations without anyone noticing.

## Current one-user live-test posture

As of the one-user rewrite, local live-contract tests use the same
configuration surface as the stdio server: `CLOCKIFY_API_KEY`,
`CLOCKIFY_WORKSPACE_ID`, and the opt-in flag
`CLOCKIFY_RUN_LIVE_E2E=1`. The primary live contract drives
`initialize` + `tools/call` against the one-user MCP harness rather
than calling tool handlers directly.

## What runs

| Test | Runs with `CLOCKIFY_RUN_LIVE_E2E=1` | Extra gates |
|---|---|---|
| `TestLiveOneUserWorkflowMCP` | yes | none; performs real workflow calls |
| `TestLiveRawClockifyReadSideSchemaDiff` | yes | none; raw Clockify model drift only |
| `TestLiveTier1ReadOnly` | yes | `CLOCKIFY_LIVE_FULL_SURFACE_ENABLED=true` |
| `TestLivePaginationOnTags` | yes | `CLOCKIFY_LIVE_FULL_SURFACE_ENABLED=true` |
| `TestLiveTier2ReadOnlySweep` | yes | `CLOCKIFY_LIVE_FULL_SURFACE_ENABLED=true` |
| `TestLiveT2*` optional-domain CRUD probes | yes | `CLOCKIFY_LIVE_FULL_SURFACE_ENABLED=true` plus category gates |

The workflow test is intentionally live and does not pass `dry_run`.
Use only the sacrificial workspace.

The static inventory guard is:

```sh
bash scripts/check-live-tool-coverage.sh
```

It proves only that the livee2e source inventory names the current
catalog surface. It does not prove the tests ran, passed, or count as
Group 1 evidence.

## The sacrificial workspace

**Rule:** This workspace is never used by humans, never linked to billing,
and never contains real customer data.

Setting it up:

1. Create a fresh Clockify account under a team domain nobody reads
   (e.g. `live-tests+ci@your-domain`).
2. Create a new workspace. Name it `go-clockify-ci-sacrificial` or
   similar so it's obvious in audit logs.
3. Generate an API key scoped to that workspace only.
4. Store the key and workspace id as `CLOCKIFY_API_KEY` and
   `CLOCKIFY_WORKSPACE_ID` in the shell or CI secret environment.
5. Set `CLOCKIFY_RUN_LIVE_E2E=1` only for runs that are meant to touch
   the live sacrificial workspace.
6. Set `CLOCKIFY_LIVE_FULL_SURFACE_ENABLED=true` only when you also
   want the optional full-surface campaign.

### Fail-soft skip behaviour (read this for fresh forks)

When `CLOCKIFY_API_KEY`, `CLOCKIFY_WORKSPACE_ID`, or
`CLOCKIFY_RUN_LIVE_E2E=1` is missing, the build-tagged tests skip. On
unfiltered local livee2e runs, `TestLiveContractSkipSentinel` fails if
everything skipped so the shell output cannot be mistaken for evidence.

This matters for anyone reading local output: a skipped run is not live
evidence.

## Secret rotation

The API key should be rotated:

- **Every 90 days** as routine hygiene.
- **Immediately** if a `live-test-failure` issue mentions auth errors.
- **Immediately** if the workflow YAML leaks into a public fork (Actions
  redacts secrets in logs but not in uncommon error paths).

To rotate:

1. Generate a new API key in Clockify for the sacrificial workspace.
2. Update the `CLOCKIFY_API_KEY` secret or local environment value.
3. Revoke the old key.
4. Trigger the workflow via `workflow_dispatch` to confirm the new key
   works before waiting on the nightly.

## Triage playbook — when the nightly fails

The workflow opens a single rolling GitHub issue labelled
`live-test-failure` when a run fails and auto-closes it when the next
run is green. If the issue is already open, the workflow comments on it
rather than spawning a duplicate.

Most failures fall into one of three buckets:

### 1. Response shape drift (most common)

Clockify occasionally renames a field, changes a type, or adds a new
required property. The test failure usually looks like:

```
--- FAIL: TestLiveRawClockifyReadSideSchemaDiff (0.12s)
    e2e_live_schema_test.go:45: /workspaces/{id}/projects[0] returned fields not represented in clockify.Project: ...
```

Fix: update the struct in `internal/clockify/` to match the new shape,
add a comment with the date of the drift, and bump the module version.
No emergency — existing clients continue to work with the old fields
until you ship the update.

### 2. Auth / permission change

Clockify sometimes changes the minimum role needed for an operation. A
failure here looks like:

```
--- FAIL: TestLiveOneUserWorkflowMCP (0.08s)
    e2e_live_test.go:34: clockify_demo_seed returned error: 403 Forbidden
```

Fix: confirm the sacrificial workspace still has write access. If
Clockify revoked a permission, grant the sacrificial workspace the
required role or disable the relevant optional category gate until the
test can be restructured.

### 3. Genuine regression

If a commit landed on `main` between nightly runs and the failure traces
to your own code, revert the offending commit. The live test is the
last line of defense before customer integrations break.

## Running locally

Prefer `make live-contract-local` — it wraps the test run with evidence
warnings reminding you that **local green is not Group 1 launch-candidate
evidence**. Only scheduled cron runs of `.github/workflows/live-contract.yml`
on the candidate SHA count.

```sh
# Preferred (with evidence warnings):
make live-contract-local

# Raw (without evidence warnings):
export CLOCKIFY_API_KEY='...'       # sacrificial workspace key
export CLOCKIFY_RUN_LIVE_E2E=1      # opt-in gate
go test -tags=livee2e -count=1 ./tests/...
```

**Beware:** running `go test -tags=livee2e` without
`CLOCKIFY_RUN_LIVE_E2E=1` and `CLOCKIFY_API_KEY` silently skips every
test and reports `ok` in <0.5s. `TestLiveContractSkipSentinel` (under
the same build tag) now fails explicitly when this happens, but the
Makefile target is the safest path — it fails on missing env vars before
the tests even start.

Never point local live tests at a production workspace. The one-user
workflow test performs real writes and may leave prefixed demo objects
behind for later inspection.

### Personal workspace smoke tests are not launch evidence

For a real single-owner workspace with valuable data, do not run the
live-contract suite or the full mutating campaign. Use the
`local-stdio` profile with `CLOCKIFY_WORKSPACE_ID` pinned, then smoke
only read-only or narrow calls first:

- `clockify_status`
- `clockify_tools_guide`
- small read-only domain pages with explicit `page` / `page_size`
- short-range reports with `include_entries=false`

Those personal checks can prove your local client wiring, but they are
not Group 1 launch-candidate evidence. The evidence that closed Group 1
is the pair of scheduled GitHub Actions runs recorded above.

## Required live coverage

The one-user live contracts are:

| Contract test | What it proves | Where it lives |
|---|---|---|
| `TestLiveOneUserWorkflowMCP` | The stdio MCP path can initialize and call workflow tools including status, tool guide, demo seed, log work, fix entry, day/week review, and no-op demo cleanup. | `tests/e2e_live_test.go` |
| `TestLiveRawClockifyReadSideSchemaDiff` | Raw Clockify responses (`/user`, `/workspaces`, projects, clients, tags, tasks when present, and user time entries when present) do not contain top-level fields missing from the corresponding `internal/clockify` structs. | `tests/e2e_live_schema_test.go` |

### Skip behaviour when secrets are missing

| Missing secret/var | What happens |
|---|---|
| `CLOCKIFY_API_KEY` / `CLOCKIFY_WORKSPACE_ID` | Live tests skip before making Clockify calls. |
| `CLOCKIFY_LIVE_FULL_SURFACE_ENABLED != "true"` | Optional full-surface campaign tests skip; one-user workflow and raw schema tests still run. |

### Running locally

```sh
export CLOCKIFY_API_KEY='...'
export CLOCKIFY_WORKSPACE_ID='...'
export CLOCKIFY_RUN_LIVE_E2E=1
go test -tags=livee2e -count=1 -timeout 5m \
  -run '^(TestLiveOneUserWorkflowMCP|TestLiveRawClockifyReadSideSchemaDiff)$' \
  ./tests/...

# Optional full-surface campaign:
export CLOCKIFY_LIVE_FULL_SURFACE_ENABLED=true
go test -tags=livee2e -count=1 -timeout 10m \
  -run '^(TestLiveTier1ReadOnly|TestLivePaginationOnTags|TestLiveTier2ReadOnlySweep|TestLiveT2)' \
  ./tests/...
```

## Why not just run in PR CI?

The sacrificial workspace has finite API quota and a Clockify 5xx burst
would cascade into false red builds on unrelated PRs. Nightly runs give
Clockify's occasional flakiness a chance to resolve without blocking
merges, while still catching upstream drift within 24 hours of it
happening.
