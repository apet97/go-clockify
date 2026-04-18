# 0010 - Metrics stack direction

## Status

Proposed — spawned out of the "deeper-read" review that closed Wave A
and landed most of Wave B. No code change yet; this ADR captures the
decision surface so the next operator-facing metrics change is
deliberate rather than accumulated.

## Context

The server ships a bespoke metrics facade under `internal/metrics/`:
registry, counter / histogram / gauge primitives, a Prometheus text
format writer, and ~15 hand-registered series. It is intentionally
small (see ADR 0001, which mandates that the default binary link
zero non-stdlib symbols).

Over the last few waves the footprint grew:

- New counters for SSE subscriber drops, replay misses, and session
  reaps (B4).
- Existing counters covering rate limiting, audit failures, panics,
  HTTP request status, upstream retries, protocol errors.

The review surfaced three valid ends of the design space:

1. **Keep homegrown** — stdlib-only, zero dependency surface, aligned
   with ADR 0001 and ADR 0006's "opt-in via build tag" pattern.
2. **Adopt OpenTelemetry metrics** — canonical cloud-native tooling,
   swap in OTLP/Prom exporters, rich attribute system.
3. **Facade + exporter** — keep the internal API, plug an OTel
   exporter behind an `otel` build tag (same shape as tracing).

The plan file's Wave E3 asked for a decision note before the next
round of metrics work; committing now (instead of after a larger
metrics push) avoids the "we already built something, now we can't
change it" trap.

## Decision

**Keep homegrown for v0.x, revisit at v1.0.**

The current facade is small, debuggable, and carries no dependency
cost. Migrating to OTel now would:

- Break the ADR 0001 contract (OTel metrics would be non-stdlib).
- Force every downstream binary (stdio, http, streamable_http, grpc)
  to decide whether to link it — we already carry that cost for
  tracing and don't want to pay it twice unless forced.
- Lose nothing concrete: Prometheus-compatible text is the only
  surface operators consume today, and the homegrown writer emits
  the exact format that Prometheus scrape, Alertmanager recording
  rules, and Grafana dashboards in `deploy/` already depend on.

Explicit non-decisions:

- We **do not** promise homegrown forever. The line of defense is
  ADR 0006's shape: if/when OTel metrics become the industry-wide
  scrape contract operators expect, add an `otel_metrics` sub-module
  that implements an adapter against the same facade. No rewrite,
  no contract break.
- We **do** prioritise the facade staying small. New metric types
  (summary, exemplar trace-to-span linking) should go through an
  ADR before code, so the surface does not grow quietly past what
  the stdlib-only constraint supports.

## Consequences

- The facade in `internal/metrics/metrics.go` stays the source of
  truth. Every new counter or histogram lands there, with an
  operator-facing comment documenting label cardinality up front.
- Dashboards and alert rules in `deploy/` stay Prometheus-native;
  operators who want OTLP today export from Prometheus.
- When the OTel adapter lands (ADR TBD), the decision table above
  gets a new row — "OTel adapter live since vX.Y" — and the
  facade's `Registry.WriteTo` grows a second consumer. No behaviour
  change for operators who stay on the Prometheus scrape.

## Alternatives considered

- **Adopt OTel metrics directly.** Rejected for the reasons above;
  the dependency cost isn't justified by what the facade can't do.
- **Delete the facade and use `expvar`.** Rejected: expvar has no
  histograms, no labels, and emits a format Prometheus has to parse
  via an adapter anyway — we'd still own the label cardinality work.
- **Adopt `prometheus/client_golang` directly.** Rejected for the
  same ADR 0001 reason; pulling it in taxes every downstream build.
