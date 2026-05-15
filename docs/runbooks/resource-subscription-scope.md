# Resource subscription scope

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


`notifications/resources/updated` is delivered **per subscribed stream**,
not broadcast to every connected stream sharing the same `*mcp.Server`.
Other server-initiated notifications (`notifications/tools/list_changed`,
`notifications/progress`, cancellation, etc.) continue to fan out to every
active `Notifier` via the `notifierHub`. This page describes the contract
so on-call understands what is wired and what regressions look like.

## What the guarantee is

For any URI `X`:

1. Stream A calls `resources/subscribe { uri: X }`. The server records the
   subscription against A's `Notifier`.
2. A mutation invokes `Server.NotifyResourceUpdated(X, …)`.
3. Only the streams whose `Notifier` had `X` in its subscription set
   receive `notifications/resources/updated`. A stream that never
   subscribed (or that subscribed only to `Y`) receives nothing.

`tools/list_changed` and any other call via `Server.Notify(...)` go
through the broadcast `notifierHub` path and reach every active notifier.

## Subscription lifecycle

- `mcp.WithNotifier(ctx, n)` carries the calling stream's `Notifier`
  through dispatch context. Every transport wraps ctx after
  `AddNotifier` / `SetNotifier`:
  - gRPC: `internal/transport/grpc/transport.go` (the Exchange handler).
  - stdio: `internal/mcp/server.go` (`Run`, single notifier for the
    process lifetime).
  - Streamable HTTP: `internal/mcp/transport_streamable_http.go`
    (the per-request dispatch site).
- `resources/subscribe` reads the notifier from ctx and stores the
  subscription against it. A subscribe call without a notifier in
  context (only test code that drives the handler directly) falls back
  to a `broadcastSubscriber` sentinel that restores the pre-fix
  server-wide fan-out for that one call. When both a per-notifier
  subscriber and the broadcast sentinel are recorded for the same URI
  (mixed-mode test setup, not reachable in production), `NotifyResourceUpdated`
  takes the broadcast branch alone — the hub fan-out is the strict
  superset, so every interested party is delivered to at most once.
- `notifierHub.add` returns a remove closure. The closure calls
  `Server.resourceSubs.dropNotifier(n)` on stream close so subscriptions
  are GC'd. Without this the `HasResourceSubscription` shortcut would
  keep reporting an active subscriber and the tools layer would keep
  paying for `ReadResource` round-trips nobody wants.

## What a regression looks like

- A client reports it received `resources/updated` frames for URIs it
  never subscribed to. Likely cause: a new transport added without
  wrapping its dispatch ctx with `WithNotifier`, or a refactor that
  routed `NotifyResourceUpdated` back through `Server.Notify`.
- `clockify_mcp_resource_updates_emitted_total` keeps climbing on a URI
  with no live subscribers after a disconnect. Likely cause: the
  notifier-removal GC stopped firing (e.g. the `onRemove` callback on
  `notifierHub` was unset by a constructor change).

## Pinned-by tests

- `internal/mcp/resource_subscription_scope_test.go`
- `internal/transport/grpc/resource_scope_test.go`
  (`TestExchange_ResourceSubscription_PerStream` — real wire path with
  two `Exchange` streams sharing one `*mcp.Server`).

When debugging a suspected scope regression, run those two files
first; they exercise the exact code path the fix relies on.

## Related decisions

- ADR 0009 — "Resource delta-sync subscriptions" (the wire contract
  and the "Subscription scope" addendum that documents this fix).
- ADR 0002 — transports that can deliver server-initiated
  notifications. `notifications/resources/updated` requires either
  streamable HTTP or gRPC; the deprecated POST-only legacy HTTP
  transport uses `droppingNotifier` and does not deliver
  notifications at all.
