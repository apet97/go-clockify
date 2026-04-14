# 0009 - Resource delta-sync subscriptions

## Status

Accepted — landed across W3-03a..f (`5fcdfaa`, `ffb2f8c`, `693d22f`,
`20cf724`, `c0702f4`, `d1a699b`); follow-ups in W4-04a..d
(`14d4095`, `1cab9dd`, `e3f4a4b`, `c53f17b`); RFC 6902 alternative
in W5-04d (`2ed30aa`).

## Context

The MCP `resources/` capability lets clients subscribe to a
`clockify://` URI and receive `notifications/resources/updated`
when the underlying resource changes. The spec leaves the
notification payload as `{"uri": "..."}`, which forces the client
to re-fetch the entire resource on every update — even if the
change was a single field.

Clockify resources can be large: a workspace's `weekly-report`
resource is ~50 KB. A typical edit (changing a description, toggling
billable, adjusting a tag) modifies a few bytes. Re-fetching the full
resource on every edit is bandwidth-wasteful for the client and
load-wasteful for the server, especially on streamable HTTP where
multiple sessions may subscribe to the same URI.

We need a way to deliver minimal delta payloads on
`notifications/resources/updated` while staying spec-compatible with
clients that only read the `uri` field.

## Decision

Extend the `notifications/resources/updated` payload with an
optional delta envelope, computed from a server-side state cache
and emitted only when a subscription is active.

Design elements:

- **Subscription gate.** `resourceSubscriptions` (a `sync.Map`-backed
  set in `internal/mcp/resources.go:48-54`) tracks which URIs have
  active subscribers. `NotifyResourceUpdated` no-ops on URIs with
  no subscribers, so unsubscribed mutations do not pay for a
  redundant `ReadResource` round trip (W4-04c).
- **State cache.** `tools.Service` keeps a per-URI cache of the most
  recently emitted resource state. On a mutation the tool layer
  reads the new state, diffs it against the cache, and emits the
  delta. Cache write-through happens on the response path (W4-04d)
  so the next mutation diffs against the freshly-served state.
- **Wire format.** The notification params carry `format` and
  `patch` keys when a delta is emitted; the legacy `{"uri": "..."}`
  shape is preserved when no delta is available. Format codes are
  defined in `internal/jsonmergepatch`:
  - `none` — URI-only legacy payload.
  - `merge` — RFC 7396 JSON Merge Patch (default).
  - `full` — full resource snapshot (when a merge-patch is larger
    than the snapshot).
  - `deleted` — resource was removed.
  - RFC 6902 JSON Patch is also available as an alternative format
    (W5-04d, opt-in via `MCP_DELTA_FORMAT=jsonpatch`).
- **Spec compatibility.** The extension is additive: clients that
  only read `uri` keep working. No MCP protocol version bump is
  required. The wire-format extension is documented in the
  `ResourceUpdateDelta` doc comment in `internal/mcp/resources.go:148-158`.
- **Mutation wiring.** Tool handlers that mutate state (entries,
  timers, projects, users, week boundaries) call
  `service.emitResourceUpdate` with the affected URIs after a
  successful mutation. The emit helper handles the cache lookup,
  the diff, and the `Notify` call; the tool handler does not need
  to know about subscriptions or formats.

## Consequences

### Positive

- Subscribed clients receive minimal patches instead of full
  re-fetches. A single field edit on a weekly report sends a
  ~50-byte merge patch instead of a 50 KB re-read.
- The subscription pre-gate (W4-04c) means unsubscribed mutations
  cost zero extra: no `ReadResource`, no diff, no marshalling.
- Cache write-through (W4-04d) eliminates a stale-cache failure
  mode where two back-to-back mutations would both diff against
  the pre-mutation state and emit overlapping patches.
- The wire format is additive — a 2024-vintage MCP client that
  only knows the `{"uri": "..."}` shape continues to work without
  changes.
- The format-code abstraction (`none` / `merge` / `full` /
  `deleted`) leaves room for RFC 6902 (already added) and any
  future delta format without another wire-format change.

### Negative

- The `tools.Service` now carries a state cache that is per-server,
  not per-tenant. For multi-tenant streamable HTTP, each session
  runtime has its own `tools.Service` instance (per ADR 0004's
  pluggable enforcement), so cross-tenant state cannot leak — but
  a contributor adding a new mutation must remember to call the
  emit helper or the cache will go stale.
- The diff cost on the server is bounded but non-zero. The
  trade-off is favourable for resources that are read-heavy
  (the dominant case) and unfavourable for resources that mutate
  faster than they are read. We accept this on the assumption
  that subscribed clients are interested in updates.
- The `format=full` fallback exists because merge-patches can be
  larger than the snapshot in pathological cases (e.g. when most
  fields change). The emit helper picks `full` when the patch
  would exceed the snapshot size.

### Neutral

- `HasResourceSubscription` is a public method on `mcp.Server` so
  the tool layer can check before paying for the `ReadResource`.
  Exposing internal subscription state is fine because the gate
  is intended to be observable.
- The default delta format is `merge` (RFC 7396). Operators who
  prefer RFC 6902 JSON Patch can set `MCP_DELTA_FORMAT=jsonpatch`,
  but the wire format is otherwise identical so clients negotiate
  per-notification rather than per-session.

## Alternatives considered

- **Always send full snapshots** — rejected on bandwidth grounds
  (see Context).
- **Require clients to poll resources/read after every change** —
  rejected because polling defeats the point of the subscriptions
  capability and adds latency.
- **Emit RFC 6902 JSON Patch as the default format** — deferred,
  not rejected. RFC 7396 merge-patch is simpler and produces
  smaller payloads for the dominant edit shape (single field
  change). RFC 6902 is available as an opt-in for clients that
  prefer it.
- **Bump the MCP protocol version to make the delta envelope
  mandatory** — rejected because the additive payload extension
  works with every existing client and a mandatory bump would
  break older clients for no benefit.

## References

- Previously referred to as "ADR 013" in
  `internal/tools/common.go:50`.
- Subscription set: `internal/mcp/resources.go:48-54`.
- Wire format: `internal/mcp/resources.go:148-198`
  (`ResourceUpdateDelta`, `NotifyResourceUpdated`).
- Diff implementation: `internal/jsonmergepatch/merge_patch.go`
  (RFC 7396), and the RFC 6902 alternative landed in W5-04d.
- Mutation wiring: `internal/tools/common.go` (`EmitResourceUpdate`)
  and the per-domain tool files (`entries.go`, `projects.go`, etc.).
- Subscription pre-gate: `internal/mcp/resources.go:165-167`
  (`HasResourceSubscription`).
- Related ADRs: 0002 (transports that can deliver
  server-initiated notifications), 0004 (per-tenant `tools.Service`
  isolation that scopes the state cache).
- Related docs: `README.md` "Resources & prompts".
- Spec: <https://datatracker.ietf.org/doc/html/rfc7396> (JSON Merge
  Patch), <https://datatracker.ietf.org/doc/html/rfc6902> (JSON
  Patch).
