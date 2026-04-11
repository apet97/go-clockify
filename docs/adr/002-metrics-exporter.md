# ADR 002 — Custom Prometheus exporter

**Status**: Accepted, 2026-04-11.

## Context

Prometheus exposition in Go almost always means importing
`prometheus/client_golang`, which pulls in a handful of transitive
modules and ~30k LOC. The exposition format itself is a well-defined
text protocol and the counters we actually need (`Counter`, `Gauge`,
`Histogram`) are trivially implementable with `sync/atomic`.

Given ADR 001, we want to avoid the third-party dependency, and we're
willing to accept that we only need a subset of the full exporter's
functionality (no summaries, no native histograms, no label cardinality
guards beyond what the call sites themselves enforce).

## Decision

`internal/metrics` implements the Prometheus text exposition format on
top of the standard library:

- `Counter`, `Gauge`, and `Histogram` types with label-bounded variants
  (`NewCounter`, `NewGauge`, `NewHistogram`, each taking variadic label
  names at registration time).
- Internal storage is `sync.Map` keyed by a label-value string plus
  `atomic.Uint64` / `atomic.Int64` counters. The hot path for a
  counter-increment is a single `sync.Map.Load` + CAS.
- Registry writes to an `io.Writer` implementing the Prometheus 0.0.4
  text format, including `# HELP` and `# TYPE` lines.
- The `/metrics` HTTP handler is a ~20-line wrapper around `Registry.WriteTo`.

## Consequences

- Zero external deps for metrics. The whole package is ~500 LOC with
  tests covering the exposition format, label escaping, and concurrency
  safety.
- Some client_golang features are intentionally not implemented —
  exemplars, summaries, native histograms, per-metric TTLs. If we ever
  need any of these, we revisit this ADR.
- Call sites must bound their label cardinality themselves: a tool or
  transport that `Inc()`s a unique label value per request would blow
  the series budget. All current call sites use bounded label value
  sets (e.g., `outcome` is one of 7 fixed values).
- The exposition format is version-pinned to what Prometheus 0.0.4
  defines. If Prometheus evolves the format incompatibly we'll need to
  update the writer.
