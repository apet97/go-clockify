# clockify-mcp-go

> A [Model Context Protocol][mcp] server for [Clockify][clockify] — plug any MCP client into your time-tracking workspace and let it log time, run reports, and manage projects on your behalf.

[![Go version](https://img.shields.io/badge/go-1.25-00ADD8?logo=go)](go.mod)
[![Release](https://img.shields.io/github/v/release/apet97/go-clockify?color=7e57c2)](https://github.com/apet97/go-clockify/releases)
[![MCP protocol](https://img.shields.io/badge/MCP-2025--06--18-4b0082)](https://modelcontextprotocol.io/specification)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

Works with **Claude Code**, **Claude Desktop**, **Cursor**, **Codex**, and anything else that speaks MCP.

## Highlights

- **124 tools** — 33 always-on (timer, entries, projects, reports, …) plus 91 on-demand (invoices, scheduling, approvals, admin, …) across 11 activatable groups.
- **Resources & prompts** — six `clockify://` URI templates and five built-in prompt templates alongside the tool surface.
- **Four policy modes** — `read_only`, `safe_core`, `standard`, `full` — plus dry-run preview support for every destructive tool.
- **Three transports** — stdio (default), streamable HTTP 2025-03-26 (shared services), opt-in gRPC behind a build tag. Cancellation, `tools/list_changed`, size limits, and malformed-JSON boundaries pinned with cross-transport parity tests.
- **Stdlib-only default build** — zero external runtime dependencies; the default binary links no OpenTelemetry, gRPC, or protobuf symbols (verified in CI).
- **Signed releases** — every binary and container image ships with cosign signatures, SPDX SBOM, and SLSA build provenance.

[mcp]: https://modelcontextprotocol.io/docs/getting-started/intro
[clockify]: https://clockify.me

## Contents

- [Start Here](#start-here) · [Install](#install) · [Connect a client](#connect-to-an-mcp-client) · [Tool tiers](#tool-tiers) · [Policy modes](#policy-modes) · [Configuration](#configuration)
- [Common workflows](#common-workflows) · [Architecture](#architecture) · [Docker](#docker) · [Build and test](#build-and-test)
- [Compatibility](#compatibility) · [Troubleshooting](#troubleshooting) · [Deployment](#deployment) · [Contributing](#contributing)

## Start Here

Pick a deployment profile and invoke the binary with `--profile=<name>` (or `MCP_PROFILE=<name>`). The profile applies a bundle of pinned defaults; explicit env overrides still win.

| Profile | Shape | Doc | Example env |
|---------|-------|-----|-------------|
| `local-stdio` | single user, stdio subprocess | [profile-local-stdio.md](docs/deploy/profile-local-stdio.md) | [env.local-stdio.example](deploy/examples/env.local-stdio.example) |
| `single-tenant-http` | one team, streamable HTTP + static bearer | [profile-single-tenant-http.md](docs/deploy/profile-single-tenant-http.md) | [env.single-tenant-http.example](deploy/examples/env.single-tenant-http.example) |
| `shared-service` | multi-tenant, postgres + OIDC, audit fail-closed | [production-profile-shared-service.md](docs/deploy/production-profile-shared-service.md) | [env.shared-service.example](deploy/examples/env.shared-service.example) |
| `private-network-grpc` | gRPC + mTLS behind a private perimeter (`-tags=grpc`) | [profile-private-network-grpc.md](docs/deploy/profile-private-network-grpc.md) | [env.private-network-grpc.example](deploy/examples/env.private-network-grpc.example) |
| `prod-postgres` | alias of `shared-service` with `ENVIRONMENT=prod` | see shared-service doc | — |

Not sure which profile matches your environment? Run `clockify-mcp doctor` — it prints every env var's effective value, its source (explicit / profile / default / empty), and whether `Load()` would succeed at startup. Exit code is 0 on a clean load and 2 on an error.

Operators upgrading from before Wave I: see [the operator overview](docs/production-readiness.md) and [docs/operators/](docs/operators/) for the deeper production checklist — profiles shortcut the common cases but do not replace ops review for critical deployments.

## Install

```sh
# Go
go install github.com/apet97/go-clockify/cmd/clockify-mcp@latest

# npm (prebuilt binaries)
npx @apet97/clockify-mcp-go

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

The examples below are the local stdio path: your MCP client launches `clockify-mcp` as a subprocess and forwards `CLOCKIFY_API_KEY` in its environment.

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
  "args": ["@apet97/clockify-mcp-go"]
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

The essentials (regenerate with `go run ./cmd/gen-config-docs -mode=all`):

<!-- CONFIG-TABLE BEGIN — generated by cmd/gen-config-docs; do not edit by hand -->
| Variable | Default | Purpose |
|----------|---------|---------|
| `CLOCKIFY_API_KEY` | `—` | API key (required for stdio/http/grpc; optional for streamable_http) |
| `CLOCKIFY_BOOTSTRAP_MODE` | `full_tier1` | Initial tool surface |
| `CLOCKIFY_DEDUPE_MODE` | `warn` | Duplicate entry detection |
| `CLOCKIFY_DRY_RUN` | `enabled` | Enable dry-run preview support for destructive tools when callers pass dry_run:true |
| `CLOCKIFY_POLICY` | `standard` | Tool-access policy tier |
| `CLOCKIFY_RATE_LIMIT` | `120` | Tool calls per 60s window (0=disabled) |
| `CLOCKIFY_WORKSPACE_ID` | `auto` | Workspace ID (auto-detected if only one) |
| `MCP_ALLOW_DEV_BACKEND` | `—` | Permit memory/file backends for streamable_http (single-process only) |
| `MCP_AUDIT_DURABILITY` | `best_effort` | Audit persist-failure behaviour (defaults to fail_closed when ENVIRONMENT=prod) |
| `MCP_AUTH_MODE` | `—` | Authentication mode (per-transport support varies; see matrix) |
| `MCP_CONTROL_PLANE_AUDIT_CAP` | `0` | File/memory audit cap (0=unbounded). Postgres uses retention instead. |
| `MCP_CONTROL_PLANE_AUDIT_RETENTION` | `720h` | Audit retention [1h,8760h]; 0=off |
| `MCP_CONTROL_PLANE_DSN` | `memory` | Control-plane DSN: memory, file://<path>, postgres://... |
| `MCP_GRPC_BIND` | `:9090` | gRPC listen address (requires -tags=grpc) |
| `MCP_HTTP_BIND` | `:8080` | HTTP listen address |
| `MCP_HTTP_INLINE_METRICS_AUTH_MODE` | `inherit_main_bearer` | Auth mode for inline /metrics |
| `MCP_HTTP_INLINE_METRICS_ENABLED` | `0` | Expose /metrics on the main HTTP listener |
| `MCP_HTTP_LEGACY_POLICY` | `warn` | Legacy HTTP startup behaviour (defaults to deny when ENVIRONMENT=prod) |
| `MCP_HTTP_MAX_BODY` | `4194304` | **Deprecated — use `MCP_MAX_MESSAGE_SIZE`.** Deprecated alias for MCP_MAX_MESSAGE_SIZE |
| `MCP_LOG_FORMAT` | `text` | Log format (stderr; PII-scrubbed) |
| `MCP_MAX_MESSAGE_SIZE` | `4194304` | Max request size in bytes (primary knob); 0 < N <= 104857600 |
| `MCP_METRICS_AUTH_MODE` | `static_bearer (when MCP_METRICS_BIND set)` | Auth mode for dedicated metrics listener |
| `MCP_METRICS_BEARER_TOKEN` | `—` | Bearer token (>=16 chars) for static_bearer metrics |
| `MCP_METRICS_BIND` | `—` | Dedicated metrics listener (optional; recommended for streamable_http) |
| `MCP_OIDC_VERIFY_CACHE_TTL` | `60s` | OIDC verify cache TTL [1s,5m] |
| `MCP_PROFILE` | `—` | Apply a bundle of pinned defaults for a named deployment shape; explicit env overrides still win |
| `MCP_TRANSPORT` | `stdio` | Transport mode; http is legacy POST-only (deprecated) |
<!-- CONFIG-TABLE END -->

Run `clockify-mcp --help` for the complete list (60+ variables covering concurrency, timeouts, control plane, metrics, auth, and CORS).

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

Discover a Tier 2 domain or tool:

```
→ clockify_search_tools { "query": "invoice" }
← { "count": 6, "all_results": [ { "type": "group", "name": "invoices" }, { "type": "tool", "name": "clockify_send_invoice" } ] }
```

Activate a Tier 2 domain:

```
→ clockify_search_tools { "activate_group": "invoices" }
← { "activated": "invoices", "tool_count": 12 }
```

Optionally activate a single Tier 2 tool:

```
→ clockify_search_tools { "activate_tool": "clockify_send_invoice" }
← { "activated": "clockify_send_invoice", "group": "invoices", "tool_count": 12 }
```

Preview a destructive operation first:

```
→ clockify_delete_entry { "entry_id": "abc123", "dry_run": true }
← { "dry_run": true, "preview": { "id": "abc123", "description": "Meeting" }, "note": "No changes were made." }
```

Execute after preview:

```
→ clockify_delete_entry { "entry_id": "abc123", "dry_run": false }
← { "ok": true, "action": "clockify_delete_entry", "data": { "deleted": true, "entryId": "abc123" } }
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
| Node.js (npm wrapper) | 18+ |

## Troubleshooting

**No tools visible** — Check `CLOCKIFY_BOOTSTRAP_MODE`. In `minimal` mode most tools are hidden; use `clockify_search_tools` to discover them.

**401 Unauthorized** — API key is invalid or expired. [Generate a new one](https://app.clockify.me/user/preferences).

**Multiple workspaces** — Set `CLOCKIFY_WORKSPACE_ID` explicitly.

**Tool not found** — It may be a Tier 2 tool. Use `clockify_search_tools` to find and activate its group.

**Dry-run not working** — Ensure `CLOCKIFY_DRY_RUN=enabled` (default) and pass `"dry_run": true` in tool call parameters.

**Stale tool list** — Stdio clients receive `notifications/tools/list_changed` after activation; HTTP clients must re-fetch `tools/list`.

## Deployment

Reference Kubernetes manifests live in [`deploy/k8s/`](deploy/k8s/) and [`deploy/helm/`](deploy/helm/): Deployment (non-root distroless, read-only root FS, dropped capabilities), NetworkPolicy (default-deny), PodDisruptionBudget, ServiceMonitor, and a PrometheusRule with burn-rate alerts for a 99.9% SLO.

For a single-page operator overview that links the threat model, transports, auth modes, deployment targets, runbooks, and compliance posture, see [the operator overview](docs/production-readiness.md).

### Operator resources
- [Shared Service profile](docs/deploy/production-profile-shared-service.md)
- [Support Matrix](docs/support-matrix.md)
- [Client Compatibility](docs/clients.md)
- [Deploy-Readiness Checklist](docs/release/deploy-readiness-checklist.md)
- [Operator Guides](docs/operators/) (Shared Service vs Self-Hosted)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Support

- Adoption expectations, response-time posture, `v1.x` wire-format stability guarantee: [SUPPORT.md](SUPPORT.md)
- Governance (single-maintainer, merge gate, sensitive-area self-review): [GOVERNANCE.md](GOVERNANCE.md)
- Security vulnerabilities: [SECURITY.md](SECURITY.md) (private disclosure channel)
- Bug reports and feature requests: [GitHub Issues](https://github.com/apet97/go-clockify/issues)
- Release history: [CHANGELOG.md](CHANGELOG.md) · [GitHub Releases](https://github.com/apet97/go-clockify/releases)
- Versioning, support window, breaking-change policy: [docs/release-policy.md](docs/release-policy.md)
- Documentation map: [docs/README.md](docs/README.md)

## License

[MIT](LICENSE)
