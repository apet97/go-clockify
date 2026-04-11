# ADR 003 — Enforcement pipeline as a pluggable interface

**Status**: Accepted, 2026-04-11.

## Context

The MCP server has multiple orthogonal safety gates: policy (allow/deny
tools), rate limiting (per-window + per-subject), dry-run (intercept
destructive mutations and return a preview), truncation (cap response
size for token budgets), bootstrap (bound tool visibility). A naive
implementation would scatter these checks across `server.handle` and
every tool handler, which:

1. Couples the protocol core to every safety subsystem directly.
2. Makes it impossible to write the server without also initializing
   all of them.
3. Makes it hard to add a new gate (e.g., audit-only mode, quota
   enforcement) without touching the protocol core again.

## Decision

The protocol core (`internal/mcp`) defines a single `Enforcement`
interface with three methods:

```go
type Enforcement interface {
    FilterTool(name string, hints ToolHints) bool
    BeforeCall(ctx, name, args, hints, lookupHandler) (result, release, err)
    AfterCall(result any) (any, error)
}
```

and calls it at well-defined points in the `tools/call` dispatch:

- `FilterTool` during `tools/list` to hide disallowed tools.
- `BeforeCall` after param decoding, before invoking the handler —
  this is the single place where policy, rate limit, and dry-run
  interact with each other.
- `AfterCall` on the handler's successful result to apply truncation.

`internal/enforcement.Pipeline` is the concrete implementation that
composes the safety subsystems. It is constructed in `cmd/clockify-mcp`
and passed to `mcp.NewServer`.

## Consequences

- The protocol core has zero imports of domain safety packages. Tests
  can pass a `nil` Enforcement to skip all gates, or a recording stub
  to assert on specific calls.
- Adding a new safety gate is a change to `Pipeline` only. The
  protocol core stays stable.
- The single `BeforeCall` point means the gate order is explicit and
  testable in one place, rather than scattered across domain handlers.
- `BeforeCall` returns an optional `release` function that callers
  must invoke regardless of success or failure. The call site in
  `Server.callTool` uses `defer release()` to guarantee this.
- `AfterCall` runs a JSON roundtrip before truncation so the walker
  sees a generic tree rather than typed structs — see the comment on
  `Pipeline.AfterCall` for the full rationale.
