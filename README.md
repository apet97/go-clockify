# clockify-mcp-go

MCP server for Clockify, built in Go. 124 tools total: 33 Tier 1 tools registered at startup and 91 Tier 2 tools activated on demand across 11 domains.

Zero external dependencies. Single static binary.

## Quickstart

### Install

**Go** (from source):

```sh
go install github.com/apet97/go-clockify/cmd/clockify-mcp@latest
```

**npm** (prebuilt binaries):

```sh
npx @anycli/clockify-mcp-go
```

**GitHub Releases** — download a prebuilt binary from [Releases](https://github.com/apet97/go-clockify/releases).

### Configure

Set your API key:

```sh
export CLOCKIFY_API_KEY=your-key
```

**Claude Desktop** — add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "clockify": {
      "command": "clockify-mcp",
      "env": { "CLOCKIFY_API_KEY": "your-key" }
    }
  }
}
```

If installed via npm:

```json
{
  "mcpServers": {
    "clockify": {
      "command": "npx",
      "args": ["@anycli/clockify-mcp-go"],
      "env": { "CLOCKIFY_API_KEY": "your-key" }
    }
  }
}
```

**Cursor** — add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "clockify": {
      "command": "clockify-mcp",
      "env": { "CLOCKIFY_API_KEY": "your-key" }
    }
  }
}
```

### Verify Installation

```sh
clockify-mcp --version
clockify-mcp --help
```

Or send an MCP `initialize` request:

```sh
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}' \
  | CLOCKIFY_API_KEY=your-key clockify-mcp 2>/dev/null \
  | head -1
```

A valid JSON-RPC response confirms the server is working. If your API key only has access to one workspace, `CLOCKIFY_WORKSPACE_ID` can be omitted.

## Architecture

```
cmd/clockify-mcp/main.go           Entrypoint — wires 8 subsystems, transport selection
internal/
  config/         Config from env vars, URL validation
  clockify/       HTTP client (retry/backoff, pagination, typed errors), entity models
  mcp/
    server.go       Stdio JSON-RPC server with enforcement pipeline (context-aware shutdown)
    types.go        MCP protocol types (Request, Response, Tool, ToolDescriptor)
    transport_http.go  HTTP transport (bearer auth, CORS, health/ready, security headers, timeouts)
  tools/
    common.go       Service struct (with lazy user/workspace cache), ResultEnvelope, helpers
    registry.go     Tier 1 tool registration (33 tools)
    {domain}.go     Domain handlers: users, workspaces, projects, clients, tags, tasks,
                    entries, timer, reports, workflows, context
    tier2_catalog.go   Tier 2 group catalog and activation
    tier2_{domain}.go  11 domain files
  policy/         Policy modes (read_only/safe_core/standard/full), group control
  resolve/        Name-to-ID resolution with email detection, ambiguity blocking
  dryrun/         3-strategy dry-run: confirm, GET preview, minimal fallback
  bootstrap/      Tool visibility modes (FullTier1/Minimal/Custom), searchable catalog
  ratelimit/      Dual control: semaphore concurrency + window-based throughput (race-safe)
  truncate/       Progressive token-aware output truncation
  dedupe/         Duplicate entry detection + time overlap checking
  timeparse/      Natural language time parsing ("now", "today 14:30", ISO 8601)
  helpers/        Error message mapping, paginated results, write envelopes
```

### Server Enforcement Pipeline

Every `tools/call` passes through this pipeline in order:
1. **Init guard** → reject with `-32002` if server not yet initialized
2. **Policy check** → blocked? return `isError: true` with human-readable reason
3. **Rate limit** → acquire semaphore + window permit, defer release
4. **Dry-run intercept** → if `dry_run=true`, route to preview strategy (before handler)
5. **Handler dispatch** → call the tool handler
6. **Truncation** → post-process if result exceeds token budget
7. **Logging** → `slog` to stderr with tool name, duration, and request ID

Tool errors return as `result.isError: true` per the MCP spec (not JSON-RPC `error`). Protocol errors (unknown method, invalid JSON, init guard) use JSON-RPC `error`.

## Tool Domains

**Tier 1 (always loaded):** timer, entries, projects, clients, tags, tasks, users, workspaces, reports, workflows, search, context.

**Tier 2 (on demand):** invoices, expenses, scheduling, time off, approvals, shared reports, user admin, webhooks, custom fields, groups/holidays, project admin.

Use `clockify_search_tools` to discover and activate Tier 2 groups or a specific hidden tool. Activation updates `tools/list` at runtime.

See [docs/tool-catalog.md](docs/tool-catalog.md) for the complete tool list.

## Policy Modes

Control tool visibility based on trust level. Set via `CLOCKIFY_POLICY`:

| Mode | Read | Write | Delete | Tier 2 | Use Case |
|------|------|-------|--------|--------|----------|
| `read_only` | yes | no | no | no | Untrusted agents — observe only |
| `safe_core` | yes | allowlist | no | no | Day-to-day time tracking |
| `standard` | yes | yes | yes | on demand | **Default** — balanced |
| `full` | yes | yes | yes | yes | Admin and automation |

Fine-grained overrides:

- `CLOCKIFY_DENY_GROUPS` — comma-separated domain groups to block
- `CLOCKIFY_ALLOW_GROUPS` — comma-separated allowed groups (overrides mode default)
- `CLOCKIFY_DENY_TOOLS` — comma-separated tool names to block

Introspection tools (`clockify_whoami`, `clockify_policy_info`, `clockify_search_tools`, `clockify_resolve_debug`) are always available regardless of policy.

See [docs/safe-usage.md](docs/safe-usage.md) for the complete safety guide.

## Configuration

### Core

| Variable | Default | Description |
|----------|---------|-------------|
| `CLOCKIFY_API_KEY` | — | API key (**required**) |
| `CLOCKIFY_WORKSPACE_ID` | auto | Workspace ID (auto-detected if only one) |
| `CLOCKIFY_BASE_URL` | `https://api.clockify.me/api/v1` | API base URL |
| `CLOCKIFY_REPORTS_URL` | — | Reports API URL |
| `CLOCKIFY_TIMEZONE` | system | IANA timezone for time parsing |
| `CLOCKIFY_INSECURE` | — | Set to `1` to allow non-HTTPS base URLs (loopback exempt) |

### Safety

| Variable | Default | Description |
|----------|---------|-------------|
| `CLOCKIFY_POLICY` | `standard` | `read_only`, `safe_core`, `standard`, `full` |
| `CLOCKIFY_DENY_TOOLS` | — | Comma-separated tools to block |
| `CLOCKIFY_DENY_GROUPS` | — | Comma-separated groups to block |
| `CLOCKIFY_ALLOW_GROUPS` | — | Comma-separated allowed groups |
| `CLOCKIFY_DRY_RUN` | `enabled` | Dry-run for destructive tools |
| `CLOCKIFY_DEDUPE_MODE` | `warn` | Duplicate detection: `warn`, `block`, `off` |
| `CLOCKIFY_DEDUPE_LOOKBACK` | `25` | Recent entries to check |
| `CLOCKIFY_OVERLAP_CHECK` | `true` | Overlapping entry detection |

### Performance

| Variable | Default | Description |
|----------|---------|-------------|
| `CLOCKIFY_MAX_CONCURRENT` | `10` | Concurrent tool call limit (`0` disables concurrency limiting) |
| `CLOCKIFY_RATE_LIMIT` | `120` | Tool calls per minute (`0` disables window limiting) |
| `CLOCKIFY_TOKEN_BUDGET` | `8000` | Response token budget (0 = off) |

### Bootstrap

| Variable | Default | Description |
|----------|---------|-------------|
| `CLOCKIFY_BOOTSTRAP_MODE` | `full_tier1` | `full_tier1`, `minimal`, `custom` |
| `CLOCKIFY_BOOTSTRAP_TOOLS` | — | Tool list for `custom` mode |

### Transport

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_TRANSPORT` | `stdio` | `stdio` or `http` |
| `MCP_HTTP_BIND` | `:8080` | HTTP listen address |
| `MCP_BEARER_TOKEN` | — | Required for HTTP mode; clients must send `Authorization: Bearer <token>` |
| `MCP_ALLOWED_ORIGINS` | — | Comma-separated CORS origins (rejected if unset) |
| `MCP_ALLOW_ANY_ORIGIN` | — | Set `1` to allow all origins |
| `MCP_HTTP_MAX_BODY` | `2097152` | Positive max request body (bytes) |
| `MCP_LOG_FORMAT` | `text` | `text` or `json` (stderr) |
| `MCP_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Common Workflows

### Start and stop a timer

```
→ clockify_start_timer { "project": "My Project" }
← { "ok": true, "action": "timer_started", "data": { "id": "abc123" } }

→ clockify_stop_timer {}
← { "ok": true, "action": "timer_stopped", "data": { "id": "abc123" } }
```

### Log time

```
→ clockify_log_time { "project": "Project Alpha", "start": "today 9:00", "end": "today 11:00", "description": "Code review" }
← { "ok": true, "action": "entry_created", "data": { "entry": { ... } } }
```

### Activate a Tier 2 domain

```
→ clockify_search_tools { "query": "invoices" }
← { "count": 1, "all_results": [{ "type": "group", "name": "invoices", "tool_count": 12, "availability": "tier2" }] }

→ clockify_search_tools { "activate_group": "invoices" }
← { "activated": "invoices", "activation_type": "group", "group": "invoices", "tool_count": 12, "activation_message": "Activated 12 tools from group \"invoices\"" }
```

### Dry-run a destructive operation

```
→ clockify_delete_entry { "entry_id": "abc123", "dry_run": true }
← { "dry_run": true, "preview": { "id": "abc123", "description": "Meeting" }, "note": "No changes were made." }
```

## Docker

Build and run with HTTP transport:

```sh
docker build -f deploy/Dockerfile -t clockify-mcp .
docker run -p 8080:8080 \
  -e CLOCKIFY_API_KEY=your-key \
  -e MCP_BEARER_TOKEN=your-secret-token \
  clockify-mcp
```

Or use Docker Compose:

```sh
cd deploy
cp ../examples/docker-compose.env .env
# Edit .env with your values
docker compose up
```

This starts the MCP server on port 8080 with a Caddy reverse proxy on port 443 for TLS termination. Edit `deploy/Caddyfile` to set your domain.

See [docs/http-transport.md](docs/http-transport.md) for the full HTTP transport guide.

## Build / Test / Run

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Format
gofmt -w ./cmd ./internal ./tests

# Run opt-in live Clockify E2E tests
CLOCKIFY_RUN_LIVE_E2E=1 CLOCKIFY_API_KEY=xxx go test -tags livee2e ./tests

# Run server — stdio mode (default)
CLOCKIFY_API_KEY=xxx go run ./cmd/clockify-mcp

# Run server — HTTP mode
CLOCKIFY_API_KEY=xxx MCP_TRANSPORT=http MCP_BEARER_TOKEN=secret go run ./cmd/clockify-mcp

# Build with version
go build -ldflags "-X main.version=v0.3.0" ./cmd/clockify-mcp

# Show all env vars
clockify-mcp --help
```

Go 1.25.0, stdlib only — zero external dependencies. Module path: `github.com/apet97/go-clockify`.

## Compatibility

| Component | Version |
|-----------|---------|
| MCP Protocol | `2025-06-18` |
| Claude Desktop | latest |
| Cursor | latest |
| Other MCP clients | any supporting stdio or Streamable HTTP |
| Go | 1.25.0+ |
| Node.js (npm wrapper) | 16+ |

## Troubleshooting

**No tools visible** — Check `CLOCKIFY_BOOTSTRAP_MODE`. In `minimal` mode, most tools are hidden. Use `clockify_search_tools` to discover them.

**401 Unauthorized** — API key is invalid or expired. Generate a new one at [Clockify Profile Settings](https://app.clockify.me/user/preferences#advanced).

**403 Forbidden** — Your Clockify user lacks permissions for this operation.

**Multiple workspaces** — Set `CLOCKIFY_WORKSPACE_ID` explicitly.

**Rate limited (429)** — The server retries 429s automatically by explicitly honoring Clockify's `Retry-After` response headers, keeping usage safe.

**Tool not found** — It may be a Tier 2 tool. Use `clockify_search_tools` to find and activate its domain group.

**Dry-run not working** — Ensure `CLOCKIFY_DRY_RUN=enabled` (default). Pass `"dry_run": true` in tool call parameters.

**HTTP connection refused** — Verify `MCP_HTTP_BIND` and `MCP_BEARER_TOKEN` are set correctly.

**Stale tool list** — The server sends `tools/list_changed` after group activation. Your client must re-fetch `tools/list`.

## Documentation

- [Tool Catalog](docs/tool-catalog.md) — all 124 tools
- [Safe Usage](docs/safe-usage.md) — policy, dry-run, dedupe, rate limiting
- [HTTP Transport](docs/http-transport.md) — setup, auth, CORS, Docker
- [Tool Annotations](docs/tool-annotations.md) — readOnlyHint, destructiveHint, idempotentHint

## Support

- Bug reports and feature requests: [GitHub Issues](https://github.com/apet97/go-clockify/issues)
- Security vulnerabilities: see [SECURITY.md](SECURITY.md)

## License

MIT
