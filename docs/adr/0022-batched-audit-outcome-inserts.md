# 0022 - Batched `audit_events` INSERT for non-strict outcomes

## Status

Proposed — 2026-05-12. This ADR captures the design for Wave 7.1.
A follow-up implementation commit on the same branch flips Status to
**Accepted** and lands the `batchedAuditor` wrapper plus the
`Store.AppendAuditEventBatch` interface method.

## Context

`internal/controlplane/postgres/postgres.go::AppendAuditEvent`
(lines 363-390) writes one audit row per call via
`pool.Exec(...)`. The default `pgxpool.Pool` size is 4 (driven from
the DSN; not explicitly overridden), so the sustained ceiling is
~400–1500 events/s depending on RTT to the Postgres primary. The
two-phase audit pattern emits two rows per mutating tool call
(intent + outcome), so a hosted multi-tenant deployment saturates
the audit path before saturating any other backend resource.

Migration `004_audit_events_tenant_at_index.sql` already shipped
the composite `(tenant_id, at)` index; that is a read-side win and
does not address the write-side bottleneck.

The synchronous `AppendAuditEvent` path carries two semantic
guarantees that constrain any batching design:

1. **Intent records gate the mutation.**
   `internal/mcp/audit.go:33` returns the intent persistence error
   to the caller whenever
   `AuditDurabilityMode == "fail_closed"` or `"fail_closed_strict"`.
   That short-circuits the handler before the mutation runs. A
   naïve "batch every audit row" loses this guarantee because the
   error would no longer surface before the mutation.
2. **`fail_closed_strict` outcome records surface persistence
   errors.** `internal/mcp/audit.go:48` returns the outcome
   persistence error to the client in strict mode. The mutation
   has already happened, but the operator contract is that the
   client learns the audit row didn't persist. Batching strict
   outcomes loses this signal because the error would land in a
   background flush.

Both guarantees are pinned by
`internal/mcp/audit_test.go::TestAuditDurability_FailClosed_RejectsOnPersistError`
(lines 84-106) and `_FailClosedStrict_RejectsOutcomePersistError`
(lines 108-127).

The remaining traffic — `best_effort` and `fail_closed` outcome
events, and any legacy single-shot rows — silences persistence
errors at the call site (the mutation already happened, the row
is best-effort durable). That traffic dominates a healthy hosted
deployment and is where batching pays off without changing any
observable contract.

## Decision

Add a `batchedAuditor` wrapper around the existing
`controlPlaneAuditor` (`internal/runtime/service.go:47-62`). The
wrapper owns the batching state. The `controlplane.Store`
interface gains one new method:

```go
AppendAuditEventBatch(events []AuditEvent) error
```

with semantics identical to a sequence of `AppendAuditEvent` calls
(`ON CONFLICT (external_id) DO NOTHING`, same external_id
synthesis) but transported as a single round trip via
`pgxpool.Pool.SendBatch` + `pgx.Batch`. The `DevFileStore`
implementation is a sequential loop calling `AppendAuditEvent` so
the interface contract stays uniform across backends.

### Q1: Where does the buffer live?

**In the runtime layer**, owned by `batchedAuditor` in a new file
`internal/runtime/audit_batch.go`. Not in the Store.

The runtime layer already has the data needed to classify an event
(`AuditDurabilityMode` is read at `runtime/service.go:114`); the
Store does not and should not. Threading a "may-batch" hint
through the `controlplane.AuditEvent` struct would leak transport
policy into the persistence contract, and adding a durability-mode
field to the Store struct would couple every backend to the same
mode taxonomy. The wrapper keeps the existing two methods clean:
`AppendAuditEvent` is "single event, single row, low latency";
`AppendAuditEventBatch` is "many events, one round trip, same row
semantics."

Alternatives rejected:

- **Buffer in Postgres `Store`.** Adds a goroutine + mutex to the
  Store and forces every backend (including `DevFileStore`) to
  understand durability semantics. Higher blast radius for the
  same win.
- **Single method with a `Phase`/`Mode` hint field on
  `AuditEvent`.** Pollutes the contract with transport-layer policy
  and gives the Store a runtime decision that belongs upstream.

### Q2: How is the strict-rule guard enforced?

The wrapper's `shouldFlushSync(phase, mode) bool` returns `true` —
forcing the synchronous path — for **every** event in any of:

- `phase == PhaseIntent` (regardless of durability mode) — intent
  gates the mutation.
- `phase == ""` (legacy single-shot record) — preserve historical
  contract for any caller that still emits a flat audit.
- `phase == PhaseOutcome && mode == "fail_closed_strict"` —
  persistence error must surface to the client.

Only the remaining cell — outcome events under `best_effort` or
non-strict `fail_closed` — is enqueued.

A property test
(`internal/runtime/audit_batch_test.go::TestAuditBatch_PropertyMatrix`,
introduced in the follow-up "test pin" commit) iterates the full
cross product of phase × durability mode and asserts which cells
land sync vs batched. A drift check at the commit time flips the
`(PhaseOutcome, fail_closed_strict)` cell and confirms the matrix
goes red, then restores. The pin is a deliberate separate commit
so a future refactor can't quietly reshape `shouldFlushSync`
without explicit consideration of the strict-rule contract.

### Q3: What are the defaults?

- `flushSize = 64` (events).
- `flushInterval = 250 * time.Millisecond`.

The defaults are hardcoded constants on the wrapper. No new env
var is introduced.

Rationale: tunability has a cost (a new spec.go entry triggers
`config-doc-parity`, regenerated `help_generated.go`, Helm + k8s
configmap edits, golden `spec_test.go` updates) that is not paid
for in operator demand. Nobody has asked for tuning. If operators
report a wrong default in production, a follow-up wave can add the
env vars and re-document the trade-off.

`64`/`250ms` is the same starting point Cloud-SQL operators use
for application-level batching; it balances commit-amortisation
(64 rows ≈ one wal segment write) against worst-case audit lag
(250ms — well under any operator SLO for audit visibility).

The wrapper is **default-on** whenever a `controlplane.Store` is
wired (production deploys and dev/file-store paths both). The
`DevFileStore` sequential fallback for `AppendAuditEventBatch`
makes this safe.

### Q4: How is shutdown drained?

The wrapper exposes `Close() error` that:

1. Stops the flush ticker.
2. Signals the ticker goroutine to exit via a `done` channel.
3. Drains the remaining buffered events with a final
   `AppendAuditEventBatch` call.
4. Joins the goroutine via `sync.WaitGroup` so the call returns
   only when the buffer is genuinely flushed.

The runtime service's existing shutdown sequence calls `Close()`
on the wrapper *before* closing the underlying `Store`, so the
final batch lands on a live `pgxpool`. The wrapper does **not**
take ownership of the Store's own `Close()`; the runtime closes
both in the correct order.

If the final `AppendAuditEventBatch` fails (Postgres unavailable
at shutdown), the wrapper logs the loss with the canonical
`audit_outcome=not_durable` field. The semantic is the same as
the pre-batching path: a `fail_closed`/`best_effort` outcome event
that the Store rejects at shutdown was already silenced at the
call site, so the operator contract is preserved.

## Alternatives considered

- **Increase `pgxpool` size instead of batching.** Rejected: a
  larger pool moves the bottleneck from per-connection RTT to
  connection acquisition + per-connection saturation, and adds
  Postgres-side connection pressure that hurts every other backend
  workload. Batching attacks the RTT directly.
- **Async fan-out per event (one goroutine per event).** Rejected:
  goroutine + channel overhead per audit row scales worse than
  even the single-row INSERT it replaces, and the absent
  back-pressure makes a slow Postgres turn into unbounded
  goroutine accumulation.
- **Postgres trigger / partitioning to absorb the write load.**
  Rejected as out of scope. The application-level batching
  reduces the application-side bottleneck by 5–10× under
  realistic flush parameters; a partitioning ADR can layer on
  later if the batched ceiling is still inadequate.
- **Gate the batching behind a new opt-in env var.** Rejected
  because the observable contract for non-strict outcome events is
  unchanged (errors already silenced at the call site) and the
  operator-experience win of "perf for free" outweighs the
  marginal control. If a hosted operator later discovers a reason
  to keep every outcome synchronous, the escape hatch is to switch
  durability mode to `fail_closed_strict` — which the wrapper
  already short-circuits to sync.

## Consequences

**Positive.**

- Audit throughput on the hosted Postgres path lifts ~5–10×
  (size-64 batch consolidates ~64 round trips into one
  `SendBatch`).
- The intent-gate and `fail_closed_strict` contracts are
  unchanged; the failing tests
  (`TestAuditDurability_FailClosed_*`,
  `_FailClosedStrict_*`) continue to pass without modification.
- The new method is a discrete benchmark surface: future
  optimisations (`COPY`, partitioning, etc.) can replace the
  `SendBatch` body without touching call sites.
- The wrapper's behaviour is exhaustively pinned by the property
  matrix test, making future strict-rule violations a compile-time
  red-build rather than a silent regression.

**Negative — accept these.**

- A `best_effort` or `fail_closed` outcome event may now be
  delayed up to 250 ms before it hits the database. Operators
  watching audit logs in real time will see a brief lag. This is
  within every documented SLO; mention in the runbook
  (`docs/runbooks/audit-durability.md` future amendment).
- A panic between event enqueue and flush could lose up to 63
  outcome events. This shape is acceptable for non-strict modes
  whose call sites already silence errors; strict-mode operators
  who cannot tolerate that exposure already get the synchronous
  path. Recovery is via the same `audit_outcome=not_durable`
  field operators already filter on.

## Migration

No operator action required. Defaults are unchanged. Operators in
`fail_closed_strict` observe identical behaviour. Operators in
`fail_closed` or `best_effort` observe at most 250 ms of added
outcome-event lag and a corresponding rise in audit throughput
ceiling.

## References

- `internal/controlplane/store.go` — `Store` interface, gains
  `AppendAuditEventBatch([]AuditEvent) error`. `DevFileStore`
  implements the new method as a sequential `AppendAuditEvent`
  loop.
- `internal/controlplane/postgres/postgres.go` — `(*Store)
  .AppendAuditEvent` (existing, unchanged); `(*Store)
  .AppendAuditEventBatch` (new) uses `pool.SendBatch` +
  `pgx.Batch`. External_id synthesis is shared between the two
  methods so the
  `live_audit_phases_test.go::TestLiveCreateUpdateDeleteEntryAuditPhases`
  collision invariant continues to hold.
- `internal/runtime/audit_batch.go` (new) — `batchedAuditor`
  wrapper; owns the buffer, ticker goroutine, and `Close()` drain.
- `internal/runtime/service.go::controlPlaneAuditor` — kept as the
  inner sync delegate; `batchedAuditor` wraps it.
- `internal/runtime/service.go` (service shutdown sequence) —
  `batchedAuditor.Close()` runs before `Store.Close()`.
- `internal/mcp/audit.go` — `recordAuditIntent`,
  `recordAuditOutcome`, `emitAudit`. Unchanged by this ADR; the
  wrapper is invisible to the MCP server.
- `internal/mcp/audit_test.go` — existing `fail_closed_*`
  contract tests continue to pass without modification.
- ADR 0004 — Policy enforcement architecture. The audit pipeline
  is the post-mutation observability gate; the batched path
  preserves that role for the non-strict cells.
- ADR 0017 — Streamable-HTTP session rehydration. Sets the
  precedent for runtime-layer wrappers that adapt persistence
  semantics without changing the Store interface contract.
- ADR 0018 — Risk-class confirmation tokens. The intent record
  that gates the high-risk mutations remains synchronous under
  this ADR's design, preserving 0018's pre-mutation enforcement.
