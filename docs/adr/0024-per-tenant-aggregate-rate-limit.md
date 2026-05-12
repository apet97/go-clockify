# 0024 - Per-tenant aggregate rate-limit

## Status

Proposed — captured here because changing rate-limit aggregation
shifts a policy boundary every hosted operator depends on. The
current `(tenant, subject)` keying gives an N-subject tenant N×
the per-pair quota; whether that is "fair-share per active user
within a tenant" (the operator-intended interpretation) or
"single tenant gets N× the load" (the operator-concerning
interpretation) depends on what the operator believes a tenant
*is*. Strict-rule #2 in `CLAUDE.md` forbids quietly changing
this — the right path is to record the policy choice in an ADR
before any code lands. This ADR records the questions a maintainer
/ operator must resolve before that implementation can ship; it
does **not** record an accepted decision. When a decision lands,
this file moves to Accepted, the `(proposed)` suffix drops from
the ADR index in [`README.md`](README.md), and the implementation
wave (formerly known as "Wave B.12 — per-tenant aggregate") can
proceed.

## Context

The current per-call gate runs in
`internal/enforcement/enforcement.go:127-151`. It calls
`RateLimiter.AcquireForSubject(ctx, subject)` where `subject` is
derived by `rateLimitSubjectKey` at
`internal/enforcement/enforcement.go:313-321`:

```go
func rateLimitSubjectKey(principal *authn.Principal) string {
    if principal == nil || principal.Subject == "" {
        return ""
    }
    if principal.TenantID == "" {
        return principal.Subject
    }
    return principal.TenantID + "\x00" + principal.Subject
}
```

So the per-token bucket is keyed on the `(tenant_id, subject)`
tuple when both are available, on the subject alone when the
tenant header is absent, and is skipped entirely for
unauthenticated traffic. The same shape is used by
`internal/mcp/transport_streamable_http.go:821-829`
(`sessionPrincipalKey`) for the per-principal session counter
shipped in the make-it-goated wave; the comment there is explicit
that it "Mirrors enforcement.rateLimitSubjectKey to keep the
shared `(tenant_id, subject)` abstraction consistent across the
rate limiter and session counter."

The per-pair quota comes from two env vars
(`internal/config/spec.go:54-55`):

- `CLOCKIFY_PER_TOKEN_CONCURRENCY` (default 5) — concurrent
  tool-call slots per `(tenant, subject)`.
- `CLOCKIFY_PER_TOKEN_RATE_LIMIT` (default 60) — calls per 60s
  window per `(tenant, subject)`.

The implementation lives at
`internal/ratelimit/ratelimit.go:283-320` (`subjectLimiterFor`),
which creates lazy per-key buckets in a `map[string]*subjectLimiter`
and reaps idle entries via `StartSubjectReaper`. `AcquireForSubject`
at `internal/ratelimit/ratelimit.go:253-281` composes the global
limit (semaphore + window) with the per-pair limit; rejections at
either layer fire `metrics.RateLimitRejections.Inc(kind,
scopeLabel)` with `scopeLabel` set to "global" or "per_token"
(`internal/enforcement/enforcement.go:140-147`).

**The operator-policy question this ADR records.** Today an
N-subject tenant gets `N * CLOCKIFY_PER_TOKEN_RATE_LIMIT` calls
per minute and `N * CLOCKIFY_PER_TOKEN_CONCURRENCY` concurrent
slots. That is the intended behaviour if a tenant is "a billing
account whose subjects are individual human users" (fair-share
per human). It is *not* the intended behaviour if a tenant is "a
service account or automation principal whose subject count can
grow without operator notice" (one noisy automation deploy
multiplies tenant load by its replica count). Hosted operators
have asked for the second interpretation; community-deploy
operators tend to want the first. The codebase cannot honour
both without operator scoping. An ADR records the choice.

The make-it-goated session caps (`MCP_MAX_SESSIONS_PER_REPLICA`,
`MCP_MAX_SESSIONS_PER_PRINCIPAL`) bound *session count*, not
*per-call throughput*, so they are not a substitute for the
per-tenant rate-limit question. A tenant can satisfy the session
caps and still saturate a replica's outbound Clockify quota.

## Decision

**Proposed.** The questions below frame the design space; each
must have an explicit answer before this ADR moves to Accepted.

### Q1: What is the policy intent for tenant aggregation?

The current keying is `(tenant, subject)`. Three coherent answers
exist for the per-tenant aggregate question:

**A. Keep current — no per-tenant aggregate.** A tenant's
effective quota grows linearly with active subjects. Operators
who interpret a tenant as a human-billing unit get fair-share
per human. Operators who run automation principals must
provision per-subject quotas conservatively because the tenant
ceiling is `subjects * per-pair`. Zero behavioural change; no
new env var; no new metric label.

**B. Add a per-tenant aggregate *above* per-(tenant, subject).**
Two layers stack: every call must satisfy both the per-pair limit
and a new per-tenant limit. Operators can keep per-pair at "fair
per-human" while declaring an absolute tenant ceiling. The
implementation adds a per-`tenant_id`-keyed `subjectLimiter` map
to the limiter struct (sibling of the existing per-pair map) and
acquires both layers in order. Adds one env var pair (concurrency
+ window) and a new `scopeLabel` value (`per_tenant`).

**C. Replace per-(tenant, subject) with per-tenant only.**
Collapse the per-pair layer. A tenant has a single bucket shared
by all its subjects. Simplest from the operator-mental-model side
("one budget per tenant") but loses isolation between subjects
within a tenant: one runaway automation under a tenant starves
every other subject under the same tenant. The session-cap
counterparts in `transport_streamable_http.go:821-829` would
mirror the change.

Each option has different fairness / DoS properties:

- Under A, no tenant can dominate another tenant; within a tenant,
  every subject gets the same share. A growing subject count under
  one tenant amplifies that tenant's effective load.
- Under B, the operator can declare both axes. A noisy
  per-subject burst within a tenant is bounded by the per-tenant
  ceiling, even if individual subjects are quiet by themselves.
  Two new env vars to tune; two layers to debug.
- Under C, single-subject monoculture under a tenant maps cleanly
  to "tenant budget" but multi-subject deployments lose the
  per-subject fairness the existing test
  `TestAcquireForSubjectIsolatesSubjects`
  (`internal/ratelimit/per_token_test.go:20`) pins.

### Q2: How is the tenant key derived when authn lacks a tenant_id?

`rateLimitSubjectKey` already handles three cases: no principal,
no tenant on the principal, both present. Any per-tenant policy
must declare what "no tenant" means:

**A. Empty tenant collapses to subject-only bucketing (current).**
A subject whose authentication path doesn't supply a tenant goes
into a key shared with no other subject. Two subjects with the
same name across two unauthenticated paths share a bucket — the
current behaviour. Safe but loses isolation when a deployment
mixes tenant-aware and tenant-unaware auth paths.

**B. Empty tenant collapses to an "anonymous" tenant bucket.**
All authentication paths without a tenant ID share one virtual
tenant — its bucket is the per-tenant limit. Tenant-aware
deployments are unaffected; tenant-unaware deployments are
bounded as a single tenant. Surfaces the asymmetry to operators
who configure both.

**C. Reject the per-tenant gate when no tenant_id.** Skip the
per-tenant layer (Q1 option B / C) for unauthenticated traffic;
fall through to the per-pair limit (Q1 option A) or the global
limit only. Operationally simple but reintroduces the "single
runaway anonymous subject can saturate the global" case the
per-token layer was built to bound.

The choice interacts with Q1: only Q1-B and Q1-C raise this
question. Q1-A leaves the current key derivation unchanged.

### Q3: How do per-(tenant, subject) and per-tenant interact at rejection time?

Today the limiter rejects with `scope` set to `ScopeGlobal` or
`ScopePerToken`, surfaced as a Prometheus label
(`metrics.RateLimitRejections.Inc(kind, scopeLabel)`,
`internal/enforcement/enforcement.go:147`). A per-tenant layer
adds a third scope. Three composition shapes:

**A. Hierarchical, acquire-in-order.** Acquire the per-tenant
slot first (cheaper to check; one map lookup), then the
per-pair slot, then the global. Rejection releases all already-
acquired slots so partial holds never strand. `scopeLabel`
takes the value `per_tenant` when the new layer rejects, exactly
mirroring the existing `per_token` semantics.

**B. Independent, parallel acquire.** Try both per-tenant and
per-pair acquire in parallel goroutines; whichever rejects first
wins the rejection. Hardware-cache hot but introduces a race
between the two map writes that complicates the back-out path.
Likely the wrong call given the existing single-threaded
critical section.

**C. Soft ceiling.** The per-tenant layer counts but does not
reject — it only emits a `metrics.RateLimitNearCap` warning
metric so operators can see who is approaching the configured
ceiling, while only per-pair and global do the rejection. Useful
as a discovery / capacity-planning surface; not a fix for the
DoS shape the per-tenant question was originally raised against.

The error code surfaced to the client must distinguish a
"per-tenant rejection" from a "per-pair rejection" so a noisy
subject and a noisy tenant generate different operator dashboards.
The existing RPCCodeRateLimited error code carries no scope
information today — the scope only appears in the metric, not the
wire error. Q3 would either keep this asymmetry (operators
distinguish via metrics, not wire codes) or introduce a new error
shape (operators distinguish via JSON-RPC error.data fields).

## Alternatives considered

- **Do nothing; document the current per-(tenant, subject)
  behaviour better.** The per-pair shape is in
  `CLOCKIFY_PER_TOKEN_CONCURRENCY` / `CLOCKIFY_PER_TOKEN_RATE_LIMIT`
  Help text; operators who don't read those carefully will be
  surprised by N-subject amplification. Picking this as the
  explicit answer means accepting the recurring operator question.
- **Push the per-tenant limit into Clockify itself.** Out of scope:
  this server is a third-party client, and Clockify's own
  upstream rate limits are global to the API key, not declared
  per tenant. We cannot delegate the per-tenant policy to upstream.
- **Combine the per-tenant question with profile-centric
  configuration (ADR 0015) so the policy is profile-tunable.**
  Defer: profile-tunability would let `local-stdio` keep the
  current behaviour while `shared-service` adopts a per-tenant
  aggregate, but that decision is itself a Q1 sub-question. The
  ADR records the option; the implementation may take it.
- **Pre-position a candidate env var name family in this ADR.**
  Rejected to keep this proposal aligned with the doc-parity
  gate rule on env-var-shaped tokens
  (`scripts/check-doc-parity.sh` §env-vars). The implementation
  commit that flips this ADR to Accepted will introduce the
  spec.go entry, the help docs, and the Helm / k8s manifest
  edits in the same commit per the `config-doc-parity` rule.

## Consequences

Once a decision lands:

- If Q1 = A: no code change. Update the
  `CLOCKIFY_PER_TOKEN_*` Help strings to call out the
  N-subject amplification explicitly so operators are not
  surprised. Close this ADR with Status `Rejected — keep
  per-(tenant, subject) only`.
- If Q1 = B: `internal/ratelimit/ratelimit.go` grows a second
  bucket map keyed by `tenant_id` alone; `AcquireForSubject`
  becomes `AcquireForTenantSubject(ctx, tenant, subject)` or
  similar (interface widens) and the enforcement call site
  updates accordingly. A new env-var pair lands in
  `internal/config/spec.go` and propagates through
  `gen-config-docs`, Helm, k8s. A new metric label value
  `per_tenant` joins `metrics.RateLimitRejections`. The
  `internal/ratelimit/per_token_test.go` matrix gains a
  per-tenant isolation test.
- If Q1 = C: `internal/ratelimit/ratelimit.go` `subjectLimiterFor`
  is rekeyed on `tenant_id` only; `rateLimitSubjectKey` simplifies
  to return the tenant. The session-cap mirror in
  `transport_streamable_http.go:821-829` updates to match. The
  existing test `TestAcquireForSubjectIsolatesSubjects` is
  rewritten or retired; a new `TestAcquireForTenantIsolatesTenants`
  pins the new contract.

Regardless of Q1: `scopeLabel` semantics in
`metrics.RateLimitRejections` are documented in
`docs/observability.md` (or sibling) so dashboards can be tuned.
The session-cap rationale doc in CHANGELOG cross-links this ADR
so future operators reading either trail land on both.

## Migration

When a decision lands:

1. Pick A / B / C for each Q.
2. If Q1 = A, close the ADR; no migration.
3. If Q1 = B or C, update `internal/ratelimit/ratelimit.go` and
   `internal/enforcement/enforcement.go` (key derivation +
   `AcquireForSubject` signature if needed). Regenerate config
   docs and `help_generated.go` in the same commit.
4. Update `internal/mcp/transport_streamable_http.go:821-829`
   if the session-cap key shape changes (it currently mirrors
   `rateLimitSubjectKey`).
5. Add a per-tenant isolation test under
   `internal/ratelimit/per_token_test.go`, with a drift check
   (flip the per-tenant cap to 0 or 1 and assert the matrix goes
   red on cells that should be bounded) in the commit body's
   `Verified:` line.
6. Flip this ADR's Status to `Accepted — <YYYY-MM-DD>` and drop
   the `(proposed)` suffix from the index in `README.md`.
7. Update `CHANGELOG.md` under `### Hardening` (or `### Performance`
   if Q1 = A) to describe the chosen behaviour; cross-link this
   ADR.

## References

- `internal/enforcement/enforcement.go:127-151` — per-call gate
  that derives the subject key and calls `AcquireForSubject`.
- `internal/enforcement/enforcement.go:313-321` —
  `rateLimitSubjectKey(principal)`: returns `""` (no principal),
  `subject` (no tenant), or `tenant_id + "\x00" + subject`.
- `internal/ratelimit/ratelimit.go:253-281` —
  `AcquireForSubject(ctx, subject)` composes global + per-pair.
- `internal/ratelimit/ratelimit.go:283-320` — `subjectLimiterFor`
  lazy-creates per-key buckets in `map[string]*subjectLimiter`.
- `internal/ratelimit/ratelimit.go:112-126` —
  `FromEnvWithAcquireTimeout` reads `CLOCKIFY_PER_TOKEN_*` and
  stores per-token limits on the limiter.
- `internal/ratelimit/per_token_test.go:20` —
  `TestAcquireForSubjectIsolatesSubjects` (per-pair isolation
  contract that Q1-C would retire).
- `internal/ratelimit/per_token_test.go:56` —
  `TestAcquireForSubjectFallsBackToGlobalWhenSubjectEmpty`
  (back-compat with unauthenticated traffic, relevant to Q2).
- `internal/config/spec.go:54-55` —
  `CLOCKIFY_PER_TOKEN_CONCURRENCY` and
  `CLOCKIFY_PER_TOKEN_RATE_LIMIT` Help strings naming
  "tenant+subject" key, with the "falls back to subject when
  tenant is absent" fallback.
- `internal/mcp/transport_streamable_http.go:821-829` —
  `sessionPrincipalKey` deliberately mirroring
  `rateLimitSubjectKey` for the session-cap path.
- `metrics.RateLimitRejections` — labels (`kind`, `scopeLabel`)
  where `scopeLabel` currently takes `"global"` or `"per_token"`;
  a per-tenant layer adds `"per_tenant"`.
- ADR 0014 — Production fail-closed defaults (informs default
  enable / disable for any new layer).
- ADR 0015 — Profile-centric configuration model (per-profile
  tunability of the new env vars).
- ADR 0021 — Hosted tenant policy ceiling (the precedent for
  tenant-scoped policy semantics in the hosted deployment shape).
