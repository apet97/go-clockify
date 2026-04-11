# Verifying Release Artifacts

Every tagged release of clockify-mcp-go ships with multiple layers of
supply-chain evidence. This document shows how to verify each one so
operators and downstream packagers can confirm a binary is authentic
before installing it.

Release artifacts for tag `vX.Y.Z` live at
`https://github.com/apet97/go-clockify/releases/tag/vX.Y.Z` and include:

- The binary (`clockify-mcp-<platform>[.exe]`)
- A sigstore bundle (`clockify-mcp-<platform>.sigstore.json`) produced by
  keyless cosign signing
- An SPDX SBOM (`clockify-mcp-<platform>.spdx.json`)
- A GitHub build provenance attestation (SLSA-aligned, stored in the
  GitHub attestation service rather than as a release asset)
- `SHA256SUMS.txt` covering every asset above

## Prerequisites

- `cosign` 2.x — `brew install cosign` or see
  https://docs.sigstore.dev/cosign/installation
- `gh` 2.50+ for build-attestation verification
- `syft` (optional) for SBOM inspection

All examples below use `clockify-mcp-linux-x64` as a placeholder. Replace
it with the platform-specific filename you downloaded (for example
`clockify-mcp-darwin-arm64` or `clockify-mcp-windows-x64.exe`). Note that
the release uses npm-style platform suffixes (`linux-x64`, `darwin-x64`,
`windows-x64`).

## 1. Verify the cosign signature bundle

Each binary ships alongside a `.sigstore.json` file produced by keyless
signing. The bundle is self-contained: signature, certificate, and
transparency-log entry are all packed inside a single file.

```sh
cosign verify-blob \
  --bundle clockify-mcp-linux-x64.sigstore.json \
  --certificate-identity-regexp "https://github.com/apet97/go-clockify/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  clockify-mcp-linux-x64
```

A successful verification prints `Verified OK` and exits zero. Any other
output means the binary should not be trusted.

The `--certificate-identity-regexp` flag asserts that the signature was
produced by a run of this repository's `release.yml` workflow. The
`--certificate-oidc-issuer` flag pins the OIDC issuer to GitHub Actions,
preventing cross-provider signature reuse.

## 2. Verify the GitHub build provenance attestation

The release workflow uses `actions/attest-build-provenance` to produce
SLSA-aligned build provenance for every artifact. The attestation is
stored in the GitHub attestation service (not as a release asset) and
can be verified with the GitHub CLI:

```sh
gh attestation verify clockify-mcp-linux-x64 \
  --repo apet97/go-clockify
```

`gh` hashes the local file, queries the GitHub attestation store for a
matching subject digest, and asserts that the attestation was produced
by the expected workflow in the expected repository. A passing result
prints the verified predicates including the source commit SHA,
workflow path, and runner environment.

To pin verification to a specific workflow path (stricter, recommended
for downstream packagers):

```sh
gh attestation verify clockify-mcp-linux-x64 \
  --repo apet97/go-clockify \
  --signer-workflow apet97/go-clockify/.github/workflows/release.yml
```

## 3. Inspect the SBOM

Every release ships an SPDX-format SBOM alongside the binary:

```sh
syft clockify-mcp-linux-x64.spdx.json
```

This lists every Go module embedded in the binary. Because clockify-mcp
is stdlib-only, the SBOM will list just the Go standard library and the
`github.com/apet97/go-clockify` module itself.

You can also view the raw SBOM directly — it is a plain SPDX JSON file:

```sh
jq '.packages[].name' clockify-mcp-linux-x64.spdx.json
```

## 4. Combined verification recipe

For an end-to-end check from a downloaded release:

```sh
# 1. Download the release assets into ./release
gh release download v0.5.0 -R apet97/go-clockify -D ./release
cd ./release

# 2. Verify cosign bundle
cosign verify-blob \
  --bundle clockify-mcp-linux-x64.sigstore.json \
  --certificate-identity-regexp "https://github.com/apet97/go-clockify/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  clockify-mcp-linux-x64

# 3. Verify GitHub build provenance
gh attestation verify clockify-mcp-linux-x64 \
  --repo apet97/go-clockify

# 4. Verify SHA256SUMS
sha256sum --check --ignore-missing SHA256SUMS.txt

# 5. Optional SBOM inspection
syft clockify-mcp-linux-x64.spdx.json
```

If all steps succeed, you have verified:

- **Authenticity** — the binary was signed by a CI run of this repo's
  release workflow (cosign bundle)
- **Provenance** — the binary was built by a specific GitHub Actions
  workflow run on a specific commit (build-provenance attestation)
- **Integrity** — the on-disk bytes match the SHA256 manifest
  (`SHA256SUMS.txt`)
- **Transparency** — a complete dependency inventory is available
  (SPDX SBOM)

## 5. FIPS 140-3 build variant (optional)

Every tagged release also ships a parallel set of binaries built
with Go 1.25's native FIPS 140-3 mode. These artefacts are named
`clockify-mcp-fips-{platform}` (for example
`clockify-mcp-fips-linux-x64`) and land in the GH Release alongside
the default binaries. The matrix is Linux + macOS only; Windows
FIPS is available on request.

FIPS binaries are built with `GOFIPS140=latest` at compile time so
the frozen FIPS 140-3 cryptographic module is embedded in the
binary. At startup they log:

```
INFO fips140_enabled version=latest enforced=false
```

`enforced=false` means approved algorithms are routed through the
FIPS module but non-approved algorithms (e.g. `sha1`) still
compile and run normally. To get hard-fail semantics on
non-approved primitives at runtime, set `GODEBUG=fips140=only` in
the environment.

FIPS binaries carry the same cosign keyless signature, the same
SPDX SBOM, and the same SLSA build-provenance attestation as their
non-FIPS counterparts:

```sh
# Verify signature (identity regexp unchanged — same workflow builds both)
cosign verify-blob \
  --bundle clockify-mcp-fips-linux-x64.sigstore.json \
  --certificate-identity-regexp "https://github.com/apet97/go-clockify/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  clockify-mcp-fips-linux-x64

# Verify build-provenance attestation
gh attestation verify clockify-mcp-fips-linux-x64 --repo apet97/go-clockify

# Inspect the FIPS SBOM
syft clockify-mcp-fips-linux-x64.spdx.json
```

To confirm the binary you downloaded is actually a FIPS binary:

```sh
./clockify-mcp-fips-linux-x64 --version
# Expect stderr to include:
#   INFO fips140_enabled version=latest enforced=false
```

If the `fips140_enabled` line is absent, the binary was not built
with `GOFIPS140=latest` and is not a FIPS-compliant artefact — do
not treat it as one even if the filename matches.

Running the FIPS binary without `GOFIPS140` at build time (or
without `GODEBUG=fips140=on` at runtime against a default build)
will print a fatal error message and exit. See
[`docs/adr/011-fips-build-target.md`](adr/011-fips-build-target.md)
for the full threat model.

## Reproducibility note

Release binaries are built with `-trimpath`, which strips local build
paths from the embedded DWARF / filename tables. Combined with
`-ldflags "-s -w"` (strip symbol and debug info), this means builds
produced from the same Go version, source commit, and `GOOS`/`GOARCH`
should be byte-for-byte reproducible. Build environments are pinned in
`.github/workflows/release.yml`.
