# Runbook — Release Candidate Evidence

Use this runbook immediately after Group 1 scheduled-cron evidence
closes and before reporting a launch candidate for official-product
review.
It prepares the Group 6 and Group 7 evidence bundle without changing
security defaults or widening the live-test blast radius.

This runbook does **not** close Group 1, cut the candidate tag, or
declare launch readiness by itself. It starts only after
`docs/launch-candidate-checklist.md` records the required scheduled
`live-contract.yml` cron evidence on the candidate SHA.

## Evidence Targets

Group 6 needs candidate-tag security evidence:

- `make check`
- `make verify-vuln`, which installs and runs the pinned
  `tools/govulncheck` module under the repo Go pin
- `make secret-scan` with `gitleaks` installed
- Semgrep: `semgrep scan --config p/default --metrics=off --error
  --exclude .git --exclude .bench --exclude clockify-mcp .`
- `make verify-fips` on a FIPS-capable Go host
- `// nosemgrep` context reviewed against the relevant ADR/runbook

Group 7 needs candidate-tag release evidence:

- `make release-check` from a clean checkout on Linux x64 and macOS
  arm64
- `make verify-bench` and `make bench-baseline-check`
- `gh release view vX.Y.Z-rc.N --json assets` archived and validated
  against the 46-asset contract in `scripts/check-release-assets.sh`
- Required workflows on `main` green, including the latest scheduled
  `live-contract.yml`
- `scripts/check-launch-external-status.sh --candidate-sha <sha>
  --expected-npm-version vX.Y.Z-rc.N --fail-open` green after the
  candidate release path publishes, so scheduled workflow, repository
  state, branch-protection, stale issue/PR, and npm expected-version
  proof are checked from one read-only snapshot
- `release-smoke.yml` green for `vX.Y.Z-rc.N`
- Signed binaries, SBOMs, Docker image signature, and SLSA evidence.
  `release-smoke.yml` samples the default and Postgres linux-x64
  artifacts; use `docs/verification.md` with the candidate tag and
  target filename for required variants it does not sample, including
  FIPS.
- Reference `clockify-mcp doctor --strict` and
  `clockify-mcp-postgres doctor --strict --check-backends` output
  archived with the release notes. The `release-smoke-doctor-output`
  artifact from `release-smoke.yml` must contain
  `release-doctor-strict-ok.txt`, `release-doctor-strict-fail.txt`,
  and `release-doctor-postgres-ok.txt`.

## Dry Run The Plan

Before cutting the candidate tag, print the exact command list:

```sh
make rc-evidence-plan TAG=vX.Y.Z-rc.N
```

This is safe from any branch. It does not call Clockify, create a
tag, dispatch workflows, or write evidence logs.

## After Group 1 Closes

1. Confirm `docs/launch-candidate-checklist.md` links the two
   scheduled cron runs for Group 1 and `make launch-checklist-parity`
   still passes.

2. Cut the candidate tag using the normal release procedure. Do not
   reuse a failed candidate tag.

3. Check out the exact candidate tag on each required host:

   ```sh
   git fetch --tags origin
   git switch --detach vX.Y.Z-rc.N
   git status -sb
   ```

4. Run the evidence automation:

   ```sh
   scripts/prepare-rc-evidence.sh vX.Y.Z-rc.N
   ```

   By default it writes under:

   ```text
   ${TMPDIR:-/tmp}/go-clockify-rc-evidence/vX.Y.Z-rc.N/
   ```

   The script refuses to run if the checkout is dirty or if `HEAD`
   is not the tag. For rehearsals only, set
   `RC_EVIDENCE_ALLOW_NON_TAG=1`; do not use that override for real
   launch evidence.

5. Repeat step 4 on both required local gate hosts:

   - Linux x64
   - macOS arm64

   Group 7's `release-check` box is not complete until both host
   logs exist.

6. If the release event did not already run `release-smoke.yml`,
   dispatch it explicitly:

   ```sh
   scripts/prepare-rc-evidence.sh --trigger-release-smoke vX.Y.Z-rc.N
   ```

   Then watch the workflow through GitHub Actions or:

   ```sh
   gh run list --workflow release-smoke.yml --limit 5
   gh run view <run-id> --log-failed
   ```

   Archive or link the workflow's `release-smoke-doctor-output`
   artifact with the launch-candidate tracking issue. A green
   `release-smoke.yml` run is expected to validate and upload all
   three doctor output files named in the Group 7 evidence list above.

7. Run the manual variant checks from `docs/verification.md` for any
   required release artifact not sampled by `release-smoke.yml`.
   At launch, that includes the FIPS variant evidence named in
   Group 7.

8. Attach or link the evidence directory, workflow URLs, and
   `workflow_run_id:` values from the launch checklist before
   checking any Group 6 or Group 7 boxes.

9. After the candidate release path publishes the wrapper package,
   verify the external gates with the expected npm version. This is
   read-only, but unlike the Makefile convenience target it fails when
   any external gate remains open or unknown:

   ```sh
   scripts/check-launch-external-status.sh \
     --candidate-sha "$(git rev-parse HEAD)" \
     --expected-npm-version vX.Y.Z-rc.N \
     --fail-open
   ```

   For a non-failing handoff snapshot, use:

   ```sh
   LAUNCH_EXPECTED_NPM_VERSION=vX.Y.Z-rc.N make launch-external-status
   ```

10. Run the evidence gate after editing the checklist:

   ```sh
   make launch-checklist-parity
   ```

## What The Script Captures

`scripts/prepare-rc-evidence.sh` writes one log per command under
`logs/<host>/`:

- Git status and exact SHA
- Group 6 local security commands
- Group 7 local release commands
- GitHub Release metadata and asset names for `vX.Y.Z-rc.N`, with the
  asset names checked by `scripts/check-release-assets.sh`
- Raw latest-run GitHub metadata snapshots for:
  - `ci.yml`
  - `build-matrix.yml`
  - `codeql.yml`
  - `dependency-review.yml`
  - `semgrep.yml`
  - `live-contract.yml`
  - `release.yml`
  - `release-smoke.yml`
  - `docker-image.yml`
  - `link-check.yml`
  - `chaos.yml`
  - `mutation.yml`
  - `reproducibility.yml`
  - `bench.yml`
- A read-only `launch-external-status` log with
  `LAUNCH_EXPECTED_NPM_VERSION=<tag>` and the candidate SHA, so the
  handoff records scheduled workflow, repository-state, branch-protection,
  stale issue/PR, and npm expected-version status in one place
- Optional `release-smoke.yml` manual-dispatch output

It also fails closed when:

- The tag does not look like `vX.Y.Z-rc.N`
- The tag does not exist locally
- `HEAD` is not the candidate tag
- The worktree is dirty
- `govulncheck`, `gitleaks`, `semgrep`, `gh`, or `npm` is missing
- `make verify-fips` reports a non-FIPS-capable local toolchain

## Checklist Update Pattern

Every Group 6 or Group 7 box that changes from `[ ]` to `[x]` must
carry nearby evidence accepted by `scripts/check-launch-evidence-gate.sh`:

- A GitHub Actions run URL
- `workflow_run_id: <id>`
- `_Closed YYYY-MM-DD by <commit-or-run>`

Local logs alone are not enough for workflow-backed boxes. Do not
check a Group 7 release-artifact box until `release-smoke.yml` is
green for the candidate tag and the release assets, including
required non-sampled variants, have verified.

Workflow metadata snapshots are audit context, not proof by
themselves. Use `scripts/check-launch-external-status.sh --fail-open`
with the final candidate SHA and expected npm version as the
fail-closed validator before treating workflow-backed boxes as closed.

## Related Files

- `scripts/prepare-rc-evidence.sh`
- `scripts/check-launch-evidence-gate.sh`
- `docs/launch-candidate-checklist.md`
- `docs/release-policy.md`
- `docs/verification.md`
- `.github/workflows/release-smoke.yml`

## Failed-evidence record — v1.2.1-rc.1 (2026-05-09)

`v1.2.1-rc.1` was cut on `a5d5f75769dc834a268f6ab24949b139ac4cff85` and is **preserved as failed-evidence only** — it is **not** a releasable RC. Recorded here so future RC work (rc.2+) does not silently re-hit the same failures and so the rc.2 fix bundle has a referenceable baseline.

| Workflow | Run ID | Result | Failure point |
|---|---|---|---|
| Docker Image | [25612037638](https://github.com/apet97/go-clockify/actions/runs/25612037638) | ✅ success | `Build, scan, sign` job; `cosign sign` pushed signature at 21:19:21Z (`tlog entry created with index: 1487221873`) |
| Release (goreleaser) | [25612037642](https://github.com/apet97/go-clockify/actions/runs/25612037642) | ❌ failure | Step `Publish npm packages` exit 1 — `npm error You must specify a tag using --tag when publishing a prerelease version.` First package staged was `@apet97/clockify-mcp-go-darwin-arm64@1.2.1-rc.1`. |
| Deploy | [25612037645](https://github.com/apet97/go-clockify/actions/runs/25612037645) | ❌ failure | Step `Verify image signature` exit 10 — `Error: no signatures found`. Verify ran at 21:19:16Z, signature not pushed until 21:19:21Z (5-second timing race against Docker Image's `cosign sign`). |
| `release-smoke.yml` (release event) | _not fired_ | — | Did not auto-trigger. The Release workflow uses `GITHUB_TOKEN` to publish the GitHub Release object, and GitHub suppresses downstream workflow triggers from `GITHUB_TOKEN`-driven actions, so `release: [published]` never reached release-smoke. |

Asset state on the rc.1 GitHub Release object remained intact:

- 47 release assets uploaded between 21:19:47Z and 21:19:56Z (binaries, `.sigstore.json` cosign bundles, `.spdx.json` SBOMs, `SHA256SUMS.txt`).
- GitHub Release object: `name=v1.2.1-rc.1`, `isPrerelease=true`, `isDraft=false`, `publishedAt=2026-05-09T21:19:56Z`.
- No partial npm publish — `Publish npm packages` aborted on the first staged package, before any `npm publish` request reached the registry. Verifiable: `npm view @apet97/clockify-mcp-go-darwin-arm64@1.2.1-rc.1` returns 404; `@apet97/clockify-mcp-go` `dist-tags.latest` remains `1.2.0`.

Why these are **not** transient and require an rc.2:

1. **npm `--tag` script bug.** `scripts/publish-npm.sh` did not pass `--tag` for prereleases. Re-running the same Release workflow on the same `v1.2.1-rc.1` tag would hit the identical failure because the workflow runs against the tag's tree and the script bug is on that tree. **Fix-and-retag (rc.2) is required.**
2. **Deploy timing race.** The image **is** correctly signed (Docker Image's `cosign sign` step succeeded with tlog index 1487221873). Deploy's `Verify image signature` simply ran 5 seconds early. Rerunning the failed Deploy run on rc.1 will pass — but rc.1's npm gate stays unproven regardless. The deploy.yml fix wraps `cosign verify` in the same 24×30s retry budget as the digest-lookup step so the race cannot recur on rc.2+.
3. **release-smoke not firing.** Architectural — `GITHUB_TOKEN`-suppressed event chain. The release.yml fix adds an explicit `gh workflow run release-smoke.yml --ref main -f tag=$RELEASE_VERSION` step after the existing `Trigger reproducibility verification` step (mirrors the same pattern reproducibility.yml uses for the same reason).

rc.2 cuts only after the fix PR for these three issues lands on `main`. This runbook is updated again with the rc.2 evidence pointers when rc.2's Release / Docker Image / Deploy / release-smoke / npm-publish all complete green and the Group 7 evidence is recordable.

## Group 6 success record — v1.2.1-rc.3 (2026-05-10)

`v1.2.1-rc.3` was cut on the rc.3 readiness fix bundle (annotated tag SHA `8f245174ca3567104a05a65a66250f0a10e5d486`, peeled commit `ce56414ae012c4a49d21ae0a319b178619c5966a`). All tag-triggered and dispatched workflows are green on rc.3: Release run [25616879096](https://github.com/apet97/go-clockify/actions/runs/25616879096), Docker Image run [25616879055](https://github.com/apet97/go-clockify/actions/runs/25616879055), Deploy run [25616879075](https://github.com/apet97/go-clockify/actions/runs/25616879075), Reproducibility run [25616925376](https://github.com/apet97/go-clockify/actions/runs/25616925376) (all 9 matrix jobs match released bytes), and release-smoke run [25616925600](https://github.com/apet97/go-clockify/actions/runs/25616925600). The GitHub Release object carries 47 assets, `isPrerelease=true`, `isDraft=false`, `publishedAt=2026-05-10T01:47:39Z`.

The Group 6 candidate-tag security walk-through ran on a clean fresh worktree off the rc.3 peeled commit (host short name `192`, macOS arm64, no `.local/`, `.serena/`, or duplicate `go-clockify/` checkouts):

| Command | Exit | Result |
|---|---|---|
| `make check` | 0 | `go vet ./...` plus `go test -race -count=1 -timeout 120s ./...` green across every package and test binary. |
| `make verify-vuln` | 0 | `govulncheck@v1.3.0` (built from `tools/govulncheck`) under `GOTOOLCHAIN=go1.25.10` against the `vuln.go.dev` DB snapshot updated `2026-05-07 19:21:40 +0000 UTC` reported `No vulnerabilities found.` |
| `make secret-scan` | 0 | `gitleaks 8.30.1 detect --no-git --source . --redact --config .gitleaks.toml` scanned ~4.97 MB in 667 ms; `no leaks found`. |
| `semgrep scan --config p/default --metrics=off --error --exclude .git --exclude .bench --exclude clockify-mcp .` | 0 | `semgrep 1.157.0` ran 558 rules across 1155 git-tracked files; `Findings: 0 (0 blocking)`. |
| `git grep -n -C 5 nosemgrep -- ':!CHANGELOG.md'` | enumeration | Five in-source directives, each justified inline within five lines: `tests/harness/grpc.go:71` (ADR 0008 — bufconn-only in-memory test transport) and `internal/mcp/transport_streamable_http.go:541,563,565,568` (ADR 0017 — server-controlled SSE `text/event-stream` framing). |
| `make verify-fips` | 0 | `-tags=fips` (GOFIPS140=latest) emitted `INFO fips140_enabled` before every package test passed; `-tags=fips,grpc` build combination green. |

This evidence closed the four candidate-tag-dependent boxes in the Group 6 row of [`docs/launch-candidate-checklist.md`](../launch-candidate-checklist.md) (`make verify-vuln`, gitleaks, Semgrep, `make verify-fips`) for `v1.2.1-rc.3`. The full transcript is preserved in [`../../SECURITY.md`](../../SECURITY.md) § "Candidate-tag security evidence". This entry records Group 6 only; Group 7 (release/sigstore/SLSA per-host `release-check`, doctor-output artifact archive, and `check-launch-external-status --expected-npm-version v1.2.1-rc.3 --fail-open`) remains the separate Group 7 lane and is not closed by this record. The mutation cron evidence, the next-release npm expected-version proof, the paid-hosted external security review, the DPA / privacy / trademark gates, and issue #78 (19-context branch-protection restoration) are also unaffected by this record and remain open.
## v1.2.1-rc.3 evidence record (2026-05-10)

`v1.2.1-rc.3` was cut from peeled commit `ce56414ae012c4a49d21ae0a319b178619c5966a` (annotated tag SHA `8f245174ca3567104a05a65a66250f0a10e5d486`). Unlike rc.1 (preserved as failed-evidence-only because of the npm prerelease `--tag` bug, the Deploy cosign timing race, and release-smoke not auto-firing) and rc.2 (preserved as failed-Group-7-evidence-only because of the Deploy `gh release download` race and the Reproducibility `vcs.modified` mismatch), every tag-triggered and dispatched workflow on rc.3 completed `success` in a single attempt:

| Workflow | Run ID | Result |
|---|---|---|
| Release (goreleaser) | [25616879096](https://github.com/apet97/go-clockify/actions/runs/25616879096) | ✅ success |
| Docker Image | [25616879055](https://github.com/apet97/go-clockify/actions/runs/25616879055) | ✅ success |
| Deploy | [25616879075](https://github.com/apet97/go-clockify/actions/runs/25616879075) | ✅ success |
| Reproducibility (workflow_dispatch from release.yml) | [25616925376](https://github.com/apet97/go-clockify/actions/runs/25616925376) | ✅ success (9-job matrix; all jobs match released bytes) |
| release-smoke (workflow_dispatch from release.yml) | [25616925600](https://github.com/apet97/go-clockify/actions/runs/25616925600) | ✅ success |

GitHub Release: <https://github.com/apet97/go-clockify/releases/tag/v1.2.1-rc.3> (`name=v1.2.1-rc.3`, `tagName=v1.2.1-rc.3`, `isPrerelease=true`, `isDraft=false`, `createdAt=2026-05-10T01:45:25Z`, `publishedAt=2026-05-10T01:47:39Z`, 47 release assets — 15 binaries + 15 SPDX SBOMs + 15 sigstore bundles + `SHA256SUMS.txt` + 1 goreleaser source-tree SBOM named `apet97-go-clockify_sha256_<source-archive-sha256>.spdx.json` (the `<source-archive-sha256>` for rc.3 is `374fbfb4bc18fd14a2fcd39fcae6c8da4054df3c162596ad476c15947b8a351f`, which also matches the `ghcr.io/apet97/go-clockify:1.2.1-rc.3` image manifest digest)). The 46-asset binary contract enforced by `scripts/check-release-assets.sh` covers everything except the source-tree SBOM, which is filtered out by the published-asset-name regex.

Local evidence run from the rc.3 worktree (`scripts/prepare-rc-evidence.sh v1.2.1-rc.3` from `opus/group7-release-rc3-20260510` HEAD `ce56414`):

- Evidence directory: `${TMPDIR:-/tmp}/go-clockify-rc-evidence/v1.2.1-rc.3/logs/darwin-arm64/`.
- Group 6 commands captured: `make check`, `make verify-vuln` (`govulncheck@v1.3.0` on Go 1.25.10 — `No vulnerabilities found.`), `make secret-scan` (gitleaks — `no leaks found`), Semgrep `p/default` scan, `git grep nosemgrep`, `make verify-fips` (`-tags=fips` and `-tags=fips,grpc` both green on a FIPS-capable Go 1.25.10 toolchain). These logs are Group 6 inputs and remain authoritative only for the Group 6 lane.
- Group 7 `make release-check` log captured for the macOS arm64 host; the script-tests subroutine hit a transient TMPDIR concurrency failure on first invocation (parallel `mktemp` under `${TMPDIR}/test-public-content-audit-*`) and reproduced clean on re-run (`make script-tests` exits 0 with all per-test counts at `0 failed`). Re-running on a host with no concurrent TMPDIR consumers is the documented workaround; no script change is required.

Group 7 release-evidence verifications run with the documented commands from [`docs/verification.md`](../verification.md) against the released asset bundle (default `clockify-mcp-linux-x64`, `clockify-mcp-postgres-linux-x64`, `clockify-mcp-fips-linux-x64`, `clockify-mcp-darwin-arm64`):

- `cosign verify-blob --bundle <binary>.sigstore.json --certificate-identity-regexp '^https://github.com/apet97/go-clockify/.github/workflows/release.yml@refs/tags/.*$' --certificate-oidc-issuer https://token.actions.githubusercontent.com <binary>` reported `Verified OK` for all four binaries.
- `gh attestation verify <binary> --owner apet97` reported `✓ Verification succeeded!` for all four binaries; the single SLSA in-toto statement covers all 15 binaries with their SHA256 digests; the predicate's `sourceRepositoryDigest` is `ce56414ae012c4a49d21ae0a319b178619c5966a` and the builder ID is `https://github.com/apet97/go-clockify/.github/workflows/release.yml@refs/tags/v1.2.1-rc.3`.
- `cosign verify ghcr.io/apet97/go-clockify:1.2.1-rc.3 --certificate-identity-regexp '^https://github.com/apet97/go-clockify/.github/workflows/docker-image.yml@refs/tags/.*$' --certificate-oidc-issuer https://token.actions.githubusercontent.com` passed against image manifest digest `sha256:374fbfb4bc18fd14a2fcd39fcae6c8da4054df3c162596ad476c15947b8a351f`. The published container image tag is bare semver (`ghcr.io/apet97/go-clockify:1.2.1-rc.3`), not v-prefixed; the `docker-image.yml` `metadata-action` config uses `type=semver,pattern={{version}}`.
- `shasum -a 256 -c SHA256SUMS.txt --ignore-missing` confirmed every downloaded binary and SBOM matched the release-staged hashes.
- `release-smoke-doctor-output` artifact downloaded from run 25616925600: `release-doctor-strict-ok.txt` exits 0 with `Strict posture: OK no fatal findings; 1 warning(s)` (default linux-x64 binary, prod-postgres profile); `release-doctor-strict-fail.txt` exits 3 with the documented `Strict posture: ERROR ... CLOCKIFY_POLICY` finding when `CLOCKIFY_POLICY=standard` is set; `release-doctor-postgres-ok.txt` exits 0 with `Strict posture: OK` from the postgres-tagged linux-x64 binary running `doctor --strict --check-backends` against a `postgres:16-alpine` service container.
- Local re-verification of `clockify-mcp-darwin-arm64` from the released asset bundle reproduced both expected doctor strict exits (0 with the documented `prod-postgres` env shape; 3 with `CLOCKIFY_POLICY=standard`).

External-status snapshot (read-only) on the candidate SHA:

```sh
scripts/check-launch-external-status.sh \
  --candidate-sha ce56414ae012c4a49d21ae0a319b178619c5966a \
  --expected-npm-version v1.2.1-rc.3 \
  --fail-open
```

Reports `4 open, 0 unknown` against `ce56414`:

- `local branch cleanup`: 14 non-main local branches require maintainer disposition (worktree-active branches plus the historical `docs/document-f3897b2-bypass` and `fwbranch` holds; outside Group 7 scope).
- `Group 1`: latest scheduled live-contract evidence is bound to the Group 1 SHA `feef83c641ced93d2ab6ba07ef766d61c82cc703` rather than the rc.3 SHA `ce56414`. Group 1 closure stays archived on `feef83c` per the May 9 evidence record; the verifier's per-SHA Group 1 check is a separate workstream.
- `mutation.yml`: latest scheduled run 25592823559 still `completed/cancelled` on pushed commit `4fe957547f9e6aea749a85f87823d17a0ccc2928` because that scheduled cron fired before the `internal/tools` matrix-leg timeout fix (`2e7b6bd`) landed. The Group 7 "All required workflows on `main` green" box stays open until the next scheduled `mutation.yml` cron records green on the final candidate SHA.
- `npm publish path` (rc.3 audit at the time of the Group 7 record): the verifier originally matched `dist-tags.latest` (designed for stable releases) and ignored `dist-tags.rc`, so it reported `[open]` against rc.3 even though the publish path had correctly used `--tag rc`. `npm view @apet97/clockify-mcp-go --json` shows `dist-tags = {"latest":"1.2.0","rc":"1.2.1-rc.3"}` — the expected prerelease shape (`rc` dist-tag bumped, `latest` unchanged because the publish path correctly used `--tag rc` for the prerelease per the rc.2 fix in PR #80). Lane D closed this validator quirk on 2026-05-10: `scripts/check-launch-external-status.sh` now derives the dist-tag from the expected prerelease identifier (rc / beta / alpha / next, mirroring `scripts/publish-npm.sh`) and validates `dist-tags.<derived>`. Re-running the rc.3 invocation above now closes the npm proof against `dist-tags.rc` directly, dropping the npm row from this snapshot's open list and leaving the remaining opens unchanged (local branch cleanup, Group 1 SHA mismatch, mutation cron).

Group 7 boxes that closed on this evidence: the **Release artefacts** box (sigstore + SLSA + SBOMs + container image) and the **`clockify-mcp doctor --strict`** box (release-smoke-doctor-output artifact + local re-verification). Group 7 boxes that remain open: **`make release-check` from clean checkouts on Linux x64 + macOS arm64** (this lane is single-host darwin-arm64 only and the macOS arm64 release-check itself transient-failed on the first invocation), and **All required workflows on `main` green** (mutation.yml's next scheduled cron green on the final candidate SHA is still pending).
