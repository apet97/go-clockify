# Verifying release artifacts

Every tagged release of `clockify-mcp` ships 15 binaries across
five tag combinations (per `scripts/check-release-assets.sh`): the
five default platforms (`darwin-arm64`, `darwin-x64`, `linux-arm64`,
`linux-x64`, `windows-x64.exe`) plus FIPS, Postgres, gRPC, and
gRPC-Postgres variants. Each binary ships:

- The raw binary (`clockify-mcp-<platform>`, e.g.
  `clockify-mcp-linux-x64`; no version in the filename, no
  archive wrapper).
- A keyless cosign sigstore bundle (`<binary>.sigstore.json`).
- An SPDX SBOM (`<binary>.spdx.json`).
- A SLSA build-provenance attestation stored in the GitHub
  attestation service (verified via `gh attestation verify`,
  not as an `.intoto.jsonl` artifact alongside the binary).

Plus a single signed checksum file per release: `SHA256SUMS.txt`.

This document walks through how a downstream consumer verifies
each piece before deploying. Every command is read-only — no
state is mutated on your machine.

## Prerequisites

Install one-time:

```sh
# Keyless signature verification
brew install cosign            # macOS
# or: go install github.com/sigstore/cosign/v2/cmd/cosign@latest

# SLSA provenance verification (via the GitHub attestation service)
# — uses the gh CLI; no slsa-verifier binary needed.
brew install gh
gh auth login

# SBOM tooling
brew install syft grype
```

## 1. Fetch the release

```sh
TAG=v1.2.0
gh release download "$TAG" \
  --repo apet97/go-clockify \
  --dir ./artifacts
ls ./artifacts
```

## 2. Verify the per-binary signatures

Goreleaser's `cosign-keyless` step signs each binary individually
(not the `SHA256SUMS.txt` file as a whole — that file is left
unsigned and is just a convenience for `sha256sum -c` cross-checks
once the binaries themselves are verified). Verify the binary
you actually intend to deploy:

```sh
BIN=clockify-mcp-linux-x64

cosign verify-blob \
  --bundle "./artifacts/${BIN}.sigstore.json" \
  --certificate-identity-regexp "^https://github.com/apet97/go-clockify/\.github/workflows/release\.yml@refs/tags/" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  "./artifacts/${BIN}"

# Cross-check that the binary matches the published checksum:
cd artifacts && sha256sum -c SHA256SUMS.txt --ignore-missing
```

A success looks like:

```
Verified OK
clockify-mcp-linux-x64: OK
```

If `cosign verify-blob` fails, the binary may have been tampered
with post-release. Do not deploy.

## 3. Verify SLSA provenance

SLSA provenance proves the binary came out of the expected CI
workflow — a protection against an attacker who has compromised a
maintainer's local machine but not the CI runner. The release
workflow uses `actions/attest-build-provenance`, which stores the
attestation in the GitHub attestation service (not as an
`.intoto.jsonl` file alongside the binary):

```sh
gh attestation verify "./artifacts/${BIN}" --owner apet97
```

The output ends with `Verification succeeded!`. A non-zero exit
means either the binary was tampered with or the attestation has
not yet propagated through the GitHub attestation service
(eventually-consistent — retry in 10 minutes for a freshly
published release).

## 4. Inspect and scan the SBOM

The SBOM is the declarative list of every module compiled into
the binary. Use it to check for known vulnerabilities before
deploying.

```sh
# List the top-level dependencies
syft packages "./artifacts/${BIN}.spdx.json" \
  -o table | head -30

# Scan for CVEs using the Grype vulnerability database
grype "sbom:./artifacts/${BIN}.spdx.json" \
  --fail-on high
```

`--fail-on high` exits non-zero if any High or Critical CVE is
unpatched in the shipped binary. Known-accepted CVEs with a
documented upgrade path belong in your internal exception list,
not the SBOM itself.

## 5. Container images

The published container images at
`ghcr.io/apet97/go-clockify:$TAG` are digest-pinned and signed
by the same workflow.

```sh
DIGEST=$(crane digest ghcr.io/apet97/go-clockify:"$TAG")

cosign verify \
  --certificate-identity-regexp "^https://github.com/apet97/go-clockify/\.github/workflows/release\.yml@refs/tags/" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  "ghcr.io/apet97/go-clockify@${DIGEST}"
```

Deploy by digest, not by tag — tag resolution is mutable; a
digest is content-addressable.

## 6. Put it in your pipeline

A production pipeline should fail closed if any verification
step fails. Minimum in CI / ArgoCD / Flux:

```yaml
# Pseudocode for a release-gate step
steps:
  - name: Verify release
    run: |
      set -euo pipefail
      cosign verify-blob --bundle "${BIN}.sigstore.json" --certificate-identity-regexp ... "${BIN}"
      gh attestation verify "${BIN}" --owner apet97
      grype "sbom:${BIN}.spdx.json" --fail-on high
```

If any step non-zeroes, the rollout does not proceed.

## When verification fails

- **cosign verify fails** — the binary was tampered with or the
  release was re-cut without re-signing. Open an issue; do not
  deploy.
- **gh attestation verify fails** — the binary in hand did not
  come out of the tagged source. Open an issue; do not deploy.
- **grype reports a new High/Critical** — check the release
  page for a security advisory; if none, open one linking the
  CVE, the affected module, and whether the exploit path is
  reachable from `clockify-mcp`'s surface.

## See also

- `.github/workflows/release.yml` — the OIDC-identified
  workflow that signs every release.
- `.github/workflows/reproducibility.yml` — SLSA L3 binary
  digest verification run on every tagged build.
- `docs/release-policy.md` — semver, EOL window, supported
  minors.
- `deploy/k8s/README.md` — how to pin a digest into the Helm
  chart / Kustomize overlay.
- `docs/runbooks/image-digest-pinning.md` — incident runbook
  for when a tag moves unexpectedly.
