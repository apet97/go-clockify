# 0025 - Upstream-5xx circuit breaker against Clockify

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


## Status

Accepted — 2026-05-13.

The implementation uses a per-endpoint circuit breaker, wraps the
existing retry loop, and opens after a configurable number of
consecutive final upstream 5xx outcomes. The default
`CLOCKIFY_CIRCUIT_BREAKER=auto` enables the breaker for hosted
profiles (`shared-service`, `prod-postgres`) and leaves local
profiles disabled unless operators explicitly set
`CLOCKIFY_CIRCUIT_BREAKER=enabled`.

2026-05-15 one-user update: the current stdio-only product wires the
breaker into the default command path and enables it by default; use
`CLOCKIFY_CIRCUIT_BREAKER=disabled` only for controlled local diagnosis.

## Context

The upstream Clockify client at `internal/clockify/client.go`
already has a retry loop in `doRequest`
(`internal/clockify/client.go:401-450`):

```go
for attempt := 0; attempt <= c.maxRetries; attempt++ {
    if attempt > 0 {
        metrics.UpstreamRetriesTotal.Inc(endpoint, reason)
        waitDur := explicitRetryAfter
        if waitDur <= 0 { waitDur = backoff(attempt) }
        if deadline, ok := ctx.Deadline(); ok {
            if time.Until(deadline) < waitDur { return lastErr }
        }
        if err := sleepWithContext(ctx, waitDur); err != nil { return err }
    }
    err := c.doOnce(...)
    if err == nil { return nil }
    lastErr = err
    apiErr, ok := err.(*APIError)
    if !ok || !isRetryableStatus(apiErr.StatusCode) { return err }
    explicitRetryAfter = apiErr.RetryAfter
}
```

The retryable statuses are 429 / 502 / 503 / 504
(`internal/clockify/client.go:603-608`). 429 honours the
upstream's `Retry-After` header; the others use exponential
backoff with jitter (`backoff(attempt)` at
`internal/clockify/client.go:632-638`: base
`250ms * 2^(attempt-1)` plus up to 125ms jitter). The retry budget
is capped by `c.maxRetries` (default 3 via `CLOCKIFY_MAX_RETRIES`)
and bounded by the request's context deadline
(`internal/clockify/client.go:422-425`).

Metrics surfaces already exist:

- `metrics.UpstreamRetriesTotal` (`endpoint`, `reason`) where
  `reason` is `rate_limited` / `bad_gateway` /
  `service_unavailable` / `gateway_timeout` (the
  `retryReason(status)` mapping at
  `internal/clockify/metrics.go:93-106`).
- `metrics.UpstreamRequestsTotal` and
  `metrics.UpstreamRequestDuration` for outcome + duration
  histograms.
- `metrics.UpstreamRequestErrorsTotal` partitioned by
  `statusBucket()` (`internal/clockify/metrics.go:71-89`).

**The problem this ADR addresses.** During an extended Clockify
5xx outage, every replica continues to send retry traffic on every
tool call. Each request consumes the upstream's recovery budget
(amplification load on a struggling backend) and the user's
Clockify API-key quota (the same key is shared across every tool
call). A circuit breaker is the canonical fix: once the failure
rate (or consecutive-failure count) crosses a threshold, the
breaker *opens*, every subsequent request fast-fails locally
without going to the wire, and a small subset of probes is
periodically allowed through to discover recovery.

The breaker has three orthogonal design dimensions that must be
declared in operator-policy terms before code lands: *scope*,
*state-machine semantics*, and *interaction with the existing
retry loop*. Each affects what operators see in dashboards, what
latency tools experience during outages, and what blast radius a
misconfigured threshold has.

Out of scope for this ADR: replacing the retry loop entirely
(retry is correct for 429 with Retry-After and short-blip 5xx —
the breaker layers above it). Also out of scope: per-tenant
shaping of the breaker; the per-tenant question is a sibling
concern tracked separately under ADR 0024.

## Decision

**Accepted shape: Q1-B + Q2-A + Q3-A.**

- **Scope:** per normalised endpoint and HTTP method. This aligns
  with the existing upstream metric labels and avoids a reports
  outage opening the breaker for unrelated time-entry calls.
- **Threshold:** consecutive final upstream 5xx outcomes. The
  default threshold is 5, the default open duration is 45 seconds,
  and the default half-open probe count is 1.
- **Composition:** the breaker wraps the existing retry loop. A
  closed or half-open breaker permits the normal retry budget to run;
  only the final outcome of that logical request updates breaker
  state. An open breaker fast-fails locally before any HTTP attempt.

Operators can tune the accepted shape with
`CLOCKIFY_CIRCUIT_BREAKER`, `CLOCKIFY_CIRCUIT_BREAKER_FAILURE_THRESHOLD`,
`CLOCKIFY_CIRCUIT_BREAKER_OPEN_DURATION`, and
`CLOCKIFY_CIRCUIT_BREAKER_HALF_OPEN_PROBES`.

### Q1: What is the breaker's scope?

The signal source determines what the breaker protects against.

**A. Global breaker.** One breaker per replica covers every
upstream Clockify call. Simplest to reason about; one set of
metrics. Trips on any 5xx-heavy traffic regardless of which
endpoint is failing. Cost: a degradation on the reports endpoint
opens the breaker for time-entry calls too, even though the two
endpoints have independent failure modes.

**B. Per-endpoint breaker.** One breaker per normalised endpoint
(the `endpoint` label already used by `UpstreamRetriesTotal`).
Aligns with the existing metric cardinality; lets the reports
endpoint open without affecting time-entries. Cost: O(number of
endpoints) breaker state lives in process; one quiet endpoint
never trips, but a steady low-volume failure on it builds up
state slowly. Endpoint normalisation already runs in
`normalizeEndpoint(path)` at `internal/clockify/client.go:402`.

**C. Per-tenant breaker.** One breaker per `tenant_id` (or
`(tenant_id, endpoint)` tuple in the limit). Protects one
tenant's quota from another tenant's amplification — relevant
only in shared-service deployments. Cost: requires
principal-aware client (the client today is principal-agnostic;
the principal lives in the enforcement layer, not the HTTP
client) and intersects with ADR 0024's per-tenant decision.

The scope choice affects what operators see: a global breaker
emits one rejection signal per replica; per-endpoint emits one
per endpoint label; per-tenant emits one per tenant. Dashboard
cardinality and noise floor differ accordingly.

### Q2: What is the state-machine threshold?

A breaker has three states (closed / open / half-open) and three
transitions (closed→open, open→half-open, half-open→closed-or-open).
Each transition needs a threshold. Two coherent threshold shapes
exist:

**A. Consecutive failures.** Closed→open trips when N consecutive
calls return retryable 5xx (or the retry loop exhausts). Cheap to
reason about; cheap to implement (one counter, reset on success).
Vulnerable to mixed traffic: a 50/50 success/fail mix never
trips because no consecutive run reaches N, even though half the
quota is wasted. Open→half-open after a configured open duration
(`open_duration`); half-open→closed after M consecutive successes
or →open after one failure.

**B. Percentage over a sliding window.** Closed→open trips when
the 5xx rate over the last W seconds exceeds X%. More robust
under mixed traffic; requires a sliding-window counter (extra
state). Open→half-open and half-open→closed thresholds use the
same window. Useful at high request volumes; needs careful
zero-traffic-edge-case handling (an idle endpoint with one
failure must not trip).

**C. Hybrid — consecutive *or* percentage.** Trip on whichever
fires first. Captures both the consecutive-spike case and the
mixed-traffic case at the cost of two thresholds to tune. Most
expressive; highest operator-cognitive load.

Each option also needs `open_duration`, `half_open_probe_count`,
and `success_threshold_to_close` defaults — those are
implementation details once the threshold shape is chosen. The
defaults must be safe enough that an idle endpoint never trips
during normal operation.

### Q3: How does the breaker compose with the existing retry loop?

The retry loop currently amortises transient blips up to
`c.maxRetries` (default 3). A breaker can sit above it, beside
it, or below it.

**A. Breaker wraps retry (outer).** `doRequest` first asks the
breaker for permission; if open, fast-fail without calling
`doOnce`. If permitted (closed or half-open probe), the existing
retry loop runs normally. Each call's *final outcome* (success or
final-attempt failure) feeds the breaker counter. Latency for
fast-failed calls is microseconds. Transient single-attempt 5xx
do not trip the breaker because they are absorbed by retry. The
breaker sees a small fraction of the wire-traffic — the
final-attempt failures only.

**B. Breaker reads every attempt (inner).** Each `doOnce` outcome
feeds the breaker, not just the final result. Transient blips
amplified by retry can themselves trip the breaker; the breaker
opens faster but at the cost of treating recovery retries as
failure signal. Operationally noisier; less common shape.

**C. Breaker replaces retry for 5xx.** Retry remains for 429
(Retry-After honoured) and short-RPC-deadline shapes; the
breaker handles 5xx with fast-fail when open. Operationally a
clear separation — retry is for "your client should slow down"
signals from upstream, breaker is for "upstream is down". Adds
a branch in `doRequest` between the 429 and 5xx code paths.

The composition choice also determines what the
`metrics.UpstreamRetriesTotal` count means after the change:
under A it is unchanged (retries inside the same call); under B
it is unchanged but the breaker may shorten the loop; under C
the retry count drops to zero for 5xx and a new
`metrics.UpstreamBreakerRejections` metric carries the breaker's
share. Operators tuning dashboards must know which.

## Alternatives considered

- **Do nothing; rely on `c.maxRetries` + context deadline to
  bound waste.** Today `CLOCKIFY_MAX_RETRIES=3` with exponential
  backoff caps a single call at ≈1.75s of in-band waiting (250ms
  + 500ms + 1000ms ≈ 1.75s plus jitter) before returning the 5xx
  to the caller. The "waste" per call is small; the aggregate
  cost only accumulates across calls during an outage.
  Acceptable if (a) outages are rare and (b) Clockify's API-key
  quota is not the binding constraint. Picking this is the
  decision that "we accept retry-storm-shaped behaviour during
  Clockify outages".
- **Set `CLOCKIFY_MAX_RETRIES=0` as the operator escape hatch.**
  This already exists. An operator who finds the retry storm
  unacceptable can flip retries off entirely. Cost: every
  transient single-blip 5xx surfaces to the agent unmediated.
  Useful as an interim mitigation; not a substitute for a
  breaker because the operator pays the "every blip is an
  error" tax even outside outages.
- **Push the breaker into a sidecar / mesh layer (Envoy /
  Linkerd).** Out of scope: the `clockify-mcp` deployment shape
  must not require an L7 sidecar. The hosted-shared profile
  could, but stdio and self-hosted deployments do not have a
  sidecar to delegate to; the breaker must live in-process to
  cover all profiles consistently.
- **Pre-position a candidate env var name family in this ADR.**
  Rejected to keep this proposal aligned with the doc-parity
  gate rule on env-var-shaped tokens
  (`scripts/check-doc-parity.sh` §env-vars). The implementation
  commit that flips this ADR to Accepted will introduce the
  spec.go entries, the help docs, and the Helm / k8s manifest
  edits in the same commit per the `config-doc-parity` rule.

## Consequences

- A new file under `internal/clockify/` (likely
  `internal/clockify/breaker.go`) carries the breaker state
  machine; the Client struct gains one breaker pointer (Q1-A),
  a `map[endpoint]*breaker` (Q1-B), or a `map[tenant]*breaker`
  (Q1-C). `doRequest` checks the breaker before / inside / instead
  of the retry loop depending on Q3.
- A new env-var family lands in `internal/config/spec.go` to
  expose the threshold knobs from Q2 (e.g. open trigger,
  open duration, half-open probe count). Defaults must not trip
  in normal operation; a pinned `TestBreakerStaysClosedUnderNormalLoad`
  prevents drift.
- A new `metrics.UpstreamBreakerState` gauge (Open / HalfOpen /
  Closed per breaker key) and `metrics.UpstreamBreakerRejections`
  counter join the upstream metric family. Existing
  `UpstreamRetriesTotal` and `UpstreamRequestsTotal` semantics
  are documented under whichever Q3 option lands.
- New tests under `internal/clockify/`: closed→open trip on the
  chosen threshold; open→half-open after the open duration;
  half-open→closed after configured successes; half-open→open on
  any failure; per-key isolation (no cross-tenant or
  cross-endpoint trip leakage). Drift-checks (e.g. flip the
  threshold to a value that should never trip and assert the
  closed→open transition stops firing) recorded in the commit
  body's `Verified:` line.
- A new operator runbook `docs/runbooks/upstream-outage.md` (or
  amendment to an existing one) describes how operators diagnose
  an open breaker, force half-open probes, and reset state during
  recovery. The runbook cross-links this ADR.

If the decision is "do nothing", instead document the trade in
`docs/operator-tradeoffs.md` (or sibling) so future operators
land on the choice without having to re-derive it.

## Migration

Implemented in the `v1.3.0-rc.1` hardening wave:

1. Picked B / A / A for Q1-Q3: per-endpoint scope,
   consecutive final 5xx threshold, breaker wrapping retry.
2. Implemented the breaker under `internal/clockify/` and wired
   it into process-level clients, streamable-HTTP ready checks, and
   per-tenant session clients.
3. Added the env-var family to `internal/config/spec.go` and
   regenerated config/help docs through the normal parity flow.
4. Added state-machine tests for closed→open and
   open→half-open→closed transitions.
5. Amended `docs/runbooks/clockify-upstream-outage.md`.
6. Flipped this ADR to Accepted and removed the `(proposed)`
   suffix from the ADR index.
7. Updated `CHANGELOG.md` under Unreleased.

## References

- `internal/clockify/client.go:401-450` — `doRequest()` retry
  loop (the call site any breaker layer must compose with).
- `internal/clockify/client.go:603-608` — `isRetryableStatus()`:
  429 / 502 / 503 / 504.
- `internal/clockify/client.go:632-638` — `backoff(attempt)`:
  exponential `250ms * 2^(attempt-1)` + ≤125ms jitter.
- `internal/clockify/client.go:422-425` — context deadline
  guard (breaker fast-fail composes with this).
- `internal/clockify/client.go:402` — `normalizeEndpoint(path)`
  (the endpoint label any per-endpoint breaker scope would use).
- `internal/clockify/metrics.go:71-89` — `statusBucket()`
  classification (signal source for breaker state-change events).
- `internal/clockify/metrics.go:93-106` — `retryReason()` labels
  already partitioned by 5xx variant.
- `internal/clockify/metrics.go` — `UpstreamRequestsTotal`,
  `UpstreamRequestErrorsTotal`, `UpstreamRequestDuration`,
  `UpstreamRetriesTotal` (the metric family any breaker metric
  must dovetail with).
- `internal/mcp/transport_streamable_http.go` (≈ line 175-189
  per `CLAUDE.md`) — the streamable-HTTP `/ready` upstream
  probe; could share breaker state with the hot path or be
  intentionally exempt.
- ADR 0010 — Metrics stack direction (signal source for the
  state-machine threshold).
- ADR 0014 — Production fail-closed defaults (informs whether
  the breaker defaults are profile-dependent).
- ADR 0024 — Per-tenant aggregate rate-limit (sibling ADR with
  overlapping scope question for hosted deployments).
