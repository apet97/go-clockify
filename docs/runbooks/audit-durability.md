# Audit durability failures

## Why this runbook exists

The server audits every mutating tool call through the
`Auditor` interface backed by the control-plane store
(`internal/controlplane/`). When the store is healthy, the audit
write is synchronous with the tool response and the two outcomes
are coupled. When the store is degraded, the two outcomes can
diverge — and what "the tool call succeeded" means depends on
`MCP_AUDIT_DURABILITY`.

This runbook is for the moment the `mcp_audit_durability_failures_total`
counter starts rising, or operators have to explain to a customer
why a mutation shows up in Clockify but not in the audit ledger.

## 1. What the two modes mean

`MCP_AUDIT_DURABILITY` (config field `AuditDurabilityMode`) has two
legal values:

| Mode | Tool-call response on audit-write failure | Mutation state |
|------|-------------------------------------------|----------------|
| `best_effort` (default) | Success — the client sees the result of the mutation. | **Executed.** The upstream Clockify state has changed. |
| `fail_closed` | Error — the client receives `audit persistence failed; mutation outcome unverifiable`. | **Executed.** The upstream Clockify state has changed; only the audit trail is missing. |

Read-only tool calls are never affected — they produce no audit
event regardless of the mode. The mode only governs mutating
(write/destructive) calls.

**The critical invariant to communicate internally:** in both
modes, the mutation *has already been committed upstream* by the
time the audit write is attempted. An audit failure never unwinds
the mutation; it only changes whether the caller is told about
the failure.

## 2. Symptoms

- `clockify_mcp_audit_failures_total{reason="persist_error"}` is
  rising (any non-zero value is actionable).
- Structured logs show:
  ```
  level=ERROR msg=audit_persist_failed
    audit_outcome=not_durable
    durability_mode=best_effort|fail_closed
    tool=<name> outcome=<success|failure> error=<...>
  ```
- On `fail_closed`, clients see repeated tool-call errors with the
  substring `audit persistence failed`.
- On `best_effort`, clients see no change — the operator is the
  only one who notices, via the counter and the log.

`audit_outcome=not_durable` is the canonical log field to alert
on; it is set exclusively by `internal/mcp/audit.go` on a failed
persist and is never emitted on success.

## 3. Where to look first

```sh
# Recent audit failures across all tools
kubectl -n clockify-mcp logs deploy/clockify-mcp --since=30m \
  | grep 'audit_outcome=not_durable'

# Counter breakdown
curl -sf http://<host>:8080/metrics \
  | grep '^clockify_mcp_audit_failures_total'

# Control-plane backend health (postgres)
kubectl -n clockify-mcp exec deploy/clockify-mcp -- \
  sh -c 'psql "$MCP_CONTROL_PLANE_DSN" -c "select now(), version();"'

# Control-plane backend health (file store)
kubectl -n clockify-mcp exec deploy/clockify-mcp -- \
  df -h "${MCP_CONTROL_PLANE_DSN#file://}"
```

## 4. Immediate mitigation

### Postgres backend: connection loss or storage exhaustion

Most commonly the Postgres backend lost a connection pool or ran
out of disk. Restore the database or fail over to a replica:

```sh
# If a managed Postgres with failover available, trigger it.
# Otherwise, the server continues to serve mutations; the audit
# trail is missing for the outage window and must be reconstructed
# from Clockify's own activity log after recovery.
kubectl -n clockify-mcp rollout restart deploy/clockify-mcp
```

### File backend: disk full or permissions changed

```sh
# Check the backing volume
kubectl -n clockify-mcp exec deploy/clockify-mcp -- \
  df -h "${MCP_CONTROL_PLANE_DSN#file://}"

# If full, expand the PV or lower MCP_CONTROL_PLANE_AUDIT_CAP to
# force FIFO eviction at a lower threshold.
kubectl -n clockify-mcp set env deploy/clockify-mcp \
  MCP_CONTROL_PLANE_AUDIT_CAP=10000
```

### Switching modes during an incident

If the backend is going to be down for hours and you would rather
keep clients working than surface errors:

```sh
kubectl -n clockify-mcp set env deploy/clockify-mcp \
  MCP_AUDIT_DURABILITY=best_effort
```

If the backend is expected back shortly and you would rather fail
the caller than commit mutations without a durable audit record:

```sh
kubectl -n clockify-mcp set env deploy/clockify-mcp \
  MCP_AUDIT_DURABILITY=fail_closed
```

Flipping the mode does not replay missed audit events. Mutations
committed during the outage remain in Clockify; only a
reconstruction from Clockify's own activity log can fill the gap.

## 5. Recovery checklist

- [ ] **Confirm the backend is healthy again** before considering
  the incident over — the `clockify_mcp_audit_failures_total`
  counter must stop rising. If it keeps rising after a restart,
  the problem is still present.
- [ ] **Catalogue the outage window.** Note the earliest and
  latest `audit_outcome=not_durable` log lines. Every mutation
  between those timestamps may be missing from the audit ledger.
- [ ] **Identify affected tenants.** Filter the logs by
  `tenant_id=…`; the `msg=audit_persist_failed` record includes
  the tenant metadata.
- [ ] **Reconstruct the trail from Clockify.** For each affected
  tenant, query Clockify's own activity endpoint (e.g.
  `/workspaces/{id}/activity` or project/task history) for the
  outage window and compare against the audit store.
- [ ] **Communicate.** If the outage affected a compliance-sensitive
  tenant (shared-service deployments with contractual audit
  retention), notify them per the runbook for that customer.
- [ ] **Decide whether to ratchet the mode.** A recurring outage
  on `best_effort` may warrant `fail_closed` plus a faster
  backend-health alert. A recurring outage on `fail_closed` may
  warrant a second backend or a queue in front of the primary.

## 6. Root-cause checklist

- [ ] **Disk or DB exhaustion.** Was the backend at capacity?
  Check retention config vs. ingress rate.
- [ ] **Network flaps.** Were there connectivity drops between the
  server pod and the Postgres instance? Correlate the audit
  failure timestamps with pod-level network-error logs or the
  Postgres server log.
- [ ] **Schema drift.** A recent migration may have left the audit
  table in an inconsistent state. Check the latest migration log.
- [ ] **Permission changes.** Did someone rotate the DB credentials
  or tighten the role grants without coordinating with the
  deployment?
- [ ] **Regression in the audit write path.** Correlate first
  failure time with the most recent release. Roll back if the
  counter only appeared post-deploy.

## 7. Postmortem template

- **Mode in use** — `best_effort` or `fail_closed`?
- **Backend** — Postgres, file, or memory (not production-appropriate)?
- **Outage window** — First and last `audit_outcome=not_durable`.
- **Mutation volume** — Count of affected mutating tool calls
  (approximately: `mcp_tool_calls_total{outcome="success",read_only="false"}`
  deltas over the window).
- **Customer impact** — Did anyone whose contract requires durable
  audit retention hit this window? Were they notified?
- **Mitigation** — What brought the backend back?
- **Reconstruction** — Was the audit trail rebuilt from Clockify
  activity logs?
- **Permanent fix** — Backend upgrade, higher retention, mode
  change, new alert, or migration to a queued audit writer?

## See also

- `internal/mcp/audit.go` — `emitAudit`, `recordAuditBestEffort`,
  `recordAuditWithDurability`, and the canonical
  `audit_outcome=not_durable` log field.
- `internal/controlplane/store.go` — `Auditor` interface.
- `internal/controlplane/postgres/` — Postgres backend and retention.
- `internal/metrics/metrics.go` — `AuditEventsTotal`
  (`clockify_mcp_audit_events_total`) and `AuditFailuresTotal`
  (`clockify_mcp_audit_failures_total`).
- `docs/production-readiness.md` — when to choose which durability
  mode per deployment profile.
- `postgres-restore-drill.md` — full DB recovery from snapshot if
  the incident required a restore.
