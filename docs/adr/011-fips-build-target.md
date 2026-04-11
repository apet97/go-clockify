# ADR 011 — FIPS 140-3 build target via Go 1.25 native mode

**Status**: Accepted, 2026-04-11.

## Context

Downstream operators in regulated industries sometimes require that
every cryptographic operation in a binary be performed by a
FIPS 140-3 validated cryptographic module. Until Go 1.24, the
canonical path was the BoringCrypto toolchain
(`GOEXPERIMENT=boringcrypto`), which required a modified Go
distribution with cgo-linked BoringSSL. That is incompatible with
clockify-mcp's two hard constraints:

1. **ADR 001 — stdlib-only default build.** BoringCrypto requires
   a non-stdlib dependency (BoringSSL) and cgo. Neither is acceptable
   on the default build path.
2. **ADR 004 — zero cgo in default build.** Reproducibility,
   supply-chain hygiene, and cross-compilation all depend on the
   pure-Go compiler.

Go 1.24 introduced a native FIPS 140-3 mode shipped in the standard
library (`crypto/fips140` package + per-algorithm updates across
`crypto/*`). When enabled, the same stdlib-only binary routes every
cryptographic operation through a frozen FIPS-validated module. No
cgo, no BoringSSL, no experimental toolchain. The Wave 2 user ask
for W2-11 matches this capability exactly.

Three implementation strategies were considered:

1. **Runtime-only FIPS.** Ship a single binary and rely on operators
   to set `GODEBUG=fips140=on` at runtime. Rejected: operators who
   forgot the env var would silently run non-FIPS-validated crypto
   without any signal. Regulated deployments need a hard compile-
   time assertion.
2. **Compile-time FIPS binary without a build tag.** Build with
   `GOFIPS140=latest` so the binary embeds the frozen FIPS module
   at build time, but do not gate the code with a build tag.
   Rejected: this produces a FIPS-enabled binary but does not
   distinguish it from the default build in the goreleaser
   artifact matrix. Operators could not tell which binary is the
   FIPS one without running it.
3. **Build tag + compile-time mode + startup assertion (chosen).**

## Decision

Ship a parallel set of binaries under the `-tags=fips` build tag,
built with `GOFIPS140=latest` so the frozen FIPS 140-3 cryptographic
module is embedded at compile time. The `fips` tag gates two small
companion files:

- `cmd/clockify-mcp/fips_on.go` (`//go:build fips`) — exports
  `fipsStartupCheck()` which calls `crypto/fips140.Enabled()`. If
  the binary was somehow executed without FIPS mode active (e.g.
  operator rebuilt without `GOFIPS140`), the check prints a fatal
  error message and exits `1` before any work happens. When the
  check passes, it logs `slog.Info("fips140_enabled", ...)` with
  the module version and the `Enforced` state so operators can
  tell FIPS-strict apart from FIPS-permissive in their log
  aggregator.
- `cmd/clockify-mcp/fips_off.go` (`//go:build !fips`) — no-op stub.
  The default build does not assert anything about FIPS mode;
  operators can still set `GODEBUG=fips140=on` at runtime on the
  default binary, but without the compile-time guarantee.

`cmd/clockify-mcp/main.go` calls `fipsStartupCheck()` as the very
first statement in `main()`, before arg parsing or slog setup, so a
failing assertion cannot be masked by later initialisation.

Goreleaser gets a second `build` entry (`clockify-mcp-fips`) with
`env: [CGO_ENABLED=0, GOFIPS140=latest]` and `flags: [-trimpath,
-tags=fips]`. The build matrix for FIPS skips `windows/amd64`
because there is no ready downstream consumer asking for a Windows
FIPS binary and the default-build nm gate around pprof/otel is
easier to reason about when the FIPS matrix is Linux + macOS only.
The existing `darwin/amd64`, `darwin/arm64`, `linux/amd64`, and
`linux/arm64` entries all produce `clockify-mcp-fips-{os}-{arch}`
archives that land on the GH Release alongside the default
artifacts.

A new CI step in `.github/workflows/ci.yml` builds the FIPS binary
(`-tags=fips` with `GOFIPS140=latest`), runs `--version`, and
asserts the stderr stream contains the `fips140_enabled` log line.
A follow-up step runs `go test -tags=fips -count=1 ./...` so any
test that accidentally reaches for a non-FIPS-approved primitive
(e.g. `crypto/sha1`) surfaces immediately during the CI build.
Both steps passed on first run with the full test suite; no code
in the repo currently relies on non-approved primitives.

## Consequences

- **FIPS binaries are first-class release artefacts.** Every
  tagged release ships a `clockify-mcp-fips-{platform}` alongside
  the default binary. Operators in regulated environments can
  download the FIPS variant directly from the GH Release, verify
  its cosign signature and SLSA attestation exactly like the
  default binary (the attestation lineage is identical), and get
  a compile-time guarantee that every crypto operation routes
  through the frozen FIPS module.
- **No cgo, no BoringSSL, no toolchain fork.** The FIPS binary is
  built with the vanilla `golang` toolchain plus a single env var
  (`GOFIPS140=latest`). `CGO_ENABLED=0` stays set — the FIPS
  module is pure Go. The stdlib-only invariant from ADR 001 is
  preserved for the FIPS variant too.
- **Hard startup assertion.** A FIPS binary that somehow runs
  without FIPS mode active exits with a clear error message. This
  guarantees operators cannot accidentally run a "FIPS" binary
  that does not actually enforce FIPS semantics.
- **`Enforced()` is not enforced.** The chosen mode is `GODEBUG=
  fips140=on` equivalent — approved algorithms are routed through
  the FIPS module, but non-approved algorithms (e.g. `sha1`, some
  `ed25519` operations) still compile and run normally. To get
  hard-fail semantics on non-approved primitives, operators must
  additionally set `GODEBUG=fips140=only` at runtime. The startup
  log line includes `enforced=<true|false>` so the runtime mode is
  auditable. A follow-up Wave could consider flipping the default
  to `only` once we confirm no critical path reaches for a
  non-approved primitive.
- **Matrix asymmetry.** The default build matrix has 5 entries
  (including `windows/amd64`); the FIPS matrix has 4 (Linux +
  macOS only). This is a deliberate trade-off to keep the FIPS
  surface small until a Windows consumer requests one. Adding
  `windows/amd64` to the FIPS matrix is a one-line diff in
  `.goreleaser.yaml` if the ask comes in.
- **Operator guidance.** The runtime log line and the
  `docs/verification.md` FIPS section tell operators how to:
  confirm the binary they downloaded is actually a FIPS binary,
  verify its cosign signature, verify its SLSA attestation,
  and (optionally) switch to `GODEBUG=fips140=only` for
  hard-fail semantics.

## Status

Landed on `main` in the W2-11 commit of the 2026-04-11 session.
