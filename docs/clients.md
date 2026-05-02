# Client Compatibility and Behavior Notes

This document provides guidance on how various Model Context Protocol (MCP) clients interact with `clockify-mcp-go`.

## Supported Client Matrix

The release-tested client surface is intentionally narrow: MCP
clients launch `clockify-mcp` as a subprocess over stdio, and the
Clockify API key is passed through the child-process environment.
HTTP and gRPC are supported server transports, but the project does
not claim that any off-the-shelf desktop client has been release-tested
against those modes unless the table says so.

| Client | Tested connection | Auth mode | Release status | Untested combinations |
|--------|-------------------|-----------|----------------|-----------------------|
| Claude Code | `stdio` subprocess | n/a; parent process supplies `CLOCKIFY_API_KEY` | Tier 1, release-tested via stdio smoke and manual client use | `streamable_http`, `grpc`, hosted OIDC, `forward_auth`, `mtls` |
| Claude Desktop | `stdio` subprocess from `claude_desktop_config.json` | n/a; env block supplies `CLOCKIFY_API_KEY` | Tier 1, release-tested manually; renders confirmation dialogs for `destructiveHint: true` tools | `streamable_http`, `grpc`, hosted OIDC, `forward_auth`, `mtls` |
| Cursor | `stdio` subprocess from `.cursor/mcp.json` | n/a; env block supplies `CLOCKIFY_API_KEY` | Tier 1, release-tested manually | `streamable_http`, `grpc`, hosted OIDC, `forward_auth`, `mtls` |
| Codex | `stdio` subprocess from Codex MCP config | n/a; env block supplies `CLOCKIFY_API_KEY` | Tier 1, release-tested manually | `streamable_http`, `grpc`, hosted OIDC, `forward_auth`, `mtls` |
| VS Code MCP | `stdio` subprocess from VS Code MCP server config | n/a; env block supplies `CLOCKIFY_API_KEY` | Compatible shape, not a release-blocking client until a repeatable VS Code smoke is added | All non-stdio transports; any hosted auth mode |
| Custom Streamable HTTP client | `streamable_http` on `/mcp` | `static_bearer`, `oidc`, `forward_auth`, or `mtls` per `docs/auth-model.md` | Server transport is release-tested by parity, smoke, and shared-service E2E; the custom client implementation is operator-owned | Client-specific UI semantics, retry policy, and auth-token refresh |
| Custom gRPC client | `grpc` bidirectional `Exchange` stream | `static_bearer`, `oidc`, `forward_auth`, or `mtls` per `docs/auth-model.md` | Server transport is release-tested behind `-tags=grpc`; use the private-network gRPC profile | Public-internet gRPC, browser clients, and clients without mTLS / token refresh support |

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

### Session Rehydration Boundaries
The streamable-HTTP transport (MCP 2025-03-26) supports cross-pod session rehydration: when a request lands on a replica that did not run the original `initialize`, the server reads the persisted session from the shared control-plane store and rebuilds the per-tenant runtime in-place. From the client's perspective this is invisible — `tools/call`, `resources/read`, etc. all succeed without the client having to retry or re-initialize. See [`docs/adr/0017-streamable-http-session-rehydration.md`](adr/0017-streamable-http-session-rehydration.md) for the design.

What survives the rehydration boundary:
- The session ID itself (clients keep using the same `MCP-Session-Id` header).
- The negotiated MCP protocol version, client name, and client version (the persisted session record carries them; the server seeds them into the rebuilt runtime via `Server.MarkInitialized`).
- The persisted `expiresAt` / TTL window — rehydration does NOT reset the eviction clock.
- The strict per-request authentication contract: the rehydrated session inherits the freshly-authenticated principal of the request that triggered it. A leaked session ID replayed with a different `X-Forwarded-Tenant` (or different OIDC subject) is rejected with **403 "session principal mismatch"**, matching the local-hit behaviour.

What does NOT survive the rehydration boundary:
- **In-flight tool-call cancellation.** A `tools/call` running on instance A cannot be cancelled by sending `notifications/cancelled` to instance B; B has no record of the in-flight call and the cancel is a silent no-op. Clients should treat cancellation as best-effort across rehydration; if a cancel is critical, route the cancel back to the original instance (when sticky-session affinity is in use, this is the common case).
- **SSE backlog.** A long-lived `GET /mcp` SSE stream that resumes against a freshly-rebuilt session sees `oldest > Last-Event-ID + 1` because the new instance's `sessionEventHub` ring buffer is empty. The server's `SSEReplayMissesTotal` Prometheus counter increments on this boundary; clients should fall back to a fresh subscription and re-fetch any state delta they were tracking (the MCP `notifications/resources/list_changed` contract permits this).
- **Server-side tool-call counters and rate-limit token state.** Rebuilt fresh; per-tenant rate-limit accounting is best-effort across the boundary. Operators who need strict cross-replica rate limiting should pair the deployment with an external proxy (Envoy, an API gateway) and cap there.

Practical client guidance:
- **Retry idempotent calls** — `tools/list`, `clockify_list_*`, `clockify_get_*`, and any `resources/read` call are safe to retry verbatim. The MCP spec already recommends this for transient-network errors; rehydration is a bounded subset of the same envelope.
- **Avoid speculative `notifications/cancelled`** — issue a cancel only after a request goes long enough that the user explicitly asks to stop. Across rehydration the cancel may not reach the original handler.
- **Subscribe to `notifications/resources/list_changed` and `notifications/tools/list_changed`** — both events fan out from the per-session notifier hub on the instance the client is currently pinned to. Across rehydration you'll receive these on the new instance once a state-affecting tool has run there.

The deployment's load balancer typically pins each client to one replica via ClientIP affinity (the default in the Helm chart's `service.sessionAffinity.enabled`), so most clients never observe a rehydration boundary in normal operation. The boundary is reached during pod restart / eviction, rolling upgrades, cross-AZ failover, and the shared-NAT egress case where many clients hash to the same backend.

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
