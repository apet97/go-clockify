# Verifying release artifacts

Every tagged release of `clockify-mcp` ships:

- Binaries for linux/amd64, linux/arm64, darwin/amd64,
  darwin/arm64, windows/amd64.
- `SHA256SUMS` with cosign signatures (keyless, OIDC-identified
  from the release workflow).
- An SPDX SBOM per binary (`<name>.spdx.json`).
- SLSA v1.0 provenance attestations (`<name>.intoto.jsonl`).

This document walks through how a downstream consumer verifies
each piece before deploying. Every command is read-only — no
state is mutated on your machine.

## Prerequisites

Install one-time:

```sh
# Keyless signature verification
brew install cosign            # macOS
# or: go install github.com/sigstore/cosign/v2/cmd/cosign@latest

# SLSA provenance verification
brew install slsa-verifier
# or: go install github.com/slsa-framework/slsa-verifier/v2/cli/slsa-verifier@latest

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

## 2. Verify the checksums

```sh
# Keyless cosign verify against the release workflow's OIDC
# identity. If the workflow path or repo changes, the identity
# regex must be updated.
cosign verify-blob \
  --bundle ./artifacts/SHA256SUMS.bundle \
  --certificate-identity-regexp "^https://github.com/apet97/go-clockify/\.github/workflows/release\.yml@refs/tags/" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ./artifacts/SHA256SUMS

# Then check the binaries against the signed checksum list.
cd artifacts && sha256sum -c SHA256SUMS --ignore-missing
```

A success looks like:

```
Verified OK
./clockify-mcp_1.2.0_linux_amd64.tar.gz: OK
./clockify-mcp_1.2.0_linux_arm64.tar.gz: OK
```

If `cosign verify-blob` fails, the binary may have been tampered
with post-release. Do not deploy.

## 3. Verify SLSA provenance

SLSA provenance proves the binary came out of the expected CI
workflow — a protection against an attacker who has compromised a
maintainer's local machine but not the CI runner.

```sh
slsa-verifier verify-artifact \
  --provenance-path ./artifacts/clockify-mcp_1.2.0_linux_amd64.intoto.jsonl \
  --source-uri github.com/apet97/go-clockify \
  --source-tag "$TAG" \
  ./artifacts/clockify-mcp_1.2.0_linux_amd64.tar.gz
```

The output ends with:

```
PASSED: SLSA verification passed
```

## 4. Inspect and scan the SBOM

The SBOM is the declarative list of every module compiled into
the binary. Use it to check for known vulnerabilities before
deploying.

```sh
# List the top-level dependencies
syft packages ./artifacts/clockify-mcp_1.2.0_linux_amd64.spdx.json \
  -o table | head -30

# Scan for CVEs using the Grype vulnerability database
grype sbom:./artifacts/clockify-mcp_1.2.0_linux_amd64.spdx.json \
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
      cosign verify-blob --bundle ... SHA256SUMS
      slsa-verifier verify-artifact ...
      grype sbom:... --fail-on high
```

If any step non-zeroes, the rollout does not proceed.

## When verification fails

- **cosign verify fails** — the checksum file was re-signed by
  a non-release workflow, or the release was re-cut without
  re-signing. Open an issue; do not deploy.
- **slsa-verifier fails** — the binary in hand did not come out
  of the tagged source. Open an issue; do not deploy.
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
