# ADR 013 — Delta-sync notifications/resources/updated via RFC 7396 Merge Patch

**Status**: Accepted, 2026-04-12.

## Context

Wave 1 (W1-04) landed the `resources/subscribe` and
`resources/unsubscribe` MCP methods plus the protocol-core machinery
to publish `notifications/resources/updated` to subscribed clients.
The notification carried only one field — `{"uri": "..."}` — so a
client that wanted to react to a change had no choice but to call
`resources/read` immediately afterwards and re-fetch the entire
document. For Clockify entities like time entries (which clients may
mutate dozens of times per session) the wasted bandwidth and the
extra Clockify API round-trips per subscriber were significant.

Worse, the Wave 1 subscription set was effectively dormant. A grep
across the tools layer at the start of Wave 3 showed **zero call sites
for `Server.NotifyResourceUpdated`**. Subscribers who opted in never
saw any notifications because no mutation tool was wired to the
emitter. Wave 3 task T-3 had two goals: (a) extend the wire format
with a delta envelope so clients can apply minimal patches instead of
re-fetching, and (b) wire every Tier 1 mutation that touches a
subscribed URI shape so the subscription set is finally reachable.

Before implementation, three delta-format options were on the table:

1. **RFC 6902 JSON Patch.** Canonical, array-of-ops format. Hand-
   rolling correctness across add/remove/replace/move/copy/test plus
   JSON Pointer escaping is ~300-500 lines of code. Strictly more
   expressive than the other options, especially for arrays.
2. **RFC 7396 JSON Merge Patch.** Simpler. The patch is itself a
   JSON document shaped like the target with `null` meaning delete.
   ~80 lines to hand-roll. Limitation: cannot encode JSON null values
   in the target document (collides with delete signalling), and
   represents array changes as full replacements rather than per-
   element ops.
3. **`changed_keys: [...]` flat list.** Trivial to compute, but
   forces clients to re-fetch anyway. Smallest code, smallest value.

User feedback in the Wave 3 plan was explicit: pick RFC 7396. The
mostly-flat shape of Clockify entities (time entries, projects, users)
makes the array-mutation limitation acceptable, and the null-encoding
limitation only affects fields that the Clockify API rarely returns
as null (and is handled with a fallback, see below).

## Decision

Implement RFC 7396 JSON Merge Patch as a stdlib-only sub-package at
`internal/jsonmergepatch/`. The package exposes two functions:

- `Diff(prev, curr []byte) ([]byte, error)` — minimal merge patch.
- `Apply(prev, patch []byte) ([]byte, error)` — reference apply
  algorithm, included for tests and downstream consumers.

A wrapper `DiffOrFull(prev, curr) (patch []byte, format string, err
error)` handles the null-encoding edge case: if `curr` contains any
JSON null value anywhere in its tree, the wrapper returns the
**full document** under `format = "full"` instead of a merge patch.
Clients see the format code, choose the right apply path, and never
silently lose null values.

### Wire format

`notifications/resources/updated` params shape (additive extension):

```jsonc
{
  "uri":    "clockify://workspace/{ws}/entry/{id}",
  "format": "merge",
  "patch":  { "description": "new", "billable": true }
}
```

`format` is one of:

| Format     | Patch field present | Semantics                                                                 |
|------------|---------------------|---------------------------------------------------------------------------|
| `none`     | no                  | No prior cached state; client should fetch via `resources/read`.          |
| `merge`    | yes                 | Apply RFC 7396 merge patch against cached state to get current.           |
| `full`     | yes                 | Replace cached state wholesale with the document under `patch`.           |
| `deleted`  | no                  | Resource is gone; drop cached state. Don't `resources/read` (will 404).   |

`format` is always emitted alongside `uri` when delta-sync is in
play. When the tools layer has no `EmitResourceUpdate` hook wired
(e.g. unit tests), notifications fall through to the legacy
`{"uri": "..."}` shape — the protocol core's
`Server.NotifyResourceUpdated(uri, ResourceUpdateDelta{})` constructs
the legacy payload when the delta is zero-valued.

The extension is **strictly additive**. Clients that read only `uri`
keep working unchanged. No MCP protocol version bump is required.

### Delta-sync pipeline

```
mutation tool                 │
─────────────────             │
clockify_update_time_entry    │
   API call returns           │
       └── emitResourceUpdate(ctx, uri)
                              │
                              ▼
              tools.Service.emitResourceUpdate
                              │
                              │   1. ReadResource(uri)        // re-read via existing path
                              │   2. cache.get(uri)            // prior snapshot
                              │   3. cache.put(uri, newState)  // refresh
                              │   4. DiffOrFull(prev, curr)   // RFC 7396
                              │   5. EmitResourceUpdate(uri, delta)
                              │
                              ▼
                Server.NotifyResourceUpdated
                              │
                              │   subscription gate (sync.Map)
                              │
                              ▼
                  notifier.Notify("notifications/resources/updated", params)
                              │
                              ▼
        stdio / streamable HTTP / gRPC transport client
```

Cache: bounded LRU at `internal/tools/resource_cache.go`, default
1024 entries, defensive-copy on every get/put. URIs that exceed the
cap fall out of the cache and the next notification for them emits
`format=none`, prompting a single re-fetch — correct, just less
efficient.

### Mutation wiring

Every Tier 1 and Tier 2 mutation that touches a URI matching one of
the five subscription templates is wired to call
`emitResourceUpdate` after the API call returns successfully:

| URI template                                           | Wired tools (Wave 3)                                                                                       |
|--------------------------------------------------------|------------------------------------------------------------------------------------------------------------|
| `clockify://workspace/{ws}/entry/{id}`                 | clockify_add_entry, clockify_update_entry, clockify_delete_entry, clockify_start_timer, clockify_stop_timer |
| `clockify://workspace/{ws}/project/{id}`               | clockify_create_project, clockify_update_project_estimate, clockify_set_project_memberships, clockify_archive_projects |
| `clockify://workspace/{ws}/user/{id}`                  | (out of scope — no Tier 1 user mutation tool yet)                                                          |
| `clockify://workspace/{ws}`                            | (out of scope — workspace mutations rare and not exposed as Tier 1 tools)                                  |
| `clockify://workspace/{ws}/report/weekly/{weekStart}`  | (out of scope — derived; see Follow-ups)                                                                   |

Mutations that target unrelated URI shapes (clients, tags, tasks,
invoices, expenses, etc.) are intentionally not wired: those URIs
are not exposed as resource templates and therefore cannot be
subscribed to.

Delete paths use `emitResourceDeleted(uri)` which drops the cache
entry and emits `format=deleted`. Re-reading would return 404 and
make `emitResourceUpdate` fall through to `format=none`, leaving
clients in an ambiguous state.

## Consequences

- **Subscribers see real notifications.** Wave 1's dormant
  subscription set is finally reachable. Existing clients that
  subscribe and only consume the `uri` field keep working unchanged
  but now receive the events they were already listening for.
- **Delta-aware clients save bandwidth.** A typical
  `clockify_update_time_entry` that flips `billable` from `true` to
  `false` produces a ~30-byte merge patch instead of a ~600-byte
  resource re-fetch. For high-mutation sessions the bandwidth and
  Clockify API quota savings are real.
- **Extra ReadResource per mutation.** The emit helper re-reads the
  resource through `ResourceProvider.ReadResource` to get the fresh
  state in the same shape a subscribed client would see. That's an
  extra Clockify GET per mutation, even when no client is
  subscribed (the gate is in the protocol core, downstream of the
  helper). A future optimisation can short-circuit when
  `Server.HasSubscriptions(uri)` is false; for Wave 3 the simpler
  always-emit pattern is acceptable because mutation rates are
  bounded by the existing rate limit and dispatch semaphore.
- **Stdlib-only invariant preserved.** The merge-patch differ is
  hand-rolled in `internal/jsonmergepatch/`, ~250 LOC plus tests.
  Zero new third-party dependencies. ADR 001 still holds.
- **Wire format is additive.** No MCP protocol version bump; clients
  that ignore the new fields are unaffected.
- **JSON null limitation is bounded.** When `curr` contains any null
  value, `DiffOrFull` falls back to `format=full` and ships the whole
  document. Clients still get a working delta envelope; they just
  don't get the bandwidth saving. The cost is opaque to callers.

## Follow-ups

- **Cache write-through.** Mutation tools could pass the post-API
  response directly into the cache (skipping the extra
  `ReadResource`) when the response shape happens to match the
  resource view. Worth measuring after Wave 3 ships.
- **Subscription gate before re-read.** Expose
  `Server.HasResourceSubscription(uri) bool` so the tools layer can
  skip the re-read entirely when nothing is subscribed. Trades a
  small surface increase for an API call savings on the unsubscribed
  hot path.
- **Weekly report URI emission.** Mutating an entry inside week W
  invalidates `clockify://workspace/{ws}/report/weekly/{W}`. Wiring
  that requires either pre-computing the week containing the entry
  or fan-outing the notification to every weekly URI in the
  subscription set. Out of scope for Wave 3.
- **User and workspace URI emission.** Tier 2 user-admin tools that
  mutate user state should emit too. Catalog of user mutations + a
  W4 wiring task.
- **RFC 6902 alternative.** If a future client really needs array-
  element granularity, an opt-in `format=jsonpatch` code can be added
  alongside the existing four. The wire-format string registry is
  already extensible.

## Status

Landed on `main` across W3-03a..f commits in the 2026-04-12 session:

- W3-03a — `feat(jsonmergepatch): RFC 7396 merge-patch differ`
- W3-03b — `feat(resources): extend notifications/resources/updated wire format`
- W3-03c — `feat(resources): state cache and emit helper in tools.Service`
- W3-03d — `feat(resources): wire entry and timer mutations to delta-sync`
- W3-03e — `feat(resources): wire project mutations to delta-sync`
- W3-03f — this ADR
