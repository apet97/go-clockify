# Client Compatibility and Behavior Notes

This document provides guidance on how various Model Context Protocol (MCP) clients interact with `clockify-mcp-go`.

## Supported Client Matrix

| Client | Connection Mode | Stability | Notes |
|--------|-----------------|-----------|-------|
| Claude Code | `stdio` | Tier 1 | Full support for all tools and resources. |
| Claude Desktop | `stdio` | Tier 1 | Renders tool confirmation dialogs for `destructiveHint: true` tools. |
| Cursor | `stdio` | Tier 1 | Supports via `.cursor/mcp.json`. |
| Codex | `stdio` | Tier 1 | Lightweight CLI. |

## Expected Client Behavior

### Tool Discovery
Most clients fetch the list of available tools at startup using `tools/list`.
- **Stdio Clients:** Automatically receive `notifications/tools/list_changed` when new tools are activated via `clockify_search_tools`.
- **`streamable_http` Clients:** Receive the same notifications via the
  spec-canonical SSE stream on `GET /mcp` (Streamable HTTP 2025-03-26
  §3.3) — no manual re-fetch needed. The `/mcp/events` alias is
  preserved for older client implementations.
- **`grpc` Clients:** Receive the same notifications fanned out
  through the bidirectional `Exchange` stream — every active client
  stream registers a per-stream `streamNotifier` (see ADR-0008
  §"Per-stream notifier registration"), so `tools/list_changed`,
  `notifications/progress`, and `notifications/resources/updated`
  reach every connected gRPC client without polling. Requires the
  `-tags=grpc` build (the `private-network-grpc` profile artifact).
- **Legacy `http` Clients:** Must manually re-fetch the tool list
  after activation. The legacy POST-only transport does not carry
  server-initiated notifications; this is a known limitation and one
  of the reasons it is deprecated (`MCP_HTTP_LEGACY_POLICY=deny`
  refuses it on hosted profiles).

### Tier-2 Activation Semantics
Each Tier-2 group (invoices, expenses, scheduling, time_off, …) is the
unit of activation. Calling `clockify_search_tools` with either
`activate_group:"<group>"` **or** `activate_tool:"<tool>"` brings the
**entire containing group** online — the response payload includes
`activated_tools: [...]` enumerating every tool name now reachable,
while `activation_message` stays concise and identifies the activated
group/count without repeating the tool list. `activate_group` is the
preferred form for new code; `activate_tool` is preserved for
backwards compatibility.

### Safety and Destructive Operations
`clockify-mcp-go` provides safety hints in tool definitions (`destructiveHint: true`)
plus a structured `RiskClass` bitmask on every descriptor
(`Read | Write | Billing | Admin | PermissionChange |
ExternalSideEffect | Destructive`). Audit events for billing /
admin / permission-change tools capture the action-defining fields
(role, status, quantity, unit_price), not just the IDs.

- Clients like Claude Desktop may display a confirmation dialog before executing a destructive tool (e.g., `clockify_delete_entry`).
- The `CLOCKIFY_DRY_RUN` environment variable (default: `enabled`) enables server-side preview support for destructive tools when the caller sends `dry_run:true`. If `dry_run` is omitted or `false`, the mutation executes.
- **Non-destructive RW tools that trigger external side effects** —
  `clockify_send_invoice`, `clockify_mark_invoice_paid`,
  `clockify_test_webhook`, `clockify_deactivate_user` — also honour
  `dry_run:true`: the handler GETs a preview without executing the
  PUT/POST. Use this for any agent flow that wants a confirmation
  step before billing or admin actions land.

### Hosted-Mode Error Sanitisation
On the `shared-service` and `prod-postgres` deployment profiles,
tool-error responses to the MCP client omit upstream Clockify response
bodies (`CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1` is the profile default,
overridable). Clients see the verb / path / status only; full bodies
are still emitted to server-side slog for operator debugging. Local
deployments keep verbose errors by default for fast diagnostics.

### Hosted-Mode Webhook URL Validation
`CreateWebhook` / `UpdateWebhook` resolve the host via DNS and reject
any reply containing a private, reserved, link-local, or loopback IP
when the deployment is on a hosted profile (or
`CLOCKIFY_WEBHOOK_VALIDATE_DNS=1` is set explicitly). Local profiles
keep the literal-IP-only check. Operators can opt specific hostnames
out of the DNS check via `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS=<host>[,<host>...]`
(exact or leading-dot suffix); a webhook URL whose host matches a
listed entry will succeed even if its DNS reply contains a private IP.
Clients seeing an unexpectedly-accepted webhook in such an environment
are not observing a security regression — the operator deliberately
admitted that hostname.

### Resource Templates
The server exposes `clockify://` URI templates.
- Clients should use `resources/templates/list` to discover these.
- When a user asks for "my current timer," the client should resolve the template to a concrete URI and fetch it via `resources/read`.

## Compatibility Expectations

### Breaking Changes
We follow Semantic Versioning (SemVer). Breaking changes to the tool schema (renaming parameters, removing tools) will only occur in major releases (v2.x, etc.).
- See `docs/release-policy.md` for our full deprecation policy.

### Backwards Compatibility
The server supports multiple versions of the MCP protocol (today: `2025-11-25`, `2025-06-18`, `2025-03-26`, and `2024-11-05`). It will negotiate the highest mutually supported version during the `initialize` handshake. The canonical list lives in `internal/mcp/server.go` (`SupportedProtocolVersions`); this doc tracks it.

## Troubleshooting Client Issues

1.  **"Tools not found":** Some clients require a restart to see new tools if they don't support `notifications/tools/list_changed`. Try `clockify_search_tools` followed by a client reload.
2.  **Authentication Errors:** Ensure environment variables are passed correctly to the sub-process. Claude Desktop requires them in its `config.json` under the `env` key.
3.  **Logs:** Stdio clients often hide `stderr`. Check the client's internal log file (e.g., `~/Library/Logs/Claude/mcp.log` for Claude Desktop) to see redacted `clockify-mcp` logs.
