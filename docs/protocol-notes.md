# MCP protocol notes

Design notes for how this one-user, stdio Clockify MCP implements optional parts of
the MCP protocol.

## `resources/list` is intentionally single-page

`resources/list` and `resources/templates/list` return their full set in one
response with no `nextCursor`. This is intentional: the static resource set is fixed
and small, and the dynamic per-run demo resources are hard-capped at 50 entries
(`maxDemoResourcesListed`). The total listing therefore has a small, bounded upper
size, so cursor pagination would add protocol surface with no benefit.

## Progress notifications

Long-running tools emit `notifications/progress` when the client supplies a
`progressToken`. The server enforces the MCP rules: progress values must strictly
increase per token, and emissions are capped at 10 per second per token. Progress
stops once the originating call is cancelled or times out.

## `tools/list` is intentionally single-page

The full tool registry (156 tools) is fixed at startup and never changes during a
session, so `tools/list` returns every tool in one response with no `nextCursor`.
`TestToolsListBudgetWire` pins the serialized payload under a fixed 280 KiB
budget, far below the 4 MiB default message size, so a single-page response is
always safe to deliver.

## Wire vs. validation: outputSchema

`tools/list` advertises `outputSchema` only for the 14 composed workflow tools
whose synthesized response shape helps an agent chain calls. Domain CRUD, raw
fallback, and the non-composed workflows (`clockify_tools_guide`,
`clockify_demo_seed`, `clockify_demo_cleanup`) omit `outputSchema` on the wire.
Server-side output validation is unchanged: every tool still validates its result
against the schema in its `mcp.ToolDescriptor`. The generated dev catalog at
`clockify://mcp/tool-catalog` exposes full schemas for clients that want them.

## Rate-control model

This local one-user MCP controls tool throughput with several layers:

- **`CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS`** — global cap on concurrently-running
  `tools/call` handlers (default 4).
- **Per-risk-family concurrency caps** — ordinary writes are capped at 2 concurrent,
  and destructive / billing / admin / permission-change / external-side-effect tools at
  1 concurrent, so high-risk writes are serialized. Reads are bounded only by the
  global cap.
- **`CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE`** — optional token-bucket rate cap on tool
  invocations. `0` (the default) disables it. When exceeded, a call returns a
  recoverable `ok:false` envelope with `error.code = "rate_limited"`.
- **`CLOCKIFY_TOOL_TIMEOUT`** — per-call deadline.
- The Clockify HTTP client adds retry/backoff and a circuit breaker, and tool results
  are size-capped by `CLOCKIFY_MAX_TOOL_RESULT_BYTES`.
