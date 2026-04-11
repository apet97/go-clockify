# ADR 001 — Stdlib-only default build

**Status**: Accepted, 2026-04-11.

## Context

Third-party Go dependencies in a small MCP server compound over time: each
adds supply-chain surface area, a license review, and an upgrade cadence
that usually drags behind the ecosystem. We want to keep `clockify-mcp`
easy to audit and fast to patch, even if that costs a little DX.

## Decision

The default `go build` produces a binary that links **only the Go
standard library**, plus the language runtime. Features that would
normally pull in a third-party package are either re-implemented using
stdlib primitives or gated behind a build tag.

Stdlib features actively used in place of typical external deps:

- **Structured logging** — `log/slog` replaces `zap`/`zerolog`.
- **HTTP client + server** — `net/http` replaces `fasthttp`/`fiber`.
- **Random jitter** — `math/rand/v2` replaces `go.uber.org/goleak` or
  third-party RNGs.
- **Constant-time token comparison** — `crypto/subtle` replaces the
  classic manual loop or third-party helpers.
- **Lock-free counters** — `sync/atomic` CAS loops (in the metrics
  exporter) replace `prometheus/client_golang`.
- **JSON** — `encoding/json` rather than `json-iterator` or `easyjson`.

The tracing package (`internal/tracing/`) allows an opt-in
OpenTelemetry build via `-tags=otel`, which does link
`go.opentelemetry.io/*`. The default build is verified to carry
**zero** `opentelemetry` symbols by a CI job running `go tool nm` on
the default binary. As of Wave 2 (W2-04) the OTel wiring lives in a
dedicated Go sub-module at `internal/tracing/otel/`, so the top-level
`go.mod` also carries **zero** `go.opentelemetry.io` rows — see ADR 009
for the sub-module layout and the `go mod tidy` caveat.

## Consequences

- Zero third-party runtime dependencies in the default build — trivially
  small SBOM, trivially small attack surface.
- Any new code must justify not using stdlib. Reviewers should push back
  on PRs that introduce deps for features the stdlib already covers.
- A little DX cost: we re-implemented Prometheus exposition format by
  hand (see ADR 002) and wrote our own tracing facade.
- Optional features like OTel are gated behind build tags, which makes
  them invisible to users who never need them — but also means users who
  do need them must rebuild rather than flip a runtime flag.
