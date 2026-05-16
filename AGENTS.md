# AGENTS.md - go-clockify

Read this first. This is the tracked, binding agent contract for the repo.
`CLAUDE.md` is local workstation context and is git-ignored.

## Product Contract

`go-clockify` is a local one-user Clockify MCP server in Go.

- One local trusted user.
- One `CLOCKIFY_API_KEY`.
- One required `CLOCKIFY_WORKSPACE_ID`.
- Stdio transport only.
- Full access from startup.
- Exactly 156 tools loaded at startup.
- Workflow tools first, domain tools second, raw API fallback last.
- Every write returns useful IDs.
- Recoverable failures return `ok:false`, an error code, and recovery guidance.
- Optional live evidence stays split into protocol/recovery vs happy-path.

Do not change these invariants unless the maintainer explicitly changes the
product definition.

## Start Here

1. `README.md` - setup and product overview.
2. `docs/agent-cookbook.md` - workflow-first agent examples.
3. `docs/tool-catalog.md` - generated runtime tool list and order.
4. `docs/goals/oneuser-tool-coverage.md` - conservative coverage ledger.
5. `docs/live-tests.md` - live-test gates and sacrificial workspace rules.
6. `docs/launch-readiness-review-may-8.md` - launch disposition ledger; do not
   mark launch-ready while it shows open external-evidence or approval gates.

Historical docs explain prior decisions; current work starts from the files
above plus the code. Do not route users to archived or bannered platform-era
docs as setup instructions.

## Safety Rules

- Never print, commit, or log API keys, workspace IDs, or tokens.
- Use only the configured sacrificial workspace for live tests, and do not
  mutate live Clockify unless the user asked or a test gate requires it.
- Do not weaken validation, schemas, or recovery behavior to pass tests.
- Do not remove tools to simplify the catalog.
- Do not reintroduce old activation, policy, control-plane, or multi-user
  concepts.
- Preserve user changes in a dirty tree; inspect before editing; prefer small
  focused diffs and repo-local helpers.
- If MCP tool errors ever become remotely exposed, revisit error-message
  sanitization before shipping that path.

## Common Commands

| Goal | Command |
| --- | --- |
| Full tests | `go test -count=1 ./...` |
| Race/check gate | `make check` |
| Diff hygiene | `git diff --check` |
| Local lint | `golangci-lint run` |
| Catalog drift / regenerate | `make catalog-drift` · `make gen-tool-catalog` |
| Focus tools / MCP | `go test -count=1 ./internal/tools` · `./internal/mcp` |
| Live compile only | `go test -tags=livee2e -count=0 ./tests/...` |

The default command must stay free of controlplane/oidc/grpc/vault/policy/
postgres/auth dependencies; check with
`go list -deps ./cmd/clockify-mcp` (`internal/runtime/...` and Go `runtime`
hits are expected, not regressions).

## Live Tests

```sh
export CLOCKIFY_API_KEY='...' CLOCKIFY_WORKSPACE_ID='...'
export CLOCKIFY_RUN_LIVE_E2E=1 CLOCKIFY_LIVE_PREFIX='MCP-LIVE-YYYYMMDD'
```

Extra mutation gates: `CLOCKIFY_LIVE_OPTIONAL_DOMAINS`,
`CLOCKIFY_LIVE_HIGH_RISK_WORKFLOWS`, `CLOCKIFY_LIVE_HAPPY_PATH_CAMPAIGNS`,
`CLOCKIFY_LIVE_WORKSPACE_CONFIRM`, `CLOCKIFY_LIVE_ADMIN_ENABLED`,
`CLOCKIFY_LIVE_BILLING_ENABLED`, `CLOCKIFY_LIVE_SETTINGS_ENABLED`.

Mark live happy-path evidence only when the tool returns `ok:true` against a
real entity; a useful recovery envelope is protocol/recovery evidence only.

## Code Map

| Need | Start Here |
| --- | --- |
| Process wiring | `cmd/clockify-mcp/main.go` |
| One-user config | `internal/config/oneuser.go` |
| MCP protocol | `internal/mcp/server.go` |
| Workflow tools | `internal/tools/oneuser_workflows.go` |
| Domain registry | `internal/tools/oneuser_domains.go` |
| Native domain logic | `internal/tools/*_view.go`, `internal/tools/tier2_*.go` |
| Resources / prompts | `internal/tools/oneuser_resources.go`, `oneuser_prompts.go` |
| Clockify client | `internal/clockify/client.go` |
| Fake server | `internal/testclockify/fake_server.go` |
| Live tests | `internal/tools/oneuser_live_test.go`, `tests/e2e_live*.go` |
| Generated catalog / ledger | `docs/tool-catalog.{md,json}`, `docs/goals/oneuser-tool-coverage.md` |

## Registry Shape

`Service.FullAccessRegistry()` composes the registry in order:
`workflowDescriptors` → `FirstSliceRegistry` → `nativeCoreDescriptors` →
`nativeHighValueDescriptors` → `nativeDomainExtras` → `timerAndReportDescriptors`
→ `rawAPIDescriptors`. `routeTool` still backs route-based native descriptors —
do not delete it.

`docs/tool-catalog.{md,json}` are generated from the registry. After any
descriptor, schema, or order change, run `make gen-tool-catalog` then
`make catalog-drift`. The catalog stays at 156 tools, workflow-first, raw-last.

## Coverage Ledger Rules

- `docs/goals/oneuser-tool-coverage.md` is the source of truth.
- Fake smoke is not live proof; live protocol/recovery and live happy-path are
  separate columns.
- Do not count bogus-ID or unavailable-feature recovery as happy path.
- Preserve recovery probes for destructive, noisy, or permission-sensitive paths.
- Update ledger validation tests when evidence changes.

## Known Clockify API Gotchas

- Time-off request listing is POST, not GET (GET returned 405 in live probes):
  `POST /workspaces/{workspaceId}/time-off/requests`.
- Invoice mark-paid may require payment creation rather than a direct status
  change; keep the ledger honest when the API rejects direct mutation.
- Holiday get/update behavior differs from list/create/delete; do not mark
  happy-path without live evidence.

## Testing Discipline

- Add or update tests before changing behavior.
- Registry/schema edits: `go test -count=1 ./internal/tools` plus catalog drift.
  MCP protocol edits: `go test -count=1 ./internal/mcp`.
- For narrow docs edits, run the focused doc tests covering the touched surface.
- Before claiming completion, run fresh verification and report exactly what
  passed and what was not run.

## Git Discipline

- Do not use destructive git commands unless explicitly asked.
- Do not commit ignored local files unless the user requests it.
- Keep commits atomic and evidence-backed.
- When pushing direct to `main`, watch GitHub checks and report their final
  status, including any branch-protection bypass notice.
