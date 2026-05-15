# Agent Handoff - One-User Clockify MCP

This handoff is the current autonomous-agent entrypoint for
`github.com/apet97/go-clockify`.

## Current Product

`cmd/clockify-mcp` starts a local stdio MCP server for exactly one Clockify
workspace. The environment contract is intentionally small:

- `CLOCKIFY_API_KEY` is required.
- `CLOCKIFY_WORKSPACE_ID` is required.
- `CLOCKIFY_TIMEZONE`, `CLOCKIFY_BASE_URL`, and log-level settings are optional.

The runtime registers all 152 tools at startup. Agents should call workflow
tools first, use domain tools for precise CRUD, and use raw API fallback tools
only when no workflow or domain tool fits.

## Invariants

- Keep stdio as the only runtime path.
- Keep the one-key, one-workspace configuration model.
- Keep all tools visible from startup.
- Keep the catalog order: workflow, domain, raw fallback.
- Keep write-style workflow outputs ID-rich.
- Keep recoverable failures structured with recovery guidance.
- Do not remove tools as a cleanup shortcut.

## First Files To Inspect

1. `README.md`
2. `docs/agent-cookbook.md`
3. `docs/tool-catalog.md`
4. `docs/goals/oneuser-tool-coverage.md`
5. `cmd/clockify-mcp/main.go`
6. `internal/tools/firstslice.go`
7. `internal/tools/oneuser_workflows.go`
8. `internal/tools/oneuser_domains.go`
9. `internal/tools/oneuser_resources.go`
10. `internal/mcp/server.go`
11. `internal/testclockify/fake_server.go`

## Verification Baseline

Run these before claiming the repo is healthy:

```sh
go test -count=1 ./...
git diff --check
go list ./...
go list -deps ./cmd/clockify-mcp | grep -E 'controlplane|oidc|grpc|vault|policy|enforcement|runtime|postgres|otel|pprof|auth' || true
```

The dependency grep is a sanity check. The default command may match Go
standard-library packages such as `runtime`; it should not pull in old
repo-local subsystems.

## Live Tests

Live tests use the same one-user configuration as the server. Set the
credentials in the shell, choose a unique prefix, and avoid printing secrets:

```sh
CLOCKIFY_RUN_LIVE_E2E=1 \
CLOCKIFY_LIVE_PREFIX=<unique-prefix> \
go test -count=1 ./internal/tools -run TestOneUserLiveWorkflow
```

`TestOneUserLiveWorkflow` exercises status, demo seed/cleanup, finished work,
start/stop/switch timer flow, entry fix, and day/week review. Paid-feature
workflow probing is separate and opt-in:

```sh
CLOCKIFY_RUN_LIVE_E2E=1 \
CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS=1 \
CLOCKIFY_LIVE_PREFIX=<unique-prefix> \
go test -count=1 ./internal/tools -run TestOneUserLivePaidFeatureWorkflowRecovery
```

Those probes accept either success with IDs or a recoverable `ok:false`
response when the workspace does not allow the feature.

Optional-domain contract probing is also opt-in and uses real live calls only;
it never passes `dry_run` and intentionally leaves sacrificial-workspace
objects in place for inspection:

```sh
CLOCKIFY_RUN_LIVE_E2E=1 \
CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1 \
CLOCKIFY_LIVE_PREFIX=<unique-prefix> \
go test -count=1 ./internal/tools -run TestOneUserLiveOptionalDomainContracts
```

## Current Follow-Ups

- Continue converting lower-priority alias-wrapper domain tools when usage or
  live evidence makes them hot.
- Keep expanding live probes for domain tools that are still marked
  `needs_live_probe` in `docs/goals/oneuser-tool-coverage.md`.
- Keep tightening remaining generic output schemas against fake-server and
  live outputs.
- Keep stale docs out of the read-first path.
