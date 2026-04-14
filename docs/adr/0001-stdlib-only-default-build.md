# 0001 - Stdlib-only default build

## Status

Accepted — the invariant has held since `1833084` (v0.6.0) and is
enforced on every build by `scripts/check-build-tags.sh`.

## Context

`clockify-mcp` is a single-binary MCP server that ships to operators
who must audit it before deployment. Every third-party Go dependency
adds supply-chain surface area, an upgrade cadence we cannot control,
and a license-review obligation. We want the default install path
(`go install`, `npx`, downloaded binary) to carry the smallest
possible SBOM and to be trivially reproducible.

The Go standard library now covers everything a small MCP server
actually needs: `log/slog` for structured logging, `net/http` for both
client and server, `crypto/subtle` for constant-time comparisons,
`encoding/json` for marshalling, `sync/atomic` for counters,
`math/rand/v2` for jitter. None of these have idiomatic third-party
replacements that justify the supply-chain cost on a project this
small.

## Decision

The default `go build` produces a binary that links **only the Go
standard library**. The top-level `go.mod` has zero non-stdlib `require`
rows. There is no `go.sum`. Features that would normally pull in a
third-party module are either re-implemented using stdlib primitives
or gated behind a build tag and physically isolated in a sub-module.

Two CI gates protect the invariant:

1. `scripts/check-build-tags.sh` runs `go tool nm` against the default
   binary and asserts the absence of `opentelemetry`, `net/http/pprof`,
   and `google.golang.org/grpc` symbols.
2. The same script greps the top-level `go.mod` for `go.opentelemetry.io`
   and `google.golang.org/grpc` rows and exits non-zero on any match.

Build-tag escape hatches that are explicitly allowed and which live in
their own sub-modules or build-tagged files:

- `-tags=otel` — OpenTelemetry tracing (ADR 0006).
- `-tags=grpc` — gRPC transport (ADR 0008).
- `-tags=fips` — FIPS 140-3 build (ADR 0007).
- `-tags=pprof` — debug pprof endpoints.

## Consequences

### Positive

- Zero non-stdlib runtime dependencies in the default binary, so the
  SBOM is trivially small and the supply-chain attack surface is the
  Go toolchain itself.
- No `go.sum` to drift, no Dependabot churn, no transitive CVE alerts.
- Reproducible builds are easy: a `-trimpath` build with the pinned Go
  version reproduces byte-for-byte (asserted by
  `.github/workflows/reproducibility.yml`).
- Every reviewer can audit the entire dependency graph by reading
  `go.mod` and the Go standard library.

### Negative

- Some features that would be a one-line import in a normal Go project
  (Prometheus exposition, OTLP export, gRPC) require either a
  hand-written stdlib equivalent or a build-tag opt-in with its own
  Go sub-module.
- Operators who want OTel tracing or gRPC must rebuild rather than
  flip a runtime flag. The build-tag UX is documented in
  `README.md` and `docs/production-readiness.md`.

### Neutral

- The Prometheus exposition format is implemented by hand in
  `internal/metrics/`. It is a deliberate cost, not an oversight.
- Build tags and sub-modules increase the matrix that
  `scripts/check-build-tags.sh` has to exercise. The script is the
  single source of truth for which combinations CI cares about.

## Alternatives considered

- **Standard third-party stack (zap + prometheus client + OTel SDK)**
  — rejected because the supply-chain cost dwarfs the DX benefit on a
  project of this size, and the stdlib alternatives are good enough.
- **Use `go.opentelemetry.io` directly in the default build, gated at
  runtime** — rejected because the symbols would still link, the SBOM
  would still inherit OTel's transitive graph, and operators auditing
  the binary could not tell whether tracing was actually enabled.
- **Vendor selected dependencies into `internal/`** — rejected because
  the licensing posture and upgrade story would still be that of the
  upstream projects, only with extra friction.

## References

- Previously referred to as "ADR 001" in `internal/tracing/tracing.go`,
  `internal/tracing/otel/otel.go:4`, `internal/jsonmergepatch/merge_patch.go:7`,
  `internal/transport/grpc/codec.go:8`, `tests/chaos/README.md:89`.
- Enforcement script: `scripts/check-build-tags.sh`.
- Related ADRs: 0006 (OTel build tag), 0007 (FIPS build tag),
  0008 (gRPC build tag).
- Related docs: `README.md` "Stdlib-only default build" bullet,
  `docs/production-readiness.md` "Compliance posture" section,
  `CONTRIBUTING.md` "Design Principles".
