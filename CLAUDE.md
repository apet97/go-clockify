# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

GOCLMCP is a production-grade Go MCP (Model Context Protocol) server for Clockify. It exposes 124 Clockify tools (33 Tier 1 tools registered at startup + 91 Tier 2 tools activated on demand) over stdio and HTTP JSON-RPC transports, intended for use with Claude Desktop, Cursor, and similar MCP clients.

## Build / Test / Run

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run a single package's tests
go test ./internal/tools/...
go test ./internal/mcp/...

# Format
gofmt -w ./cmd ./internal ./tests

# Run opt-in live Clockify E2E tests
CLOCKIFY_RUN_LIVE_E2E=1 CLOCKIFY_API_KEY=xxx go test -tags livee2e ./tests

# Run server — stdio mode (default)
CLOCKIFY_API_KEY=xxx go run ./cmd/clockify-mcp

# Run server — HTTP mode
CLOCKIFY_API_KEY=xxx MCP_TRANSPORT=http MCP_BEARER_TOKEN=secret go run ./cmd/clockify-mcp

# Show help and all env vars
go run ./cmd/clockify-mcp --help
```

Go 1.25.0, stdlib only — zero external dependencies. Module path: `github.com/apet97/go-clockify`.

## Environment Variables

### Core
| Variable | Required | Default | Purpose |
|---|---|---|---|
| `CLOCKIFY_API_KEY` | Yes | — | Clockify API key |
| `CLOCKIFY_WORKSPACE_ID` | No | auto-resolve | Workspace ID (auto if only one) |
| `CLOCKIFY_BASE_URL` | No | `https://api.clockify.me/api/v1` | API base URL |
| `CLOCKIFY_REPORTS_URL` | No | — | Separate reports API host |
| `CLOCKIFY_TIMEZONE` | No | system | IANA timezone for natural language time parsing |
| `CLOCKIFY_INSECURE` | No | `0` | Set `1` for non-loopback HTTP |

### Safety & Control
| Variable | Default | Purpose |
|---|---|---|
| `CLOCKIFY_POLICY` | `standard` | `read_only`, `safe_core`, `standard`, `full` |
| `CLOCKIFY_DENY_TOOLS` | — | Comma-separated tool names to block |
| `CLOCKIFY_DENY_GROUPS` | — | Comma-separated Tier 2 groups to block |
| `CLOCKIFY_ALLOW_GROUPS` | — | Whitelist of allowed Tier 2 groups |
| `CLOCKIFY_DRY_RUN` | enabled | Set `off` to disable dry-run |
| `CLOCKIFY_DEDUPE_MODE` | `warn` | `warn`, `block`, `off` |
| `CLOCKIFY_DEDUPE_LOOKBACK` | `25` | Recent entries to check for duplicates |
| `CLOCKIFY_OVERLAP_CHECK` | `true` | Detect overlapping time entries |
| `CLOCKIFY_BOOTSTRAP_MODE` | `full_tier1` | `full_tier1`, `minimal`, `custom` |
| `CLOCKIFY_BOOTSTRAP_TOOLS` | — | Comma-separated tools for custom mode |
| `CLOCKIFY_MAX_CONCURRENT` | `10` | Max concurrent tool calls (`0` disables this layer) |
| `CLOCKIFY_RATE_LIMIT` | `120` | Max calls per 60s window (`0` disables this layer) |
| `CLOCKIFY_TOKEN_BUDGET` | `8000` | Token truncation budget (0=off) |

### Transport
| Variable | Default | Purpose |
|---|---|---|
| `MCP_TRANSPORT` | `stdio` | `stdio` or `http` |
| `MCP_HTTP_BIND` | `:8080` | HTTP listen address |
| `MCP_BEARER_TOKEN` | — | Required for HTTP mode (`Authorization: Bearer <token>`) |
| `MCP_ALLOWED_ORIGINS` | — | Comma-separated CORS origins |
| `MCP_ALLOW_ANY_ORIGIN` | — | Set `1` to allow all origins |
| `MCP_HTTP_MAX_BODY` | `2097152` | Positive max request body (bytes) |
| `MCP_LOG_FORMAT` | `text` | `text` or `json` (stderr) |
| `MCP_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Architecture

```
cmd/clockify-mcp/main.go           Entrypoint — wires 8 subsystems, transport selection
internal/
  config/         Config from env vars, URL validation
  clockify/       HTTP client (Retry-After backoff, generic pagination, typed errors), entity models
  mcp/
    server.go       Stdio JSON-RPC server with enforcement pipeline (async tools/call dispatch)
    types.go        MCP protocol types (Request, Response, Tool, ToolDescriptor)
    transport_http.go  HTTP transport (bearer auth, CORS, health/ready, security headers)
  tools/
    common.go       Service struct (with lazy user/workspace cache), ResultEnvelope, helpers
    registry.go     Tier 1 tool registration (33 tools)
    {domain}.go     Domain handlers: users, workspaces, projects, clients, tags, tasks,
                    entries, timer, reports, workflows, context
    tier2_catalog.go   Tier 2 group catalog and activation
    tier2_{domain}.go  11 domain files: invoices, expenses, scheduling, time_off,
                       approvals, shared_reports, user_admin, webhooks,
                       custom_fields, groups_holidays, project_admin
  policy/         Policy modes (read_only/safe_core/standard/full), group control
  resolve/        Name-to-ID resolution with email detection, ambiguity blocking
  dryrun/         3-strategy dry-run: confirm pattern, GET preview, minimal fallback
  bootstrap/      Tool visibility modes (FullTier1/Minimal/Custom), searchable catalog
  ratelimit/      Dual control: semaphore concurrency + window-based throughput (race-safe)
  truncate/       Progressive token-aware output truncation
  dedupe/         Duplicate entry detection + time overlap checking
  timeparse/      Natural language time parsing ("now", "today 14:30", "2h30m", ISO 8601)
  helpers/        Error message mapping, paginated results, write envelopes
```

### Server Enforcement Pipeline

Every `tools/call` passes through this pipeline in order:
1. **Init guard** → reject with `-32002` if `initialize` has not been called
2. **Policy check** → blocked? return `isError: true` with human-readable reason
3. **Rate limit** → acquire semaphore + window permit, defer release
4. **Dry-run intercept** → if `dry_run=true`, route to preview strategy (before handler)
5. **Handler dispatch** → call the tool handler
6. **Truncation** → post-process if result exceeds token budget
7. **Logging** → `slog` to stderr with tool name, duration, and request ID

Tool errors return as `result.isError: true` per the MCP spec (not JSON-RPC `error`). Protocol errors (unknown method, invalid JSON, init guard) use JSON-RPC `error`.

### Tool Tiers

**Tier 1 (33 tools):** Always registered. Visibility controlled by bootstrap mode. Includes CRUD for entries/projects/clients/tags/tasks, timer control, reports, workflows, and context/discovery tools.

**Tier 2 (91 tools, 11 groups):** Registered lazily via `clockify_search_tools` activation. Groups: invoices (12), expenses (10), scheduling (10), time_off (12), approvals (6), shared_reports (6), user_admin (8), webhooks (7), custom_fields (6), groups_holidays (8), project_admin (6).

### Key Design Decisions

- **Stdlib only.** Zero external dependencies. Uses `log/slog` for structured logging, `net/http` for HTTP transport, `crypto/subtle` for constant-time auth, `math/rand/v2` for jitter.
- **Stdout purity.** Protocol responses only on stdout. All logs go to stderr via slog.
- **ResultEnvelope.** Every tool returns `{ok, action, data, meta}`. Write tools use `helpers.WriteResult`.
- **Fail closed.** Ambiguous name resolution errors. Multiple matches are rejected. Destructive tools require policy + dry-run.
- **Lazy caching.** `Service` caches current user and workspace ID with `sync.Mutex` to avoid redundant API calls.
- **Flat package layout.** All Tier 1 and Tier 2 tools live in `package tools` with domain-named files. No sub-packages needed.
- **Context-aware shutdown.** Stdio loop exits cleanly on SIGTERM via goroutine + `select` on `ctx.Done()`. HTTP server uses `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout`.

### Tool Registration

Tier 1: `internal/tools/registry.go` via `Service.Registry()` returning `[]mcp.ToolDescriptor`. Use `toolRO()`, `toolRW()`, `toolDestructive()`.

Tier 2: Each `tier2_*.go` file self-registers via `init()` calling `registerTier2Group()`. Activated at runtime via `clockify_search_tools` -> `Service.ActivateGroup` / `Service.ActivateTool` -> `server.ActivateGroup()` / `server.ActivateTier1Tool()`.

### Clockify Client

`internal/clockify/client.go` — stdlib HTTP client with `X-Api-Key` auth, `Retry-After` compliance for 429/503 limits, generic `ListAll[T any]` pagination, typed `APIError`. Methods: `Get`, `Post`, `Put`, `Patch`, `Delete`. Response body limited to 10MB.

### Name Resolution

`internal/resolve/` — 24-char hex passthrough, `strict-name-search=true` API queries, case-insensitive matching, email detection for users, actionable error messages with list-tool suggestions.

## Testing

The repo uses unit, integration, golden, HTTP transport, and opt-in live E2E tests. Patterns:

```go
// Mock Clockify API via httptest
client, cleanup := newTestClient(t, handler)
defer cleanup()
svc := New(client, "ws1")
```

Key test files:
- `internal/tools/golden_test.go` — golden tool list (33 Tier 1 names), Tier 2 catalog (11 groups, 91 tools), schema validation, annotation consistency
- `internal/mcp/integration_test.go` — full MCP handshake, policy filtering, bootstrap modes, truncation, dry-run pipeline, init guard, isError response format
- `internal/tools/*_test.go` — per-domain handler tests
- `internal/mcp/transport_http_test.go` — HTTP auth, CORS, health/ready, security headers

## Constraints

- No external Go dependencies — stdlib only
- No stdout pollution — protocol responses only
- No fuzzy/guessed destructive updates — fail closed
- Destructive tools must have policy + dry-run + tests
- Typed models for stable entities; `map[string]any` acceptable for Tier 2
- Tool errors use `isError: true` (MCP spec); protocol errors use JSON-RPC `error`
