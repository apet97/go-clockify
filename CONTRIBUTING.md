# Contributing to go-clockify

`go-clockify` is a local one-user Clockify MCP server written in Go. The
runtime shape is intentionally small: one API key, one required workspace id,
stdio transport, and all 156 tools loaded at startup.

## Development Setup

```sh
git clone https://github.com/apet97/go-clockify.git
cd go-clockify
go build ./...
```

Requires the pinned Go 1.25.10 toolchain. Module path:
`github.com/apet97/go-clockify`.

## Common Commands

```sh
make build
make test
make fmt
make vet
make check
make gen-tool-catalog
make catalog-drift
```

`make check` is the normal pre-PR gate: formatting, `go vet`, and the full Go
test suite with the race detector. After descriptor changes, run
`make gen-tool-catalog` and `make catalog-drift`.

Live tests intentionally call a sacrificial Clockify workspace. Read
[docs/live-tests.md](docs/live-tests.md) before running:

```sh
CLOCKIFY_RUN_LIVE_E2E=1 \
CLOCKIFY_LIVE_PREFIX=<unique-prefix> \
make live-contract-local
```

Set `CLOCKIFY_API_KEY` and `CLOCKIFY_WORKSPACE_ID` in the environment, but never
print or commit them.

## Project Structure

```text
cmd/clockify-mcp/        One-user stdio executable and doctor command
internal/config/         One-user environment loader
internal/mcp/            JSON-RPC/MCP protocol core
internal/clockify/       Clockify HTTP client and typed models
internal/tools/          Workflow, domain, resource, and raw fallback tools
internal/paths/          Workspace path builder with ID validation
internal/resolve/        Name-to-ID resolution helpers
internal/testclockify/   Fake Clockify server for local tests
tests/                   Build-tagged live MCP harness tests
docs/                    User, agent, catalog, and coverage documentation
scripts/                 Current generators plus historical validation helpers
```

The generated catalog should always show workflow tools first, domain tools
second, and raw API fallback tools last. For the layer call graph
(`workflowDescriptors` → `FirstSliceRegistry` →
`nativeHighValueDescriptorsChecked` → `FullAccessRegistryChecked`), the
toolset filter, and the current `ToolResult` vs `ResultEnvelope` state, see
[docs/architecture.md](docs/architecture.md).

## Pull Request Process

1. Create a focused branch from `main`.
2. Keep diffs scoped to the behavior or documentation you are changing.
3. Add or update tests when behavior changes.
4. Run the matching local gates and include the commands in the PR body.
5. Open a PR with a clear summary and any remaining risk.

## Commit Conventions

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation only
- `ci:` - CI changes
- `refactor:` - Code change that neither fixes a bug nor adds a feature
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks

## Design Principles

1. Keep the one-user product boundary explicit.
2. Prefer small, focused diffs.
3. Keep stdout pure MCP protocol; diagnostics belong on stderr.
4. Prefer typed models and typed output schemas for stable entities.
5. Every write-style workflow should return useful IDs.
6. Recoverable errors should return recovery guidance.
7. Do not print, commit, or log API keys.

## Go Version Pin

This project pins to **Go 1.25.10**. Patch bumps should move all checked Go
version surfaces together, then run:

```sh
make check
go list ./...
git diff --check
```
