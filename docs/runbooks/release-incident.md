# Runbook: Release pipeline incident

Triggered when a tagged release (`v*.*.*`) produces a partially-failed
workflow run, leaving some artifacts published and others not. The
canonical shape of this incident is "binaries + image shipped, but
npm tarballs or SBOM release assets did not".

## Symptoms

- Release workflow (`release.yml`) finishes with a failed job while
  other jobs in the same run show green.
- `gh run list --branch main --limit 5` shows the release run in
  `failure` state but `gh release view v<X.Y.Z> --json assets` lists a
  partial set of assets.
- One or more of:
  - `npm view @anycli/clockify-mcp-go versions` is missing the new
    version while GH Release shows the binaries.
  - `gh release view v<X.Y.Z> --json assets -q '.assets[].name'` is
    missing `clockify-mcp-image-sbom.spdx.json` even though the
    docker-tag workflow pushed the image and completed `cosign attest`.
  - Docker-tag workflow log contains
    `##[error]Resource not accessible by integration`.
- `gh run view <id> --log-failed` shows one of the two canonical errors
  listed under *Root cause investigation*.

## Immediate mitigation

1. Identify exactly which jobs failed. Don't guess:

   ```bash
   gh run list --branch main --event push --limit 5
   gh run view <id> --log-failed | tail -200
   ```

2. If the image was signed and pushed successfully but the SBOM
   release-asset upload failed, **do not delete the tag**. The image is
   trustworthy on its own terms (cosign keyless signature + SLSA
   provenance attestation on the registry). The GH Release is just
   missing one asset.

3. If `npm publish` failed but the Go binaries were uploaded to the GH
   Release, operators pulling binaries directly are unaffected. Only
   npm consumers are stuck one version behind.

4. Announce in the ops/release channel with the exact failure and
   planned fix. Tag the release version and the failing job name.

## Root cause investigation

Two canonical failures account for almost every partial release:

### `ENEEDAUTH` from `npm publish`

```
npm error code ENEEDAUTH
npm error need auth This command requires you to be logged in to ...
```

- **Cause:** `NODE_AUTH_TOKEN` resolved to empty in the `Publish npm
  packages` job env. That happens when the `NPM_TOKEN` repo secret is
  missing or not readable from the job (e.g. the job's permissions
  block is too restrictive, or the secret lives in a different
  environment than the job targets).
- **Confirm:**
  ```bash
  gh secret list --repo apet97/go-clockify | grep -i NPM_TOKEN
  ```
  If absent, generate a classic automation token at
  <https://www.npmjs.com/settings/~/tokens> with publish rights to
  the `@anycli/clockify-mcp-go*` scope and set:
  ```bash
  gh secret set NPM_TOKEN --repo apet97/go-clockify
  ```

### `Resource not accessible by integration` on SBOM attach

```
##[error]Resource not accessible by integration
Attaching SBOMs to release: 'v<X.Y.Z>'
```

- **Cause:** `anchore/sbom-action` with `upload-artifact: true` on a
  tag build also tries to upload the SBOM as a Release asset. That
  upload hits the Releases API, which requires `contents: write`.
  If the workflow's `permissions:` block only grants `contents: read`,
  the upload fails with this exact message while every other step
  in the same job (which only needs `packages: write` / `id-token:
  write`) keeps working.
- **Confirm:**
  ```bash
  grep -n 'permissions:' .github/workflows/docker-image.yml
  ```
  The top-level `contents:` scope must be `write` (not `read`).

## Recovery: rerun vs re-cut

The decision tree:

- **If no registry-pinned artifacts were produced yet** (e.g. the
  failure was at the very first job, before any `docker push` /
  `cosign sign` / `npm publish`): fix the underlying cause and
  `gh run rerun <id> --failed`. The rerun is free.
- **If binaries + images already shipped under the broken run but
  one downstream job (SBOM attach, npm publish) failed:** `gh run
  rerun <id> --failed` if (and only if) every failing step is
  idempotent on its inputs. `cosign attest`, `npm publish`, and
  `anchore/sbom-action` release-asset upload are all idempotent on
  the same tag + digest, so rerunning the failed job is safe.
- **If a downstream consumer already observed the broken state**
  (e.g. a user installed `@anycli/clockify-mcp-go@<X.Y.Z>` between
  the partial release and the rerun and got a 404 that cached): cut
  a patch bump `X.Y.Z+1` with the fix in the commit history so the
  failure and its remediation are both captured in the changelog.
  Do not force-retag; leave the broken tag in git history.

Example rerun:

```bash
gh run rerun <run-id> --failed
gh run watch <run-id> --exit-status
```

## Post-incident

- Re-run `cosign verify` on the image and `gh attestation verify` on
  each binary to confirm signatures are still valid after any rerun.
  A rerun produces new signatures, not new content; the digests must
  match the pre-rerun release assets.
- Confirm the GH Release asset list matches expected via
  `gh release view v<X.Y.Z> --json assets -q '.assets[].name'`.
- Confirm npm has the version:
  ```bash
  npm view @anycli/clockify-mcp-go versions --json | tail -5
  ```
- Update `docs/wave2-backlog.md` if the root cause warrants a standing
  fix (not just a secret set or permission grant) — e.g. switch from
  manual `npm publish` to goreleaser's `brews`/`nfpms` sidecars.

## Related

- `docs/verification.md` — release artifact verification commands.
- `docs/runbooks/metrics-scrape-failure.md` — adjacent incident shape
  (post-deploy rather than release-time).
- `CHANGELOG.md` — always annotate a patch bump that was forced by
  an infra incident with the matching `W2-*` backlog id.
