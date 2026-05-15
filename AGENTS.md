# AGENTS.md - go-clockify

Read this first. This file is the tracked, binding agent contract for the
repo. `CLAUDE.md` may exist locally, but it is workstation context and is
ignored by git.

## Product Contract

`go-clockify` is a local one-user Clockify MCP server in Go.

- One local trusted user.
- One `CLOCKIFY_API_KEY`.
- One required `CLOCKIFY_WORKSPACE_ID`.
- Stdio transport only.
- Full access from startup.
- Exactly 151 tools loaded at startup.
- Workflow tools first, domain tools second, raw API fallback last.
- Every write returns useful IDs.
- Recoverable failures return `ok:false`, an error code, and recovery guidance.
- Optional live evidence stays split into protocol/recovery vs happy-path.

Do not change those invariants unless the maintainer explicitly changes the
product definition.

## Start Here

1. `README.md` - setup and product overview.
2. `docs/agent-cookbook.md` - workflow-first agent examples.
3. `docs/tool-catalog.md` - generated runtime tool list and order.
4. `docs/goals/oneuser-tool-coverage.md` - conservative coverage ledger.
5. `docs/live-tests.md` - live-test gates and sacrificial workspace rules.
6. `docs/launch-readiness-review-may-8.md` - May 8 review disposition ledger.

The May 8 review disposition ledger contains the objective-to-artifact
completion audit. Do not mark launch-ready while that audit says external
evidence or approval gates remain open.

Historical docs can explain prior decisions, but current implementation work
starts from the files above plus the code. Do not route users to archived or
bannered platform-era docs as setup instructions.

## Safety Rules

- Never print, commit, or log API keys, workspace IDs from private context, or
  tokens.
- Use only the configured sacrificial workspace for live tests.
- Do not mutate live Clockify unless the user asked for live calls or the test
  gate explicitly requires them.
- Do not weaken validation, schemas, or recovery behavior to make tests pass.
- Do not remove tools to simplify the catalog.
- Do not reintroduce old activation, policy, control-plane, or multi-user
  concepts.
- Preserve user changes in a dirty tree; inspect before editing.
- Prefer small focused diffs and repo-local helpers.
- If MCP tool errors ever become remotely exposed, revisit error-message
  sanitization before shipping that path.

## Common Commands

| Goal | Command |
| --- | --- |
| Full tests | `go test -count=1 ./...` |
| Race/check gate | `make check` |
| Diff hygiene | `git diff --check` |
| Package list | `go list ./...` |
| Tool catalog drift | `make catalog-drift` |
| Regenerate catalog | `make gen-tool-catalog` |
| Focus tools | `go test -count=1 ./internal/tools` |
| Focus MCP | `go test -count=1 ./internal/mcp` |
| Live compile only | `go test -tags=livee2e -count=0 ./tests/...` |
| Local lint | `golangci-lint run` |

Dependency sanity for the default command:

```sh
go list -deps ./cmd/clockify-mcp \
  | grep -E 'controlplane|oidc|grpc|vault|policy|enforcement|runtime|postgres|otel|pprof|auth' \
  || true
```

Runtime-related `internal/runtime/...` and Go `runtime` hits are not
themselves product regressions; inspect any non-runtime hit.

## Live Tests

Required live env:

```sh
export CLOCKIFY_API_KEY='...'
export CLOCKIFY_WORKSPACE_ID='...'
export CLOCKIFY_RUN_LIVE_E2E=1
export CLOCKIFY_LIVE_PREFIX='MCP-LIVE-YYYYMMDD'
```

Extra gates:

```sh
export CLOCKIFY_LIVE_OPTIONAL_DOMAINS=1
export CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS=1
export CLOCKIFY_LIVE_HAPPY_PATH_CAMPAIGNS=1
export CLOCKIFY_LIVE_WORKSPACE_CONFIRM="$CLOCKIFY_WORKSPACE_ID"
export CLOCKIFY_LIVE_ADMIN_ENABLED=true
export CLOCKIFY_LIVE_BILLING_ENABLED=true
export CLOCKIFY_LIVE_SETTINGS_ENABLED=true
```

Only mark live happy-path evidence when the tool returns `ok:true` against a
real entity. A useful recovery envelope is protocol/recovery evidence only.

## Code Map

| Need | Start Here |
| --- | --- |
| Process wiring | `cmd/clockify-mcp/main.go` |
| One-user config | `internal/config/oneuser.go` |
| MCP protocol | `internal/mcp/server.go` |
| Workflow tools | `internal/tools/oneuser_workflows.go` |
| Domain registry | `internal/tools/oneuser_domains.go` |
| Native domain logic | `internal/tools/*_view.go`, `internal/tools/tier2_*.go` |
| Resources/prompts | `internal/tools/oneuser_resources.go`, `internal/tools/oneuser_prompts.go` |
| Clockify client | `internal/clockify/client.go` |
| Fake server | `internal/testclockify/fake_server.go` |
| Live tests | `internal/tools/oneuser_live_test.go`, `tests/e2e_live*.go` |
| Generated catalog | `docs/tool-catalog.md`, `docs/tool-catalog.json` |
| Coverage ledger | `docs/goals/oneuser-tool-coverage.md` |

## Registry Shape

The product registry is `Service.FullAccessRegistry()`.

Current composition:

1. `workflowDescriptors()`
2. `FirstSliceRegistry()`
3. `nativeCoreDescriptors()`
4. `nativeHighValueDescriptors()`
5. `nativeAliasDescriptors()`; should emit no wrappers.
6. `timerAndReportDescriptors()`
7. `rawAPIDescriptors()`

`routeTool` still supports native route-backed descriptors. Do not delete it
just because the older route bucket was pruned.

## Generated Files

`docs/tool-catalog.md` and `docs/tool-catalog.json` are generated from the
runtime registry. After descriptor/schema/order changes:

```sh
make gen-tool-catalog
make catalog-drift
```

The catalog should still show 151 tools, workflow tools first, and raw API
fallback tools last.

## Coverage Ledger Rules

- `docs/goals/oneuser-tool-coverage.md` is the source of truth.
- Fake smoke is not live proof.
- Live protocol/recovery and live happy-path are separate columns.
- Do not count bogus-ID or unavailable-feature recovery as happy path.
- Preserve recovery probes for destructive, noisy, or permission-sensitive
  paths.
- Update ledger validation tests when evidence changes.

## Known Clockify API Gotchas

- Time-off request listing is POST-based:
  `POST /workspaces/{workspaceId}/time-off/requests`.
- Do not convert that path to GET; prior live probes returned 405.
- Invoice mark-paid may require payment creation rather than direct status
  mutation; keep the ledger honest when the API rejects direct status changes.
- Holiday get/update behavior may differ from list/create/delete; do not mark
  happy-path unless live evidence proves it.

## Testing Discipline

- Add or update tests before changing behavior.
- For narrow docs edits, run the focused doc tests that cover the touched
  surface.
- For registry or schema edits, run `go test -count=1 ./internal/tools` plus
  catalog drift.
- For MCP protocol edits, run `go test -count=1 ./internal/mcp`.
- Before claiming completion, run fresh verification and report exactly what
  passed or what was not run.

## Git Discipline

- Do not use destructive git commands unless explicitly asked.
- Do not commit ignored local files unless the user explicitly requests that.
- Keep commits atomic and evidence-backed.
- If pushing direct to `main`, watch GitHub checks and report their final
  status.
