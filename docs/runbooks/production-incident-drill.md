# Production incident drill

A deliberately adversarial exercise that stacks multiple
failure modes simultaneously and asks the operator to bring
the service back to a known-good state inside an RTO window.
The drill simulates the worst day we have scripts for —
expired tokens, upstream throttling, a misconfigured metrics
endpoint, dropped notifier events, and a partial control-plane
outage — all at once.

Run it quarterly, or whenever the on-call rotation sees a new
person pick up `clockify-mcp` for the first time. The drill
exists to build muscle memory against the existing runbooks;
it is not a release-blocking test.

## Scope

The drill covers recovery of a **multi-tenant shared-service
deployment** (`docs/deploy/production-profile-shared-service.md`).
Single-tenant and stdio profiles don't have most of the
failure modes this drill exercises.

## Pre-drill checklist

- [ ] A dedicated staging environment mirroring production
      config. Do not run this drill against production.
- [ ] Staging has the full observability stack wired:
      Prometheus scraping, Grafana dashboard, alerting routed
      to a dead-letter channel so the on-caller isn't paged
      during the drill.
- [ ] On-caller has read access to the three primary
      runbooks (see "See also" at the bottom).
- [ ] A timekeeper is present to enforce the RTO (operator
      should not be looking at a clock; the timekeeper is).

## Faults (apply all at once)

Once the drill starts, the facilitator injects all of the
following within the first five minutes:

1. **Expired OIDC tokens.** Revoke the OIDC signing key in
   the staging issuer. Every inbound token now fails
   verification.
2. **Upstream throttling.** Configure the staging
   Clockify-proxy mock to return HTTP 429 with
   `Retry-After: 30` on 60% of responses.
3. **Metrics misconfig.** Set `MCP_HTTP_INLINE_METRICS_AUTH_MODE=none`
   on half the pods (rolling restart). Operators should
   notice the startup warning log line.
4. **Notifier drops.** In staging, use your network layer to
   drop a fraction of outbound SSE frames (tc / iptables
   probability rule) so clients see intermittent
   notification gaps without the server knowing why.
5. **Partial control-plane outage.** Block writes to the
   Postgres primary from half the pods (iptables rule or
   network-policy matchLabels). Reads succeed; writes 50%
   fail.

## RTO

**Target: 30 minutes from drill start to "green dashboard."**

"Green dashboard" means:
- `clockify_mcp_tool_calls_total{outcome="tool_error"}` is
  back within 10% of baseline over the last 5 minutes.
- `clockify_mcp_audit_failures_total{reason="persist_error"}`
  has stopped rising.
- No `audit_outcome=not_durable` log entries in the last
  5 minutes.
- New tool calls succeed end-to-end when dispatched from the
  staging client harness.

## Expected recovery sequence

The operator is not told this; they're expected to reconstruct
it from the runbooks. The facilitator checks whether the
sequence matches.

1. **Triage via metrics.** The on-caller pulls up the
   dashboard and identifies that:
   - Auth rejections are rising (→ OIDC issue).
   - Upstream 429s are rising (→ Clockify-side, not us).
   - Audit-failure counter is rising (→ audit runbook).
   - Pods are split on metrics-auth config (→ config drift).

2. **Stop the bleeding.** The correct first move is NOT to
   fix OIDC; it is to stop the audit bleeding by flipping
   `MCP_AUDIT_DURABILITY=best_effort` so tool calls aren't
   failing clients during the control-plane split. Reference:
   `audit-durability.md` §4.

3. **OIDC recovery.** Rotate to the backup issuer key pair,
   or flip `MCP_AUTH_MODE=static_bearer` with a
   pre-distributed emergency token. Reference:
   `auth-failures.md` §3 "Inbound: MCP_AUTH_MODE=oidc issuer
   outage."

4. **Control-plane split.** Identify the pods that can't
   reach Postgres primary (network tooling, not a
   clockify-mcp config change), and either remove the network
   block or drain the affected pods. Reference:
   `audit-durability.md` §4 "Postgres backend: connection
   loss."

5. **Metrics-auth drift.** Scan for
   `msg=metrics_auth_mode_unsafe` log lines, identify the
   subset of pods with the bad config, re-roll them with the
   correct env. Reference: a deployment-hygiene runbook (not
   yet in-tree; capture the gap in the post-drill writeup).

6. **Upstream throttling.** This is the expected steady-state
   during a Clockify outage — correct operator behaviour is
   to **not** take action beyond monitoring until the 429
   rate falls, referencing the client's retry-with-backoff
   behaviour (`chaos.yml:429-storm` and `upstream-429-concurrent`).
   Reference: `clockify-upstream-outage.md`.

7. **Restore strict audit.** Once the control-plane split is
   healed, flip `MCP_AUDIT_DURABILITY=fail_closed` back on
   and confirm the counter stays at zero over a 5-minute
   window.

## Drill scoring

| Criterion | Pass threshold |
|-----------|----------------|
| RTO met | ≤ 30 minutes from fault injection to green |
| First action | Audit-durability flip, not OIDC rotation |
| Runbooks referenced | At least three runbooks opened during the drill |
| Unnecessary rollbacks | Zero — the operator should NOT revert the binary |
| Post-drill audit reconstruction | Operator documents the outage window for the audit-trail gap |

## Post-drill writeup

Within 24 hours, the operator files a drill report:

- What they did, in order, with timestamps.
- Which runbooks were useful and which had gaps.
- Which metrics told the truth and which were misleading
  during the drill.
- Any new runbook the drill revealed we need to write (the
  metrics-auth drift is one known gap).
- Suggested tweaks to the injection script for the next
  drill.

File the writeup in the same place as any real incident
postmortem — the drill is treated as an incident for the
purposes of the postmortem archive.

## See also

- `audit-durability.md` — the first runbook referenced in
  the recovery sequence.
- `auth-failures.md` — OIDC and static-bearer rotation flow.
- `clockify-upstream-outage.md` — when 429s are not our
  problem.
- `rate-limit-saturation.md` — internal rate-limit path (not
  part of this drill but often relevant during a real
  incident).
- `postgres-restore-drill.md` — a sibling drill focused on
  the backend alone.
- `docs/upgrade-checklist.md` — the regular operations flow
  this drill deliberately inverts.
