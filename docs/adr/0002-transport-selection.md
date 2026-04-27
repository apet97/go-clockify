# 0002 - Transport selection

## Status

Accepted — the four-transport surface has been stable since the
streamable_http rewrite (commit `86cc7f4`, v0.6.0) and the gRPC
sub-module (commit `614b775`, v0.7.0).

## Context

MCP clients fall into three populations with very different needs:

1. **Local CLI clients** (Claude Code, Claude Desktop, Cursor, Codex)
   that launch the server as a child process. The parent process is
   trusted; there is no inbound auth.
2. **Shared HTTP services** behind a TLS-terminating reverse proxy,
   either single-tenant (one bearer secret) or multi-tenant
   (per-installation OIDC tokens, session resumption).
3. **Bidirectional low-latency clients** on a private network (service
   mesh, internal automation) that need a streaming RPC channel and
   prefer mTLS or metadata-based auth.

A single transport cannot serve all three without compromising one or
more of: protocol fidelity, security posture, supply-chain weight, or
operator UX. We need to expose the transport choice as a first-class
configuration knob and document when each is the right pick.

## Decision

Four transports, selected via `MCP_TRANSPORT` and validated in
`internal/config/config.go` `Load()` (search for
`cfg.Transport = os.Getenv("MCP_TRANSPORT")`):

| Transport         | Default | When to pick                                                                              |
|-------------------|---------|-------------------------------------------------------------------------------------------|
| `stdio`           | yes     | Local CLI clients. Parent process is trusted; no inbound auth. Required by the MCP spec for the canonical local-process binding. |
| `http`            | no      | Single-tenant HTTP service behind a TLS-terminating proxy. Original POST-only transport, simpler than streamable_http. Capability-reduced (no SSE, no listChanged). |
| `streamable_http` | no      | Multi-tenant HTTP service for spec-strict 2025-06-18 / 2025-03-26 MCP clients. Per-installation tokens, session resumption via `Last-Event-ID`, server-initiated notifications via SSE. |
| `grpc`            | no      | Bidirectional low-latency clients on a private network. Build-tag opt-in (`-tags=grpc`); not in the default binary. |

`stdio` is the default per the MCP local-process binding. The other
three transports require explicit operator opt-in via env var.

`grpc` is the only transport whose code is not compiled into the
default binary — it lives behind the `grpc` build tag in
`internal/transport/grpc/` and the top-level `go.mod` has zero
`google.golang.org/grpc` rows (enforced by ADR 0001 / ADR 0008). The
default-build stub at the gRPC dispatch site returns a clear error
explaining the build-tag requirement.

## Consequences

### Positive

- Operators get a single env var per transport choice. The
  capability matrix is documented in
  `docs/production-readiness.md` "Pick a transport".
- The two HTTP transports share security headers, CORS handling, and
  the authn pipeline (`internal/authn`), so a hardening fix lands
  once and protects both.
- gRPC is invisible to operators who do not need it. The default
  binary's SBOM has zero gRPC rows.
- `stdio` stays the smallest, fastest, and safest path for the
  populated client: there is nothing to authenticate, nothing to
  CORS-check, and the protocol frames go straight to the dispatcher.

### Negative

- Four transports is more surface area to test. The dispatch path is
  shared (`mcp.Server.DispatchMessage`) so each transport's tests
  focus on its wire-level concerns (framing, session lookup, auth
  gating) rather than re-testing the full tool surface.
- `http` (the legacy POST-only transport) is intentionally kept
  alive for back-compat. It has fewer capabilities than
  `streamable_http` and is documented as such — an operator who
  wants `tools/list_changed` notifications via HTTP must use
  `streamable_http`.
- gRPC's build-tag isolation means a contributor working on the gRPC
  code path cannot use the default `make check` workflow; they must
  pass `-tags=grpc` explicitly.

### Neutral

- All four transports terminate at the same `mcp.Server` instance, so
  there is no "transport-specific tool dispatcher" to worry about.

## Alternatives considered

- **Single HTTP transport, drop stdio** — rejected because the MCP
  local-process binding is spec-mandated and is the dominant client
  population today.
- **Single streamable_http transport, drop legacy http** — deferred,
  not rejected. The legacy `http` transport will be removed in a
  future major bump; for now it stays as a single-tenant compatibility
  path for operators who do not need SSE or session resumption.
- **gRPC in the default binary** — rejected because `google.golang.org/grpc`
  is a heavy dependency tree that would violate ADR 0001 and inflate
  the default SBOM by ~40%.

## References

- Validation: `internal/config/config.go` `Load()` — the `MCP_TRANSPORT`
  switch (search for `cfg.Transport = os.Getenv("MCP_TRANSPORT")`).
- Dispatch wiring: `internal/runtime/runtime.go` `Runtime.Run` —
  the transport switch (search for `switch r.cfg.Transport`).
- Streamable HTTP implementation: `internal/mcp/transport_streamable_http.go`.
- Legacy HTTP implementation: `internal/mcp/transport_http.go`.
- gRPC implementation: `internal/transport/grpc/transport.go`.
- Related ADRs: 0003 (auth-mode compatibility matrix), 0008 (gRPC
  build tag).
- Related docs: `docs/production-readiness.md` "Pick a transport",
  `README.md` "Configuration".
- Spec: <https://modelcontextprotocol.io/specification/2025-06-18>
