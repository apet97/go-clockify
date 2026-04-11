# ADR 007 — Streamable HTTP (2025-03-26) adoption

**Status**: Accepted, 2026-04-11.

## Context

The legacy `MCP_TRANSPORT=http` is a bespoke POST-only JSON-RPC
transport: it does not implement the 2025-03-26 MCP Streamable HTTP
spec, does not support session-scoped state, cannot deliver
server-initiated notifications, and therefore cannot advertise
`capabilities.tools.listChanged`. It's fine for backward-compatible
single-tenant compatibility deployments, but it's not a valid
shared-service transport.

The MCP Streamable HTTP 2025-03-26 spec defines a single `/mcp` path
that multiplexes POST (RPC) and GET (SSE for server→client
notifications), plus `Last-Event-ID` resumability and an
`Mcp-Protocol-Version` request-header contract. Supporting the spec
is table stakes for any 2025-era MCP client.

## Decision

Introduce `internal/mcp/transport_streamable_http.go` with:

- **Session management** (`streamSessionManager`) — creates a per-
  session `mcp.Server` via a factory callback and stores it keyed by
  a cryptographically-random `X-MCP-Session-ID`. See ADR 006.
- **Authenticated POST /mcp** — JSON-RPC dispatch with session lookup,
  principal comparison, Mcp-Protocol-Version validation, and
  principal context propagation (so the enforcement layer can bucket
  rate limits per subject — W1-07).
- **SSE GET /mcp** — a persistent SSE stream of
  server-initiated notifications for the subscribed session.
  Reconnecting clients may send `Last-Event-ID: N`, and the
  `sessionEventHub.subscribeFrom` helper replays only backlog events
  with `id > N`. Events carry monotonically-increasing per-session
  `id` stamps.
- **Back-compat alias** — `GET /mcp/events` is kept as a deprecated
  alias for `GET /mcp` through the 0.6 release and will be removed
  in 0.7. Phase D's `CHANGELOG.md` entry and `docs/migration/0.5-to-0.6.md`
  document the deprecation window.
- **Legacy transport preserved** — `MCP_TRANSPORT=http` still works
  and is documented as the compatibility transport. The two
  transports share their security headers, CORS handling, and auth
  helpers, but their dispatch paths are intentionally separate.

Spec compliance status after Phase D:

- Mcp-Session-Id header ✓
- GET /mcp SSE stream ✓
- Last-Event-ID resumability ✓
- Mcp-Protocol-Version header validation ✓
- Multiple concurrent sessions per replica ✓
- SSE event id stamping ✓

## Consequences

- Enterprise clients get a spec-compliant transport with proper
  session isolation, resumability, and server-initiated
  notifications.
- The legacy HTTP transport is now explicitly capability-reduced and
  documented as such. Users who need listChanged or SSE must
  migrate.
- Replicas are session-affine, so the cluster needs sticky-session
  routing. The control plane persists session metadata so a replica
  can reject an inbound session it doesn't own (returning 401 +
  `WWW-Authenticate`).
- Session-aware deployments must set `MCP_RESOURCE_URI` when using
  OAuth 2.1 to bind tokens to the canonical resource URI per RFC
  8707 — enforced by the authn layer (W1-06).
