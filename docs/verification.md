# Verifying release artifacts

Every `go-clockify` release ships with cryptographic provenance on
both binaries and the container image. This document is the manual
equivalent of `.github/workflows/release-smoke.yml`, which runs the
same checks on every release and weekly thereafter and opens a
`release-smoke-failure` issue when anything drifts.

You should run these checks if any of the following is true:
- You are deploying `go-clockify` into an environment that requires
  documented supply-chain verification.
- You are auditing a binary you downloaded from a mirror or a
  package proxy.
- The `release-smoke-failure` issue is open and you want to confirm
  whether the failure reproduces from your machine.

## Prerequisites

```sh
# cosign 2.x for keyless verification + container image attestation
brew install cosign

# gh CLI for GitHub attestation verification
brew install gh
gh auth login
```

The three supply-chain checks below assume you are working with the binary
`clockify-mcp-linux-x64` from tag `v1.2.0` (the current Active line per
[`SUPPORT.md`](../SUPPORT.md)). The other binaries (`darwin-arm64`,
`darwin-x64`, `linux-arm64`, `windows-x64.exe`) are signed by the same
`release.yml` run, so verifying one proves the release-time signing
pipeline was healthy. Replace the filename or `TAG` value to verify a
different platform or release.

## 1. SLSA build provenance attestation

The `release.yml` workflow stages all 15 binaries (5 default
platforms + 4 FIPS + 2 Postgres + 2 gRPC + 2 gRPC-Postgres; full
matrix in [`scripts/check-release-assets.sh`](../scripts/check-release-assets.sh))
and runs `actions/attest-build-provenance` against each one. The
attestation lives in the GitHub attestation service and is verified
through `gh`:

```sh
TAG=v1.2.0
OWNER=apet97

# Download the binary
gh release download "$TAG" \
  --repo "${OWNER}/go-clockify" \
  --pattern 'clockify-mcp-linux-x64'

# Verify the attestation. --owner pins the trust root.
gh attestation verify clockify-mcp-linux-x64 --owner "$OWNER"
```

Expected output ends with `Verification succeeded!`. A non-zero exit
code means either the binary has been tampered with, or the
attestation has not yet propagated through GitHub's storage (this is
eventually consistent — retry in 10 minutes for a freshly published
release).

> **Note — historical context for v1.0.x verification.** Before
> 2026-04-22 the repository was user-owned-private, the GitHub
> attestation service was not active for that account tier, and
> `gh attestation verify` returned "Feature not available for
> user-owned private repositories" (HTTP 404). `release.yml`'s
> `actions/attest-build-provenance` step ran with
> `continue-on-error: true`, no attestation was produced, and the
> release-smoke workflow treated this as **skipped**
> (surfaced as a `::notice::`); the two mandatory cosign checks
> below were the gate during that window. After the public flip
> on 2026-04-22 the attestation step became a mandatory gate
> (no `continue-on-error`) and every release since carries an
> attestation. Operators verifying a v1.0.x binary may still hit
> the legacy 404; verifying anything from `v1.1.0` onward should
> succeed. See
> [`docs/adr/0013-private-repo-slsa-posture.md`](adr/0013-private-repo-slsa-posture.md)
> for the rationale and the post-flip upgrade.

## 2. Cosign keyless signature on the binary

Goreleaser's `signs.cosign-keyless` step writes a sigstore bundle
named `<binary>.sigstore.json` for each binary. The bundle pins the
signer to the GitHub Actions OIDC identity for `release.yml` at the
release tag.

```sh
TAG=v1.2.0
OWNER=apet97
REPO="${OWNER}/go-clockify"

gh release download "$TAG" \
  --repo "$REPO" \
  --pattern 'clockify-mcp-linux-x64' \
  --pattern 'clockify-mcp-linux-x64.sigstore.json'

cosign verify-blob \
  --bundle clockify-mcp-linux-x64.sigstore.json \
  --certificate-identity-regexp "^https://github.com/${REPO}/.github/workflows/release.yml@refs/tags/.*$" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  clockify-mcp-linux-x64
```

Expected output: `Verified OK`.

The `--certificate-identity-regexp` is what binds the signature to
*this* repository's `release.yml`. Anyone can produce a valid cosign
keyless signature; only signatures produced by the goreleaser step in
this repo's `release.yml` will satisfy the regex, so a lookalike
binary signed by an attacker fails closed.

### Bundle format (v1.0.0 vs v1.0.1+)

`v1.0.0` was signed with the legacy rekor-bundle format and stores
the signing certificate only in the transparency log, so re-verifying
it offline with `cosign verify-blob --bundle` fails (cosign needs the
embedded cert chain). `v1.0.1` and later are signed with
`--new-bundle-format`, producing a sigstore v1 bundle that embeds the
x509 certificate chain and verifies cleanly without an online Rekor
lookup. The release workflow pins `cosign-release: v2.4.3` so future
runs don't drift with the `cosign-installer` default.

## 3. Cosign signature on the container image

The container image is built and signed by `docker-image.yml`, not
by `release.yml`. The certificate identity differs accordingly.

```sh
TAG=v1.2.0
OWNER=apet97
REPO="${OWNER}/go-clockify"
IMAGE_REF="ghcr.io/${REPO}:${TAG}"

cosign verify "$IMAGE_REF" \
  --certificate-identity-regexp "^https://github.com/${REPO}/.github/workflows/docker-image.yml@refs/tags/.*$" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

Expected output: a JSON block describing each signature, ending with
the cosign exit code `0`.

For a digest-pinned verification (recommended for production), resolve
the digest first:

```sh
docker buildx imagetools inspect "$IMAGE_REF" \
  --format '{{json .Manifest.Digest}}'
# -> "sha256:abc123…"

cosign verify "ghcr.io/${REPO}@sha256:abc123…" \
  --certificate-identity-regexp "^https://github.com/${REPO}/.github/workflows/docker-image.yml@refs/tags/.*$" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

Pinning by digest defends against tag mutation; pinning by tag is
acceptable when the registry is trusted (ghcr.io) and the release
pipeline is documented as never overwriting tags
(see `.github/workflows/release.yml` and `.goreleaser.yaml`).

## 4. Strict doctor release smoke

Release smoke also proves the shipped binary still supports hosted
strict posture checks. This is an offline config check; the synthetic
Postgres DSN is not contacted unless `--check-backends` is passed.

```sh
chmod +x clockify-mcp-linux-x64

export MCP_PROFILE=prod-postgres
export MCP_CONTROL_PLANE_DSN="postgres://user:pass@localhost:5432/clockify?sslmode=disable"
export MCP_OIDC_ISSUER="https://issuer.example.com"
export MCP_OIDC_AUDIENCE="clockify-mcp"
export CLOCKIFY_API_KEY="dummy"

./clockify-mcp-linux-x64 doctor --strict

set +e
CLOCKIFY_POLICY=standard ./clockify-mcp-linux-x64 doctor --strict
code=$?
set -e
test "$code" = "3"
```

## 5. SBOM (informational)

Both binaries and images carry SPDX SBOMs. Inspect the binary SBOM
with:

```sh
gh release download v1.2.0 \
  --repo apet97/go-clockify \
  --pattern 'clockify-mcp-linux-x64.spdx.json'
jq '.creationInfo, .name, (.packages | length)' clockify-mcp-linux-x64.spdx.json
```

The image SBOM is attached to the image as a cosign attestation:

```sh
cosign download attestation "$IMAGE_REF" \
  --predicate-type https://spdx.dev/Document \
  | jq -r '.payload' | base64 -d | jq '.predicate'
```

These are advisory: a missing or stale SBOM is a quality-of-life
concern, not a security gate. The cryptographic signatures in steps
2 and 3 are what bind the artifacts to this repository.

## What the automated smoke checks

The `.github/workflows/release-smoke.yml` workflow runs steps 1, 2,
and 3 above on every published release and once a week thereafter,
and opens a `release-smoke-failure` GitHub issue if any step fails.
The issue stays open until the next green run closes it
automatically. There is no Slack, no email, no polling dashboard —
the open issue is the exclusive failure signal.

## Related documents

- [`SECURITY.md`](../SECURITY.md) — full list of release artifacts
  and the threat model they protect against.
- [`docs/release-policy.md`](release-policy.md) — versioning, support
  window, and the contract operators rely on.
- [`docs/runbooks/image-digest-pinning.md`](runbooks/image-digest-pinning.md)
  — deploy-time digest pinning policy for the prod overlay.
- [`docs/release/deploy-readiness-checklist.md`](release/deploy-readiness-checklist.md)
  — manual checklist to run before promoting a release.
- [`docs/support-matrix.md`](support-matrix.md)
  — supported Go versions, OS/architectures, and upstream Clockify API compatibility.
