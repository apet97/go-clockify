# Future: end-to-end trace correlation (MCP → Clockify)

Status: **stub / not implemented**. This document is a placeholder
for a future wave. It captures the shape of the work so the next
observability pass doesn't have to start from a blank page, but
nothing in the codebase reads it and nothing in CI enforces it.

## Why this isn't built yet

Wave A's `internal/tracing` package exposes an OTel-capable tracer
behind the `otel` build tag (see `internal/tracing/otel/otel.go`), and
individual handlers create spans when the tag is on. That's enough
to answer "what happened inside this process?" — it is **not**
enough to answer "was this slow MCP call caused by an upstream
Clockify slow query?" The second question requires a single
`trace_id` propagated across the MCP client boundary on the ingress
side and out through the Clockify HTTP client on the egress side,
with the intermediate span tree stitching the two ends together.

Wave D deliberately does not ship this because:

1. The MCP spec is still moving on how trace context should ride on
   JSON-RPC messages. Building on a pre-standard carries a rewrite
   cost we don't need until a user actually asks for cross-boundary
   correlation.
2. The Clockify upstream API does not propagate W3C `traceparent`
   headers. Even a perfect ingress implementation would see the
   trace end at the outbound `http.Client.Do` call — we'd get the
   HTTP timing but not the upstream DB query. That makes a
   Clockify-side instrumentation ask inevitable, which is somebody
   else's roadmap item.
3. Wave-D effort was spent closing real issues (release-smoke,
   live-contract fail-soft, branch-protection snapshot gap). Adding
   a net-new feature would have been scope creep.

## What a future wave would need to add

Not a design doc — a checklist of the concrete pieces.

### Ingress (MCP client → server)

- **Extract trace context from MCP requests.** Decide whether a new
  `meta` envelope field carries `traceparent`/`tracestate`, or whether
  the transport (HTTP vs stdio) gets to inject it out-of-band. HTTP
  can use headers; stdio needs a protocol-level hook.
- **Propagate into request-scoped context.** The existing handler
  wiring in `internal/mcp/server.go` already threads a `context.Context`
  — start a root span in the transport layer and attach it with
  `trace.ContextWithSpan` before dispatch hits the handlers.
- **Record on the span.** Tool name, workspace id (if policy
  allows), request id — the attributes that would let an operator
  triage "why is this MCP session slow?" from a trace viewer.

### In-process (handler → Clockify client)

The span tree already produced by the `otel` build tag is the
starting point. No changes needed if the ingress root span is
propagated correctly — child spans inherit the context.

### Egress (Clockify HTTP client)

- **Wrap the HTTP client's `RoundTripper`.** `internal/clockify/client.go`
  uses a plain `*http.Client`. A future wave wraps it with
  `otelhttp.NewTransport` so outbound requests get child spans
  automatically. The `otel` build tag already guards the dependency.
- **Decide about `traceparent` on the wire.** Clockify ignores the
  header today, but sending it is cheap and future-proofs the
  pipeline against a hypothetical upstream instrumentation pass.
- **Span attributes on the egress span:** the sanitized request URL,
  the HTTP status, the Clockify error code envelope if the response
  body is a known error shape. Leave auth headers out (they're
  already scrubbed from our error messages).

### Operator-facing

- **README / `docs/operators/` update:** how to enable
  `trace_id` propagation, which trace exporter endpoints are
  expected, and how to configure the sampler. (`docs/safe-usage.md`
  was the original target landing page when this plan was written
  but is no longer in the repo; `docs/operators/` is the current
  home for operator-facing per-profile guidance.)
- **Runbook entry in docs/runbooks/:** "given a slow MCP tool call,
  here is how to pull the trace and identify the upstream cause."
- **Smoke test in tests/e2e_otel_test.go (build tag gated):** dispatch
  a tool call with an injected `traceparent`, assert that the outbound
  Clockify request carries the same trace id.

## Reference points in the current code

| Concern                           | File                                |
|-----------------------------------|-------------------------------------|
| Build-tag-gated OTel init         | `internal/tracing/otel/otel.go`     |
| Tracer interface + no-op fallback | `internal/tracing/tracing.go`       |
| MCP server dispatch (ingress)     | `internal/mcp/server.go`            |
| Clockify HTTP transport (egress)  | `internal/clockify/client.go`       |
| Make target                       | `make verify-tags` under `otel` tag |

## Open questions for the next wave

- Where does the trace exporter live? OTLP/HTTP? OTLP/gRPC? Stdout?
- Default sampler — `AlwaysSample`, `TraceIDRatioBased(0.1)`,
  `ParentBased`?
- Do we care about baggage? The project doesn't currently use OTel
  baggage for anything. Adding it adds a dependency on a wire
  format we then have to stabilize.
- Is correlation visible in logs too (slog attr'd with `trace_id`)?
  The existing `internal/tracing/tracing.go` exposes a
  `trace.SpanFromContext(ctx).SpanContext().TraceID()` helper — a
  slog middleware that auto-attaches this would close the
  logs-vs-traces gap for free.

---

This document will be deleted (or rewritten into a real design doc)
the moment a wave starts actually building cross-boundary
correlation. Until then it exists so the next person doesn't waste
the first half of their wave rediscovering the constraints above.
