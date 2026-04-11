# ADR 010 — Release pipeline migration to goreleaser

**Status**: Accepted, 2026-04-11.

## Context

Between Wave 1 and Wave 2, the release pipeline in
`.github/workflows/release.yml` was a hand-rolled matrix: a
`build-binaries` job that ran five platform-specific build steps (each
generating the binary with ldflags, smoke-testing native targets,
producing an SPDX SBOM via syft, and uploading two artifacts), followed
by a `create-release` job that downloaded the artifacts, signed every
binary with cosign keyless OIDC, generated a SLSA build provenance
attestation via `actions/attest-build-provenance`, and uploaded the
release assets via `softprops/action-gh-release`.

The v0.6.0 cut exposed three independent latent bugs in that workflow
that landed as W2-12:

1. `docker-image.yml` lacked `contents: write` so the image SBOM could
   not attach to the GH Release — fixed by bumping the permission.
2. `publish-npm` referenced an npm scope (`@anycli`) this project does
   not own.
3. `publish-npm` referenced `npm/clockify-mcp-go/` which had never
   existed on disk.

Both 2 and 3 were discovered while investigating 1. The `publish-npm`
job was deleted rather than papered over; W2-13 tracked the full
rebuild. With three latent bugs surfacing in a single cut, the
hand-rolled workflow was due for a structural rewrite. Two
alternatives were considered:

1. **Rebuild `publish-npm` in place, keep the rest of the hand-rolled
   pipeline.** Rejected because it leaves the matrix, smoke-test, and
   signing code intact — each of which is an independent latent-bug
   surface — and because every future target (FIPS, additional
   platforms, alternative archive formats) would need bespoke
   scripting rather than a declarative config change.
2. **Migrate to goreleaser (W2-07).** Adopted.

## Decision

Replace `build-binaries` and `create-release` with a single
goreleaser-driven job in `.github/workflows/release.yml`. Goreleaser
is configured via `.goreleaser.yaml` at the repo root and owns:

- **Binary builds** — the `builds` section covers the same 5-platform
  matrix (darwin/arm64, darwin/amd64, linux/amd64, linux/arm64,
  windows/amd64) with the same `-trimpath -s -w -X main.version
  -X main.commit -X main.buildDate` ldflags. `CGO_ENABLED=0` and the
  `ignore: [goos: windows, goarch: arm64]` stanza reproduce the
  pre-W2-07 exclusion rule.
- **Archive naming** — the `archives.name_template` uses a Go template
  that maps `amd64 → x64`, keeps `arm64` as-is, and produces
  `clockify-mcp-{os}-{arch}(.exe)` filenames that match exactly what
  `docs/verification.md` operator commands reference. The archive
  format is `binary` so there is no tarball wrapper — users download
  raw executables identical to the pre-W2-07 layout.
- **Checksum** — a single `SHA256SUMS.txt` file matching the pre-W2-07
  filename, covering every archive.
- **SBOMs** — the `sboms` section runs `syft` on each archive and
  writes `clockify-mcp-{os}-{arch}.spdx.json`, matching the pre-W2-07
  naming pattern. `anchore/sbom-action/download-syft` is installed in
  the workflow before goreleaser runs.
- **Signatures** — the `signs` section runs `cosign sign-blob --yes
  --bundle=${signature}` against every binary artifact, producing
  `{artifact}.sigstore.json` bundles. Cosign is installed via
  `sigstore/cosign-installer` before goreleaser runs. Keyless OIDC
  signing uses the GH Actions workload identity via the `id-token:
  write` permission on the job.
- **GitHub Release upload** — goreleaser's `release:` section uploads
  every archive, SBOM, sig bundle, and SHA256SUMS.txt to the GH Release
  for the pushed tag.

Two concerns could not be cleanly expressed inside goreleaser and are
handled by small companion steps in the same job after goreleaser
finishes:

### SLSA build provenance attestation

`actions/attest-build-provenance` indexes attestations by content
hash, but the subject **name** stored alongside the hash is whatever
the filename says at attestation time. Goreleaser's internal build
paths (`dist/clockify-mcp_darwin_arm64_v8.0/clockify-mcp`) do not
match the release asset filenames (`clockify-mcp-darwin-arm64`), so
running the attest action directly against `dist/` would record the
wrong subject names. The fix is a small shell step that copies each
built binary into a `staging/` directory under its release asset
filename, then runs `attest-build-provenance` against those copies.
Because the attestation is content-hash-indexed, `gh attestation
verify` against either the release asset or the staging copy produces
the same result.

### npm publishing (subsumes W2-13)

Goreleaser free does not ship an npm publisher. Rather than pay for
goreleaser Pro or revive the broken hand-rolled publish-npm job, a
dedicated `scripts/publish-npm.sh` bash script consumes goreleaser's
`dist/` output and publishes six npm packages:

- `@apet97/clockify-mcp-go-darwin-arm64`
- `@apet97/clockify-mcp-go-darwin-x64`
- `@apet97/clockify-mcp-go-linux-x64`
- `@apet97/clockify-mcp-go-linux-arm64`
- `@apet97/clockify-mcp-go-windows-x64`
- `@apet97/clockify-mcp-go` (the dispatcher — lists the five platform
  packages under `optionalDependencies` and installs a `bin/clockify-mcp.js`
  shim that selects the correct sibling at runtime via
  `require.resolve`)

The workflow step invokes the script with `NODE_AUTH_TOKEN` set from
the `NPM_TOKEN` repo secret. If the secret is not set the step
gracefully no-ops with a warning so operators can still tag a release
before npm plumbing is live — the GH Release itself does not depend
on the npm step succeeding. The script itself runs `npm publish
--access public` per package. Local validation is possible via
`NPM_CONFIG_DRY_RUN=true ./scripts/publish-npm.sh v0.7.0`, which
exercises the full staging pipeline without hitting the registry.

## Deliberately out of scope

- **Container image builds.** `docker-image.yml` was just fixed in
  W2-12 and has a different trigger flow (manifest list push on tag).
  Moving container builds into goreleaser's `dockers:` section would
  require re-plumbing the signature, SBOM, and attestation story for
  the image — a larger change with its own risk surface — and there
  is no benefit that justifies touching a recently-stabilised pipeline.
- **Homebrew tap, scoop, winget, AUR.** None were requested in the
  Wave 2 scope confirmation. They can be added as config deltas in
  `.goreleaser.yaml` without touching the workflow when they become
  useful.
- **release-please adoption.** The release cadence stays at manual
  tag-triggered pushes. The current flow of "bump version in
  `cmd/clockify-mcp/main.go` → update CHANGELOG → commit → tag → push"
  is documented in every ADR release stanza and matches how the
  Wave 1 and v0.6.1 releases were cut.

## Consequences

- **Smaller attack surface in `release.yml`.** The pre-W2-07 workflow
  had ~205 lines of YAML across two jobs; the new workflow is ~80
  lines across one job. Every removed line is a line that cannot
  harbor a future latent bug.
- **`.goreleaser.yaml` becomes the single source of truth** for
  binary filenames, ldflags, signing, SBOMs, and archive format.
  Future Wave work that adds platforms (FIPS, new architectures) or
  changes artifact naming is a config-diff, not a workflow rewrite.
- **Filename compatibility is preserved.** `docs/verification.md`
  operator commands, SHA256SUMS.txt layout, cosign `.sigstore.json`
  bundle names, and SPDX SBOM filenames match the pre-W2-07 outputs
  byte-for-byte. A v0.7.0 verification walkthrough uses the same
  commands as v0.6.1.
- **npm distribution is back.** `go install` and
  `ghcr.io/apet97/go-clockify:X.Y.Z` continue to work exactly as
  before; v0.7.0 adds `npm install -g @apet97/clockify-mcp-go` as a
  new installation path. The six-package layout follows the esbuild
  / turbo pattern: users install the dispatcher, npm auto-installs
  the right platform sibling via `optionalDependencies`, and the
  dispatcher's `bin/clockify-mcp.js` resolves and execs the native
  binary. Windows users get `clockify-mcp.exe`; everyone else gets
  `clockify-mcp`.
- **Local snapshot validation.** `goreleaser release --snapshot
  --skip=publish --skip=sign --skip=sbom --clean` reproduces the
  full build matrix locally in about 20 seconds on an M1 Mac without
  needing cosign or syft installed. CI gets the real signing + SBOM
  treatment on every tag. Developers can also run
  `NPM_CONFIG_DRY_RUN=true ./scripts/publish-npm.sh v0.7.0` against
  the local `dist/` to verify the npm staging produces six clean
  tarballs before pushing a tag.
- **Secret dependency.** `NPM_TOKEN` must be set as a repo secret
  scoped to `@apet97` with npm automation-token rights. The
  workflow step no-ops gracefully if the token is absent, so this
  is a soft dependency: releases still ship without npm if the
  token is missing, but the first v0.7.0 tag is expected to have
  the token wired up ahead of time per the session handoff.
- **SLSA attestation is content-hash-indexed.** The staging copy
  does not weaken the attestation chain; `gh attestation verify` on
  the downloaded release asset succeeds because the digest matches
  what was attested, regardless of the subject name at attest time.

## Status

Landed on `main` in the W2-07 commit of the 2026-04-11 session.
Closes W2-07 and W2-13 together. Wave 2 backlog entries for both
items move to Landed.
