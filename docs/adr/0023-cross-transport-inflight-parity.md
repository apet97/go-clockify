# 0023 - Cross-transport `MaxInFlightToolCalls` parity

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


## Status

Proposed — captured here because extending the
`MCP_MAX_INFLIGHT_TOOL_CALLS` cap beyond stdio reverses an explicit
operator-affecting decision recorded by commit `b42f074`
("docs(perf): clarify MCP_MAX_INFLIGHT_TOOL_CALLS is stdio-only").
Strict-rule #2 in `CLAUDE.md` forbids quietly broadening operator
surfaces under the cover of "consistency": the right path is to
state the question, list the candidate answers, and accept that the
chosen answer will move this ADR to Accepted before any
implementation lands. This ADR records the questions a maintainer
must resolve before that implementation can ship; it does **not**
record an accepted decision. When a decision lands, this file moves
to Accepted, the `(proposed)` suffix drops from the ADR index in
[`README.md`](README.md), and the implementation wave (formerly
known as "Wave B.4 — cross-transport parity") can proceed.

## Context

`internal/config/spec.go:60` defines
`MCP_MAX_INFLIGHT_TOOL_CALLS` with `AppliesTo: []string{"stdio"}`
and a Help string that ends "Has no effect on streamable_http or
grpc transports — those use admission rate limits
(`MCP_HTTP_RATELIMIT_*`) and gRPC flow control respectively." That
text is the operator contract today.

The cap is wired through one path only:

- `internal/mcp/server.go:278-282` — `Server.MaxInFlightToolCalls`
  field docstring: "bounds the number of concurrently-running
  tools/call goroutines spawned by the stdio dispatch loop".
- `internal/mcp/server.go:345` — `toolCallSem chan struct{} //
  dispatch-layer goroutine cap; nil = unlimited`.
- `internal/mcp/server.go:546-547` — `Run()` lazily constructs the
  semaphore at stdio start-up.
- `internal/mcp/server.go:636-675` — the stdio dispatch loop
  acquires a slot before each `tools/call` goroutine and releases
  on goroutine exit; context cancellation prevents shutdown
  deadlock.
- `internal/mcp/server.go:704-714` — `DispatchMessage()`, the entry
  point used by *both* non-stdio transports (gRPC sub-module and
  streamable HTTP), carries a docstring that ends "This method
  does NOT apply the stdio dispatch-layer toolCallSem. Callers
  that need backpressure on tools/call must implement their own
  bound."

The non-stdio transports have their own admission machinery:

- Streamable HTTP runs every request through
  `internal/mcp/http_admission.go:17-76` (`httpAdmissionLimiter`)
  with per-IP, per-principal, and per-session caps fed by
  `MCP_HTTP_RATELIMIT_*` and the session caps shipped in the
  make-it-goated wave (`MCP_MAX_SESSIONS_PER_REPLICA` and
  `MCP_MAX_SESSIONS_PER_PRINCIPAL`).
- gRPC inherits transport-native HTTP/2 flow control plus the
  per-tenant rate-limit semantics of `internal/ratelimit` once
  authn lands a principal.

The asymmetry is intentional today: stdio has no transport-level
backpressure and *must* bound goroutines explicitly; the other two
transports have richer admission surfaces and the goroutine cap
would be a coarse alternative to existing per-IP/per-principal
controls. Yet operators repeatedly ask "what does
`MCP_MAX_INFLIGHT_TOOL_CALLS` cap *globally*", and the honest answer
("nothing on HTTP or gRPC") surprises them every time. The cost of
the status quo is operator confusion. The cost of moving the cap
into the cross-transport path is a new admission layer with its own
DoS semantics. This ADR exists to force the operator-policy choice
into the open before code lands either way.

## Decision

**Proposed.** The questions below frame the design space; each
must have an explicit answer before this ADR moves to Accepted.
The implementation wave is blocked on those answers.

### Q1: Should the bound apply per-transport, or globally per replica?

Today the cap is per-transport (stdio only). Two coherent
generalisations exist:

**A. Per-transport, with one env var per transport.** Introduce a
new abstract per-transport in-flight cap env-var family — one knob
each for streamable HTTP and gRPC — alongside the existing
stdio-scoped knob. Operators reason about each ingress independently
("HTTP handles 200 in-flight; gRPC handles 500"). Composition with
existing `MCP_HTTP_RATELIMIT_*` is additive. Drift between transports
becomes the operator's responsibility.

**B. One global cap shared across all transports.** Keep
`MCP_MAX_INFLIGHT_TOOL_CALLS` as the only knob and reinterpret its
meaning as "this replica runs at most N concurrent `tools/call`
handlers regardless of ingress." A single semaphore in
`internal/mcp/server.go` is shared by stdio's `Run()` and the
`DispatchMessage()` path that streamable HTTP and gRPC take. The
cap caps the binary as a whole; operators reason about the replica
in aggregate.

**C. Per-transport bound *plus* a global ceiling.** Both A and B at
once. The global ceiling is a hard fail-safe; per-transport bounds
let operators shape ingress mix. The matrix has six knobs (the
existing one + three new ones + the global ceiling + a precedence
rule) and the operator must understand the precedence rule. Highest
expressive power, highest operator-cognitive cost.

Each option has different DoS / fairness implications:

- Under A, a single high-traffic ingress can starve a low-traffic
  one only within the global goroutine budget the OS provides; the
  policy boundary is per-ingress.
- Under B, a saturating ingress directly starves the others on the
  same replica. This is the operator-friendly behaviour for
  capacity planning ("the replica handles N total") but the
  operator-hostile behaviour for fairness ("HTTP just ate every
  slot, gRPC clients are timing out").
- Under C, the operator must explicitly reason about precedence
  (which limit fires first, what signal each emits) when tuning.

Option A is the path of least surprise vs. the current
`AppliesTo`-shaped contract. Option B is the path of operator
mental-model simplicity. Option C is the path of expressive power.

### Q2: Does the bound replace or compose with existing per-transport limits?

Streamable HTTP already has `httpAdmissionLimiter` (per-IP,
per-principal, per-session). gRPC has native HTTP/2 flow control.
If a cross-transport `MaxInFlight` lands, it must declare its
relationship to those existing layers.

**A. Inner layer — fire before admission.** The semaphore acquires
in `DispatchMessage()`, *before* tool dispatch but *after* HTTP
admission. A 503 from admission and a 503 from in-flight cap come
from different layers and the response code must carry that
distinction (Retry-After value or a header). Operators tune
admission for fairness and in-flight for resource bounds.

**B. Outer layer — fire after admission.** The semaphore acquires
*at the HTTP edge* alongside `httpAdmissionLimiter` and reuses its
rejection semantics (503 + Retry-After). Admission and in-flight
collapse to one operator-facing signal: "rate-limited" / "at
capacity". Simpler but loses the per-IP/per-principal precision
admission gives today.

**C. Replace, not compose.** The new cap subsumes the per-session
cap in `httpAdmissionLimiter`. Operators get one knob for
"concurrent work per replica" and the per-session cap is removed.
Breaks the make-it-goated session-cap contract; requires its own
ADR amendment.

Composition order matters because operators read 503 / 429 signals
when capacity-planning, and the layer that emitted the signal
changes the corrective action. A 503 from per-IP admission says
"that IP is too noisy"; a 503 from a global in-flight cap says
"this replica is at its concurrency budget"; a 503 from the
per-session cap says "this session has too many concurrent calls".
Collapsing the three into one undifferentiated 503 is a regression
in operator observability.

### Q3: What rejection signal does each transport return when the bound is hit?

The current stdio path applies *back-pressure*: the dispatch loop
blocks on `s.toolCallSem <- struct{}{}` until a slot frees, with
context cancellation as the exit. Stdio clients see latency, not
errors. Streamable HTTP and gRPC clients need a different shape
because they multiplex requests and an indefinite block on one
in-flight call would stall every other call on the same connection.

**A. Blocking under the request deadline.** The acquire is bounded
by the request's context deadline (or a configurable cap). On
timeout, the call returns an MCP-level error
(`-32030 RPCCodeServiceUnavailable`, already minted for the
session-cap path). Same JSON-RPC shape across transports; operators
read a single signal source.

**B. Immediate rejection.** Try-acquire only; if the semaphore is
full, return `RPCCodeServiceUnavailable` + Retry-After
immediately. Client-side back-pressure is delegated entirely to
the SDK / agent. Cheap to implement, easy to reason about, but
operators lose the "amortise transient burst" benefit the stdio
back-pressure shape provides today.

**C. Transport-native cancellation.** gRPC streams cancel with
`ResourceExhausted`; HTTP returns 503 + Retry-After; stdio retains
its existing block-with-context-cancel. Native idioms per
transport, no new MCP-level error code. Costs cross-transport
parity: operators of a multi-transport deployment now monitor
three different error surfaces for the same condition.

The signal-type choice interacts with Q2: an inner-layer cap (Q2-A)
needs to fail-fast (Q3-B or Q3-C) so it does not amplify the
admission layer's queuing. An outer-layer cap (Q2-B) can blend
into admission's existing signal vocabulary.

## Alternatives considered

- **Do nothing; document the stdio-only scope harder.** The
  make-it-goated wave already documented this via `b42f074`. The
  asymmetry stays; the operator question recurs. Picking this as
  the explicit answer to "what is `MCP_MAX_INFLIGHT_TOOL_CALLS`?"
  means accepting that every new operator will ask once and
  receive a doc-link answer. Acceptable if the rate-of-asks is low.
- **Repurpose the session caps as the de-facto in-flight cap.**
  `MCP_MAX_SESSIONS_PER_REPLICA` (introduced in the make-it-goated
  wave) bounds concurrent streamable-HTTP sessions, which bounds
  concurrent work indirectly. Rejected as a stand-alone answer
  because it does not address gRPC at all and conflates session
  lifetime with per-call concurrency.
- **Pre-position a candidate env var name family in this ADR.**
  Rejected to keep this proposal aligned with the doc-parity gate
  rule on env-var-shaped tokens
  (`scripts/check-doc-parity.sh` §env-vars). The implementation
  commit that flips this ADR to Accepted will introduce the
  spec.go entry, the help docs, and the Helm / k8s manifest edits
  in the same commit per the `config-doc-parity` rule.

## Consequences

Once a decision lands:

- The Help string on `MCP_MAX_INFLIGHT_TOOL_CALLS` in
  `internal/config/spec.go:60` is rewritten to describe the chosen
  semantics; the generator runs (`go run ./cmd/gen-config-docs
  -mode=all`) and the README config-table block updates in the
  same commit.
- A new cross-transport parity test joins the existing matrix in
  `tests/` (sibling of `tests/parity_test.go` and
  `tests/size_limit_parity_test.go`). The test pins that all
  participating transports respect the cap, that the rejection
  signal matches the ADR-decided shape, and that under-cap traffic
  is unaffected. Drift-checks (flip the cap to 0 or 1 and assert
  the matrix goes red on cells that should be bounded) land in
  the same commit per the repo's `Verified:` discipline.
- A new Prometheus metric joins the existing
  `metrics.ProtocolErrorsTotal` family to count cap rejections per
  transport, so operators can compare cap-rejection rates with
  admission-rejection rates and tune both layers.
- The session-cap contract (per-replica / per-principal session
  count) is unchanged unless Q2 picks option C, in which case the
  make-it-goated session-cap path is rewritten and that ADR
  amendment lands in the same wave.

If the answer is "do nothing", the only consequence is closing
this ADR with `Status: Rejected — keep stdio-only` and updating
the index suffix accordingly.

## Migration

When a decision lands:

1. Pick A / B / C for each Q.
2. Run the `config-doc-parity` precheck before staging.
3. Update `internal/config/spec.go`, regenerate config-docs and
   `help_generated.go`, edit Helm + k8s configmap defaults, all in
   one commit per the `config-doc-parity` gate.
4. Add the cross-transport parity test in `tests/` and drift-check
   the cap=0 / cap=1 boundary in the commit body's `Verified:`
   line.
5. Flip this ADR's Status to `Accepted — <YYYY-MM-DD>` and drop the
   `(proposed)` suffix from the index in `README.md`.
6. Update `CHANGELOG.md` under `### Performance` (or `### Hardening`
   per the chosen scope) to describe the cap; cross-link this
   ADR.

If the decision is "do nothing", instead flip Status to
`Rejected — <date>` and update the index suffix to `(rejected)` if
such a convention is introduced (none exists at the time of this
writing).

## References

- `internal/config/spec.go:60` — `MCP_MAX_INFLIGHT_TOOL_CALLS`
  env-var spec; `AppliesTo: []string{"stdio"}`; Help string
  explicitly excludes streamable_http and grpc.
- `internal/mcp/server.go:278-282` — `MaxInFlightToolCalls` field
  docstring.
- `internal/mcp/server.go:345` — `toolCallSem chan struct{}`
  declaration with the `nil = unlimited` invariant.
- `internal/mcp/server.go:546-547` — semaphore lazy construction
  inside `Run()`.
- `internal/mcp/server.go:636-675` — stdio dispatch acquire /
  release pair and the context-cancellation shutdown shape.
- `internal/mcp/server.go:704-714` — `DispatchMessage()` docstring
  excluding the semaphore for non-stdio callers.
- `internal/mcp/http_admission.go:17-76` — streamable-HTTP
  `httpAdmissionLimiter` (per-IP, per-principal, per-session).
- Commit `b42f074` — doc clarification doubling down on stdio-only
  scope; the operator-affecting decision this ADR proposes to
  revisit.
- `tests/parity_test.go`, `tests/size_limit_parity_test.go`,
  `tests/cancellation_parity_test.go` — cross-transport parity
  matrices any new cap must extend.
- ADR 0002 — Transport selection (foundational; defines the
  transports in scope).
- ADR 0008 — gRPC auth via stream interceptor (gRPC dispatch path
  this ADR would touch).
- ADR 0010 — Metrics stack direction (signal source for rejection
  counters added under Q2 / Q3).
- ADR 0014 — Production fail-closed defaults (informs whether the
  default cap value differs by profile).
