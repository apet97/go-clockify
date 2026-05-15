# 0019 - Postgres row-level security for the paid-hosted plane

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


## Status

Proposed — recorded as a placeholder template for the
`P1-8 paid-commercial RLS decision` external approval gate in
[`../launch-readiness-review-may-8.md`](../launch-readiness-review-may-8.md).
This ADR captures the decision questions a paid-hosted product /
commercial decision-maker must answer before the gate closes; it
does **not** record an accepted decision and does **not** close the
gate.

When the recorded decision lands (adopt / defer / refuse), this
file moves to **Accepted** under the chosen path, the
`(proposed)` suffix is dropped from the ADR index in
[`README.md`](README.md), and the launch-readiness ledger row for
"P1-8 paid-commercial RLS decision" is updated to quote the
decision, decision-maker identity, and ISO 8601 decision date per
the gate's evidence-artifact line.

## Context

The v1.x Postgres control plane uses **application-layer tenant
scoping**:

- Tables `sessions` and `audit_events` (and any future per-tenant
  tables) carry a `tenant_id` column.
- Every production access path reaches those rows through
  authenticated request handling, the streamable-HTTP session
  manager's strict re-auth check (ADR 0017 Path A), and tenant-
  keyed audit writes.
- Cross-tenant isolation is pinned by:
  - `internal/controlplane/postgres/e2e_shared_service_test.go`
    `TestSharedServicePostgresE2E` (cross-tenant query for
    `tenant_id=A AND session_id=B` returns zero rows).
  - `internal/controlplane/postgres/e2e_session_rehydration_test.go`
    `TestStreamableHTTPCrossInstanceRehydration` (cross-tenant
    replay across pods returns 403 + zero new audit rows).

The schema does **not** currently enable Postgres row-level
security or require `SET LOCAL app.current_tenant` before reads.
This is acceptable for the community / self-hosted launch profile
and is documented as such in
[`../auth-model.md`](../auth-model.md) § "Tenant resolution":

> The v1.x Postgres control-plane posture is application-layer
> tenant scoping. Tables such as `sessions` and `audit_events`
> store `tenant_id`, and every production access path reaches
> those rows through authenticated request handling, session
> principal checks, and tenant-keyed audit writes.

The paid commercial hosted plane treats database-enforced RLS as a
separate defense-in-depth design gate. The design questions that
must resolve before this ADR can move to Accepted are:

1. **Should the paid-hosted plane adopt database-enforced RLS at
   all?** A product / commercial decision: does the contractual
   posture (DPA, customer terms, security commitments) require
   defense-in-depth at the database layer, or is application-layer
   scoping plus the cross-tenant E2E pins acceptable for the
   commercial launch profile?
2. **If adopted, which tables are in scope?** At minimum
   `sessions` and `audit_events`; future per-tenant tables would
   inherit the policy. The decision must enumerate the table set
   rather than leaving it implicit.
3. **What is the tenant-context plumbing contract?** The standard
   Postgres pattern is `SET LOCAL app.current_tenant = '<id>'`
   inside every request-scoped transaction; the store API would
   set the GUC under the same transaction the application code
   uses for its query. The decision must pick the GUC name (or an
   alternative — e.g. a per-connection `app.current_tenant_id` set
   at checkout time), the connection-pool model (transaction-
   pooling vs. session-pooling implications differ for
   `SET LOCAL`), and how the policy interacts with the existing
   cross-tenant E2E tests.
4. **What is the migration / backfill posture?** Existing
   deployments would need a forward migration that enables RLS,
   defines the policies, and grants the appropriate role
   privileges. The decision must call out the migration ordering
   relative to schema changes already pinned by ADR 0011 and the
   restore-side checks already documented in
   [`../runbooks/postgres-restore.md`](../runbooks/postgres-restore.md)
   § "4. Tenant Isolation / RLS Check".
5. **Who owns the decision?** Per the May 8 ledger's "Owner role"
   line, this is a paid-hosted product / commercial decision-maker
   plus the maintainer responsible for the Postgres control-plane
   store API. A peer maintainer cannot self-approve. The recorded
   decision-maker identity must land in the launch-readiness
   ledger row.

## Decision

**To be made.** This ADR does not record an accepted decision.

When the decision lands, the recorded outcome is one of:

- **Adopt.** RLS is enabled on the listed tables; the store API
  sets `SET LOCAL app.current_tenant` (or the chosen alternative)
  under the request-scoped transaction; a forward migration lands
  under `internal/controlplane/postgres/migrations/`; a passing
  cross-tenant store-API test exercises the GUC-driven isolation
  contract; the restore-side check in
  [`../runbooks/postgres-restore.md`](../runbooks/postgres-restore.md)
  is updated to verify both `relrowsecurity` and the expected
  `pg_policies` rows; the launch-candidate checklist's Storage
  section is updated to reference the migration version and the
  store-API test name.
- **Defer.** RLS is not adopted in the recorded scope; the
  decision-maker enumerates which threat-model surfaces are
  accepted (and which are explicitly punted to a later ADR),
  the date of the next decision-review checkpoint, and any
  contractual conditions that must hold while RLS is deferred
  (e.g., audit-cadence commitments, DPA wording).
- **Refuse.** Application-layer tenant scoping is the canonical
  posture for the paid-hosted plane; the decision-maker records
  why database-enforced RLS is not the right defense-in-depth
  layer (e.g., the operational cost of the GUC contract is judged
  to outweigh the residual threat model after application-layer
  scoping plus cross-tenant E2E coverage). The decision is
  archived so future audits do not re-litigate it without new
  context.

The recorded decision must answer the five context questions
above. A decision that does not name them leaves this ADR
**Proposed** even if the file's Status line moves.

## Consequences

**Positive (if the architectural fix is landed under "Adopt").**

- Defense-in-depth at the database layer for the listed tables.
  A future application-layer regression that miswires
  `tenant_id` filters does not silently leak across tenants — the
  RLS policy rejects the read at the database boundary.
- The `paid-hosted RLS launch gate` named in
  [`../runbooks/postgres-restore.md`](../runbooks/postgres-restore.md)
  closes; the restore-side `relrowsecurity` and `pg_policies`
  checks become a verification, not just a future-proofing
  reminder.
- The `P1-8 paid-commercial RLS decision` row in the May 8
  launch-readiness ledger closes per the gate's evidence-
  artifact line.

**Negative (if the architectural fix is landed under "Adopt").**

- Every store-API call site that hits an RLS-scoped table must
  open its transaction with the GUC set, including `BEFORE`/
  `AFTER` triggers, restore-side replays, and migration runners.
  A missed call site silently returns zero rows, which is the
  failure mode the cross-tenant E2E tests already pin against.
  Test coverage must extend to "GUC unset" cases.
- Connection-pool topology is constrained. Transaction-level
  pooling (the v1.x default for the per-tenant Clockify HTTP
  client) is fine; session-level pooling against the Postgres
  control plane requires resetting the GUC at every checkin.
- The forward migration is not reversible without operator
  coordination; rolling back to the pre-RLS schema requires
  draining the per-tenant traffic first.

**Acceptable (if the recorded decision is "Defer" or "Refuse").**

- The application-layer tenant scoping posture continues to be
  the canonical contract; the cross-tenant E2E pins remain the
  primary defense.
- The DPA / customer-terms language must be consistent with the
  recorded posture; counsel acknowledges that posture in the
  `DPA / terms / privacy posture` gate's evidence bundle (see
  [`../release/dpa-privacy-evidence-checklist.md`](../release/dpa-privacy-evidence-checklist.md)).
- The `paid-hosted RLS launch gate` referenced in the restore
  runbook stays open as a future-proofing reminder; the recorded
  decision must enumerate when the next review occurs.

**Documentation contract (must be honoured by any implementation
of this ADR).**

- The chosen GUC name (or alternative) must land inline in this
  ADR's Status section once the decision is recorded. The
  store API contract depends on it.
- The migration ordering relative to ADR 0011's schema-versioning
  contract must be called out; restore-side replay must continue
  to work.
- The cross-tenant E2E pins listed under Context must be
  extended to cover the GUC contract; missing GUC ⇒ zero rows is
  a regression test, not a feature.

## Alternatives considered

The decision-maker is asked to consider, at minimum:

- **A. Adopt RLS on `sessions` and `audit_events`** with
  `SET LOCAL app.current_tenant` under the request-scoped
  transaction. The standard pattern. Cost: every store-API call
  site must set the GUC; connection-pool topology constrained.
- **B. Adopt RLS only on `audit_events`.** Audit rows are the
  highest-value cross-tenant leakage surface; sessions are
  shorter-lived and already strict-re-auth-gated by ADR 0017
  Path A. Cost: a partial defense-in-depth; the decision-maker
  must record why `sessions` is excluded.
- **C. Adopt RLS via a session-pool checkout hook** rather than
  per-transaction `SET LOCAL`. Lower per-call overhead; harder to
  reason about under transaction-level pooling and GUC reset on
  checkin. The decision-maker records connection-pool topology
  if this option is chosen.
- **D. Defer.** Application-layer tenant scoping plus the
  cross-tenant E2E pins remain the canonical posture; the
  decision is archived with a recorded next-review date.
- **E. Refuse.** Same as D, but with a stronger statement that
  database-enforced RLS is not the right defense-in-depth layer
  for this codebase's threat model. The decision-maker records
  the reasoning so future audits do not re-litigate.

The recorded decision names which alternative was selected and
why. A decision that picks none of A–E (or a mix that does not
resolve the GUC / connection-pool questions) leaves this ADR
Proposed.

## References

- [`../launch-readiness-review-may-8.md`](../launch-readiness-review-may-8.md)
  § "P1-8 paid-commercial RLS decision" — the external approval
  gate this ADR exists to resolve.
- [`../auth-model.md`](../auth-model.md) § "Tenant resolution" —
  current application-layer tenant scoping posture; this ADR's
  Decision overrides the posture for paid-hosted only if the
  recorded decision is "Adopt".
- [`../runbooks/postgres-restore.md`](../runbooks/postgres-restore.md)
  § "4. Tenant Isolation / RLS Check" — restore-side
  `relrowsecurity` and `pg_policies` checks; updated when this
  ADR moves to Accepted under "Adopt".
- [`0011-controlplane-schema-versioning.md`](0011-controlplane-schema-versioning.md)
  — schema-versioning contract the migration must respect.
- [`0014-prod-fail-closed-defaults.md`](0014-prod-fail-closed-defaults.md)
  — production fail-closed defaults that frame the paid-hosted
  posture this ADR slots into.
- [`0017-streamable-http-session-rehydration.md`](0017-streamable-http-session-rehydration.md)
  — the strict re-auth contract on session rehydration; orthogonal
  to the database-layer RLS question but cited because the two
  together determine the cross-tenant defense posture.
- [`../release/dpa-privacy-evidence-checklist.md`](../release/dpa-privacy-evidence-checklist.md)
  — the DPA / terms / privacy gate; counsel's acknowledgement
  records whether the executed terms presume RLS is in place.
- [`../release/external-security-review-request.md`](../release/external-security-review-request.md)
  — the external security review packet; the reviewer is asked to
  read this ADR's Status when assessing the cross-tenant defense
  posture.
- `internal/controlplane/postgres/e2e_shared_service_test.go`,
  `internal/controlplane/postgres/e2e_session_rehydration_test.go`
  — the cross-tenant E2E pins referenced under Context.
