# 0017 - Streamable-HTTP session rehydration on cross-pod failover

## Status

Proposed — recorded as a follow-up to the 2026-04-28 ChatGPT-review
wave that shipped ClientIP session affinity on the Helm/k8s Service
templates as the immediate band-aid (see commits `6f50551` and
`a2d99f9`). The architectural fix in this ADR is intentionally
deferred: the design questions below need explicit resolution before
landing.

## Context

The streamable-HTTP transport's session manager
(`internal/mcp/transport_streamable_http.go` `streamSessionManager.get`
near line 517) is process-local. It looks up a session ID in its
in-memory `items` map and returns nil if not present. Sessions ARE
persisted to the control-plane store on `create()` and refreshed on
`touch()`, but `get()` never consults the store on a local miss.

In a multi-replica deployment (the Helm chart defaults to
`replicaCount: 2`), this means:

1. Client sends `initialize` to pod A → session created locally,
   persisted to control-plane store.
2. Client sends `tools/call` and the load balancer picks pod B →
   pod B's `mgr.get()` returns nil → handler responds "session not
   found" and the client must re-initialize.

The 2026-04-28 wave shipped a deploy-only band-aid: Service
`sessionAffinity: ClientIP` with a 24h timeout (matching the
`MCP_SESSION_TTL` ceiling). This pins each client to one pod and
hides the bug for most deployments.

The band-aid is **not** the architectural fix:

- **Shared-NAT egress.** Every client behind a corporate NAT or VPN
  hashes to the same backend, defeating the load-balancing intent.
- **Pod restart / eviction.** Affinity does not survive a pod going
  away; the next request lands on a fresh pod with no session state.
- **Rolling upgrade.** Even with PodDisruptionBudgets, a fraction of
  in-flight sessions tear down on every chart upgrade.
- **Cross-AZ failover.** Affinity is ineffective when the failed pod
  is in another availability zone behind a regional LB.

The store *has* the data needed to reconstruct most of the session
state. The architectural fix is to extend `mgr.get()` to fall back to
the control-plane store and rebuild a `streamSession` via the same
`Factory` used in `create()`. The reason this was deferred to its own
ADR is that the Factory contract widens, the auth model changes
shape, and several persistence questions need explicit resolution.

## Decision (to be made)

Defer until the four design questions below are settled. The ADR
exists to (a) capture the band-aid as load-bearing context and (b)
prevent a future implementation from being landed without addressing
the questions.

### Q1: Factory contract widening

The current `StreamableSessionFactory` signature
(`internal/mcp/transport_streamable_http.go` near line 34) is:

```go
type StreamableSessionFactory func(
    ctx context.Context,
    principal authn.Principal,
    sessionID string,
) (*StreamableSessionRuntime, error)
```

`get()` cannot call this on a local miss because it has no
`authn.Principal` for the requesting client at that point — the
authentication that produced one happened in the request handler at
`transport_streamable_http.go:261-354`, several call frames up the
stack.

Three candidate options:

**A. Pass `*authn.Principal` into `get()` from the handler.** The
handler has freshly authenticated the request and can hand a
non-nil Principal down. `get()` invokes Factory with that Principal.
Cost: `get()` signature changes from `(method, id string)` to
`(method, id string, principal *authn.Principal)`; every call site
in the file is updated. Mechanical, no security shift.

**B. Inject Principal via request context.** Handler stores Principal
in the request context (`authn.WithPrincipal(ctx, ...)`) and Factory
reads it from there. Cost: Factory becomes context-keyed and impure;
testing requires constructing the right context shape. Hard to
reason about because the Principal source becomes invisible in the
function signature.

**C. Persist enough Principal data to reconstruct on rehydration.**
Extend `controlplane.SessionRecord` to carry `AuthMode` and
`SubjectClaims` (or a subset). Factory reads from the record and
synthesises an `authn.Principal` without consulting the request.
Cost: Postgres schema migration; `controlplane.Store` interface
widens; security review of which Claims are safe to persist.

**Recommendation in this ADR**: **Option A**, because it preserves
the request-time security model (the rehydrated session inherits
the freshly-authenticated Principal of the second request, not a
stale one frozen from initialize) and avoids a Postgres migration.
But this is not a final decision; pick after Q2 / Q3.

### Q2: Auth re-validation semantics

When pod B rehydrates a session that pod A created, what does
"this session belongs to client X" mean?

- **Strict re-authentication.** Every request must carry credentials
  that produce a Principal whose `Subject` and `TenantID` match the
  persisted record. Defeats stolen-session-ID replay across pods if
  the original session credentials have been revoked. Cost: every
  request pays the auth cost again (already happens for stateless
  modes like static_bearer; effectively free).
- **Lenient — trust the record.** Once the session ID is presented,
  the persisted Subject/TenantID are taken as truth. Fast, but a
  session ID that leaks before TTL expiry can be replayed across
  any pod after the original credentials are revoked.

Streamable-HTTP today validates the Authorization header on every
request (`transport_streamable_http.go:261-354`), so strict
re-authentication is already the runtime behaviour for the
local-hit path. **The architectural fix should preserve that.** Any
proposal that lands "lenient" semantics needs an explicit
security-review sign-off and a corresponding update to
`docs/security/threat-model.md`.

### Q3: Lost in-memory state and user-visible effect

Even with Principal sorted, several `streamSession` fields are
in-memory by design and would NOT survive rehydration:

- `*mcp.Server` — the protocol engine with negotiated capabilities,
  registered tools, internal request/response correlator. Rebuilt
  by the Factory; the client must re-send `tools/list` and any
  `roots/list` after reconnect. The MCP Streamable-HTTP spec permits
  this (see ADR-0009 for the resource-delta-sync rationale).
- `*sessionEventHub` — SSE backlog ring buffer. The Last-Event-ID
  resumption protocol explicitly tolerates loss; clients re-fetch
  state after a gap.
- In-flight tool-call cancellation handles. A request in progress
  on the old pod cannot be cancelled from the new pod's
  `notifications/cancelled` after rehydration.

**Recommendation**: rehydration produces a "fresh" session with the
same ID and persistent metadata but no carry-over for in-flight
work. Document this in `docs/clients.md` so client implementers know
to retry idempotent calls after a rehydration boundary.

### Q4: Test strategy

Cross-pod failover is hard to assert in a single-process test. The
existing harness in `tests/harness/streamable.go` runs one server.
Two options:

- **In-process two-server harness.** Spin up two `mcp.Server`
  instances backed by a shared `controlplane.Store` (file:// or
  in-memory). Send `initialize` to server A, `tools/call` to server
  B, assert the second hits the rehydration path. Cheap; covers
  the unit-of-work this ADR delivers.
- **Real multi-pod integration test.** kind/k3d cluster with
  `replicaCount: 2` and a postgres backend. Asserts the deploy
  graph the operator actually runs. Expensive; lands as a separate
  CI workflow or a release-time smoke test.

**Recommendation**: ship both. The in-process test gates the PR;
the real-cluster test runs nightly so a regression in the deploy
graph (Helm chart default change, kustomize overlay drift) is
caught within 24h.

## Consequences

**If the architectural fix is landed (positive).**
- ClientIP affinity becomes redundant for correctness; it remains a
  perf optimisation (warm caches, fewer cold-start tool catalogs).
- Rolling upgrades and pod evictions stop terminating active client
  sessions.
- Shared-NAT egress no longer concentrates on one backend.
- Postgres-backed deployments gain horizontal scalability for SSE
  subscribers.

**Until the architectural fix is landed (negative — accept these).**
- Operators on Helm `replicaCount > 1` with non-affine load
  balancers (some legacy F5 / HAProxy front-ends) see "session not
  found" errors on every load-balanced request.
- Pod-evicted clients must re-initialize; long-running agentic
  flows lose state at the boundary.
- The contract is documented at
  `deploy/helm/clockify-mcp/templates/service.yaml:6-8` (NAT-egress
  trade-off comment) and in the Wave H ADR-0014 §"Open follow-up".

**Documentation contract (must be honoured by any
implementation).**
- The chosen option for Q1 must be recorded inline in this ADR's
  Status section and cross-referenced from `docs/clients.md` so
  client-side retry logic knows whether rehydration is "fresh" or
  "carry-over".
- The Q2 decision becomes part of the security threat model.
- The Q4 test strategy lands before the implementation, not after
  (per the project's TDD-via-drift-check convention).

## References

- `internal/mcp/transport_streamable_http.go` — `streamSessionManager.get`
  (the local-only lookup) and `Factory` declaration.
- `internal/controlplane/store.go` — `SessionRecord`, the persistent
  shape that today carries ID/TenantID/Subject/Transport/TTL/
  ProtocolVersion/ClientName/Version. `AuthMode` and `Claims` are
  intentionally absent.
- `internal/runtime/streamable.go` — Factory construction call site.
- `deploy/helm/clockify-mcp/templates/service.yaml` and
  `deploy/k8s/base/service.yaml` — ClientIP affinity (the band-aid).
- ADR-0014 — Wave H production fail-closed defaults; the
  streamable-HTTP guard that landed alongside the affinity fix.
- ADR-0009 — Resource-delta-sync; permits server-side state rebuild
  after reconnection.
