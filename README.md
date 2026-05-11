# clockify-mcp-go

> A [Model Context Protocol][mcp] server for [Clockify][clockify] — plug any MCP client into your time-tracking workspace and let it log time, run reports, and manage projects on your behalf.
>
> **Unofficial community project. Not affiliated with or endorsed by Clockify.** Single-maintainer, public source under MIT, see [SUPPORT.md](SUPPORT.md) and [GOVERNANCE.md](GOVERNANCE.md) for adoption posture.

[![Go version](https://img.shields.io/badge/go-1.25-00ADD8?logo=go)](go.mod)
[![Release](https://img.shields.io/github/v/release/apet97/go-clockify?color=7e57c2)](https://github.com/apet97/go-clockify/releases)
[![MCP protocol](https://img.shields.io/badge/MCP-2025--11--25-4b0082)](https://modelcontextprotocol.io/specification)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

Works with **Claude Code**, **Claude Desktop**, **Cursor**, **Codex**, and anything else that speaks MCP.

## Highlights

- **139 tools** — 51 always-on (timer, entries, projects, clients, tags, tasks, reports, agent workflows, tool activation, …) plus 88 on-demand (invoices, scheduling, approvals, admin, …) across 11 activatable groups.
- **Resources & prompts** — six `clockify://` URI templates and five built-in prompt templates alongside the tool surface.
- **Five policy modes** — `read_only`, `time_tracking_safe`, `safe_core`, `standard`, `full` — plus dry-run previews on every write tool and HMAC confirmation tokens for high-risk tool calls (ADR 0018).
- **Three transports** — stdio (default), streamable HTTP 2025-03-26 (shared services), opt-in gRPC behind a build tag. Cancellation, `tools/list_changed`, size limits, and malformed-JSON boundaries pinned with cross-transport parity tests.
- **Stdlib-only default build** — zero external runtime dependencies; the default binary links no OpenTelemetry, gRPC, or protobuf symbols (verified in CI).
- **Signed releases** — every binary and container image ships with cosign signatures and SPDX SBOMs; SLSA build provenance is attached when GitHub artifact attestations are available for the repository account tier.

[mcp]: https://modelcontextprotocol.io/docs/getting-started/intro
[clockify]: https://clockify.me

## Contents

- [Start Here](#start-here) · [Install](#install) · [Connect a client](#connect-to-an-mcp-client) · [Common workflows](#common-workflows) · [Tool tiers](#tool-tiers)
- [Policy modes](#policy-modes) · [Configuration](#configuration) · [Architecture](#architecture) · [Docker](#docker) · [Build and test](#build-and-test)
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

Not sure which profile matches your environment? Run `clockify-mcp doctor` — it prints every env var's effective value, its source (explicit / profile / default / empty), and whether `Load()` would succeed at startup. Add `--strict` for hosted-service posture checks. Exit code is 0 on a clean load, 2 on a load error, and 3 when strict posture findings are present.

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

### Current release status

`v1.2.1` is the current stable community/self-hosted line, cut from the
rc.3 peeled commit and released on 2026-05-10
([GitHub Release](https://github.com/apet97/go-clockify/releases/tag/v1.2.1) ·
46 signed assets · npm `@apet97/clockify-mcp-go` `dist-tags.latest=1.2.1`
· container image `ghcr.io/apet97/go-clockify:1.2.1`). Operators
verifying signatures should follow [docs/verification.md](docs/verification.md);
paid-hosted / commercial / "official Clockify" framing is explicitly
out of scope per [docs/launch-readiness-review-may-8.md](docs/launch-readiness-review-may-8.md)
§ "Deferred paid-hosted/commercial follow-ups …".

If your environment routes Go module fetches through a custom proxy or
VCS resolver and you want to skip the public proxy for this module,
`go env -w GOPRIVATE=github.com/apet97/go-clockify` is supported but
not required for a standard `go install` against the public repo.

Get a Clockify API key from [Profile → Advanced](https://app.clockify.me/user/preferences) and export it:

```sh
export CLOCKIFY_API_KEY=your-key
```

The key inherits its owner's Clockify workspace role (Regular / Team
Manager / Workspace Admin / Owner). The MCP layer's `CLOCKIFY_POLICY`
controls which tools are exposed; the Clockify role controls whether
those tools succeed at the API. See
[`docs/policy/production-tool-scope.md`](docs/policy/production-tool-scope.md)
for the minimum role per tool family.

For personal testing against one real workspace, pin the local stdio
profile and workspace before connecting an AI client:

```sh
export MCP_PROFILE=local-stdio
export CLOCKIFY_WORKSPACE_ID=your-workspace-id
export CLOCKIFY_POLICY=time_tracking_safe
```

Use `time_tracking_safe` for exploratory AI-facing testing because it
allows timer and time-entry workflows without project/client/tag/task
creation. Switch to `safe_core` only when you intentionally want that
workspace-shaping surface. For large workspaces, keep
`CLOCKIFY_REPORT_MAX_ENTRIES` at its fail-closed default unless you have
a specific reason to materialize more than 10,000 raw entries in one
tool call.

## Connect to an MCP client

The examples below are the local stdio path: your MCP client launches `clockify-mcp` as a subprocess and forwards `CLOCKIFY_API_KEY` in its environment. Copy-ready snippets also live in [`examples/`](examples/).

### Claude Code (CLI)

```sh
claude mcp add clockify -- clockify-mcp
```

Then set the personal tester env vars above in your shell before
starting Claude Code. If you inline env with `claude mcp add -e`,
include `MCP_PROFILE`, `CLOCKIFY_API_KEY`, `CLOCKIFY_WORKSPACE_ID`, and
`CLOCKIFY_POLICY`.

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "clockify": {
      "command": "clockify-mcp",
      "env": {
        "MCP_PROFILE": "local-stdio",
        "CLOCKIFY_API_KEY": "your-key",
        "CLOCKIFY_WORKSPACE_ID": "your-workspace-id",
        "CLOCKIFY_POLICY": "time_tracking_safe"
      }
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
      "env": {
        "MCP_PROFILE": "local-stdio",
        "CLOCKIFY_API_KEY": "your-key",
        "CLOCKIFY_WORKSPACE_ID": "your-workspace-id",
        "CLOCKIFY_POLICY": "time_tracking_safe"
      }
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
      "env": {
        "MCP_PROFILE": "local-stdio",
        "CLOCKIFY_API_KEY": "your-key",
        "CLOCKIFY_WORKSPACE_ID": "your-workspace-id",
        "CLOCKIFY_POLICY": "time_tracking_safe"
      }
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

## Common workflows

For longer intent-keyed examples, see the
[agent cookbook](docs/agent-cookbook.md).

Built-in prompt templates are available through MCP `prompts/list` and
`prompts/get`: `log-week-from-calendar`, `weekly-review`,
`find-unbilled-hours`, `find-duplicate-entries`, and
`generate-timesheet-report`. They are short planning prompts that point
clients at the current structured tools, including `clockify_log_time`
for finished past entries and `clockify_timesheet_review` for gaps,
overlaps, and suggested next-tool actions.

Start and stop a timer:

```
→ clockify_start_timer { "project": "My Project" }
← { "ok": true, "action": "timer_started", "data": { "id": "abc123" } }

→ clockify_stop_timer {}
← { "ok": true, "action": "timer_stopped" }
```

Log time retroactively:

```
→ clockify_log_time { "project": "Project Alpha", "start": "today 9:00", "end": "today 11:00", "description": "Code review", "dry_run": true }
```

Review and close a timesheet gap:

```
→ clockify_timesheet_review { "date": "2026-05-03", "timezone": "UTC" }
← { "ok": true, "action": "clockify_timesheet_review", "data": { "issues": [], "suggestedActions": [] } }

→ clockify_timesheet_fill_gap { "project": "Project Alpha", "start": "2026-05-03T09:00:00Z", "end": "2026-05-03T10:00:00Z", "description": "Planning", "dry_run": true }
```

Discover a Tier 2 domain or tool:

```
→ clockify_list_tools { "query": "invoice" }
← { "count": 6, "all_results": [ { "type": "group", "name": "invoices" }, { "type": "tool", "name": "clockify_send_invoice" } ] }
```

Activate a Tier 2 domain:

```
→ clockify_activate_group { "name": "invoices" }
← { "activated": "invoices", "tool_count": 12, "activated_tools": ["clockify_list_invoices", "..."], "total_visible_tools": 47 }
```

Optionally activate a single Tier 2 tool:

```
→ clockify_activate_tool { "name": "clockify_send_invoice" }
← { "activated": "clockify_send_invoice", "group": "invoices", "tool_count": 12, "activated_tools": ["clockify_list_invoices", "..."], "total_visible_tools": 47 }
```

Deactivate the domain when the task is done:

```
→ clockify_deactivate_group { "name": "invoices" }
← { "deactivated": "invoices", "tool_count": 12, "deactivated_tools": ["clockify_list_invoices", "..."], "total_visible_tools": 35 }
```

Preview a high-risk operation first to mint a confirmation token (ADR 0018):

```
→ clockify_delete_entry { "entry_id": "abc123", "dry_run": true }
← {
    "dry_run": true,
    "preview": { "id": "abc123", "description": "Meeting" },
    "note": "No changes were made.",
    "confirmation_required": true,
    "confirmation_token": "eyJ...example",
    "confirmation_expires_at": "2026-05-11T19:05:00Z",
    "confirmation_risk_class": ["destructive"],
    "confirmation_note": "Re-submit the same arguments with dry_run:false and confirmation_token to execute."
  }
```

The dry-run envelope shape is "rich preview for destructive tools that
have a read counterpart, minimal envelope otherwise" — high-risk
non-destructive tools (e.g. `clockify_send_invoice`,
`clockify_test_webhook`) return only the confirmation fields plus an
`args` echo. The token binds to the tool name, the argument
fingerprint, the principal (tenant + subject + session), and an
`exp`; changing any of those between mint and execute invalidates it.

Execute with the same arguments plus the minted token:

```
→ clockify_delete_entry { "entry_id": "abc123", "dry_run": false, "confirmation_token": "eyJ...example" }
← { "ok": true, "action": "clockify_delete_entry", "data": { "deleted": true, "entryId": "abc123" } }
```

Ordinary RiskWrite tools (the time-entry / timer surface — e.g.
`clockify_log_time`, `clockify_start_timer`) do not require a
confirmation token. Pass `dry_run:false` or omit it to execute
directly; `dry_run:true` still returns a preview envelope.

Large workspace report hygiene:

- Pin `CLOCKIFY_WORKSPACE_ID` for personal API-key testing even when
  auto-detection works today. It keeps the target stable if the key later
  gains access to another workspace.
- Prefer short date ranges and `include_entries=false` for summary,
  weekly, detailed, and quick reports when you need totals rather than
  raw rows.
- Use `page` and `page_size` on list tools. The default is
  `page=1`, `page_size=50`, and the maximum page size is 200.
- Report responses expose `meta.pagination` and `meta.limits`.
  `CLOCKIFY_REPORT_MAX_ENTRIES` applies when a call materializes raw
  entries with `include_entries=true`; cap errors are intentional
  fail-closed behavior.

## Tool tiers

**Tier 1 (51 tools, always loaded):** timer, entries, projects (incl. create/update/delete/archive), clients (full CRUD), tags (full CRUD), tasks (full CRUD), users, workspaces, reports, workflows, search, activation, context.

**Tier 2 (88 tools, 11 groups, on demand):** invoices, expenses, scheduling, time off, approvals, shared reports, user admin, webhooks, custom fields, groups/holidays, project admin.

Call `clockify_list_tools` to discover a Tier 2 group or specific tool, `clockify_activate_group` / `clockify_activate_tool` to widen the current session, and `clockify_deactivate_group` to shrink the visible surface after a task. Activation updates `tools/list` at runtime, and `activated_tools` only lists names that are visible after bootstrap and policy filtering. `clockify_search_tools` remains as a deprecated compatibility shim for older clients.

## Policy modes

`CLOCKIFY_POLICY` controls which tools are exposed based on trust level:

| Mode | Read | Write | Delete | Tier 2 | Use case |
|------|------|-------|--------|--------|----------|
| `read_only` | yes | no | no | no | Untrusted agents — observe only |
| `time_tracking_safe` | yes | time-entry allowlist | no | no | **Recommended AI-facing default** for time tracking |
| `safe_core` | yes | broader allowlist | no | no | Trusted assistants that may create projects, clients, tags, and tasks |
| `standard` | yes | yes | yes | on demand | Raw no-profile default / trusted operator mode; code-level allow rules match `full` |
| `full` | yes | yes | yes | on demand | Admin automation label for operators intentionally running the unrestricted policy |

Introspection tools (`clockify_whoami`, `clockify_policy_info`, `clockify_list_tools`, `clockify_activate_group`, `clockify_activate_tool`, `clockify_deactivate_group`, `clockify_search_tools`, `clockify_resolve_name`, `clockify_resolve_debug`) are always available regardless of policy. `clockify_resolve_debug` is a compatibility alias; new clients should call `clockify_resolve_name`.

`standard` and `full` share broad policy allow rules: any non-denied
tool or group is policy-allowed. Their operational difference is
convention plus bootstrap/activation posture. **A separate ADR 0018
confirmation-token gate sits alongside the policy mode** — when
`CLOCKIFY_CONFIRMATION_TOKENS=enabled` (the default), every
high-risk tool call (billing, admin, permission-change,
external-side-effect, or destructive) requires a server-minted HMAC
token obtained via a `dry_run:true` preview, regardless of policy
mode. `time_tracking_safe` and `safe_core` remain the recommended
AI-facing modes for hosted deployments, and `CLOCKIFY_DENY_*`,
`CLOCKIFY_ALLOW_GROUPS`, and `CLOCKIFY_BOOTSTRAP_MODE` narrow a
trusted operator deployment further.

## Configuration

The essentials (regenerate with `go run ./cmd/gen-config-docs -mode=all`):

<!-- CONFIG-TABLE BEGIN — generated by cmd/gen-config-docs; do not edit by hand -->
| Variable | Default | Purpose |
|----------|---------|---------|
| `CLOCKIFY_API_KEY` | `—` | API key (required for stdio/http/grpc and MCP_PROFILE=single-tenant-http; optional for shared-service / prod-postgres streamable_http where tenant credentials come from the control plane) |
| `CLOCKIFY_BOOTSTRAP_MODE` | `full_tier1` | Initial tool surface |
| `CLOCKIFY_CONFIRMATION_TOKENS` | `enabled` | Require an HMAC confirmation token (minted on dry_run:true) for high-risk tool calls per docs/adr/0018-risk-class-confirmation-tokens.md; high-risk covers RiskBilling, RiskAdmin, RiskPermissionChange, RiskExternalSideEffect, and RiskDestructive. Set to disabled only for break-glass single-operator deployments. |
| `CLOCKIFY_DEDUPE_MODE` | `warn` | Duplicate entry detection |
| `CLOCKIFY_DRY_RUN` | `enabled` | Enable dry-run preview support for tools that expose dry_run:true |
| `CLOCKIFY_POLICY` | `standard` | Tool-access policy tier |
| `CLOCKIFY_RATE_LIMIT` | `120` | Global tool calls per 60s window (0=disabled). For multi-subject HTTP/gRPC deployments, size this at least active_subjects * CLOCKIFY_PER_TOKEN_RATE_LIMIT so per-subject fairness can engage. |
| `CLOCKIFY_WORKSPACE_ID` | `auto` | Workspace ID (auto-detected if only one) |
| `MCP_ALLOW_DEV_BACKEND` | `—` | Permit memory/file backends for streamable_http or grpc (single-process only) |
| `MCP_AUDIT_DURABILITY` | `best_effort` | Audit persist-failure behaviour (defaults to fail_closed when ENVIRONMENT=prod); fail_closed_strict also surfaces post-mutation outcome persistence failures |
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
| `MCP_HTTP_RATELIMIT_GET_PER_SESSION` | `0` | Concurrent streamable HTTP SSE GET connections allowed per session on this process (0=disabled; hosted profiles default to 4) |
| `MCP_HTTP_RATELIMIT_PER_IP` | `0` | Process-local HTTP admission limit per source IP per minute (0=disabled; hosted profiles default to 600) |
| `MCP_HTTP_RATELIMIT_PER_PRINCIPAL` | `0` | Process-local HTTP admission limit per authenticated subject+tenant per minute (0=disabled; hosted profiles default to 300) |
| `MCP_LOG_FORMAT` | `text` | Log format (stderr; PII-scrubbed) |
| `MCP_MAX_MESSAGE_SIZE` | `4194304` | Max request size in bytes (primary knob); 0 < N <= 104857600 |
| `MCP_METRICS_AUTH_MODE` | `static_bearer (when MCP_METRICS_BIND set)` | Auth mode for dedicated metrics listener |
| `MCP_METRICS_BEARER_TOKEN` | `—` | Bearer token (>=16 chars) for static_bearer metrics |
| `MCP_METRICS_BIND` | `—` | Dedicated metrics listener (optional; recommended for streamable_http) |
| `MCP_OIDC_VERIFY_CACHE_TTL` | `60s` | OIDC verify cache TTL [1s,5m]; hosted profiles clamp values above 60s |
| `MCP_PROFILE` | `—` | Apply a bundle of pinned defaults for a named deployment shape; explicit env overrides still win |
| `MCP_TENANT_POLICY_CEILING` | `—` | Maximum policy mode a control-plane tenant record may select via TenantRecord.PolicyMode. Hosted profiles (shared-service, prod-postgres) default this to time_tracking_safe so a corrupted tenant row cannot broaden the process posture. Empty = no explicit ceiling; the process mode acts as the implicit ceiling. Streamable-HTTP only — the gRPC transport does not consume control-plane tenant records. Ignored on stdio (no tenants). See docs/adr/0021-hosted-tenant-policy-ceiling.md. |
| `MCP_TRANSPORT` | `stdio` | Transport mode; http is legacy POST-only (deprecated) |
<!-- CONFIG-TABLE END -->

Run `clockify-mcp --help` for the complete list (75+ variables covering concurrency, timeouts, control plane, metrics, auth, CORS, and webhook DNS validation).

## Architecture

Four clean layers: **protocol core** (`internal/mcp/`), **Clockify client** (`internal/clockify/`), **tool surface** (`internal/tools/`), and **safety layer** (`internal/enforcement/`). The protocol core has zero domain imports and plugs into the rest via `Enforcement`, `Activator`, `Notifier`, and `ResourceProvider` interfaces.

## Docker

The published image defaults to the spec-strict streamable HTTP transport.
The `single-tenant-http` profile wires the matching auth + control-plane
defaults so the example below boots without a Postgres container or an
OIDC issuer.

```sh
docker build -f deploy/Dockerfile -t clockify-mcp .

docker volume create clockify-mcp-data
export MCP_BEARER_TOKEN="$(openssl rand -base64 24)"

docker run -p 8080:8080 \
  -v clockify-mcp-data:/var/lib/clockify-mcp \
  -e MCP_PROFILE=single-tenant-http \
  -e CLOCKIFY_API_KEY=your-key \
  -e MCP_BEARER_TOKEN="$MCP_BEARER_TOKEN" \
  clockify-mcp
```

Validate the running server with the operator-facing endpoints:

```sh
curl http://127.0.0.1:8080/health

curl --oauth2-bearer "$MCP_BEARER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  http://127.0.0.1:8080/mcp
```

The repository also ships [`deploy/docker-compose.yml`](deploy/docker-compose.yml) with a Caddy reverse proxy for TLS termination, and a Helm chart at [`deploy/helm/`](deploy/helm/).

## Build and test

```sh
make check   # fast inner loop: gofmt + go vet + go test
make verify  # pre-PR local pipeline: lint, coverage floors, fuzz-short,
             # build-tag checks, HTTP smoke, k8s render, govulncheck
             # (lint/k8s/fips/vuln tiers skip when local tools are missing)
make release-check  # pre-ship/tag gate: docs, scripts, smokes, gRPC E2E, deploy render
make cover   # coverage report
make build   # binary with version from git tags
```

`make verify` mirrors the PR-blocking CI jobs that can run on a laptop
when their tools are installed; CI remains authoritative for skipped
local tiers. `make release-check` is the laptop-runnable pre-ship gate.
Neither command replaces the external launch-candidate evidence gates
tracked in `docs/launch-candidate-checklist.md`.

Go 1.25.10, stdlib only. Module path: `github.com/apet97/go-clockify`.

## Compatibility

| Component | Version |
|-----------|---------|
| MCP Protocol | `2025-11-25` (back-compat: `2025-06-18`, `2025-03-26`, `2024-11-05`) |
| Go | 1.25.10 pinned |
| Node.js (npm wrapper) | 18+ |

## Troubleshooting

**No tools visible** — Check `CLOCKIFY_BOOTSTRAP_MODE`. In `minimal` mode most tools are hidden; use `clockify_list_tools` to discover them.

**401 Unauthorized** — API key is invalid or expired. [Generate a new one](https://app.clockify.me/user/preferences).

**Multiple workspaces** — Set `CLOCKIFY_WORKSPACE_ID` explicitly.

**Tool not found** — It may be a Tier 2 tool. Use `clockify_list_tools` to find it, then `clockify_activate_group` or `clockify_activate_tool` to activate its group.

**Dry-run not working** — Ensure `CLOCKIFY_DRY_RUN=enabled` (default) and pass `"dry_run": true` in tool call parameters.

**Stale tool list** — Stdio, `streamable_http`, and `grpc` clients all receive `notifications/tools/list_changed` after activation (streamable_http via the SSE stream on `GET /mcp`; gRPC fans the notification through every active `Exchange` stream). Only legacy `http` clients must manually re-fetch `tools/list`.

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

Autonomous agents should start with [AGENTS.md](AGENTS.md) and the
current launch-state handoff in
[docs/agent-handoff.md](docs/agent-handoff.md). The
[Claude Code continuation packet](docs/claude-code-continuation.md)
is retained as the historical post-PR #51 handoff.

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
