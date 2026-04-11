# ADR 006 — Multi-tenant control plane and per-session runtime

**Status**: Accepted, 2026-04-11.

## Context

The original `clockify-mcp` assumed one process per human, with a
single `CLOCKIFY_API_KEY` and one workspace. That model doesn't fit
a shared-service deployment where many authenticated clients each
have their own Clockify credentials, their own workspace, and their
own view of what tools are activated (a Tier 2 group enabled for
tenant A must not leak tools into tenant B's `tools/list`).

## Decision

Introduce two layers of per-tenant state:

1. **Control plane** (`internal/controlplane`) — a persisted store
   keyed by session id. Holds `SessionRecord` with tenant id,
   subject, transport, workspace id, clockify base URL, protocol
   version, client info, and lifecycle timestamps. Pluggable via a
   DSN (`memory`, file-backed JSON, …). Non-session records
   (tenants, credentials) live in the same store.
2. **Per-session runtime** (`StreamableSessionRuntime`) — a factory
   callback supplied to `ServeStreamableHTTP` that constructs a
   brand-new `mcp.Server`, `tools.Service`, and Clockify client
   per tenant. Each session holds its own Server instance so
   activation, negotiated version, client info, and notifier
   installation are completely isolated.

The streamable HTTP transport implements the session lifecycle:
`initialize` creates a session via `sessionManager.create`, which
invokes the factory; subsequent requests look up the session by
`X-MCP-Session-ID`, verify the authenticated Principal still
matches the session's principal, and dispatch to that session's
Server.

The legacy POST-only HTTP transport retains the single-tenant
semantics — it does **not** fit this model and is explicitly
marked as not enterprise-safe in the HTTP transport guide.

## Consequences

- Shared-service deployments work out of the box with
  `MCP_TRANSPORT=streamable_http`. Per-tenant isolation is
  enforced at the Server boundary; no cross-tenant leakage is
  possible through the server core.
- Replicas are session-affine: the session's `Server` is held in
  memory on the replica that created it, so the cluster needs
  sticky-session routing (`X-MCP-Session-ID` → owning pod). The
  control plane persists enough metadata that a failover replica
  can reject or re-create a session on a different pod.
- Tool activation and notifications are session-local. A
  `notifications/tools/list_changed` from a `search_tools` call
  in session A must not fire on session B's SSE stream. The
  `sessionEventHub` + per-session `SetNotifier` wiring guarantees
  this — see Phase D (W1-01).
- Audit records carry `tenant_id`, `subject`, and `session_id`
  so operators can see which tenant was responsible for an
  action, not just "the server did something".
- More moving parts: every new feature needs to think through
  "what does this look like when it lives per-session?".
