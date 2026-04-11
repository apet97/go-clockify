# ADR 004 — Dispatch-layer goroutine semaphore

**Status**: Accepted, 2026-04-11.

## Context

The stdio transport reads JSON-RPC requests line-by-line from stdin and
dispatches each `tools/call` into a goroutine so a slow handler cannot
head-of-line-block faster requests. Without a cap, a caller can queue
up thousands of pending tool calls in a burst, each spawning a goroutine
that sits waiting for the rate limiter. The goroutine count explodes
and the scheduler + GC start dominating CPU.

The existing business-layer rate limiter (`CLOCKIFY_MAX_CONCURRENT`,
`CLOCKIFY_RATE_LIMIT`) caps the number of *active* tool calls, but
goroutines have already been spawned by the time they reach the
limiter — they just park waiting to acquire a slot. That's still a
goroutine leak under sustained burst load.

## Decision

Add a **dispatch-layer** semaphore, bounded by the new
`MCP_MAX_INFLIGHT_TOOL_CALLS` env var (default 64). It is a buffered
channel of `struct{}` acquired inside `Server.Run` **before** spawning
the goroutine that will handle the `tools/call`. If the semaphore is
full, the scanner loop blocks on `<-toolCallSem` instead of spawning
another goroutine.

This creates two independent caps with different jobs:

- **Dispatch cap** (`MCP_MAX_INFLIGHT_TOOL_CALLS`, default 64) —
  bounds goroutine count. A burst backpressures through the stdio
  scanner channel, which is the natural place to push back.
- **Business cap** (`CLOCKIFY_MAX_CONCURRENT`, default 10) — bounds
  concurrent calls to the upstream Clockify API. This is smaller
  because Clockify rate limits are per-API-key.

Each cap can reject without stranding resources in the other: the
dispatch cap blocks *before* any goroutine exists, so there's nothing
to release if a rate limit kicks in later; the business cap's Acquire
and Release always pair up inside the already-running goroutine.

## Consequences

- Goroutine count is now bounded and predictable. `go_goroutines`
  oscillates around `MCP_MAX_INFLIGHT_TOOL_CALLS` even under burst
  load.
- Sustained input faster than the server can process backpressures
  through the stdio scanner — the OS buffers it in stdin, the peer
  sees slow reads, the peer's JSON-RPC round-trip latency goes up.
  This is the right failure mode: the client can see backpressure
  and slow down, rather than the server silently getting slower.
- The two caps need to be tuned together. Setting the business cap
  greater than or equal to the dispatch cap is pointless — the
  business cap becomes dead code. `MCP_MAX_INFLIGHT_TOOL_CALLS=0`
  disables the dispatch cap for users who prefer the old behaviour.
- The HTTP transports don't need the dispatch cap — `net/http` already
  bounds connection-derived goroutines via `MaxHandlers` / keepalive.
  Only the stdio loop needs this explicit guard.
