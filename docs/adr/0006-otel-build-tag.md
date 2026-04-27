# 0006 - OpenTelemetry tracing via build tag

## Status

Accepted — the OTel sub-module landed in commit `1e2c5c1` (W2-04,
v0.7.0); the integration test gate is `96de77e`.

## Context

ADR 0001 mandates that the default `clockify-mcp` binary links zero
non-stdlib symbols. Operators who run the default install path
(`go install`, `npx`, downloaded binary, distroless image) must get a
binary whose SBOM is the Go standard library and nothing else.

A subset of operators — those running the server in environments
already wired into an OTel collector — want OpenTelemetry tracing
with W3C trace-context propagation. The OTel SDK pulls in dozens of
modules under `go.opentelemetry.io/*`; adding it to the top-level
`go.mod` would inflate the default SBOM by an order of magnitude
and link OTel symbols into every binary we ship, even those of
operators who never use tracing.

We need a way to make tracing a first-class feature for the
operators who want it, without taxing the default build.

## Decision

OpenTelemetry tracing is opt-in via the `otel` build tag, with the
implementation isolated in a dedicated Go sub-module:

- **Tag-neutral facade.** `internal/tracing/tracing.go` defines
  `Tracer` and `Span` interfaces and a no-op default implementation
  (`noopTracer`). Every call site in the codebase imports this
  package and sprinkles `Start`/`End`/`SetAttribute` calls. In the
  default build these compile to no-ops with zero allocations.
- **Sub-module isolation.** `internal/tracing/otel/` is a separate
  Go module (its own `go.mod`) that imports `go.opentelemetry.io/*`
  and implements `tracing.Tracer` against an OTLP HTTP exporter
  with W3C trace-context propagation. Because it is a sub-module,
  the top-level `go.mod` has zero `go.opentelemetry.io` rows.
- **Build-tag wiring.** `cmd/clockify-mcp/otel_on.go` (`//go:build
  otel`) imports the sub-module and calls `Install` from the run
  loop. `cmd/clockify-mcp/otel_off.go` (`//go:build !otel`) provides
  a no-op stub so the call site in `run()` is unconditional.
- **Runtime gate.** `installOTel` reads `OTEL_EXPORTER_OTLP_ENDPOINT`
  as an additional gate inside the `otel`-tagged build. When unset,
  tracing stays on the no-op path even in the OTel-built binary,
  so an operator can ship a single OTel-built image and let each
  deployment opt in via env var.
- **CI enforcement.** `scripts/check-build-tags.sh` runs `go tool nm`
  against the default binary and fails CI if any `opentelemetry`
  symbol is present, and greps the top-level `go.mod` for
  `go.opentelemetry.io` rows.

## Consequences

### Positive

- Default-build operators get exactly what they got before: no OTel
  symbols, no OTel SBOM rows, no OTel transitive deps.
- OTel-build operators get a fully wired OTLP HTTP exporter with
  W3C propagation and a single env var (`OTEL_EXPORTER_OTLP_ENDPOINT`)
  to enable it.
- The sub-module isolation is structural, not procedural — a
  contributor cannot accidentally `import "go.opentelemetry.io/..."`
  in a tag-neutral file because the top-level `go.mod` does not
  list the dependency. The CI gate catches the drift on every PR.
- The shutdown closure pattern (`installOTel(ctx) func()`) means the
  caller in `run()` can `defer shutdown()` unconditionally; the
  no-op build returns a `func() {}` and the OTel build returns a
  proper provider shutdown with a 5s timeout.

### Negative

- Operators who want OTel tracing must rebuild the binary with
  `-tags=otel` rather than flipping a runtime flag. The
  README documents this and the goreleaser config builds an
  `otel`-tagged release artifact alongside the default one.
- The sub-module has its own `go.mod` and `go.sum`, which means it
  needs its own dependency-update workflow and its own test command.
  `scripts/check-build-tags.sh` runs `(cd internal/tracing/otel &&
  go build ./... && go vet ./... && go test -count=1 ./...)`
  explicitly because the top-level `go test ./...` does not descend
  into sub-modules.
- The facade's `noopSpan` methods take `any` arguments, which means
  some allocation slips through on unboxed types. We accept this on
  the hot path because the cost is below measurement noise; the
  benchmarks in `docs/performance.md` confirm.

### Neutral

- The OTel sub-module is the canonical example of how to add a
  build-tagged optional dependency to this project. ADR 0007 (FIPS)
  and ADR 0008 (gRPC) follow the same pattern with different
  isolation mechanisms.
- Span attributes are converted from `any` via a
  `attributeFrom` helper that handles `string`, `bool`, `int`,
  `int64`, `float64`, and falls back to `fmt.Sprintf("%v", v)` for
  everything else. This is intentionally permissive — losing
  attribute fidelity is preferable to dropping a span.

## Alternatives considered

- **Direct OTel dependency, runtime no-op when env vars unset** —
  rejected because the OTel symbols would still link, the SBOM
  would still inherit OTel's transitive graph, and a reviewer
  could not tell from the binary whether tracing was actually
  active.
- **Custom tracing format with stdlib-only export** — rejected
  because operators with existing OTel infrastructure want OTLP +
  W3C, not a bespoke format. The whole point of tracing is
  ecosystem interop.
- **Single binary with `--enable-tracing` flag that dynamically
  loads OTel via plugin** — rejected because Go plugins are
  platform-restricted and the operational complexity exceeds the
  benefit.

## References

- Previously referred to as "ADR 009" in `cmd/clockify-mcp/otel_on.go:15`,
  `cmd/clockify-mcp/otel_off.go:10`, `cmd/clockify-mcp/main.go:147`,
  `internal/tracing/otel/otel.go:5`, `internal/tracing/otel/span_emit_test.go:39`,
  `scripts/check-build-tags.sh:68`.
- Facade: `internal/tracing/tracing.go`.
- Sub-module: `internal/tracing/otel/otel.go`.
- Build-tagged wiring: `cmd/clockify-mcp/otel_on.go`,
  `cmd/clockify-mcp/otel_off.go`.
- CI enforcement: `scripts/check-build-tags.sh`.
- Related ADRs: 0001 (stdlib-only invariant), 0007 (FIPS build tag),
  0008 (gRPC build tag).
- Related docs: `docs/production-readiness.md` "Compliance posture",
  `docs/observability.md`.
- Spec: <https://www.w3.org/TR/trace-context/> (W3C Trace Context).
