# 0024 - Per-tenant aggregate rate-limit

## Status

Accepted — 2026-05-13. Hosted/shared-service profiles add an
in-process aggregate per-tenant request budget above the existing
per-(tenant, subject) fairness layer. The default remains disabled
for local/community profiles and enabled by profile defaults for
hosted shapes. This keeps per-human fairness inside a tenant while
preventing one hosted tenant from multiplying upstream Clockify load
through many active subjects.

## Context

The per-call gate runs in
`internal/enforcement/enforcement.go`. It passes tenant and subject
separately to `RateLimiter.AcquireForTenantSubject(ctx, tenant,
subject)`. The limiter applies the global process budget first,
then the tenant aggregate layer when a tenant is known and the
tenant caps are configured, then the existing per-token layer keyed
as `(tenant_id, subject)` when both are available or as `subject`
when tenant is absent. Unauthenticated traffic still skips the
per-token and per-tenant layers. The same principal shape is used by
`internal/mcp/transport_streamable_http.go:821-829`
(`sessionPrincipalKey`) for the per-principal session counter
shipped in the make-it-goated wave; the comment there is explicit
that it "Mirrors enforcement.rateLimitSubjectKey to keep the
shared `(tenant_id, subject)` abstraction consistent across the
rate limiter and session counter."

The per-pair quota comes from two env vars:

- `CLOCKIFY_PER_TOKEN_CONCURRENCY` (default 5) — concurrent
  tool-call slots per `(tenant, subject)`.
- `CLOCKIFY_PER_TOKEN_RATE_LIMIT` (default 60) — calls per 60s
  window per `(tenant, subject)`.

The accepted per-tenant aggregate adds two hosted-hardening env vars:

- `CLOCKIFY_PER_TENANT_CONCURRENCY` — concurrent tool-call slots per
  tenant.
- `CLOCKIFY_PER_TENANT_RATE_LIMIT` — calls per 60s window per tenant.

The implementation lives in `internal/ratelimit/ratelimit.go`, which
creates lazy per-key buckets in `map[string]*subjectLimiter` maps and
reaps idle subject and tenant entries via `StartSubjectReaper`.
Rejections fire `metrics.RateLimitRejections.Inc(kind, scopeLabel)`
with `scopeLabel` set to `"global"`, `"per_tenant"`, or
`"per_token"`.

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

**Accepted.** The selected answers are:

- Q1 = B: add a per-tenant aggregate above per-(tenant, subject).
- Q2 = C: skip the per-tenant layer when authentication lacks a
  tenant ID; keep existing subject/global fallback semantics.
- Q3 = A: compose the layers hierarchically, releasing already-held
  slots on later-layer rejection and labelling tenant rejections as
  `per_tenant`.

The options below remain as historical design context for future
operators who revisit the policy boundary.

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

`AcquireForTenantSubject` preserves the three existing cases: no
principal, no tenant on the principal, both present. Any per-tenant
policy must declare what "no tenant" means:

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

**A. Hierarchical, acquire-in-order.** Preserve the existing global
admission gate, then acquire the per-tenant slot, then the per-pair
slot. Rejection releases all already-acquired slots so partial holds
never strand. `scopeLabel` takes the value `per_tenant` when the new
layer rejects, exactly mirroring the existing `per_token` semantics.

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
information today; this ADR keeps that wire shape stable and uses the
metric scope label for operator dashboards.

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
  Accepted in this implementation: profile-tunability lets
  `local-stdio` keep the current behaviour while `shared-service`
  adopts a per-tenant aggregate.
- **Pre-position a candidate env var name family in this ADR.**
  Rejected to keep this proposal aligned with the doc-parity
  gate rule on env-var-shaped tokens
  (`scripts/check-doc-parity.sh` §env-vars). The accepted
  implementation introduces the spec.go entries, generated help, and
  Helm / k8s manifest edits in the same commit per the
  `config-doc-parity` rule.

## Consequences

With Q1-B / Q2-C / Q3-A accepted:

- `internal/ratelimit/ratelimit.go` maintains a second lazy bucket
  map keyed by `tenant_id` alone.
- `AcquireForTenantSubject(ctx, tenant, subject)` preserves the old
  `(tenant, subject)` per-token key while applying a tenant ceiling
  above it.
- `CLOCKIFY_PER_TENANT_CONCURRENCY` and
  `CLOCKIFY_PER_TENANT_RATE_LIMIT` are disabled by default in local
  profiles and set by hosted/shared-service profiles.
- `scopeLabel=per_tenant` joins the existing `global` and
  `per_token` rejection labels.
- Tenant entries are reaped by the same idle limiter reaper used for
  per-token entries, so long-lived hosted replicas do not keep stale
  tenant buckets forever.

## Migration

No data migration is required. Operators who do not configure
`CLOCKIFY_PER_TENANT_*` retain the prior global + per-token behaviour.
Hosted profiles receive the tenant aggregate defaults through profile
configuration and deployment manifests; operators can tune or disable
the layer explicitly with those env vars.

## References

- `internal/enforcement/enforcement.go` — per-call gate that passes
  tenant and subject to `AcquireForTenantSubject`.
- `internal/ratelimit/ratelimit.go` — `AcquireForTenantSubject`
  composes global + tenant aggregate + per-token layers.
- `internal/ratelimit/ratelimit.go` — `subjectLimiterFor` and
  `tenantLimiterFor` lazy-create per-key buckets in
  `map[string]*subjectLimiter`.
- `internal/ratelimit/ratelimit.go` —
  `FromEnvWithAcquireTimeout` reads `CLOCKIFY_PER_TOKEN_*` and
  `CLOCKIFY_PER_TENANT_*`.
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
  where `scopeLabel` takes `"global"`, `"per_tenant"`, or
  `"per_token"`.
- ADR 0014 — Production fail-closed defaults (informs default
  enable / disable for any new layer).
- ADR 0015 — Profile-centric configuration model (per-profile
  tunability of the new env vars).
- ADR 0021 — Hosted tenant policy ceiling (the precedent for
  tenant-scoped policy semantics in the hosted deployment shape).
