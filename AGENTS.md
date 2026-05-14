# AGENTS.md - go-clockify

Read this file first when working in this repository.

## Product Shape

This repo is a local one-user Clockify MCP server written in Go.

- One required `CLOCKIFY_API_KEY`.
- One required `CLOCKIFY_WORKSPACE_ID`.
- Stdio transport only.
- Full access from startup.
- All 151 tools loaded at startup.
- Workflow tools first, domain tools second, raw API fallback last.
- Every write-style workflow returns useful IDs.
- Recoverable failures return `ok:false`, an error code, and recovery guidance.

Do not change those invariants unless the maintainer explicitly asks for a
product-definition change.

## Read First

1. `README.md` - user-facing setup and product overview.
2. `docs/agent-cookbook.md` - workflow-first examples for agents.
3. `docs/tool-catalog.md` - generated tool list in runtime order.
4. `docs/goals/oneuser-tool-coverage.md` - conservative coverage ledger.
5. `cmd/clockify-mcp/main.go` - process wiring and doctor command.
6. `internal/tools/oneuser_workflows.go` - workflow tools.
7. `internal/tools/oneuser_domains.go` - full-access domain registry.
8. `internal/tools/oneuser_resources.go` - resource provider.
9. `internal/mcp/server.go` - MCP protocol core.
10. `internal/testclockify/fake_server.go` - fake Clockify server.

## Safety Rules

- Never print, commit, or log API keys or tokens.
- Use only the configured workspace for live tests.
- Keep docs and generated catalog files in sync with descriptor changes.
- Do not remove tools to simplify the catalog.
- Do not weaken validation or schema checks to make tests pass.
- Prefer small focused diffs and repo-local helpers.
- Run the existing tests that match the behavior you changed.

## Common Commands

| Goal | Command |
|------|---------|
| Full test suite | `go test -count=1 ./...` |
| Diff hygiene | `git diff --check` |
| Package list | `go list ./...` |
| Default command dependency sanity | `go list -deps ./cmd/clockify-mcp \| grep -E 'controlplane\|oidc\|grpc\|vault\|policy\|enforcement\|runtime\|postgres\|otel\|pprof\|auth' \|\| true` |
| Refresh tool catalog | `make gen-tool-catalog` |
| Focused tools tests | `go test -count=1 ./internal/tools` |
| Focused MCP tests | `go test -count=1 ./internal/mcp` |
| Live workflow smoke | `CLOCKIFY_RUN_LIVE_E2E=1 CLOCKIFY_LIVE_PREFIX=<prefix> go test -count=1 ./internal/tools -run TestOneUserLiveWorkflow` |

For live tests, set `CLOCKIFY_API_KEY` and `CLOCKIFY_WORKSPACE_ID` in the
environment. Do not echo them.

## Generated Files

`docs/tool-catalog.md` and `docs/tool-catalog.json` are generated from the
runtime registry. After changing a tool descriptor, run:

```sh
make gen-tool-catalog
```

Then inspect the diff. The catalog should still show 151 tools with workflow
tools first and raw fallback tools last.
