# Contributing to Clockify MCP Server (Go)

## Development Setup

```sh
git clone https://github.com/apet97/go-clockify.git
cd go-clockify
go build ./...
```

Requires Go 1.25.0+. No external dependencies. Module path: `github.com/apet97/go-clockify`.

## Running Tests

```sh
# All tests (268 tests across 13 packages)
go test ./...

# With race detector (highly recommended)
go test -race ./...

# Single package
go test ./internal/tools/...
go test ./internal/mcp/...

# Verbose output
go test -v -run TestName ./internal/mcp/...
```

## Code Quality

Before submitting a PR, ensure:

```sh
# Format
gofmt -w ./cmd ./internal

# Vet
go vet ./...

# Build
go build ./...

# Test with race detector
go test -race -count=1 ./...
```

All four must pass with no errors.

## Project Structure

```
cmd/clockify-mcp/     Entrypoint
internal/
  config/             Environment variable configuration
  clockify/           HTTP client, models, errors
  mcp/                MCP server (stdio + HTTP transport, context-aware shutdown)
  tools/              All 124 tool handlers (Tier 1 + Tier 2)
  policy/             Policy modes (read_only/safe_core/standard/full)
  resolve/            Name-to-ID resolution
  dryrun/             Dry-run interception
  bootstrap/          Tool visibility modes
  ratelimit/          Concurrency + throughput control (race-safe)
  truncate/           Token-aware output truncation
  dedupe/             Duplicate entry detection
  timeparse/          Natural language time parsing
  helpers/            Error mapping, write envelopes
```

## Pull Request Process

1. Fork the repo and create a feature branch from `main`
2. Make your changes with clear, focused commits
3. Add tests for new functionality
4. Ensure all checks pass (fmt, vet, build, test)
5. Open a PR with a clear description of what and why

## Commit Conventions

Use conventional commit prefixes:
- `feat:` — New feature
- `fix:` — Bug fix
- `docs:` — Documentation only
- `ci:` — CI/CD changes
- `refactor:` — Code change that neither fixes a bug nor adds a feature
- `test:` — Adding or updating tests
- `chore:` — Maintenance tasks

## Design Principles

1. **Stdlib only** — No external Go dependencies
2. **Fail closed** — Ambiguous operations error instead of guessing
3. **Stdout purity** — Protocol only on stdout, logs on stderr
4. **Typed models** — Prefer structs over `map[string]any` for stable entities
5. **Safety first** — Destructive tools must have policy + dry-run + tests
6. **Graceful shutdown** — Respect context and drain in-flight requests

## Questions?

Open a [discussion](https://github.com/apet97/go-clockify/discussions) or file an [issue](https://github.com/apet97/go-clockify/issues).
