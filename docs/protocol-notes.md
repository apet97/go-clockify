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

## `tools/list` pagination

The full tool registry (156 tools) is fixed at startup and never changes during a
session. `tools/list` returns the advertised toolset. Small advertised surfaces,
including the default 16 everyday tools, fit in one response. Larger advertised
surfaces are cursor-paginated over the stable sorted tool list using numeric
string cursors:

```json
{
  "tools": [],
  "nextCursor": "80"
}
```

Omit `nextCursor` on the last page. The current default page size is 80 tools.

Compatibility note: the default 16-tool surface fits a single page, but
`CLOCKIFY_TOOLSET=all` advertises the full 156-tool registry across two pages —
the first `tools/list` response carries `nextCursor:"80"`, and a client that
ignores it sees only the first 80 tools. Follow `nextCursor` (pass it back as
`params.cursor`) until it is absent to enumerate the whole advertised surface.
`scripts/smoke-stdio.sh` drives this loop in all-mode and asserts the union of
both pages is exactly 156 tool names.

## Wire vs. validation: outputSchema

`tools/list` advertises an `outputSchema` for every advertised tool. Workflow
tools keep their detailed schemas; tools whose full schema would make the wire
surface too large use the compact shared `ok`/`action` envelope. More detailed
per-tool schemas remain in the generated dev catalog at
`clockify://mcp/tool-catalog`.

## High-risk confirmation

Destructive, billing, admin, permission-change, external-side-effect, and raw
mutating tools require a local `confirm_token` before live execution. First
call the tool with `dry_run:true`; the response includes confirmation metadata
and a short-lived `confirm_token`. The live retry must keep the same arguments,
omit `dry_run`, and include `confirm_token`. Tokens are bound to tool name, the
pinned workspace, risk class, and a canonical argument hash; they expire quickly
and are single-use.

## Advertised tier vs. loaded registry

`CLOCKIFY_TOOLSET` controls model-visible advertisement and callable authority.
The stdio process still loads the 156-tool registry for deterministic startup
and self-inspection, but default/core/business/admin reject `tools/call` for
unadvertised names. The default `CLOCKIFY_TOOLSET=default` advertises the daily
16-tool surface; `core`, `business`, and `admin` widen that surface; `all`
advertises and authorizes the complete registry. There is no runtime environment
reload path today, so `notifications/tools/list_changed` is not advertised for
toolset changes.

## Rate-control model

This local one-user MCP controls tool throughput with several layers:

- **`CLOCKIFY_MAX_IN_FLIGHT_TOOL_CALLS`** — global cap on concurrently-running
  `tools/call` handlers (default 4).
- **Per-risk-family concurrency caps** — ordinary writes are capped at 2 concurrent,
  and destructive / billing / admin / permission-change / external-side-effect tools at
  1 concurrent, so high-risk writes are serialized. Reads are bounded only by the
  global cap.
- **`CLOCKIFY_TOOL_RATE_LIMIT_PER_MINUTE`** — token-bucket rate cap on tool
  invocations (default 120/min). Explicit `0` disables it and doctor reports a
  warning. Risk buckets narrow the cap to 30/min writes, 10/min billing/admin/
  permission/external-side-effect calls, and 5/min destructive calls. When
  exceeded, a call returns a recoverable `ok:false` envelope with
  `error.code = "rate_limited"` and `recovery.retryAfterSeconds`.
- **`CLOCKIFY_TOOL_TIMEOUT`** — per-call deadline.
- The Clockify HTTP client uses the same timeout, adds retry/backoff and a
  circuit breaker, and tool results are size-capped by
  `CLOCKIFY_MAX_TOOL_RESULT_BYTES`. The upstream response cap remains at least
  10 MiB and rises when the tool-result cap is configured higher, so large
  export downloads can spill to a local temp file before the small MCP envelope
  is returned.
