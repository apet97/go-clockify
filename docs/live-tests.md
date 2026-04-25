# Live contract tests

The nightly **Live contract** workflow (`.github/workflows/live-contract.yml`)
runs the build-tagged `livee2e` test suite against a dedicated sacrificial
Clockify workspace. Its job is to catch upstream drift — response shape
changes, auth policy changes, rate-limit behavior changes — before those
changes break customer integrations without anyone noticing.

## What runs

| Test | Always runs | Runs when `CLOCKIFY_LIVE_WRITE_ENABLED=true` |
|---|---|---|
| `TestE2EReadOnly`  (whoami, get_workspace, list_projects) | ✅ | ✅ |
| `TestE2EErrors`    (invalid ID, missing args) | ✅ | ✅ |
| `TestE2EMutating`  (create_client → create_project → start_timer → stop_timer → delete_entry, with full cleanup) | ❌ | ✅ |
| `TestLiveDryRunDoesNotMutate`                         (MCP-path: confirms `clockify_delete_entry` with `dry_run:true` previews instead of deleting) | ❌ | ✅ |
| `TestLivePolicyTimeTrackingSafeBlocksProjectCreate`   (MCP-path: confirms `time_tracking_safe` rejects `clockify_create_project` before the handler runs) | ❌ | ✅ |
| `TestLiveCreateUpdateDeleteEntryAuditPhases`          (MCP-path + Postgres: confirms each non-read-only call writes both intent and outcome rows) | ❌ | ✅ *(also requires `MCP_LIVE_CONTROL_PLANE_DSN`)* |

The read-only tests are always enabled because they have no side effects.
The mutating tests are gated by a repository variable so writes can be
disabled from the GitHub UI without editing the workflow — useful when
the sacrificial workspace needs a break or when Clockify is flapping.

## The sacrificial workspace

**Rule:** This workspace is never used by humans, never linked to billing,
and never contains real customer data.

Setting it up:

1. Create a fresh Clockify account under a team domain nobody reads
   (e.g. `live-tests+ci@your-domain`).
2. Create a new workspace. Name it `go-clockify-ci-sacrificial` or
   similar so it's obvious in audit logs.
3. Generate an API key scoped to that workspace only.
4. Store the key and workspace id as repo secrets:
   - `CLOCKIFY_LIVE_API_KEY`
   - `CLOCKIFY_LIVE_WORKSPACE_ID`
5. Set the repo variable `CLOCKIFY_LIVE_WRITE_ENABLED` to `true` to
   enable the mutating test path.
6. (Audit-phase test only) Provision a sacrificial Postgres database
   the audit-phase contract test can write to and store its DSN as
   the repo secret `MCP_LIVE_CONTROL_PLANE_DSN`. Setup steps:
   - Spin up a small Postgres instance (RDS, Cloud SQL, fly.io, a
     `postgres:16-alpine` container — anything reachable from the
     GitHub-hosted runner). Do **not** use a production database,
     even one with separate schemas; the test's cleanup drops every
     `audit_events` row matching its synthesised `session_id`, but a
     migration mistake would still affect that pool.
   - Create a dedicated database (e.g. `cp_live_audit`) and
     a least-privileged role limited to that database's
     `audit_events`, `tenants`, `credential_refs`, `sessions`, and
     `schema_migrations` tables (the production opener auto-applies
     migrations on first connect, so `CREATE` privileges are
     required).
   - DSN format: `postgres://USER:PASSWORD@HOST:5432/DBNAME?sslmode=require`.
     `?sslmode=disable` is acceptable for ephemeral test instances.
   - (Optional, paid-launch posture) Set the repo variable
     `CLOCKIFY_LIVE_AUDIT_REQUIRED=true` so a missing DSN fails
     the nightly instead of skipping with a `::warning::`. Leave it
     unset during onboarding; flip it on once the secret is wired
     and the test has been observed running green.

### Fail-soft skip behaviour (read this for fresh forks)

When **either** `CLOCKIFY_LIVE_API_KEY` **or**
`CLOCKIFY_LIVE_WORKSPACE_ID` is missing, the workflow exits the
`secrets_check` step with `skip=true` and downstream test steps are
gated off via their `if:` conditions. The nightly run reports
**green** — not failed — and a `::warning::` annotation surfaces in
the job summary naming the missing secret(s).

This matters for anyone reading the Actions tab: **a green nightly
does not by itself prove the live tests actually ran.** The
honest signal is the warning annotation in the job summary. If
someone forks this repo without copying the secrets, every
nightly will be silently green with a warning, which is the
intended behaviour — it avoids drowning the `live-test-failure`
label queue with drift noise from unowned forks — but it is also
the reason to skim the job summary periodically instead of
trusting the green check alone.

To force a failure when the secrets are missing (e.g. on an
internal deployment where the secrets are required), turn the
`::warning::` into a `::error::` and remove the `if: skip != 'true'`
gating from the test steps. That's a deliberate policy choice, not
the default.

## Secret rotation

The API key should be rotated:

- **Every 90 days** as routine hygiene.
- **Immediately** if a `live-test-failure` issue mentions auth errors.
- **Immediately** if the workflow YAML leaks into a public fork (Actions
  redacts secrets in logs but not in uncommon error paths).

To rotate:

1. Generate a new API key in Clockify for the sacrificial workspace.
2. Update the `CLOCKIFY_LIVE_API_KEY` secret in repo settings.
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
--- FAIL: TestE2EReadOnly (0.12s)
    e2e_live_test.go:92: Unexpected projects format
```

Fix: update the struct in `internal/clockify/` to match the new shape,
add a comment with the date of the drift, and bump the module version.
No emergency — existing clients continue to work with the old fields
until you ship the update.

### 2. Auth / permission change

Clockify sometimes changes the minimum role needed for an operation. A
failure here looks like:

```
--- FAIL: TestE2EMutating (0.08s)
    e2e_live_test.go:138: create_client failed: 403 Forbidden
```

Fix: confirm the sacrificial workspace still has write access. If
Clockify revoked a permission, you'll need to either (a) grant the
workspace the required role, or (b) disable mutating tests via
`CLOCKIFY_LIVE_WRITE_ENABLED=false` until you can restructure the test.

### 3. Genuine regression

If a commit landed on `main` between nightly runs and the failure traces
to your own code, revert the offending commit. The live test is the
last line of defense before customer integrations break.

## Running locally

```sh
export CLOCKIFY_API_KEY='...'       # sacrificial workspace key
export CLOCKIFY_RUN_LIVE_E2E=1      # opt-in gate
go test -tags=livee2e -count=1 ./tests/...
```

Never point local live tests at a production workspace. The test will
create a client, a project, a time entry, and then clean them up — if
anything crashes mid-run, the workspace will be left with orphan
entities named `AG_TEST_<timestamp>_*`.

## Required live coverage before paid hosted launch

The launch-blocking MCP-path safety contracts are now implemented
and wired into the nightly workflow alongside the long-standing
read-only and mutating suites:

| Contract test | What it proves | Where it lives |
|---|---|---|
| `TestLiveDryRunDoesNotMutate` | When `clockify_delete_entry` is invoked with `dry_run:true` through the MCP enforcement pipeline, the destructive handler never runs, the response carries the dry-run envelope, and the entry still exists upstream. | `tests/e2e_live_mcp_test.go` |
| `TestLivePolicyTimeTrackingSafeBlocksProjectCreate` | With `CLOCKIFY_POLICY=time_tracking_safe`, calling `clockify_create_project` through the MCP path returns a policy-deny error and the project is never created upstream. | `tests/e2e_live_mcp_test.go` |
| `TestLiveCreateUpdateDeleteEntryAuditPhases` | A real create→update→delete entry cycle, driven through the MCP server and persisted to a Postgres-backed control plane, lands six rows in `audit_events` (3 `intent` + 3 `outcome`, paired per tool, with `outcome=success` on the outcome row and distinct `external_id`s). | `internal/controlplane/postgres/live_audit_phases_test.go` (build tags `postgres,livee2e`) |

All three tests share the same `CLOCKIFY_LIVE_WRITE_ENABLED=true`
write-gate as `TestE2EMutating`. The audit-phase test additionally
requires `MCP_LIVE_CONTROL_PLANE_DSN` pointing at a sacrificial
Postgres database that has had the controlplane migrations applied
(the test reuses the production opener, which auto-migrates on
first connect).

### Skip behaviour when secrets are missing

| Missing secret/var | What happens |
|---|---|
| `CLOCKIFY_LIVE_API_KEY` / `CLOCKIFY_LIVE_WORKSPACE_ID` | Whole live job skips with a `::warning::` (fork-friendly fail-soft); on the main repo it is a hard failure (see `live-contract.yml`). |
| `CLOCKIFY_LIVE_WRITE_ENABLED != "true"` | Mutating + MCP-path safety + audit-phase tests all skip; read-only tests still run. |
| `MCP_LIVE_CONTROL_PLANE_DSN` | Audit-phase test skips with a `::warning::`. The other safety tests still run. Set the repo variable `CLOCKIFY_LIVE_AUDIT_REQUIRED=true` to make missing DSN a hard failure on the main repo. |

### Running locally

The audit-phase test requires Postgres locally:

```sh
export CLOCKIFY_API_KEY='...'
export CLOCKIFY_WORKSPACE_ID='...'
export CLOCKIFY_RUN_LIVE_E2E=1
export CLOCKIFY_LIVE_WRITE_ENABLED=true
export MCP_LIVE_CONTROL_PLANE_DSN='postgres://cp:cp@localhost:5432/cp_live_audit?sslmode=disable'
# Read/write/dry-run/policy contracts:
go test -tags=livee2e -count=1 -timeout 10m \
  -run '^(TestE2EMutating|TestLiveDryRunDoesNotMutate|TestLivePolicyTimeTrackingSafeBlocksProjectCreate)$' \
  ./tests/...
# Audit-phase contract (separate sub-module — must run from there):
( cd internal/controlplane/postgres && \
  go test -tags=postgres,livee2e -count=1 -timeout 10m \
    -run '^TestLiveCreateUpdateDeleteEntryAuditPhases$' ./... )
```

## Why not just run in PR CI?

The sacrificial workspace has finite API quota and a Clockify 5xx burst
would cascade into false red builds on unrelated PRs. Nightly runs give
Clockify's occasional flakiness a chance to resolve without blocking
merges, while still catching upstream drift within 24 hours of it
happening.
