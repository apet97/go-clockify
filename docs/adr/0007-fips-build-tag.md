# 0007 - FIPS 140-3 build via build tag

## Status

Accepted — landed in commit `f70252f` (W2-11, v0.7.0); the runtime
assertion is in `cmd/clockify-mcp/fips_on.go`.

## Context

Some operators must run binaries that use only FIPS 140-3 validated
cryptographic modules — typically because of FedRAMP, DoD, or
healthcare regulatory requirements. Go 1.25 introduced native FIPS
140-3 support via `crypto/fips140` and the `GOFIPS140` toolchain
mode, which replaces the standard crypto modules with the validated
FIPS module at compile time.

Two failure modes we want to defend against:

1. **Operator thinks they're running FIPS but isn't.** They forgot
   `GOFIPS140=latest` at build time, or `GODEBUG=fips140=on` at
   runtime, or both. The binary runs successfully without
   FIPS-validated crypto and the operator never finds out.
2. **The default build needlessly carries FIPS toolchain artifacts.**
   Default-build operators do not need FIPS and the binary should
   not pretend to be FIPS-compliant.

We need an opt-in path that produces a binary which (a) actually
uses FIPS-validated crypto, and (b) refuses to start if FIPS mode is
not actually active.

## Decision

FIPS 140-3 mode is opt-in via the `fips` build tag, with a
mandatory startup assertion:

- **Build tag isolation.** `cmd/clockify-mcp/fips_on.go` (`//go:build
  fips`) imports `crypto/fips140` and defines `fipsStartupCheck`,
  which calls `fips140.Enabled()` and exits the process with a
  fatal error if the result is false.
  `cmd/clockify-mcp/fips_off.go` (`//go:build !fips`) provides a
  no-op stub.
- **Mandatory startup assertion.** `main()` calls `fipsStartupCheck`
  before any other initialization (`cmd/clockify-mcp/main.go:48-51`).
  In a `-tags=fips` binary, the process exits with a clear stderr
  message if `crypto/fips140.Enabled()` returns false. The hint text
  tells the operator to rebuild with `GOFIPS140=latest` or set
  `GODEBUG=fips140=on`.
- **Goreleaser pairing.** `.goreleaser.yaml` carries a second `build`
  entry with `-tags=fips` and `GOFIPS140=latest`, so every release
  ships an explicit FIPS artifact alongside the default one. The
  artifact filename is suffixed to make it impossible to confuse
  with the default binary.
- **Toolchain limitation acknowledged.** `crypto/fips140.Version()`
  and `crypto/fips140.Enforced()` are Go 1.26+ only. Go 1.25.9 (the
  current pin per `CONTRIBUTING.md`) ships only `Enabled()`. The
  startup assertion uses what's available; operators who want the
  enforced flag can read it from `GODEBUG` or from
  `go version -m <binary>`.

## Consequences

### Positive

- An operator who downloads the FIPS release artifact and starts it
  with the wrong environment gets a fatal error on the first line
  of stderr. They cannot accidentally serve traffic from a binary
  that thinks it's FIPS but isn't.
- The default build is unchanged: no FIPS dependencies, no
  toolchain restrictions, no runtime assertion.
- The CI matrix exercises the FIPS toolchain when it is available
  and soft-fails locally when it is not — `make verify-fips`
  documents the expected behavior on each platform.

### Negative

- The FIPS artifact is platform-dependent in a way the default
  binary is not: only Go toolchain combinations that ship the
  validated module can produce it. CI must skip the FIPS gate on
  hosts without the toolchain, which means a contributor on macOS
  cannot fully exercise the FIPS path locally.
- Operators who misread the docs and set `-tags=fips` on a build
  without `GOFIPS140=latest` get a binary that fails-fast on
  startup. We consider this preferable to silent non-compliance.
- The runtime assertion adds one extra system call to startup. The
  cost is negligible compared to the rest of `main()`.

### Neutral

- The FIPS build tag does not change any other behaviour. The same
  tools, transports, auth modes, and policy modes are available;
  only the underlying crypto implementation differs.
- The startup assertion can be relaxed in the future when
  `crypto/fips140.Enforced()` is available (Go 1.26+) — the
  enforced flag would let us distinguish "FIPS mode active" from
  "FIPS mode active and enforced", which would tighten the check.

## Alternatives considered

- **Document the toolchain requirement and trust operators to
  configure it correctly** — rejected because the failure mode is
  silent: the operator runs a binary they think is FIPS-compliant
  and only finds out when an auditor complains. The startup check
  converts a silent compliance failure into a loud crash.
- **Use BoringCrypto via `GOEXPERIMENT=boringcrypto`** — rejected
  because BoringCrypto is not a Go-supported configuration on all
  the platforms we ship to, and the Go 1.25 native FIPS module is
  the upstream-blessed replacement.
- **Always assert FIPS mode in the default build** — rejected
  because the default build is not FIPS and the assertion would
  always fail.

## References

- Previously referred to as "ADR 011" in
  `cmd/clockify-mcp/main.go:50`, `cmd/clockify-mcp/fips_on.go:22`,
  `cmd/clockify-mcp/fips_off.go:9`, `.goreleaser.yaml:73`.
- Build-tagged file: `cmd/clockify-mcp/fips_on.go`.
- Default-build stub: `cmd/clockify-mcp/fips_off.go`.
- Startup wiring: `cmd/clockify-mcp/main.go:48-51`.
- Goreleaser FIPS build entry: `.goreleaser.yaml` (`fips` builder).
- Related ADRs: 0001 (stdlib-only invariant), 0006 (OTel build tag),
  0008 (gRPC build tag).
- Related docs: `docs/production-readiness.md` "Compliance posture",
  `CONTRIBUTING.md` "Go version pin".
- Spec: <https://csrc.nist.gov/pubs/fips/140-3/final> (FIPS 140-3
  standard).
