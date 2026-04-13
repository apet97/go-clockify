# Contributing to Clockify MCP Server (Go)

## Development Setup

```sh
git clone https://github.com/apet97/go-clockify.git
cd go-clockify
go build ./...
```

Requires Go 1.25.9+. No external dependencies. Module path: `github.com/apet97/go-clockify`.

## Running Tests

```sh
# All tests
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

Two local targets:

```sh
make check   # fast inner loop (seconds): gofmt + go vet + go test
make verify  # full pipeline (minutes): mirrors PR-blocking CI jobs
```

`make check` is for the edit-test cycle. `make verify` is what you run
before opening a PR — it exercises every PR-blocking CI step that can
run on a contributor laptop.

### What `make verify` runs locally

| Step | Tool | Local behavior |
|---|---|---|
| `fmt` | gofmt | always runs |
| `vet` | go vet | always runs |
| `lint` | golangci-lint | always runs (install via `brew install golangci-lint`) |
| `test` | go test -race | always runs |
| `cover-check` | go test -coverprofile + floors | always runs |
| `fuzz-short` | go test -fuzz | 10s per target (CI uses 30s) |
| `build-tags` | scripts/check-build-tags.sh | otel/grpc/pprof symbol + go.mod parity checks |
| `http-smoke` | scripts/smoke-http.sh | builds binary, curls `/health` + `/ready` |
| `config-parity` | scripts/check-config-parity.sh | env-var parity across config.go / Helm / Kustomize |
| `verify-vuln` | govulncheck | **skips** with a warning if not installed |
| `verify-k8s` | kubeconform + helm | **skips** with a warning if any tool missing |
| `verify-fips` | GOFIPS140=latest | **soft-fails** locally if toolchain lacks FIPS support |

### What CI runs that `make verify` does not

- **Trivy container scan** — CI-only (Docker image workflow)
- **Full 30s fuzz budget** — CI uses 30s per target; local uses 10s
- **Mutation testing** — nightly workflow, not PR-blocking

If `make verify` passes locally you have high confidence CI will pass, but
the definitive answer always comes from the CI workflow on your PR.

All checks must pass with no errors.

## Project Structure

```
cmd/clockify-mcp/     Entrypoint — wires all layers
internal/
  mcp/                Protocol core — pure JSON-RPC/MCP engine, Enforcement/Activator interfaces
  clockify/           HTTP client (connection pooling, retry/backoff, pagination)
  tools/              All 124 tool handlers (Tier 1 registry + Tier 2 lazy groups)
  enforcement/        Composes policy, rate limit, dry-run, truncation into Enforcement interface
  config/             Environment variable configuration (fail-fast validation)
  policy/             Policy modes (read_only/safe_core/standard/full)
  resolve/            Name-to-ID resolution
  dryrun/             Dry-run interception strategies
  bootstrap/          Tool visibility modes, searchable catalog
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

## Releases

Versioning, support window, deprecation policy, and the definition of
"breaking change" used by this project live in
[docs/release-policy.md](docs/release-policy.md). Read it before
proposing a change to a public surface (tools, env vars, CLI flags,
protocol version).

## Questions?

Open a [discussion](https://github.com/apet97/go-clockify/discussions) or file an [issue](https://github.com/apet97/go-clockify/issues).
