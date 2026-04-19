# clockify-mcp-go

A [Model Context Protocol][mcp] server for [Clockify][clockify], written in Go. Connect any MCP client — Claude Code, Claude Desktop, Cursor, Codex, or anything else that speaks MCP — to your Clockify workspace and let it log time, run reports, and manage projects on your behalf.

- **124 tools** — 33 always-on (timer, entries, projects, reports, …) + 91 on-demand (invoices, scheduling, approvals, admin, …)
- **Resources & prompts** — six `clockify://` URI templates and five built-in prompt templates alongside the tool surface
- **Four policy modes** — `read_only`, `safe_core`, `standard`, `full` — plus a dry-run preview for every destructive tool
- **Streamable HTTP + stdio + opt-in gRPC** — stdio by default, streamable HTTP 2025-03-26 for shared services, gRPC behind a build tag
- **Stdlib-only default build** — zero external runtime dependencies; the default binary links no OpenTelemetry, gRPC, or protobuf symbols (verified in CI)
- **Signed releases** — every binary and container image ships with cosign signatures, SPDX SBOM, and SLSA build provenance

[mcp]: https://modelcontextprotocol.io/docs/getting-started/intro
[clockify]: https://clockify.me

## Install

```sh
# Go
go install github.com/apet97/go-clockify/cmd/clockify-mcp@latest

# npm (prebuilt binaries)
npx @anycli/clockify-mcp-go

# Or download a prebuilt binary from Releases:
# https://github.com/apet97/go-clockify/releases
```

Verify:

```sh
clockify-mcp --version
```

Get a Clockify API key from [Profile → Advanced](https://app.clockify.me/user/preferences) and export it:

```sh
export CLOCKIFY_API_KEY=your-key
```

## Connect to an MCP client

### Claude Code (CLI)

```sh
claude mcp add clockify -- clockify-mcp
```

Then set `CLOCKIFY_API_KEY` in your shell, or inline it: `claude mcp add clockify -e CLOCKIFY_API_KEY=your-key -- clockify-mcp`.

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

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

### Cursor

Add to `.cursor/mcp.json` in your workspace:

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

### Codex CLI

Add to your Codex MCP config:

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

### npm wrapper (all clients)

If you installed via `npm`/`npx`, swap the command for:

```json
{
  "command": "npx",
  "args": ["@anycli/clockify-mcp-go"]
}
```

## Tool tiers

**Tier 1 (33 tools, always loaded):** timer, entries, projects, clients, tags, tasks, users, workspaces, reports, workflows, search, context.

**Tier 2 (91 tools, 11 groups, on demand):** invoices, expenses, scheduling, time off, approvals, shared reports, user admin, webhooks, custom fields, groups/holidays, project admin.

Call `clockify_search_tools` to discover and activate a Tier 2 group or a specific tool. Activation updates `tools/list` at runtime.

## Policy modes

`CLOCKIFY_POLICY` controls which tools are exposed based on trust level:

| Mode | Read | Write | Delete | Tier 2 | Use case |
|------|------|-------|--------|--------|----------|
| `read_only` | yes | no | no | no | Untrusted agents — observe only |
| `safe_core` | yes | allowlist | no | no | Day-to-day time tracking |
| `standard` | yes | yes | yes | on demand | **Default** — balanced |
| `full` | yes | yes | yes | yes | Admin and automation |

Introspection tools (`clockify_whoami`, `clockify_policy_info`, `clockify_search_tools`, `clockify_resolve_debug`) are always available regardless of policy.

## Configuration

The essentials:

| Variable | Default | Purpose |
|----------|---------|---------|
| `CLOCKIFY_API_KEY` | — | API key (**required**) |
| `CLOCKIFY_WORKSPACE_ID` | auto | Workspace ID (auto-detected if only one) |
| `CLOCKIFY_POLICY` | `standard` | `read_only`, `safe_core`, `standard`, `full` |
| `CLOCKIFY_DRY_RUN` | `enabled` | Dry-run preview for destructive tools |
| `CLOCKIFY_DEDUPE_MODE` | `warn` | Duplicate detection: `warn`, `block`, `off` |
| `CLOCKIFY_RATE_LIMIT` | `120` | Tool calls per 60s window (`0` disables) |
| `CLOCKIFY_BOOTSTRAP_MODE` | `full_tier1` | `full_tier1`, `minimal`, `custom` |
| `MCP_TRANSPORT` | `stdio` | `stdio`, `http`, `streamable_http`, or `grpc` |
| `MCP_HTTP_BIND` | `:8080` | HTTP listen address |
| `MCP_BEARER_TOKEN` | — | Required for HTTP `static_bearer` mode |
| `MCP_AUTH_MODE` | — | `static_bearer`, `oidc`, `forward_auth`, `mtls` (mTLS rejected with `MCP_TRANSPORT=http`; terminate TLS upstream and use `forward_auth`) |
| `MCP_OIDC_VERIFY_CACHE_TTL` | `60s` | Cached OIDC verify ceiling, clamped to `[1s, 5m]`; larger values trade revocation latency for verify cost |
| `MCP_ALLOW_DEV_BACKEND` | — | Set `1` to permit `streamable_http` against `memory`/`file` control-plane backends (single-process only) |
| `MCP_CONTROL_PLANE_DSN` | `memory` | `memory`, `file://<path>`, or `postgres://...`. Postgres requires `go build -tags=postgres` (ADR 0001). |
| `MCP_CONTROL_PLANE_AUDIT_CAP` | `0` | File/memory backend only — caps in-memory audit events (0 = unbounded). Postgres uses time-based retention instead. |
| `MCP_CONTROL_PLANE_AUDIT_RETENTION` | `720h` | Max age of retained audit events; the reaper drops anything older on a 1h ticker. `0` disables retention. Range `1h`–`8760h`. |
| `MCP_HTTP_LEGACY_POLICY` | `warn` | `warn`, `allow`, `deny` — controls `MCP_TRANSPORT=http` startup behaviour |
| `MCP_AUDIT_DURABILITY` | `best_effort` | `best_effort` or `fail_closed` — audit persist failures abort tool calls when `fail_closed` |
| `MCP_HTTP_INLINE_METRICS_ENABLED` | `false` | Set `1` to expose `/metrics` on the main HTTP listener; disabled by default |
| `MCP_HTTP_INLINE_METRICS_AUTH_MODE` | `inherit_main_bearer` | `inherit_main_bearer`, `static_bearer`, or `none` |
| `MCP_LOG_FORMAT` | `text` | `text` or `json` (stderr; PII-scrubbed) |

Run `clockify-mcp --help` for the complete list (40+ variables covering concurrency, timeouts, control plane, metrics, and CORS).

## Common workflows

Start and stop a timer:

```
→ clockify_start_timer { "project": "My Project" }
← { "ok": true, "action": "timer_started", "data": { "id": "abc123" } }

→ clockify_stop_timer {}
← { "ok": true, "action": "timer_stopped" }
```

Log time retroactively:

```
→ clockify_log_time { "project": "Project Alpha", "start": "today 9:00", "end": "today 11:00", "description": "Code review" }
```

Dry-run a destructive operation:

```
→ clockify_delete_entry { "entry_id": "abc123", "dry_run": true }
← { "dry_run": true, "preview": { "id": "abc123", "description": "Meeting" }, "note": "No changes were made." }
```

Activate a Tier 2 domain:

```
→ clockify_search_tools { "activate_group": "invoices" }
← { "activated": "invoices", "tool_count": 12 }
```

## Architecture

Four clean layers: **protocol core** (`internal/mcp/`), **Clockify client** (`internal/clockify/`), **tool surface** (`internal/tools/`), and **safety layer** (`internal/enforcement/`). The protocol core has zero domain imports and plugs into the rest via `Enforcement`, `Activator`, `Notifier`, and `ResourceProvider` interfaces.

## Docker

```sh
docker build -f deploy/Dockerfile -t clockify-mcp .
docker run -p 8080:8080 \
  -e CLOCKIFY_API_KEY=your-key \
  -e MCP_BEARER_TOKEN=your-secret-token \
  clockify-mcp
```

The repository also ships [`deploy/docker-compose.yml`](deploy/docker-compose.yml) with a Caddy reverse proxy for TLS termination, and a Helm chart at [`deploy/helm/`](deploy/helm/).

## Build and test

```sh
make check   # fast inner loop: gofmt + go vet + go test
make verify  # full local pipeline: lint, coverage floors, fuzz-short,
             # build-tag checks, HTTP smoke, k8s render, govulncheck
             # (k8s/fips/vuln tiers auto-skip when their tools are missing)
make cover   # coverage report
make build   # binary with version from git tags
```

`make verify` mirrors the PR-blocking CI jobs that can run on a laptop —
see `CONTRIBUTING.md` for the exact list of checks it runs locally versus
the full CI set.

Go 1.25.9, stdlib only. Module path: `github.com/apet97/go-clockify`.

## Compatibility

| Component | Version |
|-----------|---------|
| MCP Protocol | `2025-06-18` (back-compat: `2025-03-26`, `2024-11-05`) |
| Go | 1.25.9+ |
| Node.js (npm wrapper) | 16+ |

## Troubleshooting

**No tools visible** — Check `CLOCKIFY_BOOTSTRAP_MODE`. In `minimal` mode most tools are hidden; use `clockify_search_tools` to discover them.

**401 Unauthorized** — API key is invalid or expired. [Generate a new one](https://app.clockify.me/user/preferences).

**Multiple workspaces** — Set `CLOCKIFY_WORKSPACE_ID` explicitly.

**Tool not found** — It may be a Tier 2 tool. Use `clockify_search_tools` to find and activate its group.

**Dry-run not working** — Ensure `CLOCKIFY_DRY_RUN=enabled` (default) and pass `"dry_run": true` in tool call parameters.

**Stale tool list** — Stdio clients receive `notifications/tools/list_changed` after activation; HTTP clients must re-fetch `tools/list`.

## Deployment

Reference Kubernetes manifests live in [`deploy/k8s/`](deploy/k8s/) and [`deploy/helm/`](deploy/helm/): Deployment (non-root distroless, read-only root FS, dropped capabilities), NetworkPolicy (default-deny), PodDisruptionBudget, ServiceMonitor, and a PrometheusRule with burn-rate alerts for a 99.9% SLO.

For a single-page operator overview that links the threat model, transports, auth modes, deployment targets, runbooks, and compliance posture, see [docs/production-readiness.md](docs/production-readiness.md).

### Production Resources
- [Production Profile: Shared Service](docs/deploy/production-profile-shared-service.md)
- [Support Matrix](docs/support-matrix.md)
- [Client Compatibility](docs/clients.md)
- [Deploy-Readiness Checklist](docs/release/deploy-readiness-checklist.md)
- [Operator Guides](docs/operators/) (Shared Service vs Self-Hosted)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Support

- Bug reports and feature requests: [GitHub Issues](https://github.com/apet97/go-clockify/issues)
- Security vulnerabilities: [SECURITY.md](SECURITY.md)
- Release history: [CHANGELOG.md](CHANGELOG.md)
- Versioning, support window, breaking-change policy: [docs/release-policy.md](docs/release-policy.md)

## License

[MIT](LICENSE)
