# ADR 009 — Tracing OTel code isolated as a Go sub-module

**Status**: Accepted, 2026-04-11.

## Context

ADR 001 ("stdlib-only default build") promised that `go build` without
tags produces a binary that links only the Go standard library. The
tracing package (`internal/tracing/`) delivered a tag-neutral `Tracer`
interface with a noop implementation, and wired an OpenTelemetry-backed
`otelTracer` behind `//go:build otel` in a single file,
`internal/tracing/otel.go`. Because Go does not evaluate build tags in
the module graph, every `go.opentelemetry.io/*` package that file
imported showed up as an indirect row in the top-level `go.mod` — nine
rows in total (auto/sdk, otel, otlptrace, otlptracehttp, metric, sdk,
trace, proto/otlp, grpc). Nothing in those rows was linked into the
default binary — the CI nm gate in `.github/workflows/ci.yml` has been
enforcing zero `opentelemetry` symbols in `/tmp/clockify-mcp-default`
since ADR 001 landed — but the rows made the top-level module graph
look heavier than the binary actually was. Any downstream consumer
reading `go.mod` at face value would conclude that this project
depends on OpenTelemetry, even though the default build provably does
not. The W1 close-out note in ADR 001 explicitly deferred the clean-up
to Wave 2 because the sub-module migration did not fit the W1 cut.

Wave 2 scope (`docs/wave2-backlog.md` W2-04) closes that deferred
trade-off. Two alternatives were considered before landing on the
sub-module design:

1. **Leave the OTel rows in `go.mod` as `// indirect`.** Rejected
   because the whole point of ADR 001 is that third-party rows in the
   module graph should be *true statements* about what the binary
   links. Leaving the rows as indirect hides the fact that the default
   build never reaches any of them.
2. **Move the OTel code into an entirely separate repository**
   (e.g. `github.com/apet97/go-clockify-otel`). Rejected because the
   sub-module must import `internal/tracing` for the `Tracer`/`Span`
   interfaces, and `internal/` paths are private to their owning
   module. Splitting repos would force the interface out of
   `internal/`, which is a larger surface change than the OTel
   isolation warrants.

## Decision

Move `internal/tracing/otel.go` into a new Go sub-module at
`internal/tracing/otel/` with its own `go.mod` (module path
`github.com/apet97/go-clockify/internal/tracing/otel`). The sub-module
imports the parent `internal/tracing` package for the `Tracer` and
`Span` interfaces via a `replace github.com/apet97/go-clockify =>
../../..` directive; the parent module has a matching `replace
github.com/apet97/go-clockify/internal/tracing/otel =>
./internal/tracing/otel` so `-tags=otel` builds from the parent tree
resolve the sub-module locally without an external proxy fetch.

The public surface shape inside the sub-module changed at the same
time: the previous `init()` block that auto-registered an OTLP exporter
if `OTEL_EXPORTER_OTLP_ENDPOINT` was set is replaced by an exported
`Install(ctx context.Context) (shutdown func(), err error)`. A new
build-tag pair in `cmd/clockify-mcp/` — `otel_on.go` (`//go:build otel`)
and `otel_off.go` (`//go:build !otel`) — wraps the install so `run()`
can unconditionally call `installOTel(ctx)` and `defer` the shutdown.
The on-side reads `OTEL_EXPORTER_OTLP_ENDPOINT` itself, logs through
`slog` if the endpoint is unset or the exporter fails to construct,
and always returns a non-nil `func()` so the caller's defer is safe.
The off-side is a no-op stub. This mirrors the `pprof_on.go` /
`pprof_off.go` pair that W2-02 established.

The top-level `go.mod` now carries zero `go.opentelemetry.io` rows. A
new CI gate in the `build` job (`Verify go.mod has zero OpenTelemetry
rows`) runs `grep -c 'go.opentelemetry.io' go.mod` and fails the build
if the count is non-zero. A second new gate builds and vets the
sub-module independently (`cd internal/tracing/otel && go build ./... &&
go vet ./...`) so a developer cannot break the tag-gated path without
CI noticing. A `go.work` file at the repo root lists both modules so
local `go build -tags=otel ./...` from the parent resolves cleanly.

### The `go mod tidy` trap

The sub-module's transitive dependencies (`otel`, `otel/sdk`,
`otlptracehttp`, `semconv`, `grpc`, `protobuf`, ...) are reachable
from the top-level module graph via the sub-module replace. Running
`go mod tidy` on the top-level module will re-add all of them as
`// indirect` rows because Go 1.17+ lazy-loading requires the main
module to list every transitively reachable module. **This is expected
and inescapable at the module-system level.** Developers who run
`go mod tidy` must follow up with `git restore go.mod` to undo the
re-addition; the CI gate catches accidental commits. A future Wave
could add a `make tidy` target that wraps `go mod tidy` with an
auto-restore, but that is not required for ADR 009 to land.

## Consequences

- **Top-level `go.mod` drops from 28 lines to 7.** Zero
  `go.opentelemetry.io` rows, zero `google.golang.org/grpc`, zero
  `go-logr`, zero `protobuf`. The only require is the local
  sub-module itself, pinned via `replace` to `./internal/tracing/otel`.
  `go.sum` retains 18 `go.opentelemetry.io` rows because the
  checksums are needed to verify the sub-module's dependencies at
  build time; that is orthogonal to the module-graph claim the ADR
  is about.
- **Default binary symbol count unchanged.** `go tool nm
  /tmp/clockify-mcp-default | grep -c opentelemetry` still returns 0.
  The existing nm gate in `.github/workflows/ci.yml` continues to
  enforce that invariant; ADR 009 adds a second line of defence at
  the module-graph level rather than replacing the symbol gate.
- **`-tags=otel` binary symbol count unchanged.** `go build -tags=otel
  -o /tmp/clockify-mcp-otel ./cmd/clockify-mcp` still links the OTel
  sub-module and the resulting binary carries approximately 2077
  `opentelemetry` symbols, identical to pre-W2-04.
- **Install API is now explicit.** Operators who enable tracing via
  `-tags=otel` must still set `OTEL_EXPORTER_OTLP_ENDPOINT`; the gate
  moved from an `init()` hook to `cmd/clockify-mcp/otel_on.go` where
  it is easier to audit and where failures log through `slog` rather
  than silently no-op'ing the tracer. Misconfigured exporters now
  warn instead of swallowing the error.
- **`go mod tidy` on the main module will re-add OTel indirect rows.**
  The ADR 009 CI gate catches this immediately on the PR. Developers
  who need to run `go get` + `go mod tidy` for an unrelated dependency
  change must `git restore go.mod` after tidy finishes and manually
  apply any non-OTel updates by hand, or use the `git checkout -p`
  workflow to keep only the intended diff.
- **`go.work` is now a committed artefact.** Prior to ADR 009 the repo
  had no workspace file; the sub-module would not resolve from the
  parent otherwise. The workspace lists two `use` entries: `.` (the
  main module) and `./internal/tracing/otel` (the sub-module). Go
  tooling that inspects the workspace will see both modules; commands
  like `go build ./...` from the parent still only build the main
  module because Go does not descend into directories that own a
  separate `go.mod`.

## Status

Landed on `main` in the W2-04 commit of the 2026-04-11 session.
Closes the ADR 001 W1 deferred trade-off. Wave 2 backlog W2-04 moved
to Landed.
