# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Client Reliability**: Clockify API client now accurately listens to `Retry-After` HTTP headers on 429 errors.
- **Server Concurrency**: Asynchronous multiplexing inside `stdio` transport using goroutines for `tools/call` requests.
- **Generic Pagination**: Cleanly typed internal API pagination (`ListAll[T any]`) instead of vulnerable map casts.
- **Data Safety**: `server.initialized` is now safeguarded with `atomic.Bool` to prevent read/write lifecycle panics.

## [0.3.0] - 2026-04-06

### Added

- **Module path**: Changed to `github.com/apet97/go-clockify` — `go install` now works
- **`--help` flag**: Prints all 25+ environment variables with descriptions
- **`MCP_LOG_LEVEL`**: Configurable log level (`debug`, `info`, `warn`, `error`)
- **Init guard**: `tools/call` before `initialize` returns error code `-32002`
- **`isError` responses**: Tool errors now return `result.isError: true` per MCP spec (not JSON-RPC `error`)
- **Request ID correlation**: Monotonic counter for log entry correlation across tool calls
- **HTTP server timeouts**: `ReadHeaderTimeout` (10s), `ReadTimeout` (30s), `WriteTimeout` (60s), `IdleTimeout` (120s)
- **Security headers**: `X-Content-Type-Options: nosniff` and `Cache-Control: no-store` on all HTTP responses
- **JSON error responses**: HTTP error bodies are now JSON instead of `text/plain`
- **Structured access logging**: HTTP requests logged with method, path, rpc_method, status, req_id, duration_ms
- **`Patch()` method**: Added to Clockify client for Tier 2 tools requiring PATCH
- **Response body limit**: 10MB limit on Clockify API responses to prevent OOM
- **`TestToolCallBeforeInitialize`**: New integration test for the initialization guard

### Changed

- Version bumped to `0.3.0`
- Stdio loop now uses goroutine + `select` on `ctx.Done()` for context-aware shutdown
- HTTP shutdown consolidated in main.go (removed duplicate `signal.NotifyContext` from transport)
- Graceful HTTP shutdown with 10s drain timeout
- Encoder writes protected by `sync.Mutex` for thread-safe notifications
- Replaced deprecated `math/rand.Intn` with `math/rand/v2.IntN` for backoff jitter
- Startup log now includes transport, workspace, and bootstrap mode
- Client User-Agent updated to `clockify-mcp-go/0.3.0`
- Integration tests updated for `isError` response format

### Fixed

- **Rate limiter race condition**: Window reset (`windowStart.Store` + `windowCount.Store(0)`) was not atomic — two goroutines could both see an expired window, both reset, and lose a count. Fixed with `sync.Mutex` + double-checked locking.
- **Stdio shutdown hang**: Server could block indefinitely on `scanner.Scan()` when SIGTERM arrived with idle stdin. Now exits cleanly via context cancellation.
- **Notification errors silently dropped**: `encoder.Encode` errors for `tools/list_changed` are now logged.

### Security

- HTTP error responses now use `Content-Type: application/json` (prevents content-type sniffing)
- Added `X-Content-Type-Options: nosniff` to all HTTP responses
- Added `Cache-Control: no-store` to MCP HTTP responses

## [0.2.0] - 2026-04-06

### Added

- GitHub Actions CI pipeline (format, vet, build, test with race detector, HTTP smoke test)
- GitHub Actions release workflow (5-platform binaries, cosign signing, SBOM, npm publish)
- Docker deployment (distroless multi-stage build, docker-compose, Caddy TLS)
- npm binary distribution (`@anycli/clockify-mcp-go`)
- `--version` flag support
- Signal handling (SIGINT/SIGTERM) for graceful shutdown
- CORS security fix: cross-origin requests rejected by default
- `MCP_ALLOW_ANY_ORIGIN` environment variable for explicit CORS opt-in
- Documentation: tool catalog, safe usage guide, HTTP transport guide, tool annotations
- Example configs: Claude Desktop, Cursor, Docker Compose environment
- Community files: SECURITY.md, CONTRIBUTING.md, LICENSE, issue templates, PR template
- `.env.example` with all configuration options documented
- Dependabot configuration for Go modules

### Changed

- Client User-Agent now uses build version instead of hardcoded `0.1.0`
- Error response body read limited to 64KB (prevents OOM on malicious responses)
- HTTP 502/503/504 now retried with backoff (matching 429/5xx behavior)

### Fixed

- CORS allowing all origins by default when `MCP_ALLOWED_ORIGINS` not set (security fix)

## [0.1.0] - 2026-04-06

### Added

- 124 tools across 12 domain groups (33 Tier 1 + 91 Tier 2)
- Tiered tool loading: core tools at startup, domain groups on demand via `clockify_search_tools`
- Policy modes: `read_only`, `safe_core`, `standard`, `full`
- Dry-run support for destructive tools (3 strategies: confirm, preview, minimal)
- Duplicate entry detection (warn/block/off) + time overlap checking
- Name-to-ID resolution with ambiguity blocking for projects, clients, tags, users, tasks
- Bootstrap modes: `full_tier1`, `minimal`, `custom`
- Token-aware output truncation (progressive: strip nulls, empties, truncate strings, reduce arrays)
- MCP-layer rate limiting (concurrent + per-window) and concurrency control
- Natural language date/time parsing (`now`, `today 14:30`, `yesterday`, ISO 8601)
- ISO 8601 duration parsing (`PT1H30M`)
- Structured audit logging for write operations (`slog` to stderr)
- Stdio and HTTP transports with bearer auth and CORS
- Health (`/health`) and readiness (`/ready`) endpoints for HTTP mode
- Graceful shutdown for HTTP transport (SIGINT/SIGTERM)
- Config validation (API key required, HTTPS enforcement, workspace auto-resolve)
- `tools/list_changed` notifications on Tier 2 group activation
- 5 workflow shortcuts: log time, switch project, weekly summary, quick report, find and update
- ResultEnvelope consistency across all tool handlers (`{ok, action, data, meta}`)
- 265 tests across 13 packages (unit, integration, golden, HTTP transport)

### Security

- API keys via environment variables only
- Constant-time bearer token comparison (`crypto/subtle`)
- ID validation rejects path traversal characters
- Non-HTTPS base URLs blocked unless loopback or `CLOCKIFY_INSECURE=1`
- Zero external dependencies (stdlib only)

[Unreleased]: https://github.com/apet97/go-clockify/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/apet97/go-clockify/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/apet97/go-clockify/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/apet97/go-clockify/releases/tag/v0.1.0
