# clockify-mcp-go

Production-grade MCP server for Clockify, built in Go. 124 tools total: 33 Tier 1 registered at startup and 91 Tier 2 activated on demand across 11 domains.

Zero external dependencies. Single static binary. Every release is signed with cosign keyless OIDC, ships a SPDX SBOM, and carries SLSA build provenance for both binaries and container images.

**Highlights**

- **Layered architecture** — protocol core, Clockify client, tool surface, safety layer. `Enforcement` / `Activator` / `Notifier` interfaces keep the protocol core domain-free.
- **MCP protocol negotiation** — parses `InitializeParams`, negotiates against `{2025-06-18, 2025-03-26, 2024-11-05}`, advertises `tools.listChanged` only on transports that can actually push it, and returns an `instructions` string for agentic clients.
- **Four policy modes** (`read_only`, `safe_core`, `standard`, `full`) + per-tool/group deny/allow lists + three-strategy dry-run for every destructive operation.
- **Bounded dispatch + dual rate limiting** — stdio dispatch semaphore, per-process concurrency semaphore, and fixed-window throughput limiter. Neither layer can strand resources in the other.
- **Observability built in** — Prometheus metrics, upstream Clockify metrics, Go runtime + process metrics, panic counters, and protocol-error counters. Shared-service deployments can isolate metrics onto a dedicated listener with `MCP_METRICS_BIND`.
- **PII-redacting structured logs** — every slog handler is wrapped in a recursive scrubber that masks 20+ secret-key patterns before they reach the encoder.
- **Hardened Kubernetes reference manifests** — non-root distroless pod, read-only root FS, dropped ALL capabilities, NetworkPolicy (default-deny), PodDisruptionBudget, dedicated ServiceAccount, image pinned by version.
- **Multi-arch container image pipeline** — buildx → Trivy scan → cosign keyless sign → SPDX SBOM → SLSA provenance attested to the registry.

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
cmd/clockify-mcp/main.go           Entrypoint — wires layers, transport selection
internal/
  config/         Config from env vars, URL validation
  enforcement/    Concrete Enforcement + Activator (composes policy, rate limit, dry-run, truncation)
  clockify/       HTTP client (connection pooling, retry/backoff, pagination, typed errors)
  mcp/
    server.go       Pure JSON-RPC/MCP engine — zero domain imports, pluggable Enforcement interface
    types.go        MCP protocol types + Enforcement/Activator interfaces
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
  ratelimit/      Dual control: semaphore concurrency + fixed-window throughput (race-safe)
  truncate/       Progressive token-aware output truncation
  dedupe/         Duplicate entry detection + time overlap checking
  timeparse/      Natural language time parsing ("now", "today 14:30", ISO 8601)
  helpers/        Error message mapping, paginated results, write envelopes
```

### Layered Architecture

The server is structured in four clean layers:

1. **Protocol core** (`mcp/`) — pure JSON-RPC/MCP engine with zero domain imports. Pluggable via `Enforcement` and `Activator` interfaces.
2. **Clockify client** (`clockify/`) — stdlib HTTP client with connection pooling, retry/backoff, pagination, and `Close()`.
3. **Tool surface** (`tools/`) — 33 Tier 1 tools in a declarative registry, 91 Tier 2 tools across 11 lazy-loaded groups.
4. **Safety layer** (`enforcement/`) — composes policy, rate limiting, dry-run, truncation, and bootstrap into the interfaces consumed by the protocol core.

### Enforcement Pipeline

Every `tools/call` is gated by the `Enforcement` interface:
1. **Init guard** → reject with `-32002` if server not yet initialized (protocol core)
2. **`BeforeCall`** → policy check, rate limit acquire, dry-run intercept (enforcement layer)
3. **Handler dispatch** → call the tool handler with 45s context timeout (protocol core)
4. **`AfterCall`** → truncation post-processing (enforcement layer)
5. **Logging** → `slog` to stderr with tool name, duration, and request ID (protocol core)

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
| `CLOCKIFY_TIMEZONE` | system | IANA timezone for time parsing (used as default when no per-request timezone is provided) |
| `CLOCKIFY_INSECURE` | — | Set to `1` to allow non-HTTPS base URL on non-loopback hosts. Note: this bypasses URL scheme validation only — it does NOT disable TLS certificate verification in the HTTP client. |

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
| `CLOCKIFY_CONCURRENCY_ACQUIRE_TIMEOUT` | `100ms` | How long to wait for a concurrency slot before rejecting the call. Must be between `1ms` and `30s`. |
| `CLOCKIFY_RATE_LIMIT` | `120` | Tool calls per fixed 60s window (`0` disables window limiting) |
| `CLOCKIFY_PER_TOKEN_RATE_LIMIT` | `60` | Tool calls per 60s window per authenticated `Principal.Subject`. Applies only to requests that carry a principal (OIDC, forward-auth, mTLS). `0` disables the per-token layer. |
| `CLOCKIFY_PER_TOKEN_CONCURRENCY` | `5` | Max in-flight tool calls per `Principal.Subject`. `0` disables. |
| `CLOCKIFY_TOKEN_BUDGET` | `8000` | Response token budget (0 = off) |
| `MCP_MAX_INFLIGHT_TOOL_CALLS` | `64` | Stdio dispatch-layer goroutine cap. Acquired before goroutine spawn, independent of business rate limiting. `0` disables. |
| `CLOCKIFY_REPORT_MAX_ENTRIES` | `10000` | Hard cap on entries aggregated by report tools. When `include_entries=true` and the range exceeds the cap, the tool fails closed with an actionable error. `0` disables the cap. |

### Bootstrap

| Variable | Default | Description |
|----------|---------|-------------|
| `CLOCKIFY_BOOTSTRAP_MODE` | `full_tier1` | `full_tier1`, `minimal`, `custom` |
| `CLOCKIFY_BOOTSTRAP_TOOLS` | — | Tool list for `custom` mode |

### Transport

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_TRANSPORT` | `stdio` | `stdio`, compatibility `http`, or session-aware `streamable_http` |
| `MCP_AUTH_MODE` | transport-dependent | `static_bearer`, `oidc`, `forward_auth`, `mtls` |
| `MCP_HTTP_BIND` | `:8080` | HTTP listen address |
| `MCP_BEARER_TOKEN` | — | Required for `static_bearer`; clients send `Authorization: Bearer <token>` |
| `MCP_ALLOWED_ORIGINS` | — | Comma-separated CORS origins (rejected if unset) |
| `MCP_ALLOW_ANY_ORIGIN` | — | Set `1` to allow all origins |
| `MCP_STRICT_HOST_CHECK` | — | Set `1` to enforce DNS rebinding protection: the inbound `Host` header must match `localhost`, `127.0.0.1`, `::1`, or a host in `MCP_ALLOWED_ORIGINS`. In strict mode, non-loopback hosts are rejected unless explicitly allowlisted. Default off to preserve reverse-proxy deployments that rewrite Host. |
| `MCP_HTTP_MAX_BODY` | `2097152` | Positive max request body (bytes) |
| `MCP_CONTROL_PLANE_DSN` | `memory` | Control-plane store for tenants, credential refs, sessions, and audit events (`streamable_http`) |
| `MCP_SESSION_TTL` | `30m` | Session TTL for `streamable_http` |
| `MCP_METRICS_BIND` | — | Dedicated metrics listener (recommended for `streamable_http`) |
| `MCP_LOG_FORMAT` | `text` | `text` or `json` (stderr). All handlers are wrapped in a PII-redacting layer that scrubs secret keys (`authorization`, `api_key`, `bearer`, `token`, …) before encoding. |
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

Legacy `MCP_TRANSPORT=http` remains the documented compatibility POST JSON-RPC transport. It does not provide server-initiated streaming, session-backed notifications, or per-client initialization state. For shared-service deployments, use `MCP_TRANSPORT=streamable_http`.

See [docs/http-transport.md](docs/http-transport.md) for the full HTTP transport guide.

## Build / Test / Run

A `Makefile` is provided for common operations:

```bash
make build       # Build binary with version from git tags
make test        # Run all tests with race detector
make cover       # Run tests with coverage report
make fmt         # Check formatting
make vet         # Run go vet
make check       # fmt + vet + test (CI equivalent)
make clean       # Remove build artifacts
```

Or use Go commands directly:

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

# Run server — legacy HTTP mode
CLOCKIFY_API_KEY=xxx MCP_TRANSPORT=http MCP_BEARER_TOKEN=secret go run ./cmd/clockify-mcp

# Run server — session-aware shared-service HTTP mode
MCP_TRANSPORT=streamable_http MCP_AUTH_MODE=oidc MCP_OIDC_ISSUER=https://issuer.example.com MCP_CONTROL_PLANE_DSN=memory go run ./cmd/clockify-mcp

# Build with explicit metadata
go build -ldflags "-X main.version=v0.5.0 -X main.commit=$(git rev-parse HEAD) -X main.buildDate=$(git show -s --format=%cI HEAD)" ./cmd/clockify-mcp

# Show all env vars
clockify-mcp --help
```

Go 1.25.9, stdlib only — zero external dependencies. Module path: `github.com/apet97/go-clockify`.

## Compatibility

| Component | Version |
|-----------|---------|
| MCP Protocol | `2025-06-18` |
| Claude Desktop | latest |
| Cursor | latest |
| Other MCP clients | any supporting stdio or the documented legacy POST JSON-RPC HTTP mode |
| Go | 1.25.9+ |
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

**Stale tool list** — Stdio clients receive `notifications/tools/list_changed` after activation. HTTP clients do not; they must re-fetch `tools/list` after activation.

**Shared HTTP state** — Legacy HTTP mode is not session-aware. Repeating `initialize` from one caller updates the process-global negotiated client metadata for all HTTP callers.

## Deployment and operations

- **Kubernetes reference manifests** in [`deploy/k8s/`](deploy/k8s/): hardened `Deployment`, `Service`, `ConfigMap`, `Secret` template, `ServiceAccount`, `PodDisruptionBudget`, and `NetworkPolicy` (default-deny ingress except labelled allowed pods, default-deny egress except DNS + HTTPS).
- **Docker image pipeline** in [`.github/workflows/docker-image.yml`](.github/workflows/docker-image.yml): multi-arch buildx (linux/amd64, linux/arm64) → Trivy scan (fail on HIGH/CRITICAL) → cosign keyless OIDC sign → SPDX SBOM → SLSA build provenance attested to the registry. The image ships at `ghcr.io/apet97/go-clockify:v<version>`.
- **Incident runbooks**: [`docs/runbooks/`](docs/runbooks/).
- **Observability reference**: [`docs/observability.md`](docs/observability.md) — metric names, SLOs, alert rules, log event taxonomy.

## Documentation

- [Tool Catalog](docs/tool-catalog.md) — all 124 tools
- [Safe Usage](docs/safe-usage.md) — policy, dry-run, dedupe, rate limiting
- [HTTP Transport](docs/http-transport.md) — setup, auth, CORS, Docker
- [Tool Annotations](docs/tool-annotations.md) — readOnlyHint, destructiveHint, idempotentHint
- [Observability](docs/observability.md) — Prometheus metrics, SLOs, alert rules, log taxonomy
- [Security Threat Model](docs/security-threat-model.md) — trust boundaries, session/tenant isolation, residual risks
- [Stability Policy](docs/stability-policy.md) — compatibility tiers and deprecation rules
- [Wave 1 Backlog](docs/wave1-backlog.md) — curated next-iteration roadmap. **Landed**: cancellation map (W1-02), `outputSchema` sweep (W1-09), tier2 coverage push (W1-11), OAuth 2.1 Resource Server completion (W1-06). **Remaining**: Streamable HTTP completion (W1-01), progress notifications (W1-03), resources/prompts capabilities (W1-04/05), per-token rate limiting (W1-07), OTel tracing (W1-12), alerts/runbooks/manifests (W1-13/14), architecture + ADR docs (W1-15/16/17).

## Support

- Bug reports and feature requests: [GitHub Issues](https://github.com/apet97/go-clockify/issues)
- Security vulnerabilities: see [SECURITY.md](SECURITY.md)

## Verifying releases

Each release includes cosign sigstore bundles, SPDX SBOMs, and GitHub
build provenance attestations. Release binaries are built with
`-trimpath` for reproducibility. See [docs/verification.md](docs/verification.md)
for step-by-step verification commands.

## License

MIT
